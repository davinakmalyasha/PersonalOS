package server

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// ---- Settings roundtrip + validation ----

func TestHealthSettingsRoundtripAndMerge(t *testing.T) {
	h := newTestAPI(t)

	// Defaults before first PUT.
	rec := doJSON(t, h, http.MethodGet, "/v1/health/settings", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get settings: %d %s", rec.Code, rec.Body.String())
	}
	var s map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &s)
	if s["calorie_target"] != nil || s["weekly_workout_target"] != nil {
		t.Fatalf("expected nil defaults, got %v", s)
	}

	rec = doJSON(t, h, http.MethodPut, "/v1/health/settings", map[string]interface{}{
		"calorie_target":        2200,
		"protein_target_g":      130,
		"weekly_workout_target": 4,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("put settings: %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &s)
	if s["calorie_target"].(float64) != 2200 || s["protein_target_g"].(float64) != 130 {
		t.Fatalf("targets not stored: %v", s)
	}

	// Merge semantics: changing one field keeps the others.
	rec = doJSON(t, h, http.MethodPut, "/v1/health/settings", map[string]interface{}{
		"water_target_ml": 3200,
	})
	_ = json.Unmarshal(rec.Body.Bytes(), &s)
	if s["calorie_target"].(float64) != 2200 || s["water_target_ml"].(float64) != 3200 {
		t.Fatalf("merge broke stored targets: %v", s)
	}

	// Validation.
	for _, bad := range []map[string]interface{}{
		{"weekly_workout_target": 20},
		{"calorie_target": -5},
	} {
		rec = doJSON(t, h, http.MethodPut, "/v1/health/settings", bad)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("bad target %+v should 400, got %d", bad, rec.Code)
		}
	}
}

// ---- Meal macros feed the summary rings ----

func TestMealMacrosInSummaryRings(t *testing.T) {
	h := newTestAPI(t)
	_ = doJSON(t, h, http.MethodPut, "/v1/health/settings", map[string]interface{}{
		"calorie_target": 2000, "protein_target_g": 120, "carbs_target_g": 200, "fat_target_g": 65,
	})
	now := time.Now().UTC().Format(time.RFC3339)
	for _, m := range []map[string]interface{}{
		{"eaten_at": now, "title": "breakfast", "calories": 500, "protein_g": 30.5, "carbs_g": 60, "fat_g": 15},
		{"eaten_at": now, "title": "lunch", "calories": 700, "protein_g": 40, "carbs_g": 70, "fat_g": 20},
	} {
		rec := doJSON(t, h, http.MethodPost, "/v1/meals", m)
		if rec.Code != http.StatusCreated {
			t.Fatalf("meal: %d %s", rec.Code, rec.Body.String())
		}
	}

	rec := doJSON(t, h, http.MethodGet, "/v1/health/summary", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("summary: %d %s", rec.Code, rec.Body.String())
	}
	var sum struct {
		Macros struct {
			Calories *float64 `json:"calories"`
			ProteinG *float64 `json:"protein_g"`
			CarbsG   *float64 `json:"carbs_g"`
			FatG     *float64 `json:"fat_g"`
		} `json:"macros"`
		Settings *struct {
			ProteinTargetG *int64 `json:"protein_target_g"`
		} `json:"settings"`
		Meals struct {
			CaloriesTotal *int64 `json:"calories_total"`
		} `json:"meals"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sum)
	if sum.Macros.Calories == nil || *sum.Macros.Calories != 1200 {
		t.Fatalf("macro calories total wrong: %+v", sum.Macros)
	}
	if sum.Macros.ProteinG == nil || *sum.Macros.ProteinG != 70.5 {
		t.Fatalf("protein total wrong: %+v", sum.Macros)
	}
	if sum.Macros.CarbsG == nil || *sum.Macros.CarbsG != 130 {
		t.Fatalf("carbs total wrong: %+v", sum.Macros)
	}
	if sum.Settings == nil || sum.Settings.ProteinTargetG == nil || *sum.Settings.ProteinTargetG != 120 {
		t.Fatalf("settings targets missing from summary: %+v", sum.Settings)
	}
}

// ---- Weekly tonnage ----

func TestWeeklyVolumeAggregation(t *testing.T) {
	h := newTestAPI(t)
	day := func(offset int) string {
		return time.Now().UTC().AddDate(0, 0, offset).Format(time.RFC3339)
	}
	workouts := []map[string]interface{}{
		{
			"performed_at": day(-2), "title": "push A",
			"exercises": []map[string]interface{}{
				{"name": "Bench Press", "weight_kg": 60, "reps": 8},
				{"name": "Bench Press", "weight_kg": 70, "reps": 5},
			},
		},
		{
			"performed_at": day(-1), "title": "push B",
			"exercises": []map[string]interface{}{
				{"name": "bench press", "weight_kg": 65, "reps": 8}, // same exercise, case-insensitive
				{"name": "Overhead Press", "weight_kg": 40, "reps": 8},
			},
		},
	}
	for _, wk := range workouts {
		rec := doJSON(t, h, http.MethodPost, "/v1/workouts", wk)
		if rec.Code != http.StatusCreated {
			t.Fatalf("workout: %d %s", rec.Code, rec.Body.String())
		}
	}

	rec := doJSON(t, h, http.MethodGet, "/v1/health/volume", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("volume: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Items []struct {
			Exercise    string  `json:"exercise"`
			Sets        int     `json:"sets"`
			RepsTotal   int     `json:"reps_total"`
			VolumeKg    float64 `json:"volume_kg"`
			MaxWeightKg float64 `json:"max_weight_kg"`
		} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Items) != 2 {
		t.Fatalf("expected 2 aggregated exercises, got %+v", out.Items)
	}
	bench := out.Items[0] // sorted by volume desc
	if bench.Exercise != "Bench Press" || bench.Sets != 3 || bench.RepsTotal != 21 {
		t.Fatalf("bench aggregation wrong: %+v", bench)
	}
	want := 60.0*8 + 70.0*5 + 65.0*8
	if bench.VolumeKg != want {
		t.Fatalf("volume want %v got %v", want, bench.VolumeKg)
	}
	if bench.MaxWeightKg != 70 {
		t.Fatalf("max weight want 70 got %v", bench.MaxWeightKg)
	}
}

// ---- Measurement trends ----

func TestMeasurementTrendsSeries(t *testing.T) {
	h := newTestAPI(t)
	post := func(day string, measurements string) {
		rec := doJSON(t, h, http.MethodPost, "/v1/body-metrics", map[string]interface{}{
			"measured_at": day + "T12:00:00Z", "measurements": json.RawMessage(measurements),
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("body-metric %s: %d %s", day, rec.Code, rec.Body.String())
		}
	}
	post("2026-08-01", `{"chest_cm":100.5}`)
	post("2026-08-02", `{"chest_cm":100.0,"waist_cm":82}`)
	post("2026-08-03", `{"waist_cm":81.5}`)

	rec := doJSON(t, h, http.MethodGet, "/v1/body-metrics/trends?from=2026-08-01&to=2026-08-31", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("trends: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Trends map[string][]struct {
			Date  string  `json:"date"`
			Value float64 `json:"value"`
		} `json:"trends"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	chest := out.Trends["chest_cm"]
	waist := out.Trends["waist_cm"]
	if len(chest) != 2 || chest[0].Value != 100.5 || chest[1].Value != 100.0 || chest[1].Date != "2026-08-02" {
		t.Fatalf("chest series wrong: %+v", chest)
	}
	if len(waist) != 2 || waist[1].Value != 81.5 {
		t.Fatalf("waist series wrong: %+v", waist)
	}
}
