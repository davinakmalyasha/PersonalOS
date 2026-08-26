package store

import (
	"database/sql"
	"strconv"
)

type Rule struct {
	ID         string `json:"id"`
	Pattern    string `json:"pattern"`
	CategoryID string `json:"category_id"`
	Priority   int    `json:"priority"`
	AmountMin  *int64 `json:"amount_min,omitempty"`
	AmountMax  *int64 `json:"amount_max,omitempty"`
	CreatedAt  string `json:"created_at"`
}

func (f *Finance) CreateRule(pattern, categoryID string, priority int, amountMin, amountMax *int64) (Rule, error) {
	if _, err := f.GetCategory(categoryID); err != nil {
		return Rule{}, err
	}
	r := Rule{ID: NewID(), Pattern: pattern, CategoryID: categoryID, Priority: priority,
		AmountMin: amountMin, AmountMax: amountMax, CreatedAt: NowRFC3339()}
	_, err := f.DB.Exec(`INSERT INTO categorization_rules (id,pattern,category_id,priority,amount_min,amount_max,created_at) VALUES (?,?,?,?,?,?,?)`,
		r.ID, r.Pattern, r.CategoryID, r.Priority, r.AmountMin, r.AmountMax, r.CreatedAt)
	if err != nil {
		return Rule{}, err
	}
	logChange(f.DB, "rule", r.ID, "create", "rule: "+r.Pattern)
	return r, nil
}

func (f *Finance) ListRules() ([]Rule, error) {
	rows, err := f.DB.Query(`SELECT id,pattern,category_id,priority,amount_min,amount_max,created_at FROM categorization_rules ORDER BY priority, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.Pattern, &r.CategoryID, &r.Priority, &r.AmountMin, &r.AmountMax, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (f *Finance) UpdateRule(id string, pattern *string, categoryID *string, priority *int, amountMin, amountMax **int64) (Rule, error) {
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
	if amountMin != nil {
		cur.AmountMin = *amountMin // ptr-to-nil clears
	}
	if amountMax != nil {
		cur.AmountMax = *amountMax
	}
	_, err = f.DB.Exec(`UPDATE categorization_rules SET pattern=?, category_id=?, priority=?, amount_min=?, amount_max=? WHERE id=?`,
		cur.Pattern, cur.CategoryID, cur.Priority, cur.AmountMin, cur.AmountMax, id)
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
	row := f.DB.QueryRow(`SELECT id,pattern,category_id,priority,amount_min,amount_max,created_at FROM categorization_rules WHERE id=?`, id)
	err := row.Scan(&r.ID, &r.Pattern, &r.CategoryID, &r.Priority, &r.AmountMin, &r.AmountMax, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return Rule{}, ErrNotFound
	}
	return r, err
}

// ApplyBackfill re-runs this rule over transaction history: every non-transfer
// transaction matching pattern (+ optional amount window) is moved to the
// rule's category. Returns how many rows changed.
func (f *Finance) ApplyBackfill(id string) (int64, error) {
	r, err := f.getRule(id)
	if err != nil {
		return 0, err
	}
	res, err := f.DB.Exec(`
		UPDATE transactions SET category_id=?
		WHERE is_transfer=0 AND category_id IS NOT ?
		  AND LOWER(raw_description) LIKE '%' || LOWER(?) || '%'
		  AND (? IS NULL OR ABS(amount) >= ?)
		  AND (? IS NULL OR ABS(amount) <= ?)`,
		r.CategoryID, r.CategoryID, r.Pattern,
		r.AmountMin, r.AmountMin,
		r.AmountMax, r.AmountMax)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	logChange(f.DB, "rule", id, "update", "backfilled "+strconv.FormatInt(n, 10)+" transactions")
	return n, nil
}
