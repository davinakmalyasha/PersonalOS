package server

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// ---- Natural-language dates ----

func TestParseDateEndpoint(t *testing.T) {
	h := newTestAPI(t)
	rec := doJSON(t, h, http.MethodGet, "/v1/planner/parse-date?q=fri", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("parse-date: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Date string  `json:"date"`
		Time *string `json:"time"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Date) != 10 {
		t.Fatalf("date missing: %+v", out)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/planner/parse-date?q=tomorrow%20at%207pm", nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Time == nil || *out.Time != "19:00" {
		t.Fatalf("time part wrong: %+v", out)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/planner/parse-date?q=zzz", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unparseable should 400, got %d", rec.Code)
	}
}

// ---- Task due-time + validation ----

func TestTaskDueTimeLifecycle(t *testing.T) {
	h := newTestAPI(t)

	task := createTask(t, h, map[string]interface{}{"title": "Dentist", "due_date": "2026-09-01", "due_time": "14:30"})
	if task["due_time"] != "14:30" {
		t.Fatalf("due_time not stored: %v", task["due_time"])
	}
	id := task["id"].(string)

	// Invalid time rejected at create and update.
	rec := doJSON(t, h, http.MethodPost, "/v1/tasks", map[string]interface{}{
		"title": "bad", "due_time": "25:99",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid due_time should 400, got %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPatch, "/v1/tasks/"+id, map[string]interface{}{"due_time": "nope"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("patch invalid due_time should 400, got %d", rec.Code)
	}

	// Clear with empty string.
	rec = doJSON(t, h, http.MethodPatch, "/v1/tasks/"+id, map[string]interface{}{"due_time": ""})
	var patched map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &patched)
	if patched["due_time"] != nil {
		t.Fatalf("due_time should clear, got %v", patched["due_time"])
	}
}

// ---- Recurring series lineage + skip ----

func TestRecurringSeriesAndSkip(t *testing.T) {
	h := newTestAPI(t)

	created := createTask(t, h, map[string]interface{}{
		"title": "Water plants", "recurrence_rule": "FREQ=WEEKLY",
	})
	sid1, _ := created["series_id"].(string)
	if sid1 == "" {
		t.Fatalf("series_id should be assigned on create: %v", created["series_id"])
	}

	// Complete → spawned instance shares the series id.
	id1 := created["id"].(string)
	doJSON(t, h, http.MethodPatch, "/v1/tasks/"+id1, map[string]interface{}{"status": "done"})
	rec := doJSON(t, h, http.MethodGet, "/v1/tasks?status=open&page_size=50", nil)
	var lr struct {
		Items []map[string]interface{} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &lr)
	var next map[string]interface{}
	for _, it := range lr.Items {
		if it["title"] == "Water plants" && it["status"] == "todo" {
			next = it
		}
	}
	if next == nil {
		t.Fatal("spawned instance missing")
	}
	if next["series_id"] != sid1 {
		t.Fatalf("spawned series_id %v want %v", next["series_id"], sid1)
	}

	// Skip removes the current instance and spawns another in the same series.
	before := len(lr.Items)
	rec = doJSON(t, h, http.MethodPost, "/v1/tasks/"+next["id"].(string)+"/skip", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("skip: %d %s", rec.Code, rec.Body.String())
	}
	var skipped map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &skipped)
	if skipped["series_id"] != sid1 {
		t.Fatalf("skipped-to instance broke lineage: %v", skipped["series_id"])
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/tasks?status=open&parent_id=root&page_size=50", nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &lr)
	openTitles := 0
	for _, it := range lr.Items {
		if it["title"] == "Water plants" {
			openTitles++
		}
	}
	if openTitles != 1 || len(lr.Items) < before {
		t.Fatalf("skip should leave exactly one open instance (got %d)", openTitles)
	}

	// Non-recurring tasks cannot be skipped.
	plain := createTask(t, h, map[string]interface{}{"title": "one-off"})
	rec = doJSON(t, h, http.MethodPost, "/v1/tasks/"+plain["id"].(string)+"/skip", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("skip non-recurring should 400, got %d", rec.Code)
	}
}

// ---- Measurable habit entries ----

func TestMeasurableHabitCheckoff(t *testing.T) {
	h := newTestAPI(t)

	rec := doJSON(t, h, http.MethodPost, "/v1/habits", map[string]interface{}{
		"name": "Drink water", "cadence": "daily",
	})
	var habit struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &habit)

	today := time.Now().UTC().Format("2006-01-02")
	rec = doJSON(t, h, http.MethodPost, "/v1/habits/"+habit.ID+"/checkoffs", map[string]interface{}{
		"value": 8, "note": "hit target",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("measurable checkoff: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Done bool `json:"done"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if !out.Done {
		t.Fatal("upsert checkoff should mark done")
	}

	// Negative value rejected.
	rec = doJSON(t, h, http.MethodPost, "/v1/habits/"+habit.ID+"/checkoffs", map[string]interface{}{
		"value": -2,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("negative value should 400, got %d", rec.Code)
	}

	// Checkoff list still shows the day (presence-based streaks intact).
	rec = doJSON(t, h, http.MethodGet, "/v1/habits/"+habit.ID+"/checkoffs?from="+today+"&to="+today, nil)
	var cl struct {
		Items []string `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &cl)
	if len(cl.Items) != 1 || cl.Items[0] != today {
		t.Fatalf("checkoff date missing: %+v", cl.Items)
	}
}
