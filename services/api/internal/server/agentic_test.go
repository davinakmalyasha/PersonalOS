package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// ---- Goals ----

func TestGoalsSavingsAndCalorieUpsert(t *testing.T) {
	h := newTestAPI(t)

	rec := doJSON(t, h, http.MethodPost, "/v1/goals", map[string]interface{}{
		"kind": "savings", "name": "Emergency fund", "target_minor": 50000000, "deadline": "2026-12-31",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("goal create: %s", rec.Body.String())
	}
	var g struct {
		ID         string `json:"id"`
		SavedMinor int64  `json:"saved_minor"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &g)

	add := doJSON(t, h, http.MethodPost, "/v1/goals/"+g.ID+"/add", map[string]int64{"amount_minor": 12500000})
	var added struct {
		SavedMinor int64 `json:"saved_minor"`
	}
	_ = json.Unmarshal(add.Body.Bytes(), &added)
	if added.SavedMinor != 12500000 {
		t.Fatalf("add-to-goal = %d, want 12500000", added.SavedMinor)
	}

	// Calorie goal: single-row upsert.
	r1 := doJSON(t, h, http.MethodPost, "/v1/goals", map[string]interface{}{"kind": "calorie", "name": "Cut", "target_minor": 2200})
	r2 := doJSON(t, h, http.MethodPost, "/v1/goals", map[string]interface{}{"kind": "calorie", "name": "Bulk", "target_minor": 3200})
	if r1.Code != http.StatusCreated || r2.Code != http.StatusCreated {
		t.Fatalf("calorie goals: %d/%d", r1.Code, r2.Code)
	}
	list := doJSON(t, h, http.MethodGet, "/v1/goals?kind=calorie", nil)
	var lr struct {
		Items []struct {
			Name        string `json:"name"`
			TargetMinor int64  `json:"target_minor"`
		} `json:"items"`
	}
	_ = json.Unmarshal(list.Body.Bytes(), &lr)
	if len(lr.Items) != 1 || lr.Items[0].Name != "Bulk" || lr.Items[0].TargetMinor != 3200 {
		t.Fatalf("calorie upsert failed: %+v", lr.Items)
	}
}

// ---- Recurring detection ----

func TestRecurringDetection(t *testing.T) {
	h := newTestAPI(t)
	account := mustCreateAccount(t, h)

	// Netflix: 3 monthly charges, same amount.
	for _, d := range []string{"2026-06-05", "2026-07-05", "2026-08-05"} {
		doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
			"account_id": account, "amount_minor": -186000, "date": d, "merchant": "NETFLIX",
		})
	}
	// One-off: not recurring.
	doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": account, "amount_minor": -500000, "date": "2026-08-01", "merchant": "ONEOFF",
	})

	rec := doJSON(t, h, http.MethodGet, "/v1/finance/recurring", nil)
	var lr struct {
		Items []struct {
			Merchant    string `json:"merchant"`
			Occurrences int    `json:"occurrences"`
			NextGuess   string `json:"next_guess"`
		} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &lr)
	if len(lr.Items) != 1 || lr.Items[0].Merchant != "netflix" || lr.Items[0].Occurrences != 3 {
		t.Fatalf("recurring detection wrong: %+v", lr.Items)
	}
	if lr.Items[0].NextGuess != "2026-09-04" && lr.Items[0].NextGuess != "2026-09-05" {
		t.Fatalf("next_guess implausible: %+v", lr.Items[0])
	}
}

// ---- Transfers: paired + excluded from spend ----

func TestTransferPairingExcludedFromSummary(t *testing.T) {
	h := newTestAPI(t)
	a1 := mustCreateAccount(t, h)
	rec2 := doJSON(t, h, http.MethodPost, "/v1/accounts", map[string]string{"name": "Wallet", "type": "cash"})
	var a2 struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &a2)

	// Move 1jt: out of checking, into wallet, same day.
	doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": a1, "amount_minor": -1000000, "date": "2026-08-20", "merchant": "MOVE",
	})
	doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": a2.ID, "amount_minor": 1000000, "date": "2026-08-20", "merchant": "MOVE",
	})

	sum := doJSON(t, h, http.MethodGet, "/v1/finance/summary?month=2026-08", nil)
	var s struct {
		Income  int64 `json:"income_minor"`
		Outcome int64 `json:"outcome_minor"`
	}
	_ = json.Unmarshal(sum.Body.Bytes(), &s)
	if s.Income != 0 || s.Outcome != 0 {
		t.Fatalf("transfers must not count as income/outcome: %+v", s)
	}

	// Transactions carry the flag.
	list := doJSON(t, h, http.MethodGet, "/v1/transactions?account_id="+a1, nil)
	var lr struct {
		Items []struct {
			IsTransfer bool `json:"is_transfer"`
		} `json:"items"`
	}
	_ = json.Unmarshal(list.Body.Bytes(), &lr)
	if len(lr.Items) != 1 || !lr.Items[0].IsTransfer {
		t.Fatalf("is_transfer flag missing: %+v", lr.Items)
	}
}

// ---- Recurring tasks spawn next instance ----

func TestRecurringTaskSpawnsOnComplete(t *testing.T) {
	h := newTestAPI(t)

	rec := doJSON(t, h, http.MethodPost, "/v1/tasks", map[string]interface{}{
		"title": "Take out trash", "due_date": "2026-08-25", "recurrence_rule": "FREQ=WEEKLY",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %s", rec.Body.String())
	}
	var t1 struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &t1)

	doJSON(t, h, http.MethodPatch, "/v1/tasks/"+t1.ID, map[string]string{"status": "done"})

	list := doJSON(t, h, http.MethodGet, "/v1/tasks?status=todo", nil)
	var lr struct {
		Items []struct {
			Title   string  `json:"title"`
			DueDate *string `json:"due_date"`
			Status  string  `json:"status"`
		} `json:"items"`
	}
	_ = json.Unmarshal(list.Body.Bytes(), &lr)
	if len(lr.Items) != 1 {
		t.Fatalf("expected exactly one spawned todo, got %+v", lr.Items)
	}
	if lr.Items[0].DueDate == nil || *lr.Items[0].DueDate != "2026-09-01" {
		t.Fatalf("spawned due date wrong: %+v", lr.Items[0].DueDate)
	}
	// Completed original keeps its rule + done state.
	got := doJSON(t, h, http.MethodGet, "/v1/tasks/"+t1.ID, nil)
	var done struct {
		Status         string  `json:"status"`
		RecurrenceRule *string `json:"recurrence_rule"`
	}
	_ = json.Unmarshal(got.Body.Bytes(), &done)
	if done.Status != "done" || done.RecurrenceRule == nil {
		t.Fatalf("original task altered: %+v", done)
	}
}

// ---- Water + PRs ----

func TestWaterAndPRs(t *testing.T) {
	h := newTestAPI(t)

	w1 := doJSON(t, h, http.MethodPost, "/v1/body-metrics/water", map[string]int{"ml": 500})
	var tot struct {
		WaterMl int `json:"water_ml"`
	}
	_ = json.Unmarshal(w1.Body.Bytes(), &tot)
	if tot.WaterMl != 500 {
		t.Fatalf("water 1 = %d", tot.WaterMl)
	}
	doJSON(t, h, http.MethodPost, "/v1/body-metrics/water", map[string]int{"ml": 250})
	w3 := doJSON(t, h, http.MethodPost, "/v1/body-metrics/water", map[string]int{"ml": 300})
	_ = json.Unmarshal(w3.Body.Bytes(), &tot)
	if tot.WaterMl != 1050 {
		t.Fatalf("water accumulates: %d, want 1050", tot.WaterMl)
	}

	// Workouts for PRs.
	createWorkout(t, h, "2026-08-01T18:00:00Z", "Day A", 60)
	rec := doJSON(t, h, http.MethodPost, "/v1/workouts", map[string]interface{}{
		"performed_at": "2026-08-20T18:00:00Z", "title": "Day B", "duration_minutes": 45,
		"exercises": []map[string]any{{"name": "bench press", "sets": 5, "reps": 3, "weight_kg": 92.5}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("workout B: %s", rec.Body.String())
	}

	prs := doJSON(t, h, http.MethodGet, "/v1/health/prs", nil)
	var lr struct {
		Items []struct {
			Exercise    string  `json:"exercise"`
			MaxWeightKg float64 `json:"max_weight_kg"`
		} `json:"items"`
	}
	_ = json.Unmarshal(prs.Body.Bytes(), &lr)
	found := false
	for _, it := range lr.Items {
		if strings.EqualFold(it.Exercise, "bench press") && it.MaxWeightKg == 92.5 {
			found = true
		}
	}
	if !found {
		t.Fatalf("bench PR missing: %+v", lr.Items)
	}
}

// ---- Expiring items ----

func TestExpiringItemsScan(t *testing.T) {
	h := newTestAPI(t)
	doJSON(t, h, http.MethodPost, "/v1/items", map[string]interface{}{
		"type": "warranty", "title": "Headphones",
		"data": `{"expires":"2026-09-10"}`,
	})
	doJSON(t, h, http.MethodPost, "/v1/items", map[string]interface{}{
		"type": "misc", "title": "Far future",
		"data": `{"expires":"2028-01-01"}`,
	})

	exp := doJSON(t, h, http.MethodGet, "/v1/items/expiring?days=30", nil)
	var lr struct {
		Items []struct {
			Title    string `json:"title"`
			Date     string `json:"date"`
			DaysLeft int    `json:"days_left"`
		} `json:"items"`
	}
	_ = json.Unmarshal(exp.Body.Bytes(), &lr)
	if len(lr.Items) != 1 || lr.Items[0].Title != "Headphones" {
		t.Fatalf("expiry scan wrong: %+v", lr.Items)
	}
	if lr.Items[0].Date != "2026-09-10" || lr.Items[0].DaysLeft < 14 || lr.Items[0].DaysLeft > 18 {
		t.Fatalf("date/days_left implausible: %+v", lr.Items[0])
	}
}

// ---- Habit weekdays + consistency ----

func TestHabitWeekdaysSchedule(t *testing.T) {
	h := newTestAPI(t)

	bad := doJSON(t, h, http.MethodPost, "/v1/habits", map[string]interface{}{
		"name": "Bad", "cadence": "daily", "weekdays": "11111",
	})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("invalid weekdays accepted: %d", bad.Code)
	}

	rec := doJSON(t, h, http.MethodPost, "/v1/habits", map[string]interface{}{
		"name": "Gym MWF", "cadence": "daily", "weekdays": "1010100",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %s", rec.Body.String())
	}
	var habit struct {
		Weekdays string `json:"weekdays"`
		Streaks  struct {
			Consistency30 int `json:"consistency_30"`
		} `json:"streaks"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &habit)
	if habit.Weekdays != "1010100" {
		t.Fatalf("weekdays not stored: %+v", habit)
	}
	if habit.Streaks.Consistency30 != 0 {
		t.Fatalf("fresh consistency should be 0: %d", habit.Streaks.Consistency30)
	}
}

// ---- Reading highlights ----

func TestReadingHighlights(t *testing.T) {
	h := newTestAPI(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/reading", map[string]interface{}{
		"title": "DDIA", "status": "reading",
	})
	var rd struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &rd)

	patch := doJSON(t, h, http.MethodPatch, "/v1/reading/"+rd.ID, map[string]interface{}{
		"highlights": []map[string]any{
			{"quote": "Databases are everyone's favorite subject", "at": "2026-08-25"},
		},
	})
	if patch.Code != http.StatusOK {
		t.Fatalf("patch highlights: %s", patch.Body.String())
	}
	var out struct {
		Highlights []map[string]any `json:"highlights"`
	}
	_ = json.Unmarshal(patch.Body.Bytes(), &out)
	if len(out.Highlights) != 1 {
		t.Fatalf("highlights not stored: %+v", out)
	}

	bad := doJSON(t, h, http.MethodPatch, "/v1/reading/"+rd.ID, map[string]interface{}{
		"highlights": "not an array",
	})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("non-array highlights accepted: %d", bad.Code)
	}
}
