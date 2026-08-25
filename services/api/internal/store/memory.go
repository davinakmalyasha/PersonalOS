package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// ---- Changelog: what changed, for the agent-activity feed ----

type Change struct {
	Entity   string `json:"entity"`
	EntityID string `json:"entity_id"`
	Action   string `json:"action"`
	Title    string `json:"title"`
	At       string `json:"at"`
}

// LogChange records one mutation. Best-effort by design: callers pass a dbtx
// so it joins the surrounding transaction when one exists.
func logChange(db dbtx, entity, entityID, action, title string) {
	switch action {
	case "create", "update", "delete", "complete":
	default:
		action = "update"
	}
	_, _ = db.Exec(
		`INSERT INTO changelog (id,entity,entity_id,action,title,at) VALUES (?,?,?,?,?,?)`,
		NewID(), entity, entityID, action, title, NowRFC3339())
}

// ActivityFeed returns the most recent changes, newest first.
func (a *ActivityStore) Feed(limit int, entity string) ([]Change, error) {
	if limit < 1 || limit > 200 {
		limit = 30
	}
	where := []string{"1=1"}
	args := []interface{}{}
	if entity != "" {
		where = append(where, "entity=?")
		args = append(args, entity)
	}
	rows, err := a.DB.Query(
		`SELECT entity,entity_id,action,title,at FROM changelog WHERE `+strings.Join(where, " AND ")+
			` ORDER BY at DESC LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Change{}
	for rows.Next() {
		var c Change
		if err := rows.Scan(&c.Entity, &c.EntityID, &c.Action, &c.Title, &c.At); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---- Saved searches ----

type SavedSearch struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Query     json.RawMessage `json:"query"`
	CreatedAt string          `json:"created_at"`
}

type SavedSearchStore struct {
	DB *sql.DB
}

func (s *SavedSearchStore) Create(name string, query json.RawMessage) (SavedSearch, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return SavedSearch{}, ErrInvalid
	}
	q := "{}"
	if len(strings.TrimSpace(string(query))) > 0 {
		if !json.Valid(query) {
			return SavedSearch{}, ErrInvalid
		}
		q = string(query)
	}
	id := NewID()
	now := NowRFC3339()
	_, err := s.DB.Exec(
		`INSERT INTO saved_searches (id,name,query,created_at) VALUES (?,?,?,?)`,
		id, name, q, now)
	if isUniqueErr(err) {
		return SavedSearch{}, ErrConflict
	}
	if err != nil {
		return SavedSearch{}, err
	}
	return s.Get(id)
}

func (s *SavedSearchStore) Get(id string) (SavedSearch, error) {
	var ss SavedSearch
	var raw string
	err := s.DB.QueryRow(`SELECT id,name,query,created_at FROM saved_searches WHERE id=?`, id).
		Scan(&ss.ID, &ss.Name, &raw, &ss.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SavedSearch{}, ErrNotFound
	}
	if err != nil {
		return SavedSearch{}, err
	}
	ss.Query = json.RawMessage(raw)
	return ss, nil
}

func (s *SavedSearchStore) List() ([]SavedSearch, error) {
	rows, err := s.DB.Query(`SELECT id,name,query,created_at FROM saved_searches ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SavedSearch{}
	for rows.Next() {
		var ss SavedSearch
		var raw string
		if err := rows.Scan(&ss.ID, &ss.Name, &raw, &ss.CreatedAt); err != nil {
			return nil, err
		}
		ss.Query = json.RawMessage(raw)
		out = append(out, ss)
	}
	return out, rows.Err()
}

func (s *SavedSearchStore) Update(id, name string, query json.RawMessage) (SavedSearch, error) {
	cur, err := s.Get(id)
	if err != nil {
		return SavedSearch{}, err
	}
	if strings.TrimSpace(name) != "" {
		cur.Name = strings.TrimSpace(name)
	}
	if len(strings.TrimSpace(string(query))) > 0 {
		if !json.Valid(query) {
			return SavedSearch{}, ErrInvalid
		}
		cur.Query = query
	}
	_, err = s.DB.Exec(`UPDATE saved_searches SET name=?, query=? WHERE id=?`, cur.Name, string(cur.Query), id)
	if err != nil {
		return SavedSearch{}, err
	}
	return s.Get(id)
}

func (s *SavedSearchStore) Delete(id string) error {
	res, err := s.DB.Exec(`DELETE FROM saved_searches WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
