package store

import (
	"sort"
	"strings"
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
// identical amount, >= 3 occurrences, average gap 25â€“35 days (monthly-ish).
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

// ---- Merchant aliases ----

type MerchantAlias struct {
	ID        string `json:"id"`
	Pattern   string `json:"pattern"` // substring match, case-insensitive
	Canonical string `json:"canonical"`
	CreatedAt string `json:"created_at"`
}

func (f *Finance) CreateAlias(pattern, canonical string) (MerchantAlias, error) {
	pattern = strings.TrimSpace(pattern)
	canonical = strings.TrimSpace(canonical)
	if pattern == "" || canonical == "" {
		return MerchantAlias{}, ErrInvalid
	}
	a := MerchantAlias{ID: NewID(), Pattern: strings.ToLower(pattern), Canonical: canonical, CreatedAt: NowRFC3339()}
	_, err := f.DB.Exec(
		`INSERT INTO merchant_aliases (id,pattern,canonical,created_at) VALUES (?,?,?,?)`,
		a.ID, a.Pattern, a.Canonical, a.CreatedAt)
	if isUniqueErr(err) {
		return MerchantAlias{}, ErrConflict
	}
	return a, err
}

func (f *Finance) ListAliases() ([]MerchantAlias, error) {
	rows, err := f.DB.Query(`SELECT id,pattern,canonical,created_at FROM merchant_aliases ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MerchantAlias{}
	for rows.Next() {
		var a MerchantAlias
		if err := rows.Scan(&a.ID, &a.Pattern, &a.Canonical, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (f *Finance) DeleteAlias(id string) error {
	res, err := f.DB.Exec(`DELETE FROM merchant_aliases WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ApplyAlias rewrites merchant when an alias pattern matches (first hit wins).
func (f *Finance) ApplyAlias(merchant string) string {
	aliases, err := f.ListAliases()
	if err != nil || len(aliases) == 0 {
		return merchant
	}
	lower := strings.ToLower(merchant)
	for _, a := range aliases {
		if a.Pattern != "" && strings.Contains(lower, a.Pattern) {
			return a.Canonical
		}
	}
	return merchant
}

// ---- Net worth (cumulative balances over time, derived) ----

type NetWorthPoint struct {
	Date       string `json:"date"` // YYYY-MM-DD
	TotalMinor int64  `json:"total_minor"`
}

type NetWorth struct {
	Points          []NetWorthPoint          `json:"points"`
	Accounts        []map[string]interface{} `json:"accounts"` // latest per-account balance
	AssetsMinor     int64                    `json:"assets_minor"`
	LiabilitiesMinor int64                   `json:"liabilities_minor"`
	NetMinor        int64                    `json:"net_minor"`
}

// NetWorthSeries walks all transactions in date order accumulating per-account
// balances (seeded with opening balances), emitting the portfolio total at
// each date where anything moved. Liability accounts subtract from net.
func (f *Finance) NetWorthSeries() (NetWorth, error) {
	out := NetWorth{Points: []NetWorthPoint{}, Accounts: []map[string]interface{}{}}

	kinds := map[string]string{}
	openings := map[string]int64{}
	acctRows, err := f.DB.Query(`SELECT id, kind, opening_balance_minor FROM accounts`)
	if err != nil {
		return out, err
	}
	for acctRows.Next() {
		var id, kind string
		var opening int64
		if err := acctRows.Scan(&id, &kind, &opening); err != nil {
			acctRows.Close()
			return out, err
		}
		kinds[id] = kind
		openings[id] = opening
	}
	acctRows.Close()

	rows, err := f.DB.Query(
		`SELECT date, account_id, amount FROM transactions ORDER BY date ASC, created_at ASC`)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	balances := map[string]int64{}
	for id, op := range openings {
		balances[id] = op
	}
	sign := func(id string) int64 {
		if kinds[id] == "liability" {
			return -1
		}
		return 1
	}
	netWorth := func() int64 {
		var total int64
		for id, v := range balances {
			total += sign(id) * v
		}
		return total
	}

	var points []NetWorthPoint
	lastDate := ""
	for rows.Next() {
		var date, accountID string
		var amount int64
		if err := rows.Scan(&date, &accountID, &amount); err != nil {
			return out, err
		}
		// Emit the previous date's close BEFORE applying the new date's txn,
		// so each point reflects end-of-day balances for its date.
		if lastDate != "" && date != lastDate {
			points = append(points, NetWorthPoint{Date: lastDate, TotalMinor: netWorth()})
		}
		balances[accountID] += amount
		lastDate = date
	}
	if lastDate != "" {
		points = append(points, NetWorthPoint{Date: lastDate, TotalMinor: netWorth()})
	} else if len(openings) > 0 {
		points = append(points, NetWorthPoint{Date: time.Now().UTC().Format("2006-01-02"), TotalMinor: netWorth()})
	}
	out.Points = points

	for accountID, bal := range balances {
		out.Accounts = append(out.Accounts, map[string]interface{}{
			"account_id": accountID, "balance_minor": bal, "kind": kinds[accountID],
		})
	}
	sort.Slice(out.Accounts, func(i, j int) bool {
		a, _ := out.Accounts[i]["account_id"].(string)
		b, _ := out.Accounts[j]["account_id"].(string)
		return a < b
	})

	for id, bal := range balances {
		if sign(id) < 0 {
			out.LiabilitiesMinor += bal
		} else {
			out.AssetsMinor += bal
		}
	}
	out.NetMinor = out.AssetsMinor - out.LiabilitiesMinor
	return out, rows.Err()
}

// ---- Upcoming bills (derived from recurring detection) ----

type UpcomingBill struct {
	Recurring
	DaysUntil int `json:"days_until"`
}

// UpcomingBills returns recurring charges whose next_guess falls within the
// next `days` days (default 7).
func (f *Finance) UpcomingBills(days int) ([]UpcomingBill, error) {
	if days < 1 || days > 90 {
		days = 7
	}
	recs, err := f.FindRecurring()
	if err != nil {
		return nil, err
	}
	today := time.Now().UTC()
	var out []UpcomingBill
	for _, r := range recs {
		next, perr := time.Parse("2006-01-02", r.NextGuess)
		if perr != nil {
			continue
		}
		daysUntil := int(next.Sub(today).Hours() / 24)
		if daysUntil >= 0 && daysUntil <= days {
			out = append(out, UpcomingBill{Recurring: r, DaysUntil: daysUntil})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DaysUntil < out[j].DaysUntil })
	return out, nil
}