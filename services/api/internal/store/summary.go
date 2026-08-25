package store

import (
	"fmt"
)

type MonthSummary struct {
	Month       string         `json:"month"`
	Income      int64          `json:"income_minor"`
	Outcome     int64          `json:"outcome_minor"` // positive number = money out
	Net         int64          `json:"net_minor"`
	ByCategory  []CategorySpen `json:"by_category"`
	BudgetLines []BudgetLine   `json:"budgets"`
}

type CategorySpen struct {
	CategoryID string `json:"category_id,omitempty"`
	Name       string `json:"name"`
	SpentMinor int64  `json:"spent_minor"`
}

type BudgetLine struct {
	CategoryID   string `json:"category_id"`
	CategoryName string `json:"category_name"`
	BudgetMinor  int64  `json:"budget_minor"`
	SpentMinor   int64  `json:"spent_minor"`
	Over         bool   `json:"over"`
}

func monthBounds(month string) (string, string, error) {
	if len(month) != 7 || month[4] != '-' {
		return "", "", ErrInvalid
	}
	var y, m int
	if _, err := fmt.Sscanf(month, "%d-%d", &y, &m); err != nil {
		return "", "", ErrInvalid
	}
	if y < 1900 || y > 2999 || m < 1 || m > 12 {
		return "", "", ErrInvalid
	}
	start := fmt.Sprintf("%04d-%02d-01", y, m)
	ny, nm := y, m+1
	if nm == 13 {
		ny, nm = y+1, 1
	}
	end := fmt.Sprintf("%04d-%02d-01", ny, nm)
	return start, end, nil
}

// SummaryMonth aggregates a calendar month: totals, spend rolled up to
// top-level categories, and budget-vs-spent lines.
func (f *Finance) SummaryMonth(month string) (*MonthSummary, error) {
	start, end, err := monthBounds(month)
	if err != nil {
		return nil, err
	}
	s := &MonthSummary{Month: month, ByCategory: []CategorySpen{}, BudgetLines: []BudgetLine{}}

	err = f.DB.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN amount>0 THEN amount ELSE 0 END),0),
		       COALESCE(SUM(CASE WHEN amount<0 THEN -amount ELSE 0 END),0)
		FROM transactions WHERE date>=? AND date<? AND is_transfer=0`, start, end).
		Scan(&s.Income, &s.Outcome)
	if err != nil {
		return nil, err
	}
	s.Net = s.Income - s.Outcome

	rows, err := f.DB.Query(`
		SELECT t.category_id, c.name, SUM(-t.amount) AS spent
		FROM transactions t LEFT JOIN categories c ON c.id=t.category_id
		WHERE t.date>=? AND t.date<? AND t.amount<0 AND t.is_transfer=0
		GROUP BY t.category_id ORDER BY spent DESC`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type catRow struct {
		id    *string
		name  *string
		spent int64
	}
	var catRows []catRow
	for rows.Next() {
		var r catRow
		if err := rows.Scan(&r.id, &r.name, &r.spent); err != nil {
			return nil, err
		}
		catRows = append(catRows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	parents, err := f.ListCategories()
	if err != nil {
		return nil, err
	}
	parentOf := map[string]*string{}
	nameOf := map[string]string{}
	for _, c := range parents {
		cc := c.ParentID
		parentOf[c.ID] = cc
		nameOf[c.ID] = c.Name
	}
	rootFor := func(id string) (string, bool) {
		cur := id
		for i := 0; i < 32 && cur != ""; i++ {
			p, has := parentOf[cur]
			if !has || p == nil || *p == "" {
				return cur, true
			}
			cur = *p
		}
		return "", false
	}
	rolled := map[string]int64{}
	for _, r := range catRows {
		if r.id == nil {
			rolled["__uncat"] += r.spent
			continue
		}
		root, ok := rootFor(*r.id)
		if !ok {
			root = *r.id
		}
		rolled[root] += r.spent
	}
	for id, spent := range rolled {
		cs := CategorySpen{SpentMinor: spent}
		if id == "__uncat" {
			cs.Name = "Uncategorized"
		} else {
			cs.CategoryID = id
			cs.Name = nameOf[id]
		}
		s.ByCategory = append(s.ByCategory, cs)
	}

	budgetRows, err := f.ListBudgets(month, month)
	if err != nil {
		return nil, err
	}
	for _, b := range budgetRows {
		var spent int64
		_ = f.DB.QueryRow(`SELECT COALESCE(SUM(-amount),0) FROM transactions WHERE category_id=? AND date>=? AND date<? AND is_transfer=0`,
			b.CategoryID, start, end).Scan(&spent)
		s.BudgetLines = append(s.BudgetLines, BudgetLine{
			CategoryID: b.CategoryID, CategoryName: b.Category,
			BudgetMinor: b.Amount, SpentMinor: spent, Over: spent > b.Amount,
		})
	}
	return s, nil
}

type SpendingPoint struct {
	Label      string `json:"label"`
	SpentMinor int64  `json:"spent_minor"`
}

// SpendingSeries buckets negative flow: by "month" (YYYY-MM) or "category".
func (f *Finance) SpendingSeries(groupBy, from, to string) ([]SpendingPoint, error) {
	switch groupBy {
	case "", "month":
		rows, err := f.DB.Query(`
			SELECT substr(date,1,7) AS m, SUM(-amount)
			FROM transactions WHERE amount<0 AND date>=? AND date<=? AND is_transfer=0
			GROUP BY m ORDER BY m`, from, toEnd(from, to))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []SpendingPoint{}
		for rows.Next() {
			var p SpendingPoint
			if err := rows.Scan(&p.Label, &p.SpentMinor); err != nil {
				return nil, err
			}
			out = append(out, p)
		}
		return out, rows.Err()
	case "category":
		rows, err := f.DB.Query(`
			SELECT COALESCE(c.name,'Uncategorized') AS name, SUM(-t.amount)
			FROM transactions t LEFT JOIN categories c ON c.id=t.category_id
			WHERE t.amount<0 AND t.date>=? AND t.date<=? AND t.is_transfer=0
			GROUP BY t.category_id, c.name ORDER BY 2 DESC`, from, toEnd(from, to))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []SpendingPoint{}
		for rows.Next() {
			var p SpendingPoint
			if err := rows.Scan(&p.Label, &p.SpentMinor); err != nil {
				return nil, err
			}
			out = append(out, p)
		}
		return out, rows.Err()
	default:
		return nil, ErrInvalid
	}
}

// toEnd keeps 'from' when 'to' is absent; callers pass inclusive dates.
func toEnd(from, to string) string {
	if to != "" {
		return to
	}
	if len(from) >= 7 {
		return from[:7] + "~"
	}
	return "9999-99~"
}
