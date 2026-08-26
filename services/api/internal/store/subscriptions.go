package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ---- Managed subscriptions (phase 12a) ----
//
// Detection stays derived from transactions; this table mirrors candidates so
// they can be confirmed, muted or cancelled like Monarch/Rocket Money do.

type Subscription struct {
	ID          string  `json:"id"`
	Merchant    string  `json:"merchant"`
	AmountMinor int64   `json:"amount_minor"`
	Cadence     string  `json:"cadence"`
	NextDue     *string `json:"next_due"` // YYYY-MM-DD when known
	Status      string  `json:"status"`   // active|muted|cancelled
	Occurrences int     `json:"occurrences"`
	CreatedAt   string  `json:"created_at,omitempty"`
	UpdatedAt   string  `json:"updated_at,omitempty"`
}

var subscriptionStatuses = map[string]bool{"active": true, "muted": true, "cancelled": true}

// SyncSubscriptions upserts detection results (FindRecurring output) into
// managed rows. Existing lifecycle state (muted/cancelled) is never
// overwritten; new merchants land as 'active'. Returns how many were created.
func (f *Finance) SyncSubscriptions(detected []Recurring) (int, error) {
	now := NowRFC3339()
	created := 0
	for _, d := range detected {
		var id, status, nextDue string
		err := f.DB.QueryRow(
			`SELECT id,status,COALESCE(next_due,'') FROM subscriptions WHERE merchant=? AND amount_minor=?`,
			d.Merchant, d.AmountMinor).Scan(&id, &status, &nextDue)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			_, e := f.DB.Exec(
				`INSERT INTO subscriptions (id,merchant,amount_minor,cadence,next_due,status,occurrences,created_at,updated_at)
				 VALUES (?,?,?,?,?,'active',?,?,?)`,
				NewID(), d.Merchant, d.AmountMinor, "monthly", d.NextGuess, d.Occurrences, now, now)
			if e != nil {
				return created, e
			}
			logChange(f.DB, "subscription", d.Merchant, "create", "detected: "+d.Merchant)
			created++
		case err != nil:
			return created, err
		default:
			if _, e := f.DB.Exec(
				`UPDATE subscriptions SET occurrences=?, next_due=?, updated_at=? WHERE id=?`,
				d.Occurrences, d.NextGuess, now, id); e != nil {
				return created, e
			}
		}
	}
	return created, nil
}

// CreateSubscription adds a managed bill manually (e.g., rent paid offline).
func (f *Finance) CreateSubscription(merchant string, amountMinor int64, cadence, nextDue string) (Subscription, error) {
	if strings.TrimSpace(merchant) == "" || amountMinor == 0 {
		return Subscription{}, ErrInvalid
	}
	if cadence == "" {
		cadence = "monthly"
	}
	if cadence != "weekly" && cadence != "monthly" && cadence != "yearly" {
		return Subscription{}, ErrInvalid
	}
	var due interface{}
	if nextDue != "" {
		if _, err := time.Parse("2006-01-02", nextDue); err != nil {
			return Subscription{}, ErrInvalid
		}
		due = nextDue
	}
	now := NowRFC3339()
	s := Subscription{ID: NewID(), Merchant: merchant, AmountMinor: amountMinor,
		Cadence: cadence, NextDue: nil, Status: "active", CreatedAt: now, UpdatedAt: now}
	if due != nil {
		v := nextDue
		s.NextDue = &v
	}
	_, err := f.DB.Exec(
		`INSERT INTO subscriptions (id,merchant,amount_minor,cadence,next_due,status,occurrences,created_at,updated_at)
		 VALUES (?,?,?,?,?,'active',0,?,?)`,
		s.ID, s.Merchant, s.AmountMinor, s.Cadence, due, now, now)
	if isUniqueErr(err) {
		return Subscription{}, ErrConflict
	}
	if err != nil {
		return Subscription{}, err
	}
	logChange(f.DB, "subscription", s.Merchant, "create", s.Merchant)
	return s, nil
}

// ListSubscriptions returns managed subs newest-first, optionally by status.
func (f *Finance) ListSubscriptions(status string) ([]Subscription, error) {
	if status != "" && !subscriptionStatuses[status] {
		return nil, ErrInvalid
	}
	where := "1=1"
	args := []interface{}{}
	if status != "" {
		where = "status=?"
		args = append(args, status)
	}
	rows, err := f.DB.Query(
		`SELECT id,merchant,amount_minor,cadence,next_due,status,occurrences FROM subscriptions
		 WHERE `+where+` ORDER BY next_due ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Subscription{}
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.ID, &s.Merchant, &s.AmountMinor, &s.Cadence,
			&s.NextDue, &s.Status, &s.Occurrences); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpdateSubscriptionStatus moves one sub through active|muted|cancelled.
func (f *Finance) UpdateSubscriptionStatus(id, status string) (Subscription, error) {
	if !subscriptionStatuses[status] {
		return Subscription{}, ErrInvalid
	}
	res, err := f.DB.Exec(`UPDATE subscriptions SET status=?, updated_at=? WHERE id=?`,
		status, NowRFC3339(), id)
	if err != nil {
		return Subscription{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Subscription{}, ErrNotFound
	}
	var s Subscription
	err = f.DB.QueryRow(
		`SELECT id,merchant,amount_minor,cadence,next_due,status,occurrences FROM subscriptions WHERE id=?`, id).
		Scan(&s.ID, &s.Merchant, &s.AmountMinor, &s.Cadence, &s.NextDue, &s.Status, &s.Occurrences)
	if err != nil {
		return Subscription{}, err
	}
	action := "update"
	if status == "cancelled" {
		action = "delete"
	} else if status == "active" {
		action = "create"
	}
	logChange(f.DB, "subscription", s.Merchant, action, s.Merchant+" → "+status)
	return s, nil
}

// ---- Safe to spend (PocketGuard-style "leftover") ----

type SafeToSpend struct {
	Month        string `json:"month"`         // YYYY-MM
	IncomeMinor  int64  `json:"income_mtd_minor"`
	SpendMinor   int64  `json:"spend_mtd_minor"`
	BudgetLeft   int64  `json:"budget_left_minor"`  // Σ max(0, budget−spent) incl rollover
	BillsAhead   int64  `json:"bills_ahead_minor"`  // active subs due rest of month
	SafeToSpend  int64  `json:"safe_to_spend_minor"`
}

// SafeToSpend computes what's still spendable for the month: income MTD −
// spend MTD − unspent budget − committed subscription charges still ahead in
// the month. Falls back gracefully with zeros when budgets/subs are unset.
func (f *Finance) SafeToSpend(month string) (SafeToSpend, error) {
	out := SafeToSpend{Month: month}
	sum, err := f.SummaryMonth(month)
	if err != nil {
		return out, err
	}
	out.IncomeMinor = sum.Income
	out.SpendMinor = sum.Outcome
	for _, b := range sum.BudgetLines {
		if left := b.BudgetMinor - b.SpentMinor; left > 0 {
			out.BudgetLeft += left
		}
	}

	today := time.Now().UTC()
	monthEnd := 31
	if base, perr := time.Parse("2006-01", month+"-01"); perr == nil {
		monthEnd = time.Date(base.Year(), base.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	}
	subs, err := f.ListSubscriptions("active")
	if err != nil {
		return out, err
	}
	for _, s := range subs {
		if s.NextDue == nil || *s.NextDue == "" {
			continue
		}
		day, perr := time.Parse("2006-01-02", *s.NextDue)
		if perr != nil {
			continue
		}
		// Monthly cadence: charge lands this month if next_due within it, or a
		// later multiple still falls inside (next_due early-next-month edge).
		for k := 0; k < 3; k++ {
			candidate := day.AddDate(0, k, 0)
			if candidate.Format("2006-01") == month && candidate.After(today) &&
				candidate.Day() <= monthEnd {
				out.BillsAhead += s.AmountMinor
				break
			}
		}
	}
	out.SafeToSpend = out.IncomeMinor - out.SpendMinor - out.BudgetLeft - out.BillsAhead
	return out, nil
}
