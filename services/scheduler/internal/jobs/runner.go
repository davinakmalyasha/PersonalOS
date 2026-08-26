package jobs

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Runner executes the nightly jobs and renders the digest text.
type Runner struct {
	Client *Client

	// LowBalance config: 0 threshold disables the check.
	LowBalanceDays      int
	LowBalanceThreshold int64

	// ICSUrl: when set, the runner asks the API to import this calendar
	// each pass (idempotent by UID).
	ICSUrl string
}

// Digest is the outcome of one full pass.
type Digest struct {
	Paired           int
	Subscriptions    int
	BillsDue         []Subscription // next_guess within 7 days
	Expiring         []ExpiringItem
	OverBudgetLines  []BudgetLine
	ICSImported      int
	LowBalanceOn     string // date of lowest projected balance, "" when n/a
	LowBalanceAmount int64
}

// HasFindings reports whether anything is worth nudging about.
func (d Digest) HasFindings() bool {
	return d.Paired > 0 || len(d.BillsDue) > 0 || len(d.Expiring) > 0 ||
		len(d.OverBudgetLines) > 0 || d.LowBalanceOn != ""
}

// Render produces the human/agent-facing message (empty when no findings).
func (d Digest) Render(now time.Time) string {
	var b strings.Builder
	b.WriteString("Personal OS nightly digest — ")
	b.WriteString(now.UTC().Format("2006-01-02"))
	b.WriteString("\n")

	if d.Paired > 0 {
		fmt.Fprintf(&b, "• transfers paired: %d\n", d.Paired)
	}
	if len(d.BillsDue) > 0 {
		fmt.Fprintf(&b, "• bills due ≤7d: %d\n", len(d.BillsDue))
		for _, s := range d.BillsDue {
			day := "???"
			if s.NextGuess != nil && *s.NextGuess != "" {
				day = *s.NextGuess
			}
			fmt.Fprintf(&b, "  - %s: %.2f on %s\n", s.Merchant, float64(s.AmountMinor)/100, day)
		}
	}
	if len(d.Expiring) > 0 {
		fmt.Fprintf(&b, "• expiring ≤30d: %d\n", len(d.Expiring))
		for _, e := range d.Expiring {
			fmt.Fprintf(&b, "  - %s (%s, %dd)\n", e.Title, e.Date, e.DaysLeft)
		}
	}
	if len(d.OverBudgetLines) > 0 {
		fmt.Fprintf(&b, "• over budget: %d\n", len(d.OverBudgetLines))
		for _, l := range d.OverBudgetLines {
			fmt.Fprintf(&b, "  - %s: %.2f / %.2f\n", l.CategoryName,
				float64(l.SpentMinor)/100, float64(l.BudgetMinor)/100)
		}
	}
	if d.ICSImported > 0 {
		fmt.Fprintf(&b, "• calendar events imported: %d\n", d.ICSImported)
	}
	if d.LowBalanceOn != "" {
		fmt.Fprintf(&b, "• LOW BALANCE: projected %.2f on %s\n",
			float64(d.LowBalanceAmount)/100, d.LowBalanceOn)
	}

	out := strings.TrimRight(b.String(), "\n")
	if !d.HasFindings() {
		return ""
	}
	return out
}

// Run executes all four nightly passes; individual job failures are collected
// into the error list but never abort the rest.
func (r *Runner) Run(ctx context.Context, now time.Time) (Digest, []error) {
	d := Digest{}
	var errs []error

	if n, err := r.Client.DetectTransfers(ctx); err != nil {
		errs = append(errs, fmt.Errorf("transfers: %w", err))
	} else {
		d.Paired = n
	}

	subs, err := r.Client.Recurring(ctx)
	if err != nil {
		errs = append(errs, fmt.Errorf("recurring: %w", err))
	} else {
		d.Subscriptions = len(subs)
		for _, s := range subs {
			if s.DaysLeft != nil && *s.DaysLeft >= 0 && *s.DaysLeft <= 7 {
				d.BillsDue = append(d.BillsDue, s)
			}
		}
		sort.Slice(d.BillsDue, func(i, j int) bool { return subLess(d.BillsDue[i], d.BillsDue[j]) })
	}

	exp, err := r.Client.Expiring(ctx, 30)
	if err != nil {
		errs = append(errs, fmt.Errorf("expiring: %w", err))
	} else {
		d.Expiring = exp
	}

	month := now.UTC().Format("2006-01")
	lines, err := r.Client.OverBudgets(ctx, month)
	if err != nil {
		errs = append(errs, fmt.Errorf("budgets: %w", err))
	} else {
		d.OverBudgetLines = lines
	}

	if r.ICSUrl != "" {
		res, ierr := r.Client.ImportICS(ctx, r.ICSUrl)
		if ierr != nil {
			errs = append(errs, fmt.Errorf("ics import: %w", ierr))
		} else {
			d.ICSImported = res.Imported
		}
	}

	if r.LowBalanceThreshold != 0 {
		days := r.LowBalanceDays
		if days <= 0 {
			days = 14
		}
		fc, ferr := r.Client.LowBalance(ctx, days, r.LowBalanceThreshold)
		if ferr != nil {
			errs = append(errs, fmt.Errorf("forecast: %w", ferr))
		} else if fc.Alert && fc.Lowest != nil {
			d.LowBalanceOn = fc.Lowest.Date
			d.LowBalanceAmount = fc.Lowest.ProjectedMinor
		}
	}

	return d, errs
}

func subLess(a, b Subscription) bool {
	ad, bd := "9999", "9999"
	if a.NextGuess != nil && *a.NextGuess != "" {
		ad = *a.NextGuess
	}
	if b.NextGuess != nil && *b.NextGuess != "" {
		bd = *b.NextGuess
	}
	return ad < bd
}
