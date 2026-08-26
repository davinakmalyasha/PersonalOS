package planner

import "testing"

const sampleICS = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//EN
BEGIN:VEVENT
UID:evt-001@example.com
DTSTAMP:20260801T000000Z
DTSTART:20260910T090000Z
DTEND:20260910T100000Z
SUMMARY:Standup meeting
LOCATION:Room 4
DESCRIPTION:Daily sync\, bring notes
END:VEVENT
BEGIN:VEVENT
UID:evt-002@example.com
DTSTART;VALUE=DATE:20260915
SUMMARY:All-day company offsite
END:VEVENT
BEGIN:VEVENT
UID:evt-003@example.com
DTSTART;TZID=Asia/Jakarta:20260920T140000
RRULE:FREQ=WEEKLY;BYDAY=MO,WE
SUMMARY:Gym session
DESCRIPTION:Recurring workout
END:VEVENT
BEGIN:VEVENT
SUMMARY:Folded title continues
 here
DTSTART:20261001T120000Z
END:VEVENT
END:VCALENDAR
`

func TestParseICSEvents(t *testing.T) {
	evs := ParseICS(sampleICS)
	if len(evs) != 4 {
		t.Fatalf("want 4 events, got %d: %+v", len(evs), evs)
	}

	e1 := evs[0]
	if e1.UID != "evt-001@example.com" || e1.Title != "Standup meeting" {
		t.Fatalf("e1 wrong: %+v", e1)
	}
	if e1.StartsAt != "2026-09-10T09:00:00Z" || e1.EndsAt == nil || *e1.EndsAt != "2026-09-10T10:00:00Z" {
		t.Fatalf("e1 times wrong: %+v", e1)
	}
	if e1.Location != "Room 4" || e1.Description != "Daily sync, bring notes" {
		t.Fatalf("e1 fields wrong: %+v", e1)
	}

	e2 := evs[1]
	if e2.StartsAt != "2026-09-15T00:00:00Z" {
		t.Fatalf("all-day start wrong: %+v", e2)
	}

	e3 := evs[2]
	if e3.StartsAt != "2026-09-20T14:00:00Z" {
		t.Fatalf("tzid start (read as UTC) wrong: %+v", e3)
	}
	if e3.Description == "" || !contains(e3.Description, "recurrence ignored") {
		t.Fatalf("rrule note missing: %q", e3.Description)
	}

	e4 := evs[3]
	if e4.Title != "Folded title continueshere" && e4.Title != "Folded title continues here" {
		t.Fatalf("unfold failed: %q", e4.Title)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestParseICSEmptyAndBad(t *testing.T) {
	if evs := ParseICS("not a calendar"); len(evs) != 0 {
		t.Fatalf("expected 0, got %d", len(evs))
	}
	if evs := ParseICS(""); len(evs) != 0 {
		t.Fatalf("expected 0, got %d", len(evs))
	}
	// Event without SUMMARY is skipped.
	if evs := ParseICS("BEGIN:VEVENT\nDTSTART:20260101T000000Z\nEND:VEVENT"); len(evs) != 0 {
		t.Fatalf("untitled event should be skipped, got %d", len(evs))
	}
}
