package main

import (
	"testing"
	"time"

	"github.com/davinakmalyasha/PersonalOS/services/scheduler/internal/config"
)

func TestDueNowEveryTickWithoutRunHour(t *testing.T) {
	cfg := config.Config{RunHourUTC: -1}
	day := ""
	for _, h := range []int{1, 7, 13, 23} {
		now := time.Date(2026, 8, 26, h, 0, 0, 0, time.UTC)
		if !dueNow(cfg, now, &day) {
			t.Fatalf("hour %d should be due when no run hour set", h)
		}
	}
}

func TestDueNowNightlyGating(t *testing.T) {
	cfg := config.Config{RunHourUTC: 2} // 02:00 UTC nightly
	day := ""

	// Not the hour → skip.
	if dueNow(cfg, time.Date(2026, 8, 26, 9, 30, 0, 0, time.UTC), &day) {
		t.Fatal("09:30 must not fire for run-hour=2")
	}
	// Right hour first day → fires once.
	if !dueNow(cfg, time.Date(2026, 8, 26, 2, 5, 0, 0, time.UTC), &day) {
		t.Fatal("02:05 on day one should fire")
	}
	// Same hour later ticks same day → deduped.
	if dueNow(cfg, time.Date(2026, 8, 26, 2, 55, 0, 0, time.UTC), &day) {
		t.Fatal("second tick within the run hour must not re-fire")
	}
	// Next day at the hour → fires again.
	if !dueNow(cfg, time.Date(2026, 8, 27, 2, 0, 0, 0, time.UTC), &day) {
		t.Fatal("next day's run hour should fire")
	}
}
