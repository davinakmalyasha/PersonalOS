// Package planner holds pure domain logic for the Planner pillar:
// RRULE-lite recurrence parsing/expansion and habit streak math.
package planner

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

// Recurrence is a parsed RRULE-lite rule.
//
// Grammar (v1): FREQ=DAILY|WEEKLY|MONTHLY;INTERVAL=n;COUNT=n|UNTIL=YYYYMMDD
// INTERVAL defaults to 1. Exactly one of COUNT or UNTIL may be present;
// without either the expansion window passed to Expand bounds it.
type Recurrence struct {
	Freq     string // DAILY | WEEKLY | MONTHLY
	Interval int
	Count    int    // 0 = unbounded by count
	Until    string // YYYYMMDD, "" = unbounded by until
}

// ParseRecurrence validates an RRULE-lite string. Unknown tokens or values
// return an error so handlers can map it to 400.
func ParseRecurrence(s string) (Recurrence, error) {
	r := Recurrence{Interval: 1}
	if strings.TrimSpace(s) == "" {
		return r, fmt.Errorf("empty recurrence")
	}
	sawCount, sawUntil := false, false
	for _, part := range strings.Split(s, ";") {
		if strings.TrimSpace(part) == "" {
			return r, fmt.Errorf("empty segment in %q", s)
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return r, fmt.Errorf("segment %q must be KEY=VALUE", part)
		}
		key := strings.ToUpper(strings.TrimSpace(kv[0]))
		val := strings.TrimSpace(kv[1])
		switch key {
		case "FREQ":
			switch strings.ToUpper(val) {
			case "DAILY", "WEEKLY", "MONTHLY":
				r.Freq = strings.ToUpper(val)
			default:
				return r, fmt.Errorf("unsupported FREQ %q (DAILY|WEEKLY|MONTHLY)", val)
			}
		case "INTERVAL":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 || n > 366 {
				return r, fmt.Errorf("INTERVAL %q must be an integer in [1,366]", val)
			}
			r.Interval = n
		case "COUNT":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 || n > 10000 {
				return r, fmt.Errorf("COUNT %q must be an integer in [1,10000]", val)
			}
			r.Count, sawCount = n, true
		case "UNTIL":
			t, err := time.Parse("20060102", val)
			if err != nil {
				return r, fmt.Errorf("UNTIL %q must be YYYYMMDD", val)
			}
			r.Until, sawUntil = t.Format("20060102"), true
		default:
			return r, fmt.Errorf("unknown token %q", key)
		}
	}
	if r.Freq == "" {
		return r, fmt.Errorf("FREQ required")
	}
	if sawCount && sawUntil {
		return r, fmt.Errorf("COUNT and UNTIL are mutually exclusive")
	}
	return r, nil
}

// Expand returns occurrence start dates (YYYY-MM-DD) for the rule beginning at
// firstStart (parsed from the event's starts_at), within [from, to] inclusive.
// The first occurrence is the event itself; it appears only when inside the
// window and allowed by COUNT/UNTIL semantics (COUNT includes the original).
// UNTIL is inclusive by calendar day.
func (r Recurrence) Expand(firstStart time.Time, from, to string) []string {
	fromDate, err := time.Parse(dateLayout, from)
	if err != nil {
		return nil
	}
	toDate, err := time.Parse(dateLayout, to)
	if err != nil {
		return nil
	}
	first := time.Date(firstStart.Year(), firstStart.Month(), firstStart.Day(),
		firstStart.Hour(), firstStart.Minute(), firstStart.Second(), 0, time.UTC)

	var out []string
	for n := 1; ; n++ {
		if r.Count > 0 && n > r.Count {
			break
		}
		// Each occurrence derives from the ORIGINAL start (no compounding),
		// so monthly clamping never drifts: Jan 31 → Feb 28 → Mar 31.
		var occ time.Time
		switch r.Freq {
		case "DAILY":
			occ = first.AddDate(0, 0, r.Interval*(n-1))
		case "WEEKLY":
			occ = first.AddDate(0, 0, 7*r.Interval*(n-1))
		case "MONTHLY":
			occ = addMonthsClamped(first, r.Interval*(n-1))
		default:
			return out
		}
		day := occ.Format(dateLayout)
		if day > toDate.Format(dateLayout) {
			break
		}
		if day >= fromDate.Format(dateLayout) {
			if r.Until == "" || day <= untilDay(r.Until) {
				out = append(out, day)
			}
		}
		// Hard safety bound for UNTIL-less, COUNT-less rules.
		if r.Count == 0 && r.Until == "" && len(out) >= 1826 {
			break
		}
		if occ.After(toDate.AddDate(1, 0, 0)) {
			break
		}
	}
	return out
}

func untilDay(untilYYYYMMDD string) string {
	if t, err := time.Parse("20060102", untilYYYYMMDD); err == nil {
		return t.Format(dateLayout)
	}
	return untilYYYYMMDD
}

// addMonthsClamped moves t forward n months keeping the day-of-month; when
// the target month is shorter the date clamps to its last day (Jan 31 + 1mo
// → Feb 28/29).
func addMonthsClamped(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	firstOfTarget := time.Date(y, m+time.Month(n), 1, t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
	lastDay := firstOfTarget.AddDate(0, 1, -1).Day()
	if d > lastDay {
		d = lastDay
	}
	return time.Date(firstOfTarget.Year(), firstOfTarget.Month(), d,
		t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
}

// ValidRule reports whether s parses as RRULE-lite.
func ValidRule(s string) bool {
	_, err := ParseRecurrence(s)
	return err == nil
}
