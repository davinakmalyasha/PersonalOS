package server

import (
	"errors"
	"net/http"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) mountExtras(r chi.Router) {
	// Goals (savings + calorie).
	r.Route("/goals", func(r chi.Router) {
		r.Post("/", s.handleCreateGoal)
		r.Get("/", s.handleListGoals)
		r.Get("/{id}", s.handleGetGoal)
		r.Patch("/{id}", s.handleUpdateGoal)
		r.Delete("/{id}", s.handleDeleteGoal)
		r.Post("/{id}/add", s.handleAddToGoal)
	})
	// Finance extras.
	r.Get("/finance/recurring", s.handleRecurring)
	r.Post("/finance/transfers/detect", s.handleDetectTransfers)
	// Health extras.
	r.Post("/body-metrics/water", s.handleLogWater)
	r.Get("/health/prs", s.handleExercisePRs)
	// Universal extras.
	r.Get("/items/expiring", s.handleExpiringItems)
}

// ---- Goals ----

func (s *Server) handleCreateGoal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind        string  `json:"kind"`
		Name        string  `json:"name"`
		TargetMinor *int64  `json:"target_minor"`
		Deadline    *string `json:"deadline"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	g, err := s.finance.CreateGoal(req.Kind, req.Name, req.TargetMinor, req.Deadline)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func (s *Server) handleListGoals(w http.ResponseWriter, r *http.Request) {
	items, err := s.finance.ListGoals(r.URL.Query().Get("kind"))
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid kind filter")
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleGetGoal(w http.ResponseWriter, r *http.Request) {
	g, err := s.finance.GetGoal(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleUpdateGoal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        *string  `json:"name"`
		TargetMinor **int64  `json:"target_minor"`
		SavedMinor  *int64   `json:"saved_minor"`
		Deadline    **string `json:"deadline"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	g, err := s.finance.UpdateGoal(chiURLParam(r, "id"), store.GoalUpdate{
		Name: req.Name, TargetMinor: req.TargetMinor,
		SavedMinor: req.SavedMinor, Deadline: req.Deadline,
	})
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleAddToGoal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AmountMinor int64 `json:"amount_minor"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	g, err := s.finance.AddToGoal(chiURLParam(r, "id"), req.AmountMinor)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleDeleteGoal(w http.ResponseWriter, r *http.Request) {
	if err := s.finance.DeleteGoal(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Finance extras ----

func (s *Server) handleRecurring(w http.ResponseWriter, r *http.Request) {
	items, err := s.finance.FindRecurring()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleDetectTransfers(w http.ResponseWriter, r *http.Request) {
	n, err := s.finance.PairTransfers()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"paired": n})
}

// ---- Health extras ----

func (s *Server) handleLogWater(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req struct {
		Ml  int    `json:"ml"`
		Day string `json:"day"` // optional YYYY-MM-DD, default today
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	total, err := h.LogWater(req.Day, req.Ml)
	if err != nil {
		fail(w, http.StatusBadRequest, "ml must be 1..10000; day YYYY-MM-DD")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"water_ml": total})
}

func (s *Server) handleExercisePRs(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	prs, err := h.ExercisePRs(q.Get("from"), q.Get("to"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": prs})
}

// ---- Universal extras ----

func (s *Server) handleExpiringItems(w http.ResponseWriter, r *http.Request) {
	days := 30
	if n, ok := queryInt(r, "days"); ok {
		days = int(n)
	}
	items, err := s.items.ExpiringItems(days)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}
