package store

import (
	"database/sql"
)

type Rule struct {
	ID         string `json:"id"`
	Pattern    string `json:"pattern"`
	CategoryID string `json:"category_id"`
	Priority   int    `json:"priority"`
	CreatedAt  string `json:"created_at"`
}

func (f *Finance) CreateRule(pattern, categoryID string, priority int) (Rule, error) {
	if _, err := f.GetCategory(categoryID); err != nil {
		return Rule{}, err
	}
	r := Rule{ID: NewID(), Pattern: pattern, CategoryID: categoryID, Priority: priority, CreatedAt: NowRFC3339()}
	_, err := f.DB.Exec(`INSERT INTO categorization_rules (id,pattern,category_id,priority,created_at) VALUES (?,?,?,?,?)`,
		r.ID, r.Pattern, r.CategoryID, r.Priority, r.CreatedAt)
	if err != nil {
		return Rule{}, err
	}
	logChange(f.DB, "rule", r.ID, "create", "rule: "+r.Pattern)
	return r, nil
}

func (f *Finance) ListRules() ([]Rule, error) {
	rows, err := f.DB.Query(`SELECT id,pattern,category_id,priority,created_at FROM categorization_rules ORDER BY priority, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.Pattern, &r.CategoryID, &r.Priority, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (f *Finance) UpdateRule(id string, pattern *string, categoryID *string, priority *int) (Rule, error) {
	cur, err := f.getRule(id)
	if err != nil {
		return Rule{}, err
	}
	if pattern != nil {
		cur.Pattern = *pattern
	}
	if categoryID != nil {
		if _, err := f.GetCategory(*categoryID); err != nil {
			return Rule{}, err
		}
		cur.CategoryID = *categoryID
	}
	if priority != nil {
		cur.Priority = *priority
	}
	_, err = f.DB.Exec(`UPDATE categorization_rules SET pattern=?, category_id=?, priority=? WHERE id=?`,
		cur.Pattern, cur.CategoryID, cur.Priority, id)
	if err != nil {
		return Rule{}, err
	}
	logChange(f.DB, "rule", id, "update", "rule: "+cur.Pattern)
	return f.getRule(id)
}

func (f *Finance) DeleteRule(id string) error {
	cur, err := f.getRule(id)
	if err != nil {
		return err
	}
	res, err := f.DB.Exec(`DELETE FROM categorization_rules WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	logChange(f.DB, "rule", id, "delete", "rule: "+cur.Pattern)
	return nil
}

func (f *Finance) getRule(id string) (Rule, error) {
	var r Rule
	row := f.DB.QueryRow(`SELECT id,pattern,category_id,priority,created_at FROM categorization_rules WHERE id=?`, id)
	err := row.Scan(&r.ID, &r.Pattern, &r.CategoryID, &r.Priority, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return Rule{}, ErrNotFound
	}
	return r, err
}
