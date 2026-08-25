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

// Phase 10b: memory & findability — activity feed, search v2, saved searches,
// daily note, resurface, full export.

func (s *Server) handleActivityFeed(w http.ResponseWriter, r *http.Request) {
	limit, _ := queryInt(r, "limit")
	feed, err := s.activity.Feed(int(limit), r.URL.Query().Get("entity"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": feed})
}

// handleGlobalSearch is search v2: ranked items plus typed hits from tasks,
// meals, workouts and transactions — one "find anything" call.
func (s *Server) handleGlobalSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := queryInt(r, "limit")
	items, err := s.items.SearchItems(q.Get("q"), nil, q.Get("type"), q.Get("tag"), int(limit), false)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	hits, err := store.GlobalSearch(s.db, q.Get("q"), int(limit))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items, "hits": hits, "q": q.Get("q")})
}

// ---- Saved searches ----

func (s *Server) savedSearches() *store.SavedSearchStore {
	if s.db == nil {
		return nil
	}
	return &store.SavedSearchStore{DB: s.db}
}

func (s *Server) mountSavedSearches(r chi.Router) {
	r.Route("/saved_searches", func(r chi.Router) {
		r.Post("/", s.handleCreateSavedSearch)
		r.Get("/", s.handleListSavedSearches)
		r.Get("/{id}", s.handleGetSavedSearch)
		r.Patch("/{id}", s.handleUpdateSavedSearch)
		r.Delete("/{id}", s.handleDeleteSavedSearch)
		r.Post("/{id}/run", s.handleRunSavedSearch)
	})
}

func (s *Server) handleCreateSavedSearch(w http.ResponseWriter, r *http.Request) {
	ss := s.savedSearches()
	var req struct {
		Name  string          `json:"name"`
		Query json.RawMessage `json:"query"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	out, err := ss.Create(req.Name, req.Query)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleListSavedSearches(w http.ResponseWriter, r *http.Request) {
	items, err := s.savedSearches().List()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleGetSavedSearch(w http.ResponseWriter, r *http.Request) {
	ss, err := s.savedSearches().Get(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, ss)
}

func (s *Server) handleUpdateSavedSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string          `json:"name"`
		Query json.RawMessage `json:"query"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	ss, err := s.savedSearches().Update(chiURLParam(r, "id"), req.Name, req.Query)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, ss)
}

func (s *Server) handleDeleteSavedSearch(w http.ResponseWriter, r *http.Request) {
	if err := s.savedSearches().Delete(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleRunSavedSearch executes the stored query via search v2. The stored
// query object accepts {q,type,tag,limit}.
func (s *Server) handleRunSavedSearch(w http.ResponseWriter, r *http.Request) {
	ss, err := s.savedSearches().Get(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	var params struct {
		Q     string `json:"q"`
		Type  string `json:"type"`
		Tag   string `json:"tag"`
		Limit int    `json:"limit"`
	}
	if len(ss.Query) > 0 {
		if err := json.Unmarshal(ss.Query, &params); err != nil {
			fail(w, http.StatusBadRequest, "saved query must be a {q,type,tag,limit} object",
				fieldError{"query", err.Error()})
			return
		}
	}
	items, err := s.items.SearchItems(params.Q, nil, params.Type, params.Tag, params.Limit, false)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	hits, err := store.GlobalSearch(s.db, params.Q, params.Limit)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"search": ss, "items": items, "hits": hits,
	})
}

// ---- Daily note + resurface ----

func (s *Server) handleDailyNote(w http.ResponseWriter, r *http.Request) {
	day := r.URL.Query().Get("date")
	if day == "" {
		day = time.Now().UTC().Format("2006-01-02")
	}
	note, created, err := s.knowledge.DailyNote(day)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid date", fieldError{"date", "YYYY-MM-DD"})
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"note": note, "created": created, "date": day})
}

func (s *Server) handleAppendDailyNote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
		Date string `json:"date"` // optional YYYY-MM-DD, default today
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		fail(w, http.StatusBadRequest, "text required", fieldError{"text", "required"})
		return
	}
	if req.Date == "" {
		req.Date = time.Now().UTC().Format("2006-01-02")
	}
	note, err := s.knowledge.AppendDailyNote(req.Date, req.Text)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid input",
				fieldError{"date/text", "YYYY-MM-DD; non-empty text"})
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (s *Server) handleResurface(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	limit, _ := queryInt(r, "limit")
	items, err := store.Resurface(s.db, date, int(limit))
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid date", fieldError{"date", "YYYY-MM-DD"})
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items, "date": date})
}

// ---- Export ----

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	dump, err := store.ExportAll(s.db)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="personal-os-export.json"`)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"exported_at": time.Now().UTC().Format(time.RFC3339),
		"version":     1,
		"data":        dump,
	})
}
