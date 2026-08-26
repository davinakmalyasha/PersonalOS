package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) mountHealth(r chi.Router) {
	r.Route("/meals", func(r chi.Router) {
		r.Post("/", s.handleCreateMeal)
		r.Get("/", s.handleListMeals)
		r.Get("/{id}", s.handleGetMeal)
		r.Patch("/{id}", s.handleUpdateMeal)
		r.Delete("/{id}", s.handleDeleteMeal)
	})
	r.Route("/recipes", func(r chi.Router) {
		r.Post("/", s.handleCreateRecipe)
		r.Get("/", s.handleListRecipes)
		r.Get("/{id}", s.handleGetRecipe)
		r.Patch("/{id}", s.handleUpdateRecipe)
		r.Delete("/{id}", s.handleDeleteRecipe)
		r.Post("/{id}/use", s.handleUseRecipe)
	})
	r.Route("/grocery", func(r chi.Router) {
		r.Get("/", s.handleListGrocery)
		r.Post("/", s.handleCreateGroceryItem)
		r.Post("/clear-checked", s.handleClearCheckedGrocery)
		r.Patch("/{id}", s.handleUpdateGroceryItem)
		r.Delete("/{id}", s.handleDeleteGroceryItem)
	})
	r.Route("/workouts", func(r chi.Router) {
		r.Post("/", s.handleCreateWorkout)
		r.Get("/", s.handleListWorkouts)
		r.Get("/{id}", s.handleGetWorkout)
		r.Patch("/{id}", s.handleUpdateWorkout)
		r.Delete("/{id}", s.handleDeleteWorkout)
	})
	r.Route("/body-metrics", func(r chi.Router) {
		r.Post("/", s.handleUpsertBodyMetric)
		r.Get("/", s.handleListBodyMetrics)
		r.Get("/{id}", s.handleGetBodyMetric)
		r.Delete("/{id}", s.handleDeleteBodyMetric)
	})
	r.Get("/health/summary", s.handleHealthSummary)
	r.Get("/health/weight-series", s.handleWeightSeries)
	r.Get("/health/volume", s.handleHealthVolume)
	r.Get("/health/settings", s.handleGetHealthSettings)
	r.Put("/health/settings", s.handleUpdateHealthSettings)
	r.Patch("/health/settings", s.handleUpdateHealthSettings)
	r.Get("/body-metrics/trends", s.handleMeasurementTrends)
	r.Get("/health/macros-series", s.handleMacroSeries)

	r.Route("/exercises", func(r chi.Router) {
		r.Get("/", s.handleListExercises)
	})
	r.Route("/routines", func(r chi.Router) {
		r.Post("/", s.handleCreateRoutine)
		r.Get("/", s.handleListRoutines)
		r.Get("/{id}", s.handleGetRoutine)
		r.Delete("/{id}", s.handleDeleteRoutine)
		r.Post("/{id}/start", s.handleStartRoutine)
	})
	r.Route("/foods", func(r chi.Router) {
		r.Put("/", s.handleUpsertFood)
		r.Get("/", s.handleListFoods)
	})
	r.Post("/foods/{id}/log", s.handleLogFromFood)
}

func (s *Server) requireHealth(w http.ResponseWriter) (*store.Health, bool) {
	if s.health == nil {
		fail(w, http.StatusServiceUnavailable, "health module unavailable")
		return nil, false
	}
	return s.health, true
}

// ---- Meals ----

type mealReq struct {
	EatenAt  string          `json:"eaten_at"`
	Title    string          `json:"title"`
	Notes    string          `json:"notes"`
	Items    json.RawMessage `json:"items"` // JSON array of {name, qty, unit}
	Calories *int64          `json:"calories"`
	ProteinG *float64        `json:"protein_g"`
	CarbsG   *float64        `json:"carbs_g"`
	FatG     *float64        `json:"fat_g"`
	Slot     string          `json:"slot"` // breakfast|lunch|dinner|snack
	Tags     []string        `json:"tags"`
}

func (s *Server) handleCreateMeal(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req mealReq
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	details := []fieldError{}
	if req.EatenAt == "" {
		details = append(details, fieldError{"eaten_at", "required RFC3339"})
	}
	if strings.TrimSpace(req.Title) == "" {
		details = append(details, fieldError{"title", "required"})
	}
	if len(details) > 0 {
		fail(w, http.StatusBadRequest, "invalid meal", details...)
		return
	}
	items := ""
	if req.Items != nil {
		items = string(req.Items)
	}
	m, err := h.CreateMeal(req.EatenAt, req.Title, req.Notes, items, req.Calories, req.ProteinG, req.CarbsG, req.FatG, req.Tags, req.Slot)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid fields",
				fieldError{"eaten_at/items/calories", "RFC3339; JSON array; >= 0"})
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleListMeals(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, _ := queryInt(r, "page")
	size, _ := queryInt(r, "page_size")
	items, total, err := h.ListMeals(store.MealFilter{
		From: q.Get("from"), To: q.Get("to"), Q: q.Get("q"),
		Page: int(page), PageSize: int(size),
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items, "total": total, "page": page, "page_size": size,
	})
}

func (s *Server) handleGetMeal(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	m, err := h.GetMeal(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, m)
}

type mealPatch struct {
	EatenAt  *string          `json:"eaten_at"`
	Title    *string          `json:"title"`
	Notes    *string          `json:"notes"`
	Items    *json.RawMessage `json:"items"`
	Calories **int64          `json:"calories"`
	ProteinG **float64        `json:"protein_g"`
	CarbsG   **float64        `json:"carbs_g"`
	FatG     **float64        `json:"fat_g"`
	Slot     **string         `json:"slot"`
	Tags     *[]string        `json:"tags"`
}

func (s *Server) handleUpdateMeal(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req mealPatch
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	var itemsStr *string
	if req.Items != nil {
		s := string(*req.Items)
		itemsStr = &s
	}
	m, err := h.UpdateMeal(chiURLParam(r, "id"), store.MealUpdate{
		EatenAt: req.EatenAt, Title: req.Title, Notes: req.Notes,
		Items: itemsStr, Calories: req.Calories,
		ProteinG: req.ProteinG, CarbsG: req.CarbsG, FatG: req.FatG,
		Slot: req.Slot, Tags: req.Tags,
	})
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid fields")
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleDeleteMeal(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	if err := h.DeleteMeal(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Recipes ----

func (s *Server) handleCreateRecipe(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req struct {
		Title              string          `json:"title"`
		Ingredients        json.RawMessage `json:"ingredients"`
		Instructions       string          `json:"instructions"`
		Servings           *int64          `json:"servings"`
		CaloriesPerServing *int64          `json:"calories_per_serving"`
		Tags               []string        `json:"tags"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	ingredients := ""
	if req.Ingredients != nil {
		ingredients = string(req.Ingredients)
	}
	rec, err := h.CreateRecipe(req.Title, ingredients, req.Instructions, req.Servings, req.CaloriesPerServing, req.Tags)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid recipe", fieldError{"title/ingredients/servings", "check values"})
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleListRecipes(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, _ := queryInt(r, "page")
	size, _ := queryInt(r, "page_size")
	items, total, err := h.ListRecipes(store.RecipeFilter{
		Tag: q.Get("tag"), Q: q.Get("q"), Page: int(page), PageSize: int(size),
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items, "total": total, "page": page, "page_size": size,
	})
}

func (s *Server) handleGetRecipe(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	rec, err := h.GetRecipe(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleUpdateRecipe(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req struct {
		Title              *string          `json:"title"`
		Ingredients        *json.RawMessage `json:"ingredients"`
		Instructions       *string          `json:"instructions"`
		Servings           **int64          `json:"servings"`
		CaloriesPerServing **int64          `json:"calories_per_serving"`
		Tags               *[]string        `json:"tags"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	var ingStr *string
	if req.Ingredients != nil {
		s := string(*req.Ingredients)
		ingStr = &s
	}
	rec, err := h.UpdateRecipe(chiURLParam(r, "id"), store.RecipeUpdate{
		Title: req.Title, Ingredients: ingStr, Instructions: req.Instructions,
		Servings: req.Servings, CaloriesPerServing: req.CaloriesPerServing, Tags: req.Tags,
	})
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid fields")
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleDeleteRecipe(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	if err := h.DeleteRecipe(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /recipes/{id}/use {eaten_at, servings?} Ã¢â€ â€™ meal copied from recipe.
func (s *Server) handleUseRecipe(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req struct {
		EatenAt  string `json:"eaten_at"`
		Servings *int64 `json:"servings"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if req.EatenAt == "" {
		fail(w, http.StatusBadRequest, "eaten_at required", fieldError{"eaten_at", "required RFC3339"})
		return
	}
	m, err := h.UseRecipeAsMeal(chiURLParam(r, "id"), req.EatenAt, req.Servings)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid eaten_at", fieldError{"eaten_at", "RFC3339 required"})
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

// ---- Grocery ----

func (s *Server) handleListGrocery(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	v := r.URL.Query().Get("checked")
	var checked *bool
	switch v {
	case "true":
		t := true
		checked = &t
	case "false":
		f := false
		checked = &f
	}
	items, err := h.ListGrocery(checked)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleCreateGroceryItem(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req struct {
		Name     string  `json:"name"`
		Qty      string  `json:"qty"`
		Unit     *string `json:"unit"`
		RecipeID *string `json:"recipe_id"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	g, err := h.CreateGroceryItem(req.Name, req.Qty, req.Unit, req.RecipeID)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func (s *Server) handleUpdateGroceryItem(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req struct {
		Name    *string  `json:"name"`
		Qty     *string  `json:"qty"`
		Unit    **string `json:"unit"`
		Checked *bool    `json:"checked"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	g, err := h.UpdateGroceryItem(chiURLParam(r, "id"), store.GroceryUpdate{
		Name: req.Name, Qty: req.Qty, Unit: req.Unit, Checked: req.Checked,
	})
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleDeleteGroceryItem(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	if err := h.DeleteGroceryItem(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Acceptance gate: empties ONLY checked=true rows.
func (s *Server) handleClearCheckedGrocery(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	n, err := h.ClearCheckedGroceries()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"removed": n})
}

// ---- Workouts ----

func (s *Server) handleCreateWorkout(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req struct {
		PerformedAt     string          `json:"performed_at"`
		Title           *string         `json:"title"`
		Notes           string          `json:"notes"`
		DurationMinutes *int64          `json:"duration_minutes"`
		Exercises       json.RawMessage `json:"exercises"`
		Tags            []string        `json:"tags"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	exercises := ""
	if req.Exercises != nil {
		exercises = string(req.Exercises)
	}
	wk, err := h.CreateWorkout(req.PerformedAt, req.Title, req.Notes, exercises, req.DurationMinutes, req.Tags)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid workout",
				fieldError{"performed_at/exercises/duration_minutes", "RFC3339; JSON array; >= 0"})
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, wk)
}

func (s *Server) handleListWorkouts(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, _ := queryInt(r, "page")
	size, _ := queryInt(r, "page_size")
	items, total, err := h.ListWorkouts(store.WorkoutFilter{
		From: q.Get("from"), To: q.Get("to"), Q: q.Get("q"),
		Page: int(page), PageSize: int(size),
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items, "total": total, "page": page, "page_size": size,
	})
}

func (s *Server) handleGetWorkout(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	wk, err := h.GetWorkout(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, wk)
}

func (s *Server) handleUpdateWorkout(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req struct {
		PerformedAt     *string          `json:"performed_at"`
		Title           **string         `json:"title"`
		Notes           *string          `json:"notes"`
		DurationMinutes **int64          `json:"duration_minutes"`
		Exercises       *json.RawMessage `json:"exercises"`
		Tags            *[]string        `json:"tags"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	var exStr *string
	if req.Exercises != nil {
		s := string(*req.Exercises)
		exStr = &s
	}
	wk, err := h.UpdateWorkout(chiURLParam(r, "id"), store.WorkoutUpdate{
		PerformedAt: req.PerformedAt, Title: req.Title, Notes: req.Notes,
		DurationMinutes: req.DurationMinutes, Exercises: exStr, Tags: req.Tags,
	})
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid fields")
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, wk)
}

func (s *Server) handleDeleteWorkout(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	if err := h.DeleteWorkout(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Body metrics ----

// POST /body-metrics upserts by calendar day Ã¢â‚¬â€ same-day re-post replaces.
func (s *Server) handleUpsertBodyMetric(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req struct {
		MeasuredAt   string          `json:"measured_at"`
		WeightKg     *float64        `json:"weight_kg"`
		BodyFatPct   *float64        `json:"body_fat_pct"`
		Notes        string          `json:"notes"`
		Measurements json.RawMessage `json:"measurements"` // free-form {key: number}
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if req.MeasuredAt == "" {
		fail(w, http.StatusBadRequest, "measured_at required", fieldError{"measured_at", "required RFC3339"})
		return
	}
	measurements := ""
	if req.Measurements != nil {
		measurements = string(req.Measurements)
	}
	m, err := h.UpsertBodyMetric(req.MeasuredAt, req.WeightKg, req.BodyFatPct, req.Notes, measurements)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid metric values",
				fieldError{"measured_at/weight_kg/body_fat_pct", "RFC3339; > 0; 0 < bf < 100"})
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleListBodyMetrics(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	items, err := h.ListBodyMetrics(q.Get("from"), q.Get("to"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleGetBodyMetric(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	m, err := h.GetBodyMetric(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleDeleteBodyMetric(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	if err := h.DeleteBodyMetric(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Summary + series ----

func (s *Server) handleHealthSummary(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	sum, err := h.Summary(q.Get("from"), q.Get("to"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

func (s *Server) handleWeightSeries(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	points, err := h.WeightSeries(q.Get("from"), q.Get("to"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	bucket := orDefault(q.Get("bucket"), "day")
	writeJSON(w, http.StatusOK, map[string]interface{}{"bucket": bucket, "points": points})
}

// ---- Phase 10c: macros, volume, trends, settings ----

func (s *Server) handleHealthVolume(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	vol, err := h.WeeklyVolume(q.Get("from"), q.Get("to"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"from": q.Get("from"), "to": q.Get("to"), "items": vol,
	})
}

func (s *Server) handleMeasurementTrends(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	trends, err := h.MeasurementTrends(q.Get("from"), q.Get("to"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"trends": trends})
}

func (s *Server) handleGetHealthSettings(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	settings, err := h.GetSettings()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleUpdateHealthSettings(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req store.HealthSettings
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	settings, err := h.UpdateSettings(store.HealthSettings{
		CalorieTarget: req.CalorieTarget, ProteinTargetG: req.ProteinTargetG,
		CarbsTargetG: req.CarbsTargetG, FatTargetG: req.FatTargetG,
		WaterTargetMl: req.WaterTargetMl, WeeklyWorkoutTarget: req.WeeklyWorkoutTarget,
		GoalWeightKg: req.GoalWeightKg,
	})
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid targets",
				fieldError{"targets", "non-negative; weekly_workout_target 1..14"})
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// ---- Phase 12c: library, routines, foods, macro history ----

func (s *Server) handleListExercises(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	limit, _ := queryInt(r, "limit")
	items, err := h.ListExercises(q.Get("q"), q.Get("muscle"), q.Get("equipment"), int(limit))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

type routineExerciseReq struct {
	Name       string `json:"name"`
	Sets       int    `json:"sets"`
	TargetReps int    `json:"target_reps"`
}

func (s *Server) handleCreateRoutine(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req struct {
		Name      string                 `json:"name"`
		Notes     string                 `json:"notes"`
		Tags      []string               `json:"tags"`
		Exercises []routineExerciseReq   `json:"exercises"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	exs := make([]store.RoutineExercise, len(req.Exercises))
	for i, e := range req.Exercises {
		exs[i] = store.RoutineExercise{Position: i, Name: e.Name, Sets: e.Sets, TargetReps: e.TargetReps}
	}
	routine, err := h.CreateRoutine(req.Name, req.Notes, req.Tags, exs)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "name required")
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, routine)
}

func (s *Server) handleListRoutines(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	items, err := h.ListRoutines(r.URL.Query().Get("q"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleGetRoutine(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	routine, err := h.GetRoutine(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, routine)
}

func (s *Server) handleDeleteRoutine(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	if err := h.DeleteRoutine(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /routines/{id}/start {performed_at?} — copy into a logged workout.
func (s *Server) handleStartRoutine(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req struct {
		PerformedAt string `json:"performed_at"`
	}
	if decodeJSON(r, &req, 0) != nil {
		// body optional for start
	}
	if req.PerformedAt == "" {
		req.PerformedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if _, err := time.Parse(time.RFC3339, req.PerformedAt); err != nil {
		fail(w, http.StatusBadRequest, "performed_at must be RFC3339")
		return
	}
	wk, err := h.StartRoutine(chiURLParam(r, "id"), req.PerformedAt)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, wk)
}

func (s *Server) handleUpsertFood(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req store.Food
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	food, err := h.UpsertFood(store.Food{Name: req.Name, ServingDesc: req.ServingDesc,
		Calories: req.Calories, ProteinG: req.ProteinG, CarbsG: req.CarbsG, FatG: req.FatG})
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid food", fieldError{"name/macros", "name required; values >= 0"})
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, food)
}

func (s *Server) handleListFoods(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	limit, _ := queryInt(r, "limit")
	items, err := h.ListFoods(q.Get("q"), int(limit))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

// POST /foods/{id}/log {servings?, eaten_at?, slot?} — meal from library food.
func (s *Server) handleLogFromFood(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req struct {
		Servings float64 `json:"servings"`
		EatenAt  string  `json:"eaten_at"`
		Slot     string  `json:"slot"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if req.Servings == 0 {
		req.Servings = 1
	}
	if req.EatenAt == "" {
		req.EatenAt = time.Now().UTC().Format(time.RFC3339)
	}
	m, err := h.LogMealFromFood(chiURLParam(r, "id"), req.Servings, req.EatenAt, req.Slot)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) || errors.Is(err, store.ErrNotFound) {
			fail(w, http.StatusBadRequest, "bad servings/eaten_at or unknown food id")
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleMacroSeries(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	points, err := h.MacroSeries(q.Get("from"), q.Get("to"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"points": points})
}
