package server

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// ---- Net worth ----

func TestNetWorthSeries(t *testing.T) {
	h := newTestAPI(t)
	a1 := mustCreateAccount(t, h)
	rec2 := doJSON(t, h, http.MethodPost, "/v1/accounts", map[string]string{"name": "Wallet", "type": "cash"})
	var a2 struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &a2)

	doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": a1, "amount_minor": 5000000, "date": "2026-08-01", "merchant": "SALARY",
	})
	doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": a1, "amount_minor": -500000, "date": "2026-08-10", "merchant": "GROCER",
	})
	doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": a2.ID, "amount_minor": 200000, "date": "2026-08-15", "merchant": "CASH",
	})

	res := doJSON(t, h, http.MethodGet, "/v1/finance/net-worth", nil)
	var nw struct {
		Points []struct {
			Date       string `json:"date"`
			TotalMinor int64  `json:"total_minor"`
		} `json:"points"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &nw)

	// Cumulative totals at each change date: 5jt → 4.5jt → 4.7jt.
	want := []struct {
		date  string
		total int64
	}{{"2026-08-01", 5000000}, {"2026-08-10", 4500000}, {"2026-08-15", 4700000}}
	if len(nw.Points) != len(want) {
		t.Fatalf("points = %+v, want %d", nw.Points, len(want))
	}
	for i, w := range want {
		if nw.Points[i].Date != w.date || nw.Points[i].TotalMinor != w.total {
			t.Fatalf("point[%d] = %+v, want %s/%d", i, nw.Points[i], w.date, w.total)
		}
	}
}

// ---- Upcoming bills ----

func TestUpcomingBills(t *testing.T) {
	h := newTestAPI(t)
	a := mustCreateAccount(t, h)

	// Spotify charged on the 10th for the last 3 months → next ~Sep 10.
	for _, d := range []string{"2026-06-10", "2026-07-10", "2026-08-10"} {
		doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
			"account_id": a, "amount_minor": -149000, "date": d, "merchant": "SPOTIFY",
		})
	}

	res := doJSON(t, h, http.MethodGet, "/v1/finance/bills?days=30", nil)
	var lr struct {
		Items []struct {
			Merchant  string `json:"merchant"`
			DaysUntil int    `json:"days_until"`
		} `json:"items"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &lr)
	if len(lr.Items) != 1 || lr.Items[0].Merchant != "spotify" {
		t.Fatalf("bills wrong: %+v", lr.Items)
	}
	if lr.Items[0].DaysUntil < 0 || lr.Items[0].DaysUntil > 30 {
		t.Fatalf("days_until out of horizon: %+v", lr.Items[0])
	}
}

// ---- Merchant aliases ----

func TestMerchantAliasAppliedOnCreate(t *testing.T) {
	h := newTestAPI(t)
	a := mustCreateAccount(t, h)

	alias := doJSON(t, h, http.MethodPost, "/v1/merchant_aliases", map[string]string{
		"pattern": "starbuck", "canonical": "Starbucks",
	})
	if alias.Code != http.StatusCreated {
		t.Fatalf("alias create: %s", alias.Body.String())
	}

	doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": a, "amount_minor": -85000, "date": "2026-08-20", "merchant": "STARBUCKS COFFEE #12",
	})

	list := doJSON(t, h, http.MethodGet, "/v1/transactions?account_id="+a, nil)
	var lr struct {
		Items []struct {
			Merchant string `json:"merchant"`
		} `json:"items"`
	}
	_ = json.Unmarshal(list.Body.Bytes(), &lr)
	if len(lr.Items) != 1 || lr.Items[0].Merchant != "Starbucks" {
		t.Fatalf("alias not applied: %+v", lr.Items)
	}
}

// ---- Budget rollover ----

func TestBudgetRollover(t *testing.T) {
	h := newTestAPI(t)
	a := mustCreateAccount(t, h)
	cat := doJSON(t, h, http.MethodPost, "/v1/categories", map[string]string{"name": "Food"})
	var c struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(cat.Body.Bytes(), &c)

	// July: budget 1jt, spent 300k → 700k carries into August (rollover on).
	doJSON(t, h, http.MethodPost, "/v1/budgets", map[string]interface{}{
		"category_id": c.ID, "month": "2026-07", "amount_minor": 1000000,
	})
	doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": a, "amount_minor": -300000, "date": "2026-07-15", "merchant": "JUL EATS",
		"category_id": c.ID,
	})
	doJSON(t, h, http.MethodPost, "/v1/budgets", map[string]interface{}{
		"category_id": c.ID, "month": "2026-08", "amount_minor": 1000000, "rollover": true,
	})

	sum := doJSON(t, h, http.MethodGet, "/v1/finance/summary?month=2026-08", nil)
	var s struct {
		Budgets []struct {
			BudgetMinor int64 `json:"budget_minor"`
			SpentMinor  int64 `json:"spent_minor"`
			Over        bool  `json:"over"`
		} `json:"budgets"`
	}
	_ = json.Unmarshal(sum.Body.Bytes(), &s)
	if len(s.Budgets) != 1 {
		t.Fatalf("budget lines: %+v", s.Budgets)
	}
	if s.Budgets[0].BudgetMinor != 1700000 {
		t.Fatalf("rollover effective budget = %d, want 1700000", s.Budgets[0].BudgetMinor)
	}

	// Non-rollover control: same setup without the flag → stays 1jt.
	doJSON(t, h, http.MethodPost, "/v1/budgets", map[string]interface{}{
		"category_id": c.ID, "month": "2026-09", "amount_minor": 1000000, "rollover": false,
	})
	sum = doJSON(t, h, http.MethodGet, "/v1/finance/summary?month=2026-09", nil)
	_ = json.Unmarshal(sum.Body.Bytes(), &s)
	if s.Budgets[0].BudgetMinor != 1000000 {
		t.Fatalf("non-rollover budget mutated: %d", s.Budgets[0].BudgetMinor)
	}
}

// ---- Subtasks + blocked ----

func TestSubtasksAndBlocked(t *testing.T) {
	h := newTestAPI(t)

	parent := doJSON(t, h, http.MethodPost, "/v1/tasks", map[string]interface{}{"title": "Move apartment"})
	var p struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(parent.Body.Bytes(), &p)

	// Subtask under parent.
	sub := doJSON(t, h, http.MethodPost, "/v1/tasks", map[string]interface{}{
		"title": "Pack boxes", "parent_id": p.ID,
	})
	if sub.Code != http.StatusCreated {
		t.Fatalf("subtask: %s", sub.Body.String())
	}
	var subT struct {
		ID       string  `json:"id"`
		ParentID *string `json:"parent_id"`
	}
	_ = json.Unmarshal(sub.Body.Bytes(), &subT)
	if subT.ParentID == nil || *subT.ParentID != p.ID {
		t.Fatalf("parent_id not stored: %+v", subT)
	}

	// Grandchild rejected.
	gc := doJSON(t, h, http.MethodPost, "/v1/tasks", map[string]interface{}{
		"title": "No grandchildren", "parent_id": subT.ID,
	})
	if gc.Code != http.StatusBadRequest {
		t.Fatalf("grandchild accepted: %d", gc.Code)
	}

	// Blocked: blocker task not done → sub shows blocked=true in today bundle.
	blocker := doJSON(t, h, http.MethodPost, "/v1/tasks", map[string]interface{}{
		"title": "Find truck",
	})
	var b struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(blocker.Body.Bytes(), &b)

	today := time.Now().UTC().Format("2006-01-02")
	doJSON(t, h, http.MethodPost, "/v1/tasks", map[string]interface{}{
		"title": "Move stuff", "due_date": today, "blocked_by": b.ID,
	})

	todayRes := doJSON(t, h, http.MethodGet, "/v1/planner/today", nil)
	var tb struct {
		DueToday []struct {
			Title   string `json:"title"`
			Blocked bool   `json:"blocked"`
		} `json:"due_today"`
		TaskLoadMinutes int64 `json:"task_load_minutes"`
	}
	_ = json.Unmarshal(todayRes.Body.Bytes(), &tb)
	found := false
	for _, task := range tb.DueToday {
		if task.Title == "Move stuff" {
			found = true
			if !task.Blocked {
				t.Fatal("task should be blocked")
			}
		}
	}
	if !found {
		t.Fatalf("blocked task missing from today: %+v", tb.DueToday)
	}
	_ = tb.TaskLoadMinutes
}

// ---- Estimates: day load ----

func TestDayLoadMinutes(t *testing.T) {
	h := newTestAPI(t)
	today := time.Now().UTC().Format("2006-01-02")

	doJSON(t, h, http.MethodPost, "/v1/tasks", map[string]interface{}{
		"title": "Deep work", "due_date": today, "estimate_minutes": 120,
	})
	createEvent(t, h, map[string]interface{}{
		"title": "Meeting", "starts_at": today + "T10:00:00Z", "ends_at": today + "T11:30:00Z",
	})

	res := doJSON(t, h, http.MethodGet, "/v1/planner/today", nil)
	var tb struct {
		TaskLoadMinutes  int64 `json:"task_load_minutes"`
		EventLoadMinutes int64 `json:"event_load_minutes"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &tb)
	if tb.TaskLoadMinutes != 120 || tb.EventLoadMinutes != 90 {
		t.Fatalf("day load wrong: %+v", tb)
	}
}

// ---- Event exceptions ----

func TestEventOverrideCancelAndEdit(t *testing.T) {
	h := newTestAPI(t)
	createEvent(t, h, map[string]interface{}{
		"title": "Team sync", "starts_at": "2026-08-05T09:00:00Z",
		"recurrence_rule": "FREQ=WEEKLY",
	})
	listEvents := func() int {
		res := doJSON(t, h, http.MethodGet, "/v1/events?from=2026-08-01&to=2026-08-31", nil)
		var lr struct {
			Items []struct {
				Date  string `json:"date"`
				Title string `json:"title"`
			} `json:"items"`
		}
		_ = json.Unmarshal(res.Body.Bytes(), &lr)
		t.Logf("occurrences: %+v", lr.Items)
		return len(lr.Items)
	}

	if n := listEvents(); n != 4 {
		t.Fatalf("baseline occurrences = %d, want 4", n)
	}

	// Find the series id via overrides endpoint? Simpler: fetch any occurrence's event id.
	res := doJSON(t, h, http.MethodGet, "/v1/events?from=2026-08-01&to=2026-08-31", nil)
	var lr struct {
		Items []struct {
			EventID string `json:"event_id"`
		} `json:"items"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &lr)
	eventID := lr.Items[0].EventID

	// Cancel the 2026-08-19 occurrence.
	ov := doJSON(t, h, http.MethodPost, "/v1/events/"+eventID+"/exceptions", map[string]interface{}{
		"date": "2026-08-19", "action": "cancel",
	})
	if ov.Code != http.StatusOK {
		t.Fatalf("cancel exception: %s", ov.Body.String())
	}
	if n := listEvents(); n != 3 {
		t.Fatalf("after cancel = %d, want 3", n)
	}

	// Edit the 2026-08-26 occurrence: new title.
	doJSON(t, h, http.MethodPost, "/v1/events/"+eventID+"/exceptions", map[string]interface{}{
		"date": "2026-08-26", "action": "edit", "title": "Retro instead",
	})
	res = doJSON(t, h, http.MethodGet, "/v1/events?from=2026-08-20&to=2026-08-31", nil)
	var titled struct {
		Items []struct {
			Date  string `json:"date"`
			Title string `json:"title"`
		} `json:"items"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &titled)
	edited := false
	for _, it := range titled.Items {
		if it.Date == "2026-08-26" {
			edited = true
			if it.Title != "Retro instead" {
				t.Fatalf("edit not applied: %+v", it)
			}
		}
	}
	if !edited {
		t.Fatal("edited occurrence disappeared")
	}
}

// ---- Habit pause ----

func TestHabitPause(t *testing.T) {
	h := newTestAPI(t)
	habit := createHabit(t, h, map[string]interface{}{"name": "Read", "cadence": "daily"})

	patch := doJSON(t, h, http.MethodPatch, "/v1/habits/"+idOf(habit), map[string]interface{}{
		"paused_until": "2026-12-31",
	})
	if patch.Code != http.StatusOK {
		t.Fatalf("pause: %s", patch.Body.String())
	}

	todayRec := doJSON(t, h, http.MethodGet, "/v1/planner/today", nil)
	var tb struct {
		Habits []struct {
			ID       string `json:"id"`
			DueToday bool   `json:"due_today"`
		} `json:"habits"`
	}
	_ = json.Unmarshal(todayRec.Body.Bytes(), &tb)
	for _, hb := range tb.Habits {
		if hb.ID == idOf(habit) && hb.DueToday {
			t.Fatal("paused habit must not be due")
		}
	}

	// Unpause → due again (empty string clears per API convention).
	doJSON(t, h, http.MethodPatch, "/v1/habits/"+idOf(habit), map[string]interface{}{
		"paused_until": "",
	})
	todayRec = doJSON(t, h, http.MethodGet, "/v1/planner/today", nil)
	_ = json.Unmarshal(todayRec.Body.Bytes(), &tb)
	for _, hb := range tb.Habits {
		if hb.ID == idOf(habit) && !hb.DueToday {
			t.Fatal("unpaused habit should be due")
		}
	}
}

// ---- Weekly review ----

func TestWeeklyReview(t *testing.T) {
	h := newTestAPI(t)
	a := mustCreateAccount(t, h)

	today := time.Now().UTC()
	// Complete two tasks this week.
	for _, title := range []string{"Review item 1", "Review item 2"} {
		rec := doJSON(t, h, http.MethodPost, "/v1/tasks", map[string]interface{}{"title": title})
		var tk struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &tk)
		doJSON(t, h, http.MethodPatch, "/v1/tasks/"+tk.ID, map[string]string{"status": "done"})
	}
	// Some spend this month.
	doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": a, "amount_minor": -250000, "date": today.Format("2006-01-02"), "merchant": "REVIEW SPEND",
	})

	res := doJSON(t, h, http.MethodGet, "/v1/planner/review", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("review: %d %s", res.Code, res.Body.String())
	}
	var rb struct {
		WeekStart string `json:"week_start"`
		WeekEnd   string `json:"week_end"`
		TasksDone []struct {
			Title string `json:"title"`
		} `json:"tasks_done"`
		SpendMinor int64  `json:"spend_minor"`
		Month      string `json:"month"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &rb)
	if len(rb.TasksDone) != 2 {
		t.Fatalf("tasks_done = %d, want 2", len(rb.TasksDone))
	}
	if rb.SpendMinor != 250000 {
		t.Fatalf("spend = %d, want 250000", rb.SpendMinor)
	}
	if rb.Month != today.Format("2006-01") {
		t.Fatalf("month = %s", rb.Month)
	}
}
