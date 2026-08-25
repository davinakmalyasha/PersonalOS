package store

import (
	"database/sql"
	"errors"
)

type Budget struct {
	ID         string `json:"id"`
	CategoryID string `json:"category_id"`
	Category   string `json:"category,omitempty"`
	Month      string `json:"month"`
	Amount     int64  `json:"amount_minor"`
	Rollover   bool   `json:"rollover"`
	CreatedAt  string `json:"created_at,omitempty"`
}

// UpsertBudget inserts or updates the (category, month) budget.
func (f *Finance) UpsertBudget(categoryID, month string, amount int64, rollover bool) (Budget, error) {
	if _, err := f.GetCategory(categoryID); err != nil {
		return Budget{}, err
	}
	var existing string
	err := f.DB.QueryRow(`SELECT id FROM budgets WHERE category_id=? AND month=?`, categoryID, month).Scan(&existing)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		b := Budget{ID: NewID(), CategoryID: categoryID, Month: month, Amount: amount, Rollover: rollover, CreatedAt: NowRFC3339()}
		_, err = f.DB.Exec(`INSERT INTO budgets (id,category_id,month,amount,rollover,created_at) VALUES (?,?,?,?,?,?)`,
			b.ID, b.CategoryID, b.Month, b.Amount, b.Rollover, b.CreatedAt)
		if err != nil {
			return Budget{}, err
		}
		b.Category = ""
		return f.GetBudget(b.ID)
	case err != nil:
		return Budget{}, err
	default:
		if _, err := f.DB.Exec(`UPDATE budgets SET amount=?, rollover=? WHERE id=?`, amount, rollover, existing); err != nil {
			return Budget{}, err
		}
		return f.GetBudget(existing)
	}
}

func (f *Finance) GetBudget(id string) (Budget, error) {
	var b Budget
	err := f.DB.QueryRow(`
		SELECT b.id,b.category_id,b.month,b.amount,b.rollover,c.name,b.created_at
		FROM budgets b JOIN categories c ON c.id=b.category_id WHERE b.id=?`, id).
		Scan(&b.ID, &b.CategoryID, &b.Month, &b.Amount, &b.Rollover, &b.Category, &b.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Budget{}, ErrNotFound
	}
	return b, err
}

func (f *Finance) ListBudgets(monthFrom, monthTo string) ([]Budget, error) {
	q := `SELECT b.id,b.category_id,b.month,b.amount,b.rollover,c.name FROM budgets b JOIN categories c ON c.id=b.category_id WHERE 1=1`
	var args []interface{}
	if monthFrom != "" {
		q += ` AND b.month>=?`
		args = append(args, monthFrom)
	}
	if monthTo != "" {
		q += ` AND b.month<=?`
		args = append(args, monthTo)
	}
	q += ` ORDER BY b.month DESC, c.name`
	rows, err := f.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Budget{}
	for rows.Next() {
		var b Budget
		if err := rows.Scan(&b.ID, &b.CategoryID, &b.Month, &b.Amount, &b.Rollover, &b.Category); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (f *Finance) DeleteBudget(id string) error {
	res, err := f.DB.Exec(`DELETE FROM budgets WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
