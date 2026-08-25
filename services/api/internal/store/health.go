package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ---- Health pillar ----

// Health owns meals, recipes, grocery list, workouts and body metrics.
type Health struct {
	DB *sql.DB
}

// validRFC3339 accepts RFC3339 timestamps.
func validRFC3339(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

func validJSONArray(s string) bool {
	if s == "" {
		return true
	}
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "[") || !strings.HasSuffix(t, "]") {
		return false
	}
	return json.Valid([]byte(t))
}

// rawJSONArray normalizes an already-validated JSON array string into
// RawMessage so it serializes as a real array, never an escaped string.
func rawJSONArray(s string) json.RawMessage {
	if strings.TrimSpace(s) == "" {
		return json.RawMessage("[]")
	}
	return json.RawMessage(s)
}

// ---- Meals ----

type Meal struct {
	ID        string          `json:"id"`
	EatenAt   string          `json:"eaten_at"` // RFC3339 UTC
	Title     string          `json:"title"`
	Notes     string          `json:"notes"`
	Items     json.RawMessage `json:"items"` // JSON array of {name, qty, unit}
	Calories  *int64          `json:"calories"`
	Tags      []string        `json:"tags"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`

	tagsRaw string
}

const mealCols = `id,eaten_at,title,notes,items,calories,tags,created_at,updated_at`

func mealScan(m *Meal, tagsRaw *string) []interface{} {
	return []interface{}{&m.ID, &m.EatenAt, &m.Title, &m.Notes, &m.Items,
		&m.Calories, tagsRaw, &m.CreatedAt, &m.UpdatedAt}
}

type MealFilter struct {
	From, To string // YYYY-MM-DD on eaten_at date
	Q        string
	Page     int
	PageSize int
}

func (h *Health) CreateMeal(eatenAt, title, notes, itemsJSON string, calories *int64, tags []string) (Meal, error) {
	if !validRFC3339(eatenAt) || strings.TrimSpace(title) == "" {
		return Meal{}, ErrInvalid
	}
	if !validJSONArray(itemsJSON) {
		return Meal{}, ErrInvalid
	}
	if calories != nil && *calories < 0 {
		return Meal{}, ErrInvalid
	}
	now := NowRFC3339()
	m := Meal{
		ID: NewID(), EatenAt: eatenAt, Title: title, Notes: notes,
		Items: rawJSONArray(itemsJSON), Calories: calories,
		Tags: normalizeTagList(tags), CreatedAt: now, UpdatedAt: now,
	}
	var cal interface{}
	if calories != nil {
		cal = *calories
	}
	_, err := h.DB.Exec(
		`INSERT INTO meals (`+mealCols+`) VALUES (?,?,?,?,?,?,?,?,?)`,
		m.ID, m.EatenAt, m.Title, m.Notes, m.Items, cal, joinTags(m.Tags), m.CreatedAt, m.UpdatedAt)
	if err != nil {
		return Meal{}, err
	}
	logChange(h.DB, "meal", m.ID, "create", m.Title)
	return h.GetMeal(m.ID)
}

func defaultIfEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func (h *Health) GetMeal(id string) (Meal, error) {
	var m Meal
	err := h.DB.QueryRow(`SELECT `+mealCols+` FROM meals WHERE id=?`, id).
		Scan(mealScan(&m, &m.tagsRaw)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Meal{}, ErrNotFound
	}
	if err != nil {
		return Meal{}, err
	}
	m.Tags = splitTags(m.tagsRaw)
	return m, nil
}

func (h *Health) buildDayRange(col, from, to string) (string, []interface{}) {
	where := []string{"1=1"}
	args := []interface{}{}
	if from != "" {
		where = append(where, "substr("+col+",1,10)>=?")
		args = append(args, from)
	}
	if to != "" {
		where = append(where, "substr("+col+",1,10)<=?")
		args = append(args, to)
	}
	return strings.Join(where, " AND "), args
}

func (h *Health) ListMeals(f MealFilter) ([]Meal, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 50
	}
	where, args := h.buildDayRange("eaten_at", f.From, f.To)
	extra := []string{}
	if f.Q != "" {
		extra = append(extra, "(LOWER(title) LIKE ? OR LOWER(notes) LIKE ?)")
		pat := "%" + strings.ToLower(f.Q) + "%"
		args = append(args, pat, pat)
	}
	where = strings.Join(append([]string{where}, extra...), " AND ")

	var total int
	if err := h.DB.QueryRow(`SELECT COUNT(*) FROM meals WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT ` + mealCols + ` FROM meals WHERE ` + where +
		` ORDER BY eaten_at DESC LIMIT ? OFFSET ?`
	pageArgs := append(append([]interface{}{}, args...), f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := h.DB.Query(q, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Meal{}
	for rows.Next() {
		var m Meal
		if err := rows.Scan(mealScan(&m, &m.tagsRaw)...); err != nil {
			return nil, 0, err
		}
		m.Tags = splitTags(m.tagsRaw)
		out = append(out, m)
	}
	return out, total, rows.Err()
}

type MealUpdate struct {
	EatenAt  *string
	Title    *string
	Notes    *string
	Items    *string
	Calories **int64 // ptr-to-nil clears; ptr-to-value sets
	Tags     *[]string
}

func (h *Health) UpdateMeal(id string, u MealUpdate) (Meal, error) {
	cur, err := h.GetMeal(id)
	if err != nil {
		return Meal{}, err
	}
	if u.EatenAt != nil {
		if !validRFC3339(*u.EatenAt) {
			return Meal{}, ErrInvalid
		}
		cur.EatenAt = *u.EatenAt
	}
	if u.Title != nil && strings.TrimSpace(*u.Title) != "" {
		cur.Title = *u.Title
	}
	if u.Notes != nil {
		cur.Notes = *u.Notes
	}
	if u.Items != nil {
		if !validJSONArray(*u.Items) {
			return Meal{}, ErrInvalid
		}
		cur.Items = rawJSONArray(*u.Items)
	}
	if u.Calories != nil {
		if *u.Calories == nil {
			cur.Calories = nil
		} else if **u.Calories < 0 {
			return Meal{}, ErrInvalid
		} else {
			v := **u.Calories
			cur.Calories = &v
		}
	}
	if u.Tags != nil {
		cur.Tags = normalizeTagList(*u.Tags)
	}
	cur.UpdatedAt = NowRFC3339()

	var cal interface{}
	if cur.Calories != nil {
		cal = *cur.Calories
	}
	_, err = h.DB.Exec(
		`UPDATE meals SET eaten_at=?, title=?, notes=?, items=?, calories=?, tags=?, updated_at=? WHERE id=?`,
		cur.EatenAt, cur.Title, cur.Notes, cur.Items, cal, joinTags(cur.Tags), cur.UpdatedAt, id)
	if err != nil {
		return Meal{}, err
	}
	logChange(h.DB, "meal", id, "update", cur.Title)
	return h.GetMeal(id)
}

func (h *Health) DeleteMeal(id string) error {
	cur, err := h.GetMeal(id)
	if err != nil {
		return err
	}
	res, err := h.DB.Exec(`DELETE FROM meals WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	logChange(h.DB, "meal", id, "delete", cur.Title)
	return nil
}
