package planner

import (
	"fmt"
	"sort"
	"time"
)

// Streaks summarizes a habit's checkoff history.
type Streaks struct {
	Current       int  `json:"current"`
	Longest       int  `json:"longest"`
	DoneToday     bool `json:"done_today"`
	WeekDone      int  `json:"week_done"` // checkoffs in the current ISO week
	TargetPerWeek int  `json:"target_per_week"`
	Consistency30 int  `json:"consistency_30"` // % of scheduled days checked, trailing 30d
}

// scheduledDaysInWindow counts how many of the last `window` days (inclusive
// of today) are scheduled per the weekdays bitmask ("1111111", Mon-first).
// Days on/before pausedUntil (when set) are excused — neither scheduled nor
// penalized.
func scheduledDaysInWindow(weekdays string, today time.Time, window int, pausedUntil *string) int {
	if len(weekdays) != 7 {
		weekdays = "1111111"
	}
	n := 0
	for i := 0; i < window; i++ {
		d := today.AddDate(0, 0, -i)
		if pausedUntil != nil && !d.After(parseDayOrToday(*pausedUntil, today)) {
			continue
		}
		wd := int(d.Weekday()+6) % 7 // Mon=0..Sun=6
		if weekdays[wd] == '1' {
			n++
		}
	}
	return n
}

func parseDayOrToday(day string, today time.Time) time.Time {
	if t, err := time.Parse(dateLayout, day); err == nil {
		return t
	}
	return today
}

// Consistency30 computes the trailing-30-day consistency percentage against
// the habit's weekday schedule. Paused days are excused (excluded from the
// denominator).
func Consistency30(dates []string, weekdays string, today time.Time, pausedUntil *string) int {
	sched := scheduledDaysInWindow(weekdays, today, 30, pausedUntil)
	if sched == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(dates))
	for _, d := range normalizeDates(dates) {
		set[d] = struct{}{}
	}
	done := 0
	for i := 0; i < 30; i++ {
		day := today.AddDate(0, 0, -i)
		if pausedUntil != nil && !day.After(parseDayOrToday(*pausedUntil, today)) {
			continue
		}
		if _, ok := set[day.Format(dateLayout)]; ok {
			done++
		}
	}
	return done * 100 / sched
}

// ComputeStreaks calculates current + longest streak and weekly progress from
// an ascending-sorted or arbitrary-order list of YYYY-MM-DD dates.
//
// Daily habits: streak = consecutive calendar days ending today, or ending
// yesterday when today is not yet checked (grace so the streak doesn't read 0
// all morning).
//
// Weekly habits: streak counts consecutive ISO weeks meeting target_per_week,
// with the same grace for the in-progress week; WeekDone tracks progress
// toward the target inside the current ISO week (Mon–Sun).
func ComputeStreaks(dates []string, cadence string, targetPerWeek int, today time.Time) Streaks {
	s := Streaks{TargetPerWeek: targetPerWeek}
	if targetPerWeek < 1 {
		targetPerWeek = 1
	}
	clean := normalizeDates(dates)
	if len(clean) == 0 {
		return s
	}
	todayDay := today.Format(dateLayout)
	for _, d := range clean {
		if d == todayDay {
			s.DoneToday = true
		}
	}

	switch cadence {
	case "weekly":
		counts := map[string]int{} // iso-week-start → count
		for _, d := range clean {
			counts[weekStartKey(d)]++
		}
		curWeekStart := weekStart(today)
		s.WeekDone = counts[curWeekStart.Format(dateLayout)]
		// Longest run of consecutive target-MEETING weeks (same semantics as
		// the daily case's consecutive-day run).
		longest, run := 0, 0
		var prev time.Time
		havePrev := false
		keys := make([]string, 0, len(counts))
		for k := range counts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if counts[k] < targetPerWeek {
				run, havePrev = 0, false
				continue
			}
			wk, err := time.Parse(dateLayout, k)
			if err != nil {
				continue
			}
			if havePrev && wk.Equal(prev.AddDate(0, 0, 7)) {
				run++
			} else {
				run = 1
			}
			if run > longest {
				longest = run
			}
			prev, havePrev = wk, true
		}
		s.Longest = longest
		// Current streak: consecutive qualifying weeks ending now/last week.
		cur := 0
		w := curWeekStart
		if counts[w.Format(dateLayout)] < targetPerWeek {
			w = w.AddDate(0, 0, -7)
		}
		for {
			key := w.Format(dateLayout)
			if counts[key] < targetPerWeek {
				break
			}
			cur++
			w = w.AddDate(0, 0, -7)
		}
		s.Current = cur
	default: // daily
		set := map[string]bool{}
		for _, d := range clean {
			set[d] = true
		}
		longest, run := 0, 0
		var prev time.Time
		havePrev := false
		for _, d := range clean {
			t, err := time.Parse(dateLayout, d)
			if err != nil {
				continue
			}
			if havePrev && t.Equal(prev.AddDate(0, 0, 1)) {
				run++
			} else {
				run = 1
			}
			if run > longest {
				longest = run
			}
			prev, havePrev = t, true
		}
		s.Longest = longest
		// Current: walk back from today (or yesterday if today unchecked).
		start := today
		if !set[start.Format(dateLayout)] {
			start = start.AddDate(0, 0, -1)
		}
		cur := 0
		for i := 0; i < 36500; i++ {
			day := start.AddDate(0, 0, -i).Format(dateLayout)
			if !set[day] {
				break
			}
			cur++
		}
		s.Current = cur
	}
	return s
}

// HabitDueToday: daily habits are due on their scheduled weekdays; weekly
// habits are due while the current week's count is below target.
func HabitDueToday(cadence string, targetPerWeek, weekDone int, weekdays string, today time.Time) bool {
	if len(weekdays) == 7 {
		wd := int(today.Weekday()+6) % 7 // Mon=0..Sun=6
		if weekdays[wd] != '1' {
			return false
		}
	}
	if cadence == "weekly" {
		return weekDone < targetPerWeek
	}
	return true
}

// normalizeDates trims, validates format, dedupes, sorts ascending.
func normalizeDates(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, d := range in {
		d = trimSpace(d)
		if d == "" || seen[d] {
			continue
		}
		if _, err := time.Parse(dateLayout, d); err != nil {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

func weekStart(t time.Time) time.Time {
	t = t.UTC()
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7 // Sunday → 7, Monday=1..Sunday=7
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(wd - 1))
}

func weekStartKey(day string) string {
	t, err := time.Parse(dateLayout, day)
	if err != nil {
		return day
	}
	return weekStart(t).Format(dateLayout)
}

func nextWeekKey(weekStartKeyStr string) string {
	t, err := time.Parse(dateLayout, weekStartKeyStr)
	if err != nil {
		return weekStartKeyStr
	}
	return t.AddDate(0, 0, 7).Format(dateLayout)
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && isSpaceByte(s[start]) {
		start++
	}
	for end > start && isSpaceByte(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// ParseDateStrict parses YYYY-MM-DD or returns a friendly error.
func ParseDateStrict(s string) (time.Time, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("date %q must be YYYY-MM-DD", s)
	}
	return t, nil
}
