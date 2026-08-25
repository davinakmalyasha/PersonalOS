package server

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// ---- helpers ----

func createTask(t *testing.T, h http.Handler, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/tasks", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create task: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func createHabit(t *testing.T, h http.Handler, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/habits", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create habit: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func createEvent(t *testing.T, h http.Handler, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/events", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create event: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func idOf(m map[string]interface{}) string {
	s, _ := m["id"].(string)
	return s
}

// ---- Tasks ----

func TestTaskLifecycleAndFilters(t *testing.T) {
	h := newTestAPI(t)

	task := createTask(t, h, map[string]interface{}{
		"title": "Pay rent", "priority": "high", "due_date": "2026-09-01", "tags": []string{"home"},
	})
	id := idOf(task)

	// PATCH status → done sets completed_at.
	patchRec := doJSON(t, h, http.MethodPatch, "/v1/tasks/"+id, map[string]string{"status": "done"})
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", patchRec.Code, patchRec.Body.String())
	}
	var patched struct {
		Status      string  `json:"status"`
		CompletedAt *string `json:"completed_at"`
	}
	_ = json.Unmarshal(patchRec.Body.Bytes(), &patched)
	if patched.Status != "done" || patched.CompletedAt == nil {
		t.Fatalf("done should stamp completed_at: %+v", patched)
	}

	// Reopen clears completed_at.
	patchRec = doJSON(t, h, http.MethodPatch, "/v1/tasks/"+id, map[string]string{"status": "todo"})
	_ = json.Unmarshal(patchRec.Body.Bytes(), &patched)
	if patched.CompletedAt != nil {
		t.Fatalf("reopening should clear completed_at: %+v", patched)
	}

	// Filter by due_before catches it (due 2026-09-01 <= 2026-09-05).
	list := doJSON(t, h, http.MethodGet, "/v1/tasks?status=open&due_before=2026-09-05", nil)
	var lr struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(list.Body.Bytes(), &lr)
	if lr.Total != 1 {
		t.Fatalf("open+due_before total = %d, want 1", lr.Total)
	}

	// Invalid priority rejected.
	bad := doJSON(t, h, http.MethodPost, "/v1/tasks", map[string]interface{}{"title": "x", "priority": "urgent"})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad priority = %d, want 400", bad.Code)
	}

	// Delete → 404 on get.
	del := doJSON(t, h, http.MethodDelete, "/v1/tasks/"+id, nil)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", del.Code)
	}
	got := doJSON(t, h, http.MethodGet, "/v1/tasks/"+id, nil)
	if got.Code != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", got.Code)
	}
}

// ---- Habits: streak + toggle idempotence ----

func TestHabitToggleIdempotenceAndStreaks(t *testing.T) {
	h := newTestAPI(t)

	habit := createHabit(t, h, map[string]interface{}{"name": "Read", "cadence": "daily"})
	habitID := idOf(habit)

	today := time.Now().UTC().Format("2006-01-02")
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")

	// Toggle on.
	r1 := doJSON(t, h, http.MethodPost, "/v1/habits/"+habitID+"/checkoffs", map[string]string{"date": yesterday})
	if r1.Code != http.StatusOK {
		t.Fatalf("toggle1: %d %s", r1.Code, r1.Body.String())
	}
	var res1 struct {
		Done    bool `json:"done"`
		Streaks struct {
			Current   int  `json:"current"`
			Longest   int  `json:"longest"`
			DoneToday bool `json:"done_today"`
		} `json:"streaks"`
	}
	_ = json.Unmarshal(r1.Body.Bytes(), &res1)
	if !res1.Done || res1.Streaks.Current != 1 {
		t.Fatalf("after first toggle done=%v current=%d, want true/1", res1.Done, res1.Streaks.Current)
	}

	// Toggle off — back to zero state. THE idempotence gate.
	r2 := doJSON(t, h, http.MethodPost, "/v1/habits/"+habitID+"/checkoffs", map[string]string{"date": yesterday})
	_ = json.Unmarshal(r2.Body.Bytes(), &res1)
	if res1.Done || res1.Streaks.Current != 0 {
		t.Fatalf("second toggle must undo: done=%v current=%d", res1.Done, res1.Streaks.Current)
	}

	// Toggle again → done once more (state machine is stable across cycles).
	r3 := doJSON(t, h, http.MethodPost, "/v1/habits/"+habitID+"/checkoffs", map[string]string{"date": yesterday})
	_ = json.Unmarshal(r3.Body.Bytes(), &res1)
	if !res1.Done {
		t.Fatal("third toggle must re-check")
	}

	// Check today too → streak 2, done_today true.
	doJSON(t, h, http.MethodPost, "/v1/habits/"+habitID+"/checkoffs", map[string]string{"date": today})
	getRec := doJSON(t, h, http.MethodGet, "/v1/habits/"+habitID, nil)
	var habitOut struct {
		Streaks struct {
			Current   int  `json:"current"`
			Longest   int  `json:"longest"`
			DoneToday bool `json:"done_today"`
		} `json:"streaks"`
	}
	_ = json.Unmarshal(getRec.Body.Bytes(), &habitOut)
	if habitOut.Streaks.Current != 2 || !habitOut.Streaks.DoneToday || habitOut.Streaks.Longest != 2 {
		t.Fatalf("manual streak calc mismatch: %+v (want current=2 longest=2 doneToday=true)", habitOut.Streaks)
	}

	// Checkoff list endpoint.
	listRec := doJSON(t, h, http.MethodGet, "/v1/habits/"+habitID+"/checkoffs?from=2000-01-01", nil)
	var dates struct {
		Items []string `json:"items"`
	}
	_ = json.Unmarshal(listRec.Body.Bytes(), &dates)
	if len(dates.Items) != 2 {
		t.Fatalf("checkoff list = %v, want 2 dates", dates.Items)
	}
}

func TestWeeklyHabitTargetFlow(t *testing.T) {
	h := newTestAPI(t)
	habit := createHabit(t, h, map[string]interface{}{
		"name": "Gym x3", "cadence": "weekly", "target_per_week": 3,
	})
	habitID := idOf(habit)

	now := time.Now().UTC()
	thisMonday := now.AddDate(0, 0, -(int(now.Weekday()+6) % 7)) // Mon=this week start
	dates := []string{
		thisMonday.Format("2006-01-02"),
		thisMonday.AddDate(0, 0, 1).Format("2006-01-02"),
	}
	for _, d := range dates {
		doJSON(t, h, http.MethodPost, "/v1/habits/"+habitID+"/checkoffs", map[string]string{"date": d})
	}

	getRec := doJSON(t, h, http.MethodGet, "/v1/habits/"+habitID, nil)
	var habitOut struct {
		Streaks struct {
			WeekDone      int `json:"week_done"`
			TargetPerWeek int `json:"target_per_week"`
		} `json:"streaks"`
	}
	_ = json.Unmarshal(getRec.Body.Bytes(), &habitOut)
	if habitOut.Streaks.WeekDone != 2 || habitOut.Streaks.TargetPerWeek != 3 {
		t.Fatalf("weekly progress = %+v, want week_done=2 target=3", habitOut.Streaks)
	}

	// Today bundle marks weekly habit as still-due (below target).
	todayRec := doJSON(t, h, http.MethodGet, "/v1/planner/today", nil)
	var tb struct {
		Habits []struct {
			ID        string `json:"id"`
			DueToday  bool   `json:"due_today"`
			DoneToday bool   `json:"done_today"`
		} `json:"habits"`
	}
	_ = json.Unmarshal(todayRec.Body.Bytes(), &tb)
	found := false
	for _, hb := range tb.Habits {
		if hb.ID == habitID {
			found = true
			// Below weekly target → still due; DoneToday depends on whether
			// today itself was checked (true Tue–Sun in this fixture).
			if !hb.DueToday {
				t.Fatalf("weekly habit below target must be due: %+v", hb)
			}
		}
	}
	if !found {
		t.Fatal("habit missing from today bundle")
	}
}

// ---- Events: recurrence expansion ----

func TestWeeklyEventExpandsForNWeeks(t *testing.T) {
	h := newTestAPI(t)
	// Wednesdays starting 2026-08-05T09:00Z.
	createEvent(t, h, map[string]interface{}{
		"title": "Team sync", "starts_at": "2026-08-05T09:00:00Z",
		"recurrence_rule": "FREQ=WEEKLY",
	})

	list := doJSON(t, h, http.MethodGet, "/v1/events?from=2026-08-01&to=2026-09-01", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list events: %d %s", list.Code, list.Body.String())
	}
	var lr struct {
		Items []struct {
			EventID  string `json:"event_id"`
			Date     string `json:"date"`
			Series   bool   `json:"series"`
			StartsAt string `json:"starts_at"`
		} `json:"items"`
	}
	_ = json.Unmarshal(list.Body.Bytes(), &lr)

	want := []string{"2026-08-05", "2026-08-12", "2026-08-19", "2026-08-26"}
	if len(lr.Items) != len(want) {
		t.Fatalf("expansion count = %d (%v), want %d", len(lr.Items), lr.Items, len(want))
	}
	for i, w := range want {
		if lr.Items[i].Date != w {
			t.Fatalf("occurrence[%d] = %s, want %s", i, lr.Items[i].Date, w)
		}
		if !lr.Items[i].Series {
			t.Fatalf("occurrence %s not marked as series", w)
		}
		if lr.Items[i].StartsAt[:11] != w[:10]+"T" {
			t.Fatalf("occurrence starts_at %s not on day %s", lr.Items[i].StartsAt, w)
		}
	}
}

func TestEventValidationAndCountBound(t *testing.T) {
	h := newTestAPI(t)

	bad := doJSON(t, h, http.MethodPost, "/v1/events", map[string]interface{}{
		"title": "Bad rule", "starts_at": "2026-08-05T09:00:00Z", "recurrence_rule": "FREQ=YEARLY",
	})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("unknown token = %d, want 400", bad.Code)
	}

	ev := createEvent(t, h, map[string]interface{}{
		"title": "Sprint retro", "starts_at": "2026-08-01T10:00:00Z",
		"recurrence_rule": "FREQ=DAILY;INTERVAL=7;COUNT=3",
	})
	_ = ev

	list := doJSON(t, h, http.MethodGet, "/v1/events?from=2026-07-01&to=2026-12-31", nil)
	var lr struct {
		Total int `json:"-"`
		Items []struct {
			Date string `json:"date"`
		} `json:"items"`
	}
	_ = json.Unmarshal(list.Body.Bytes(), &lr)
	if len(lr.Items) != 3 {
		t.Fatalf("COUNT=3 expanded to %d, want 3", len(lr.Items))
	}
	wantDates := map[string]bool{"2026-08-01": true, "2026-08-08": true, "2026-08-15": true}
	for _, it := range lr.Items {
		if !wantDates[it.Date] {
			t.Fatalf("unexpected occurrence %s", it.Date)
		}
	}
}

// ---- Bundles: today aggregates three sources ----

func TestTodayBundleAggregatesThreeSources(t *testing.T) {
	h := newTestAPI(t)

	today := time.Now().UTC().Format("2006-01-02")
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")

	// Task due today, task overdue, task future.
	createTask(t, h, map[string]interface{}{"title": "Due today", "due_date": today})
	createTask(t, h, map[string]interface{}{"title": "Overdue", "due_date": yesterday})
	createTask(t, h, map[string]interface{}{"title": "Future", "due_date": time.Now().UTC().AddDate(0, 0, 5).Format("2006-01-02")})

	// Habit with a checkoff today.
	habit := createHabit(t, h, map[string]interface{}{"name": "Meditate"})
	doJSON(t, h, http.MethodPost, "/v1/habits/"+idOf(habit)+"/checkoffs", map[string]string{"date": today})

	// Event happening right now (today).
	createEvent(t, h, map[string]interface{}{
		"title": "Standup", "starts_at": today + "T09:00:00Z",
	})

	rec := doJSON(t, h, http.MethodGet, "/v1/planner/today", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("today: %d %s", rec.Code, rec.Body.String())
	}
	var b struct {
		Date    string `json:"date"`
		Overdue []struct {
			Title string `json:"title"`
		} `json:"overdue"`
		DueToday []struct {
			Title string `json:"title"`
		} `json:"due_today"`
		Habits []struct {
			ID        string `json:"id"`
			DoneToday bool   `json:"done_today"`
		} `json:"habits"`
		Events []struct {
			Title string `json:"title"`
			Date  string `json:"date"`
		} `json:"events"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &b)

	if b.Date != today {
		t.Fatalf("bundle date %s != %s", b.Date, today)
	}
	if len(b.DueToday) != 1 || b.DueToday[0].Title != "Due today" {
		t.Fatalf("due_today wrong: %+v", b.DueToday)
	}
	if len(b.Overdue) != 1 || b.Overdue[0].Title != "Overdue" {
		t.Fatalf("overdue wrong: %+v", b.Overdue)
	}
	if len(b.Habits) != 1 || !b.Habits[0].DoneToday || b.Habits[0].ID != idOf(habit) {
		t.Fatalf("habits wrong: %+v", b.Habits)
	}
	if len(b.Events) != 1 || b.Events[0].Title != "Standup" {
		t.Fatalf("events wrong: %+v", b.Events)
	}

	// Overview for a specific date mirrors the shape.
	ov := doJSON(t, h, http.MethodGet, "/v1/planner/overview?date="+yesterday, nil)
	if ov.Code != http.StatusOK {
		t.Fatalf("overview: %d %s", ov.Code, ov.Body.String())
	}

	// Upcoming includes future task within window.
	up := doJSON(t, h, http.MethodGet, "/v1/planner/upcoming?days=7", nil)
	if up.Code != http.StatusOK {
		t.Fatalf("upcoming: %d", up.Code)
	}
	var upb struct {
		Items []struct {
			Date  string `json:"date"`
			Tasks []struct {
				Title string `json:"title"`
			} `json:"tasks"`
		} `json:"items"`
	}
	_ = json.Unmarshal(up.Body.Bytes(), &upb)
	futureDay := time.Now().UTC().AddDate(0, 0, 5).Format("2006-01-02")
	foundFuture := false
	for _, d := range upb.Items {
		if d.Date == futureDay && len(d.Tasks) == 1 && d.Tasks[0].Title == "Future" {
			foundFuture = true
		}
	}
	if !foundFuture {
		t.Fatalf("future task missing from upcoming agenda for %s: %+v", futureDay, upb.Items)
	}
}
