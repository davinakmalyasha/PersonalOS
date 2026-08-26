package server

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// ---- Exercise library ----

func TestExerciseLibrarySearch(t *testing.T) {
	h := newTestAPI(t)

	rec := doJSON(t, h, http.MethodGet, "/v1/exercises?q=squat", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("exercises: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Items []struct {
			Name        string `json:"name"`
			MuscleGroup string `json:"muscle_group"`
			Equipment   string `json:"equipment"`
		} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Items) < 3 { // Back Squat, Front Squat, Goblet Squat
		t.Fatalf("squat search too narrow: %+v", out.Items)
	}
	for _, e := range out.Items {
		if e.MuscleGroup == "" || e.Equipment == "" {
			t.Fatalf("seed rows should carry taxonomy: %+v", e)
		}
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/exercises?muscle=back", nil)
	out.Items = nil
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Items) < 3 {
		t.Fatalf("muscle filter too narrow: %+v", out.Items)
	}
}

// ---- Routines: create → list → start copies to workout ----

func TestRoutineCreateAndStart(t *testing.T) {
	h := newTestAPI(t)

	rec := doJSON(t, h, http.MethodPost, "/v1/routines", map[string]interface{}{
		"name": "Push A",
		"exercises": []map[string]interface{}{
			{"name": "Bench Press", "sets": 3, "target_reps": 8},
			{"name": "Overhead Press", "sets": 3, "target_reps": 10},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create routine: %d %s", rec.Code, rec.Body.String())
	}
	var routine struct {
		ID        string `json:"id"`
		Exercises []struct {
			Name       string `json:"name"`
			Sets       int    `json:"sets"`
			TargetReps int    `json:"target_reps"`
		} `json:"exercises"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &routine)
	if len(routine.Exercises) != 2 || routine.Exercises[0].Sets != 3 {
		t.Fatalf("routine exercises wrong: %+v", routine.Exercises)
	}

	// Start → workout created with one logged set per target set.
	now := time.Now().UTC().Format(time.RFC3339)
	rec = doJSON(t, h, http.MethodPost, "/v1/routines/"+routine.ID+"/start", map[string]interface{}{
		"performed_at": now,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("start routine: %d %s", rec.Code, rec.Body.String())
	}
	var wk struct {
		Title     string `json:"title"`
		Exercises []struct {
			Name string `json:"name"`
		} `json:"exercises"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &wk)
	if len(wk.Exercises) != 6 { // 3+3 target sets materialized
		t.Fatalf("started workout should carry 6 sets, got %d", len(wk.Exercises))
	}
	if wk.Title != "Push A (routine)" {
		t.Fatalf("workout title wrong: %q", wk.Title)
	}

	// List shows it; delete removes.
	rec = doJSON(t, h, http.MethodGet, "/v1/routines", nil)
	var lr struct {
		Items []map[string]interface{} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &lr)
	if len(lr.Items) != 1 {
		t.Fatalf("routines list: %+v", lr.Items)
	}
	rec = doJSON(t, h, http.MethodDelete, "/v1/routines/"+routine.ID, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete routine: %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/routines/"+routine.ID, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete: %d", rec.Code)
	}
}

// ---- Foods: upsert + log meal from food with servings math ----

func TestFoodsUpsertAndLogFromFood(t *testing.T) {
	h := newTestAPI(t)

	food := doJSON(t, h, http.MethodPut, "/v1/foods", map[string]interface{}{
		"name": "Chicken Rice Bowl", "serving_desc": "1 bowl",
		"calories": 600, "protein_g": 45, "carbs_g": 60, "fat_g": 12,
	})
	if food.Code != http.StatusCreated {
		t.Fatalf("upsert food: %d %s", food.Code, food.Body.String())
	}
	var f struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(food.Body.Bytes(), &f)

	// Upsert same name updates (no duplicate).
	food2 := doJSON(t, h, http.MethodPut, "/v1/foods", map[string]interface{}{
		"name": "Chicken Rice Bowl", "calories": 620,
	})
	var f2 struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(food2.Body.Bytes(), &f2)
	if f2.ID != f.ID {
		t.Fatal("same-name food should upsert in place")
	}

	// Log 1.5 servings.
	eaten := time.Now().UTC().Format(time.RFC3339)
	meal := doJSON(t, h, http.MethodPost, "/v1/foods/"+f.ID+"/log", map[string]interface{}{
		"servings": 1.5, "eaten_at": eaten, "slot": "lunch",
	})
	if meal.Code != http.StatusCreated {
		t.Fatalf("log from food: %d %s", meal.Code, meal.Body.String())
	}
	var m struct {
		Title    string   `json:"title"`
		Calories *int64   `json:"calories"`
		ProteinG *float64 `json:"protein_g"`
		Slot     *string  `json:"slot"`
	}
	_ = json.Unmarshal(meal.Body.Bytes(), &m)
	if m.Calories == nil || *m.Calories != 930 { // 620 × 1.5
		t.Fatalf("calorie scaling wrong: %+v", m)
	}
	if m.ProteinG == nil || *m.ProteinG != 67.5 {
		t.Fatalf("protein scaling wrong: %+v", m)
	}
	if m.Slot == nil || *m.Slot != "lunch" {
		t.Fatalf("slot not stored: %+v", m)
	}

	// Food search.
	rec := doJSON(t, h, http.MethodGet, "/v1/foods?q=chicken", nil)
	var lf struct {
		Items []map[string]interface{} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &lf)
	if len(lf.Items) != 1 {
		t.Fatalf("food search: %+v", lf.Items)
	}
}

// ---- Macro history series + weekly target progress ----

func TestMacroSeriesAndWeeklyTarget(t *testing.T) {
	h := newTestAPI(t)

	doJSON(t, h, http.MethodPut, "/v1/health/settings", map[string]interface{}{
		"weekly_workout_target": 4, "goal_weight_kg": 72.5,
	})

	day := func(n int) string { return time.Now().UTC().AddDate(0, 0, -n).Format(time.RFC3339) }
	doJSON(t, h, http.MethodPost, "/v1/meals", map[string]interface{}{
		"title": "day-2 food", "eaten_at": day(2), "calories": 500, "protein_g": 30,
	})
	doJSON(t, h, http.MethodPost, "/v1/meals", map[string]interface{}{
		"title": "day-1 food", "eaten_at": day(1), "calories": 700, "protein_g": 40, "fat_g": 15,
	})

	rec := doJSON(t, h, http.MethodGet, "/v1/health/macros-series?from=2020-01-01&to=2030-01-01", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("macro series: %d %s", rec.Code, rec.Body.String())
	}
	var ms struct {
		Points []struct {
			Date     string   `json:"date"`
			Calories *int64   `json:"calories"`
			ProteinG *float64 `json:"protein_g"`
		} `json:"points"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ms)
	if len(ms.Points) != 2 {
		t.Fatalf("want 2 daily buckets, got %+v", ms.Points)
	}
	if ms.Points[0].Calories == nil || *ms.Points[0].Calories != 500 {
		t.Fatalf("ascending order / totals wrong: %+v", ms.Points[0])
	}

	// Log a workout today → weekly target progress reflects 1/4 = 25%.
	doJSON(t, h, http.MethodPost, "/v1/workouts", map[string]interface{}{
		"performed_at": time.Now().UTC().Format(time.RFC3339), "title": "quick session",
	})
	rec = doJSON(t, h, http.MethodGet, "/v1/health/summary?from=2020-01-01&to=2030-01-01", nil)
	var sum struct {
		WeekDone   int `json:"week_workouts_done"`
		WeekTarget int `json:"week_workout_target"`
		WeekPct    int `json:"week_target_pct"`
		Settings   *struct {
			GoalWeightKg *float64 `json:"goal_weight_kg"`
		} `json:"settings"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sum)
	if sum.WeekDone != 1 || sum.WeekTarget != 4 || sum.WeekPct != 25 {
		t.Fatalf("weekly target progress wrong: %+v", sum)
	}
	if sum.Settings == nil || sum.Settings.GoalWeightKg == nil || *sum.Settings.GoalWeightKg != 72.5 {
		t.Fatalf("goal weight missing from settings: %+v", sum.Settings)
	}
}
