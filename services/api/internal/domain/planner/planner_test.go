package planner

import (
	"reflect"
	"testing"
	"time"
)

func TestParseRecurrence(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
		check   func(Recurrence) bool
	}{
		{in: "FREQ=DAILY;COUNT=5", check: func(r Recurrence) bool { return r.Freq == "DAILY" && r.Count == 5 && r.Interval == 1 }},
		{in: "FREQ=WEEKLY;INTERVAL=2", check: func(r Recurrence) bool { return r.Freq == "WEEKLY" && r.Interval == 2 && r.Count == 0 }},
		{in: "FREQ=MONTHLY;UNTIL=20261231", check: func(r Recurrence) bool { return r.Until == "20261231" }},
		{in: "freq=daily;count=3", check: func(r Recurrence) bool { return r.Freq == "DAILY" && r.Count == 3 }},
		{in: "", wantErr: true},
		{in: "FREQ=YEARLY", wantErr: true},
		{in: "INTERVAL=2", wantErr: true},       // FREQ required
		{in: "FREQ=DAILY;FOO=1", wantErr: true}, // unknown token
		{in: "FREQ=DAILY;INTERVAL=0", wantErr: true},
		{in: "FREQ=DAILY;COUNT=x", wantErr: true},
		{in: "FREQ=DAILY;UNTIL=31-12-2026", wantErr: true},       // wrong until format
		{in: "FREQ=DAILY;COUNT=3;UNTIL=20260101", wantErr: true}, // mutually exclusive
	}
	for _, tc := range cases {
		r, err := ParseRecurrence(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseRecurrence(%q): expected error, got %+v", tc.in, r)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRecurrence(%q): unexpected error %v", tc.in, err)
			continue
		}
		if tc.check != nil && !tc.check(r) {
			t.Errorf("ParseRecurrence(%q) = %+v, failed check", tc.in, r)
		}
	}
}

func TestExpandWeekly(t *testing.T) {
	// Weekly event starting Wed 2026-08-05, every week, no end.
	first := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)
	r, err := ParseRecurrence("FREQ=WEEKLY")
	if err != nil {
		t.Fatal(err)
	}
	got := r.Expand(first, "2026-08-01", "2026-09-01")
	want := []string{"2026-08-05", "2026-08-12", "2026-08-19", "2026-08-26"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("weekly expand = %v, want %v", got, want)
	}
}

func TestExpandDailyIntervalCount(t *testing.T) {
	first := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	r, _ := ParseRecurrence("FREQ=DAILY;INTERVAL=2;COUNT=4")
	got := r.Expand(first, "2026-08-01", "2026-12-31")
	want := []string{"2026-08-01", "2026-08-03", "2026-08-05", "2026-08-07"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("daily/2/count=4 = %v, want %v", got, want)
	}
}

func TestExpandUntilInclusiveAndWindowClip(t *testing.T) {
	first := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	r, _ := ParseRecurrence("FREQ=DAILY;UNTIL=20260802")
	got := r.Expand(first, "2026-07-31", "2026-08-31")
	want := []string{"2026-07-31", "2026-08-01", "2026-08-02"} // 07-30 clipped by window
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("until expand = %v, want %v", got, want)
	}
}

func TestExpandMonthlyClampShortMonth(t *testing.T) {
	first := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	r, _ := ParseRecurrence("FREQ=MONTHLY;COUNT=3")
	got := r.Expand(first, "2026-01-01", "2026-12-31")
	want := []string{"2026-01-31", "2026-02-28", "2026-03-31"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("monthly clamp = %v, want %v", got, want)
	}
}

func TestStreaksDaily(t *testing.T) {
	today := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)

	// Checked today + previous 3 days → current 4.
	s := ComputeStreaks([]string{"2026-08-22", "2026-08-23", "2026-08-24", "2026-08-25"}, "daily", 7, today)
	if s.Current != 4 || !s.DoneToday {
		t.Fatalf("current streak = %d doneToday=%v, want 4/true", s.Current, s.DoneToday)
	}

	// Grace: today missing but yesterday checked → streak continues (ends yesterday).
	s = ComputeStreaks([]string{"2026-08-22", "2026-08-23", "2026-08-24"}, "daily", 7, today)
	if s.Current != 3 || s.DoneToday {
		t.Fatalf("grace streak = %d doneToday=%v, want 3/false", s.Current, s.DoneToday)
	}

	// Gap breaks the run.
	s = ComputeStreaks([]string{"2026-08-20", "2026-08-22", "2026-08-23", "2026-08-24", "2026-08-25"}, "daily", 7, today)
	if s.Current != 4 {
		t.Fatalf("gap current = %d, want 4", s.Current)
	}
	if s.Longest != 4 {
		t.Fatalf("gap longest = %d, want 4", s.Longest)
	}
}

func TestStreaksLongestVsCurrent(t *testing.T) {
	today := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	// Long historical run of 6, then a gap, then current run of 2 (+today).
	dates := []string{
		"2026-08-03", "2026-08-04", "2026-08-05", "2026-08-06", "2026-08-07", "2026-08-08",
		"2026-08-11", "2026-08-12",
		"2026-08-24", "2026-08-25",
	}
	s := ComputeStreaks(dates, "daily", 7, today)
	if s.Longest != 6 {
		t.Fatalf("longest = %d, want 6", s.Longest)
	}
	if s.Current != 2 {
		t.Fatalf("current = %d, want 2", s.Current)
	}
}

func TestStreaksWeeklyTarget(t *testing.T) {
	// Current week: Mon 2026-08-24 .. Sun 2026-08-30. Today = Tue 08-25.
	today := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	// Target 3x/week. Last week (08-17..23): 3 checkoffs → qualifies. Week before: 2 → not.
	dates := []string{"2026-08-18", "2026-08-20", "2026-08-21", "2026-08-22", "2026-08-24"}
	s := ComputeStreaks(dates, "weekly", 3, today)
	if s.WeekDone != 1 {
		t.Fatalf("week_done = %d, want 1 (only the 24th)", s.WeekDone)
	}
	if s.Current != 1 {
		t.Fatalf("current weekly streak = %d, want 1 (last week met target)", s.Current)
	}
	// Only the week of Aug 17 (4 checkoffs) meets target=3; current week has 1.
	if s.Longest != 1 {
		t.Fatalf("longest qualifying-week run = %d, want 1", s.Longest)
	}
}

func TestStreaksWeeklyLongestRun(t *testing.T) {
	today := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	// Weeks of Jul 27, Aug 3, Aug 10 each have 3+ checkoffs (target 3) → run of 3.
	dates := []string{
		"2026-07-28", "2026-07-30", "2026-08-01",
		"2026-08-03", "2026-08-05", "2026-08-06",
		"2026-08-10", "2026-08-12", "2026-08-13",
	}
	s := ComputeStreaks(dates, "weekly", 3, today)
	if s.Longest != 3 {
		t.Fatalf("longest = %d, want 3", s.Longest)
	}
	// Current week (Aug 24..30) empty; grace looks at last week (Aug 17..23):
	// no checkoffs there → current streak 0.
	if s.Current != 0 {
		t.Fatalf("current = %d, want 0", s.Current)
	}
}

func TestHabitDueToday(t *testing.T) {
	if !HabitDueToday("daily", 7, 0) {
		t.Fatal("daily habit always due")
	}
	if HabitDueToday("weekly", 3, 3) {
		t.Fatal("weekly habit with target met should not be due")
	}
	if !HabitDueToday("weekly", 3, 2) {
		t.Fatal("weekly habit below target should be due")
	}
}
