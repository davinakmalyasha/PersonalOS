package planner

import (
	"testing"
	"time"
)

var base = time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC) // Wednesday

func TestParseNaturalDateBasics(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"today", "2026-08-26"},
		{"tod", "2026-08-26"},
		{"tomorrow", "2026-08-27"},
		{"tom", "2026-08-27"},
		{"next week", "2026-08-31"},   // next Monday
		{"next month", "2026-09-26"},  // same date next month
		{"fri", "2026-08-28"},         // upcoming Friday
		{"monday", "2026-08-31"},      // Monday already passed this week → next week's
		{"in 3 days", "2026-08-29"},
		{"in 2 weeks", "2026-09-09"},
		{"27 aug", "2026-08-27"},
		{"aug 27", "2026-08-27"},
		{"27/1", "2027-01-27"},        // Jan 27 already passed in 2026
		{"1/9", "2026-09-01"},         // day-first, still ahead
		{"2026-12-01", "2026-12-01"}, // passthrough
	}
	for _, c := range cases {
		got, err := ParseNaturalDate(c.in, base)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if got.Date != c.want {
			t.Errorf("%q: got %s want %s", c.in, got.Date, c.want)
		}
		if got.Time != nil {
			t.Errorf("%q: unexpected time %v", c.in, *got.Time)
		}
	}
}

func TestParseNaturalDateTimes(t *testing.T) {
	cases := []struct {
		in       string
		wantDate string
		wantTime string
	}{
		{"today @17:00", "2026-08-26", "17:00"},
		{"fri at 7pm", "2026-08-28", "19:00"},
		{"tomorrow at 8am", "2026-08-27", "08:00"},
		{"in 2 days @9", "2026-08-28", "09:00"},
		{"2026-12-01 @07:15", "2026-12-01", "07:15"},
	}
	for _, c := range cases {
		got, err := ParseNaturalDate(c.in, base)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if got.Date != c.wantDate || got.Time == nil || *got.Time != c.wantTime {
			t.Errorf("%q: got %+v want %s/%s", c.in, got, c.wantDate, c.wantTime)
		}
		if got.ISO == nil || *got.ISO != c.wantDate+"T"+c.wantTime+":00Z" {
			t.Errorf("%q: iso wrong: %v", c.in, got.ISO)
		}
	}
}

func TestParseNaturalDateEdgeCases(t *testing.T) {
	if p, err := ParseNaturalDate("", base); err != nil || p.Date != "" {
		t.Fatalf("empty should be no-op, got %+v err=%v", p, err)
	}
	if _, err := ParseNaturalDate("gibberish", base); err == nil {
		t.Fatal("gibberish must fail")
	}
	if _, err := ParseNaturalDate("@5pm", base); err == nil {
		t.Fatal("time without date must fail")
	}
	// "next friday" from a Friday skips to next week.
	fri := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	got, _ := ParseNaturalDate("friday", fri)
	if got.Date != "2026-09-04" {
		t.Fatalf("friday-on-friday: %s", got.Date)
	}
	// Month-end day clamps naturally via Go date math ("in 1 month" from Aug 31).
	aug31 := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	got, _ = ParseNaturalDate("next month", aug31)
	if got.Date != "2026-10-01" { // Sep 31 normalizes to Oct 1
		t.Fatalf("month clamp: %s", got.Date)
	}
}
