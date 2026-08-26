package planner

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParsedDate is the outcome of natural-language date parsing.
type ParsedDate struct {
	Date string  `json:"date"`       // YYYY-MM-DD ("" when unparseable)
	Time *string `json:"time"`       // HH:MM when a time was present
	ISO  *string `json:"iso"`        // full RFC3339 when both parts known (UTC)
}

// ParseNaturalDate parses Todoist-style snippets: "today", "tomorrow",
// "next week", weekday names ("fri"), "in 3 days/weeks", "27 aug", "aug 27",
// optional "@17:00"/"at 5pm" times, or plain YYYY-MM-DD[THH:MM].
// base anchors day math (tests pass fixed days).
func ParseNaturalDate(input string, base time.Time) (ParsedDate, error) {
	out := ParsedDate{}
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return out, nil // empty = no date requested, not an error
	}

	var day time.Time
	daySet := false

	// Extract a trailing time token: "@7pm", "at 7pm", "@17:00", "at 17:00".
	var clock *string
	for _, prefix := range []string{"@", " at "} {
		if idx := strings.Index(s, prefix); idx >= 0 {
			if t, ok := parseClock(strings.TrimSpace(s[idx+len(prefix):])); ok {
				clock = &t
				s = strings.TrimSpace(s[:idx])
				break
			}
		}
	}
	// A bare time like "17:00" or "5pm".
	if !daySet && s == "" {
		return out, fmt.Errorf("time without date")
	}

	switch s {
	case "":
		// time-only handled above; unreachable
	case "today", "tod":
		day, daySet = base, true
	case "tomorrow", "tom":
		day, daySet = base.AddDate(0, 0, 1), true
	case "next week":
		day, daySet = nextMonday(base), true
	case "next month":
		day, daySet = base.AddDate(0, 1, 0), true
	default:
		if d, ok := parseInRelative(s, base); ok {
			day, daySet = d, true
			break
		}
		if d, ok := parseWeekdayName(s, base); ok {
			day, daySet = d, true
			break
		}
		if d, ok := parseDayMonth(s, base); ok {
			day, daySet = d, true
			break
		}
		if d, err := time.Parse("2006-01-02", s); err == nil {
			day, daySet = d, true
			break
		}
		if d, ok := parseSlashDate(s); ok {
			day, daySet = d, true
		}
	}

	if !daySet && clock != nil {
		return out, fmt.Errorf("could not parse date part of %q", input)
	}
	if !daySet {
		return out, fmt.Errorf("could not parse %q as a date", input)
	}
	out.Date = day.Format("2006-01-02")
	out.Time = clock
	if clock != nil {
		hhmm := *clock
		iso := fmt.Sprintf("%sT%s:00Z", out.Date, hhmm)
		out.ISO = &iso
	}
	return out, nil
}

func nextWeekday(base time.Time, want time.Weekday) time.Time {
	d := base.AddDate(0, 0, 1)
	for d.Weekday() != want {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

func nextMonday(base time.Time) time.Time { return nextWeekday(base, time.Monday) }

var weekdayNames = map[string]time.Weekday{
	"sunday": time.Sunday, "sun": time.Sunday,
	"monday": time.Monday, "mon": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday, "tues": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday,
	"friday": time.Friday, "fri": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday,
}

func parseWeekdayName(s string, base time.Time) (time.Time, bool) {
	if w, ok := weekdayNames[s]; ok {
		return nextWeekday(base, w), true
	}
	return time.Time{}, false
}

// parseInRelative handles "in 3 days", "in 2 weeks", "in 1 month".
func parseInRelative(s string, base time.Time) (time.Time, bool) {
	parts := strings.Fields(s) // ["in","3","days"]
	if len(parts) != 3 || parts[0] != "in" {
		return time.Time{}, false
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil || n <= 0 || n > 365 {
		return time.Time{}, false
	}
	switch parts[2] {
	case "day", "days":
		return base.AddDate(0, 0, n), true
	case "week", "weeks":
		return base.AddDate(0, 0, n*7), true
	case "month", "months":
		return base.AddDate(0, n, 0), true
	}
	return time.Time{}, false
}

var monthNames = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

// parseDayMonth handles "27 aug" / "aug 27" (current or next occurrence).
func parseDayMonth(s string, base time.Time) (time.Time, bool) {
	fields := strings.Fields(s)
	if len(fields) != 2 {
		return time.Time{}, false
	}
	tryOrders := [][2]string{{fields[0], fields[1]}, {fields[1], fields[0]}}
	for _, o := range tryOrders {
		dayN, err1 := strconv.Atoi(o[0])
		monN, monOK := monthLookup(o[1])
		if err1 == nil && monOK && dayN >= 1 && dayN <= 31 {
			year := base.Year()
			cand := time.Date(year, time.Month(monN), dayN, 0, 0, 0, 0, time.UTC)
			if cand.Before(base) {
				cand = cand.AddDate(1, 0, 0)
			}
			return cand, true
		}
	}
	return time.Time{}, false
}

func monthLookup(s string) (int, bool) {
	s = strings.TrimSuffix(s, ".")
	for k, v := range monthNames {
		if strings.HasPrefix(k, s) && len(s) >= 3 {
			return v, true
		}
	}
	return 0, false
}

// parseSlashDate handles 27/1 and 27/1/2027 (day-first, Indonesian style).
func parseSlashDate(s string) (time.Time, bool) {
	parts := strings.Split(s, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return time.Time{}, false
	}
	dayN, e1 := strconv.Atoi(parts[0])
	monN, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil || dayN < 1 || dayN > 31 || monN < 1 || monN > 12 {
		return time.Time{}, false
	}
	year := time.Now().UTC().Year()
	if len(parts) == 3 {
		y, e3 := strconv.Atoi(parts[2])
		if e3 != nil {
			return time.Time{}, false
		}
		year = y
	}
	cand := time.Date(year, time.Month(monN), dayN, 0, 0, 0, 0, time.UTC)
	if len(parts) == 2 && cand.Before(time.Now().UTC()) {
		cand = cand.AddDate(1, 0, 0)
	}
	return cand, true
}

// parseClock accepts "17:00", "7pm", "7:30am", "9". Returns HH:MM.
func parseClock(s string) (string, bool) {
	s = strings.TrimSpace(s)
	pm := strings.HasSuffix(s, "pm")
	am := strings.HasSuffix(s, "am")
	if pm || am {
		s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(s, "pm"), "am"))
	}
	hh, mm := 0, 0
	if i := strings.IndexByte(s, ':'); i >= 0 {
		h, err1 := strconv.Atoi(s[:i])
		m, err2 := strconv.Atoi(s[i+1:])
		if err1 != nil || err2 != nil {
			return "", false
		}
		hh, mm = h, m
	} else {
		h, err := strconv.Atoi(s)
		if err != nil {
			return "", false
		}
		hh = h
	}
	if am && hh == 12 {
		hh = 0
	}
	if pm && hh < 12 {
		hh += 12
	}
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return "", false
	}
	return fmt.Sprintf("%02d:%02d", hh, mm), true
}
