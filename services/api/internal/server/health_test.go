package server

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func createWorkout(t *testing.T, h http.Handler, performedAt string, title string, minutes int64) map[string]interface{} {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/workouts", map[string]interface{}{
		"performed_at": performedAt, "title": title,
		"duration_minutes": minutes,
		"exercises":        []map[string]any{{"name": "squat", "sets": 3, "reps": 8, "weight_kg": 100}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create workout: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

// ---- Body metrics: same-day upsert replaces (THE gate) ----

func TestBodyMetricSameDayUpsertReplaces(t *testing.T) {
	h := newTestAPI(t)

	day := "2026-08-20"
	r1 := doJSON(t, h, http.MethodPost, "/v1/body-metrics", map[string]interface{}{
		"measured_at": day + "T07:30:00Z", "weight_kg": 80.2,
	})
	if r1.Code != http.StatusOK {
		t.Fatalf("first metric: %d %s", r1.Code, r1.Body.String())
	}

	r2 := doJSON(t, h, http.MethodPost, "/v1/body-metrics", map[string]interface{}{
		"measured_at": day + "T21:15:00Z", "weight_kg": 79.6, "body_fat_pct": 18.4,
	})
	if r2.Code != http.StatusOK {
		t.Fatalf("same-day re-post: %d %s", r2.Code, r2.Body.String())
	}
	var second struct {
		ID         string   `json:"id"`
		MeasuredAt string   `json:"measured_at"`
		WeightKg   *float64 `json:"weight_kg"`
		BodyFatPct *float64 `json:"body_fat_pct"`
	}
	_ = json.Unmarshal(r2.Body.Bytes(), &second)
	if second.WeightKg == nil || *second.WeightKg != 79.6 {
		t.Fatalf("upsert did not replace weight: %+v", second)
	}
	if second.BodyFatPct == nil || *second.BodyFatPct != 18.4 {
		t.Fatalf("upsert lost body_fat: %+v", second)
	}

	list := doJSON(t, h, http.MethodGet, "/v1/body-metrics", nil)
	var lr struct {
		Items []struct {
			ID       string   `json:"id"`
			WeightKg *float64 `json:"weight_kg"`
		} `json:"items"`
	}
	_ = json.Unmarshal(list.Body.Bytes(), &lr)
	if len(lr.Items) != 1 {
		t.Fatalf("expected exactly ONE row for the day, got %d", len(lr.Items))
	}
	if *lr.Items[0].WeightKg != 79.6 {
		t.Fatalf("kept stale row: %+v", lr.Items[0])
	}

	// Different day → new row.
	doJSON(t, h, http.MethodPost, "/v1/body-metrics", map[string]interface{}{
		"measured_at": "2026-08-21T07:00:00Z", "weight_kg": 79.4,
	})
	list = doJSON(t, h, http.MethodGet, "/v1/body-metrics", nil)
	_ = json.Unmarshal(list.Body.Bytes(), &lr)
	if len(lr.Items) != 2 {
		t.Fatalf("different day must create a row: got %d", len(lr.Items))
	}

	// Invalid values rejected.
	bad := doJSON(t, h, http.MethodPost, "/v1/body-metrics", map[string]interface{}{
		"measured_at": "2026-08-22T07:00:00Z", "weight_kg": -5,
	})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("negative weight accepted: %d", bad.Code)
	}
}

// ---- Weight series: daily bucketing ----

func TestWeightSeriesDailyBuckets(t *testing.T) {
	h := newTestAPI(t)

	for _, m := range []struct {
		at string
		kg float64
	}{
		{"2026-08-01T07:00:00Z", 81.0},
		{"2026-08-08T07:00:00Z", 80.4},
		{"2026-08-15T07:00:00Z", 79.8},
	} {
		rec := doJSON(t, h, http.MethodPost, "/v1/body-metrics", map[string]interface{}{
			"measured_at": m.at, "weight_kg": m.kg,
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("seed %s: %d %s", m.at, rec.Code, rec.Body.String())
		}
	}

	res := doJSON(t, h, http.MethodGet, "/v1/health/weight-series?from=2026-08-01&to=2026-08-31", nil)
	var ws struct {
		Bucket string `json:"bucket"`
		Points []struct {
			Date     string  `json:"date"`
			WeightKg float64 `json:"weight_kg"`
		} `json:"points"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &ws)

	want := []struct {
		date string
		kg   float64
	}{{"2026-08-01", 81.0}, {"2026-08-08", 80.4}, {"2026-08-15", 79.8}}
	if len(ws.Points) != len(want) {
		t.Fatalf("points = %+v, want %d daily buckets", ws.Points, len(want))
	}
	for i, w := range want {
		if ws.Points[i].Date != w.date || ws.Points[i].WeightKg != w.kg {
			t.Fatalf("point[%d] = %+v, want %s/%v (ascending, one per day)", i, ws.Points[i], w.date, w.kg)
		}
	}
}

// ---- Grocery: toggle + clear-checked only removes checked=true ----

func TestGroceryToggleAndClearCheckedOnly(t *testing.T) {
	h := newTestAPI(t)

	add := func(name string) string {
		rec := doJSON(t, h, http.MethodPost, "/v1/grocery", map[string]string{"name": name})
		if rec.Code != http.StatusCreated {
			t.Fatalf("grocery create %s: %s", name, rec.Body.String())
		}
		var g struct {
			ID      string `json:"id"`
			Checked bool   `json:"checked"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &g)
		if g.Checked {
			t.Fatalf("%s created checked", name)
		}
		return g.ID
	}

	a := add("oat milk")
	b := add("coffee beans")
	c := add("eggs")

	// Toggle a + c on.
	for _, id := range []string{a, c} {
		patch := doJSON(t, h, http.MethodPatch, "/v1/grocery/"+id, map[string]bool{"checked": true})
		var g struct {
			Checked bool `json:"checked"`
		}
		_ = json.Unmarshal(patch.Body.Bytes(), &g)
		if !g.Checked {
			t.Fatalf("toggle on failed for %s", id)
		}
	}

	clear := doJSON(t, h, http.MethodPost, "/v1/grocery/clear-checked", nil)
	if clear.Code != http.StatusOK {
		t.Fatalf("clear-checked: %d %s", clear.Code, clear.Body.String())
	}
	var res struct {
		Removed int64 `json:"removed"`
	}
	_ = json.Unmarshal(clear.Body.Bytes(), &res)
	if res.Removed != 2 {
		t.Fatalf("removed = %d, want exactly the 2 checked items", res.Removed)
	}

	remaining := doJSON(t, h, http.MethodGet, "/v1/grocery", nil)
	var lr struct {
		Items []struct {
			ID      string `json:"id"`
			Checked bool   `json:"checked"`
		} `json:"items"`
	}
	_ = json.Unmarshal(remaining.Body.Bytes(), &lr)
	if len(lr.Items) != 1 || lr.Items[0].ID != b || lr.Items[0].Checked {
		t.Fatalf("unchecked item must survive clear-checked: %+v", lr.Items)
	}

	// Filter endpoint.
	onlyUnchecked := doJSON(t, h, http.MethodGet, "/v1/grocery?checked=false", nil)
	_ = json.Unmarshal(onlyUnchecked.Body.Bytes(), &lr)
	if len(lr.Items) != 1 {
		t.Fatalf("?checked=false filter wrong: %+v", lr.Items)
	}
}

// ---- Recipe → meal copy ----

func TestRecipeUseCopiesToMeal(t *testing.T) {
	h := newTestAPI(t)

	rec := doJSON(t, h, http.MethodPost, "/v1/recipes", map[string]interface{}{
		"title": "Overnight oats",
		"ingredients": []map[string]any{
			{"name": "oats", "qty": "60", "unit": "g"},
			{"name": "milk", "qty": "200", "unit": "ml"},
		},
		"instructions":         "Mix, refrigerate overnight.",
		"servings":             1,
		"calories_per_serving": 350,
		"tags":                 []string{"breakfast"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("recipe create: %s", rec.Body.String())
	}
	var recipe struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &recipe)

	use := doJSON(t, h, http.MethodPost, "/v1/recipes/"+recipe.ID+"/use", map[string]interface{}{
		"eaten_at": "2026-08-25T08:00:00Z",
	})
	if use.Code != http.StatusCreated {
		t.Fatalf("recipe use: %d %s", use.Code, use.Body.String())
	}
	var meal struct {
		Title    string           `json:"title"`
		Items    []map[string]any `json:"items"`
		Calories *int64           `json:"calories"`
	}
	_ = json.Unmarshal(use.Body.Bytes(), &meal)

	if meal.Calories == nil || *meal.Calories != 350 {
		t.Fatalf("calories not copied per serving: %+v", meal)
	}
	if len(meal.Items) != 2 || meal.Items[0]["name"] != "oats" {
		t.Fatalf("ingredients not copied to meal items: %+v", meal.Items)
	}
	if meal.Title == "" {
		t.Fatal("meal title missing")
	}
}

// ---- Summary aggregates three sources ----

func TestHealthSummaryRollup(t *testing.T) {
	h := newTestAPI(t)
	now := time.Now().UTC().Format("2006-01-02")

	createWorkout(t, h, now+"T18:00:00Z", "Push day", 55)
	createWorkout(t, h, now+"T19:00:00Z", "Run", 30)

	doJSON(t, h, http.MethodPost, "/v1/meals", map[string]interface{}{
		"eaten_at": now + "T12:00:00Z", "title": "Lunch", "calories": 700,
	})

	sum := doJSON(t, h, http.MethodGet, "/v1/health/summary?from="+now+"&to="+now, nil)
	if sum.Code != http.StatusOK {
		t.Fatalf("summary: %d %s", sum.Code, sum.Body.String())
	}
	var s struct {
		Workouts struct {
			Count        int   `json:"count"`
			TotalMinutes int64 `json:"total_minutes"`
		} `json:"workouts"`
		Meals struct {
			Count         int    `json:"count"`
			CaloriesTotal *int64 `json:"calories_total"`
		} `json:"meals"`
	}
	_ = json.Unmarshal(sum.Body.Bytes(), &s)
	if s.Workouts.Count != 2 || s.Workouts.TotalMinutes != 85 {
		t.Fatalf("workout rollup wrong: %+v", s.Workouts)
	}
	if s.Meals.Count != 1 || s.Meals.CaloriesTotal == nil || *s.Meals.CaloriesTotal != 700 {
		t.Fatalf("meal rollup wrong: %+v", s.Meals)
	}
}
