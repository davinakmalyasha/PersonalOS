package store

import (
	"database/sql"
	"time"
)

// ---- Cash-flow forecast (phase 12a) ----
//
// Projected balance = current balance (opening + all transactions) advanced
// day by day with: active subscription charges landing on their due dates and
// a smoothed daily net flow from the trailing 90 days.

type ForecastPoint struct {
	Date           string `json:"date"` // YYYY-MM-DD
	ProjectedMinor int64  `json:"projected_minor"`
}

type Forecast struct {
	Days        int             `json:"days"`
	StartMinor  int64           `json:"start_minor"`
	AvgDailyNet int64           `json:"avg_daily_net_minor"`
	Lowest      *ForecastPoint  `json:"lowest,omitempty"`
	Points      []ForecastPoint `json:"points"`
}

// Forecast projects the combined balance across all accounts for N days.
func (f *Finance) Forecast(days int) (Forecast, error) {
	if days < 1 || days > 120 {
		days = 30
	}
	out := Forecast{Days: days}

	accounts, err := f.ListAccounts()
	if err != nil {
		return out, err
	}
	fx, ferr := f.loadRates()
	if ferr != nil {
		return out, err
	}
	var opening int64
	for _, a := range accounts {
		opening += fx.toBase(a.Currency, a.BalanceMinor)
	}

	// Smoothed daily net over trailing 90 days (transfers excluded).
	today := time.Now().UTC()
	from := today.AddDate(0, 0, -89).Format("2006-01-02")
	to := today.Format("2006-01-02")
	var net sql.NullInt64
	if err := f.DB.QueryRow(
		`SELECT SUM(amount) FROM transactions WHERE is_transfer=0 AND date>=? AND date<=?`,
		from, to).Scan(&net); err == nil && net.Valid {
		out.AvgDailyNet = net.Int64 / 90
	}

	// Active subscriptions: charges land on next_due, then monthly multiples.
	type charge struct {
		day time.Time
		amt int64
	}
	var charges []charge
	subs, err := f.ListSubscriptions("active")
	if err != nil {
		return out, err
	}
	for _, s := range subs {
		if s.NextDue == nil || *s.NextDue == "" {
			continue
		}
		base, perr := time.Parse("2006-01-02", *s.NextDue)
		if perr != nil {
			continue
		}
		for k := 0; k <= days/30+1; k++ {
			c := base.AddDate(0, k, 0)
			delta := int(c.Sub(today).Hours() / 24)
			if delta >= 1 && delta <= days {
				charges = append(charges, charge{day: c, amt: s.AmountMinor})
			}
		}
	}

	balance := opening
	lowest := ForecastPoint{}
	for d := 1; d <= days; d++ {
		day := today.AddDate(0, 0, d)
		balance += out.AvgDailyNet
		for _, ch := range charges {
			if sameDay(ch.day, day) {
				balance += ch.amt // amounts are negative for charges
			}
		}
		fp := ForecastPoint{Date: day.Format("2006-01-02"), ProjectedMinor: balance}
		out.Points = append(out.Points, fp)
		if out.Lowest == nil || fp.ProjectedMinor < lowest.ProjectedMinor {
			lowest = fp
		}
	}
	out.StartMinor = opening
	cp := lowest
	out.Lowest = &cp
	return out, nil
}

// LowBalanceAlert returns true when any projected day dips below threshold.
func (f *Finance) LowBalanceAlert(days int, thresholdMinor int64) (bool, Forecast, error) {
	fc, err := f.Forecast(days)
	if err != nil {
		return false, fc, err
	}
	return fc.Lowest != nil && fc.Lowest.ProjectedMinor < thresholdMinor, fc, nil
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
