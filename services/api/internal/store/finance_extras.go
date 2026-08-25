package store

import (
	"sort"
	"time"
)

// ---- Recurring (subscriptions) + transfer detection ----

type Recurring struct {
	Merchant    string `json:"merchant"`
	AmountMinor int64  `json:"amount_minor"`
	Occurrences int    `json:"occurrences"`
	FirstDate   string `json:"first_date"`
	LastDate    string `json:"last_date"`
	NextGuess   string `json:"next_guess"`
}

// FindRecurring detects subscription-like patterns: same normalized merchant,
// identical amount, >= 3 occurrences, average gap 25–35 days (monthly-ish).
func (f *Finance) FindRecurring() ([]Recurring, error) {
	rows, err := f.DB.Query(`
		SELECT LOWER(TRIM(merchant)) AS m, amount, COUNT(*) AS n,
		       MIN(date) AS first_d, MAX(date) AS last_d
		FROM transactions
		WHERE is_transfer = 0 AND amount < 0 AND merchant != ''
		GROUP BY m, amount
		HAVING n >= 3`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Recurring{}
	for rows.Next() {
		var r Recurring
		var first, last string
		if err := rows.Scan(&r.Merchant, &r.AmountMinor, &r.Occurrences, &first, &last); err != nil {
			return nil, err
		}
		fd, e1 := time.Parse("2006-01-02", first)
		ld, e2 := time.Parse("2006-01-02", last)
		if e1 != nil || e2 != nil || r.Occurrences < 2 {
			continue
		}
		days := ld.Sub(fd).Hours() / 24
		avgGap := days / float64(r.Occurrences-1)
		if avgGap < 25 || avgGap > 35 {
			continue
		}
		r.FirstDate = first
		r.LastDate = last
		r.NextGuess = ld.AddDate(0, 0, int(avgGap+0.5)).Format("2006-01-02")
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastDate > out[j].LastDate })
	return out, rows.Err()
}

// PairTransfers flags same-day opposite-amount transactions on different
// accounts as transfers (both directions), excluding already-flagged rows.
// Returns the number of newly flagged pairs.
func (f *Finance) PairTransfers() (int64, error) {
	res, err := f.DB.Exec(`
		UPDATE transactions AS a SET is_transfer = 1
		WHERE a.is_transfer = 0 AND EXISTS (
			SELECT 1 FROM transactions b
			WHERE b.is_transfer = 0
			  AND b.date = a.date
			  AND b.amount = -a.amount
			  AND b.account_id != a.account_id
		)`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// pairDetectFor re-scans pairs involving the given transaction and flags both
// sides. Called after create/import.
func (f *Finance) pairDetectFor(id string) {
	_, _ = f.DB.Exec(`
		UPDATE transactions SET is_transfer = 1
		WHERE id IN (
			SELECT a.id FROM transactions a
			JOIN transactions b
			  ON b.date = a.date AND b.amount = -a.amount AND b.account_id != a.account_id
			WHERE a.is_transfer = 0 AND b.is_transfer = 0 AND (a.id = ? OR b.id = ?)
		)`, id, id)
}
