package server

import (
	"errors"
	"net/http"
	"time"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) mountPlanner(r chi.Router) {
	r.Route("/tasks", func(r chi.Router) {
		r.Post("/", s.handleCreateTask)
		r.Get("/", s.handleListTasks)
		r.Get("/{id}", s.handleGetTask)
		r.Patch("/{id}", s.handleUpdateTask)
		r.Delete("/{id}", s.handleDeleteTask)
	})
	r.Route("/habits", func(r chi.Router) {
		r.Post("/", s.handleCreateHabit)
		r.Get("/", s.handleListHabits)
		r.Get("/{id}", s.handleGetHabit)
		r.Patch("/{id}", s.handleUpdateHabit)
		r.Delete("/{id}", s.handleDeleteHabit)
		r.Post("/{id}/checkoffs", s.handleToggleCheckoff)
		r.Get("/{id}/checkoffs", s.handleListCheckoffs)
	})
	r.Route("/events", func(r chi.Router) {
		r.Post("/", s.handleCreateEvent)
		r.Get("/", s.handleListEvents)
		r.Get("/{id}", s.handleGetEvent)
		r.Patch("/{id}", s.handleUpdateEvent)
		r.Delete("/{id}", s.handleDeleteEvent)
	})
	r.Get("/planner/today", s.handlePlannerToday)
	r.Get("/planner/upcoming", s.handlePlannerUpcoming)
	r.Get("/planner/overview", s.handlePlannerOverview)
}

func (s *Server) requirePlanner(w http.ResponseWriter) (*store.Planner, bool) {
	if s.planner == nil {
		fail(w, http.StatusServiceUnavailable, "planner unavailable")
		return nil, false
	}
	return s.planner, true
}

// ---- Tasks ----

type taskReq struct {
	Title          string   `json:"title"`
	Notes          string   `json:"notes"`
	Status         string   `json:"status"`
	Priority       string   `json:"priority"`
	DueDate        *string  `json:"due_date"`
	Project        *string  `json:"project"`
	RecurrenceRule *string  `json:"recurrence_rule"`
	Tags           []string `json:"tags"`
}

var taskStatuses = map[string]bool{"todo": true, "doing": true, "done": true}
var taskPriorities = map[string]bool{"low": true, "med": true, "high": true}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	var req taskReq
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	details := []fieldError{}
	if req.Title == "" {
		details = append(details, fieldError{"title", "required"})
	}
	if req.Status != "" && !taskStatuses[req.Status] {
		details = append(details, fieldError{"status", "one of todo|doing|done"})
	}
	if req.Priority != "" && !taskPriorities[req.Priority] {
		details = append(details, fieldError{"priority", "one of low|med|high"})
	}
	if req.DueDate != nil && *req.DueDate != "" {
		if err := validDate(*req.DueDate); err != nil {
			details = append(details, fieldError{"due_date", err.Error()})
		}
	}
	if len(details) > 0 {
		fail(w, http.StatusBadRequest, "invalid task", details...)
		return
	}
	t, err := p.CreateTask(req.Title, req.Notes, req.Status, req.Priority, req.DueDate, req.Project, req.Tags, req.RecurrenceRule)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func validDate(s string) error {
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return errors.New("must be YYYY-MM-DD")
	}
	return nil
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, _ := queryInt(r, "page")
	size, _ := queryInt(r, "page_size")
	fl := store.TaskFilter{
		Status:    q.Get("status"),
		Priority:  q.Get("priority"),
		Due:       q.Get("due"),
		DueBefore: q.Get("due_before"),
		Project:   q.Get("project"),
		Tag:       q.Get("tag"),
		Q:         q.Get("q"),
		Page:      int(page),
		PageSize:  int(size),
	}
	if fl.Status != "" && fl.Status != "open" && !taskStatuses[fl.Status] {
		fail(w, http.StatusBadRequest, "invalid status filter", fieldError{"status", "one of open|todo|doing|done"})
		return
	}
	items, total, err := p.ListTasks(fl)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid filter")
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items, "total": total, "page": fl.Page, "page_size": fl.PageSize,
	})
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	t, err := p.GetTask(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, t)
}

type taskPatch struct {
	Title          *string   `json:"title"`
	Notes          *string   `json:"notes"`
	Status         *string   `json:"status"`
	Priority       *string   `json:"priority"`
	DueDate        *string   `json:"due_date"`
	Project        **string  `json:"project"`
	RecurrenceRule **string  `json:"recurrence_rule"`
	Tags           *[]string `json:"tags"`
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	var req taskPatch
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if req.Status != nil && !taskStatuses[*req.Status] {
		fail(w, http.StatusBadRequest, "invalid status", fieldError{"status", "one of todo|doing|done"})
		return
	}
	if req.Priority != nil && !taskPriorities[*req.Priority] {
		fail(w, http.StatusBadRequest, "invalid priority", fieldError{"priority", "one of low|med|high"})
		return
	}
	if req.DueDate != nil && *req.DueDate != "" {
		if err := validDate(*req.DueDate); err != nil {
			fail(w, http.StatusBadRequest, "invalid due_date", fieldError{"due_date", err.Error()})
			return
		}
	}
	u := store.TaskUpdate{
		Title: req.Title, Notes: req.Notes, Status: req.Status,
		Priority: req.Priority, DueDate: req.DueDate, Tags: req.Tags,
	}
	if req.Project != nil {
		u.Project = *req.Project
	}
	if req.RecurrenceRule != nil {
		u.RecurrenceRule = *req.RecurrenceRule
	}
	t, err := p.UpdateTask(chiURLParam(r, "id"), u)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	if err := p.DeleteTask(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Habits ----

type habitReq struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Cadence       string  `json:"cadence"`
	TargetPerWeek *int    `json:"target_per_week"`
	Weekdays      *string `json:"weekdays"`
	Color         *string `json:"color"`
}

func (s *Server) handleCreateHabit(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	var req habitReq
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if req.Name == "" {
		fail(w, http.StatusBadRequest, "name required", fieldError{"name", "required"})
		return
	}
	target := 0
	switch req.Cadence {
	case "", "daily":
		target = 7
	case "weekly":
		target = 3
	default:
		fail(w, http.StatusBadRequest, "invalid cadence", fieldError{"cadence", "one of daily|weekly"})
		return
	}
	if req.TargetPerWeek != nil {
		target = *req.TargetPerWeek
	}
	h, err := p.CreateHabit(req.Name, req.Description, req.Cadence, target, req.Weekdays, req.Color)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, h)
}

func (s *Server) handleListHabits(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	includeArchived := r.URL.Query().Get("archived") == "all"
	items, err := p.ListHabits(includeArchived)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleGetHabit(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	h, err := p.GetHabit(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, h)
}

type habitPatch struct {
	Name          *string  `json:"name"`
	Description   *string  `json:"description"`
	Cadence       *string  `json:"cadence"`
	TargetPerWeek *int     `json:"target_per_week"`
	Weekdays      *string  `json:"weekdays"`
	Color         **string `json:"color"`
	Archived      *bool    `json:"archived"`
}

func (s *Server) handleUpdateHabit(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	var req habitPatch
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if req.Cadence != nil && *req.Cadence != "daily" && *req.Cadence != "weekly" {
		fail(w, http.StatusBadRequest, "invalid cadence", fieldError{"cadence", "one of daily|weekly"})
		return
	}
	if req.TargetPerWeek != nil && (*req.TargetPerWeek < 1 || *req.TargetPerWeek > 7) {
		fail(w, http.StatusBadRequest, "invalid target_per_week", fieldError{"target_per_week", "1..7"})
		return
	}
	h, err := p.UpdateHabit(chiURLParam(r, "id"), store.HabitUpdate{
		Name: req.Name, Description: req.Description, Cadence: req.Cadence,
		TargetPerWeek: req.TargetPerWeek, Weekdays: req.Weekdays,
		Color: req.Color, Archived: req.Archived,
	})
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, h)
}

func (s *Server) handleDeleteHabit(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	if err := p.DeleteHabit(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Checkoffs ----

func (s *Server) handleToggleCheckoff(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	var req struct {
		Date string `json:"date"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if req.Date == "" {
		req.Date = time.Now().UTC().Format("2006-01-02")
	} else if err := validDate(req.Date); err != nil {
		fail(w, http.StatusBadRequest, "invalid date", fieldError{"date", err.Error()})
		return
	}
	done, err := p.ToggleCheckoff(chiURLParam(r, "id"), req.Date)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	resp := map[string]interface{}{
		"habit_id": chiURLParam(r, "id"),
		"date":     req.Date,
		"done":     done,
	}
	if h, gerr := p.GetHabit(chiURLParam(r, "id")); gerr == nil {
		resp["streaks"] = h.Streaks
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListCheckoffs(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	dates, err := p.CheckoffsBetween(chiURLParam(r, "id"), q.Get("from"), q.Get("to"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": dates})
}

// ---- Events ----

type eventReq struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	StartsAt    string   `json:"starts_at"`
	EndsAt      *string  `json:"ends_at"`
	Location    string   `json:"location"`
	Recurrence  *string  `json:"recurrence_rule"`
	Tags        []string `json:"tags"`
}

func (s *Server) handleCreateEvent(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	var req eventReq
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	details := []fieldError{}
	if req.Title == "" {
		details = append(details, fieldError{"title", "required"})
	}
	if req.StartsAt == "" {
		details = append(details, fieldError{"starts_at", "required RFC3339"})
	}
	if len(details) > 0 {
		fail(w, http.StatusBadRequest, "invalid event", details...)
		return
	}
	e, err := p.CreateEvent(req.Title, req.Description, req.StartsAt, req.EndsAt, req.Location, req.Recurrence, req.Tags)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			badField := "starts_at"
			if req.StartsAt != "" {
				badField = "recurrence_rule"
				fail(w, http.StatusBadRequest, "invalid recurrence_rule",
					fieldError{badField, "FREQ=DAILY|WEEKLY|MONTHLY;INTERVAL=n;COUNT=n|UNTIL=YYYYMMDD"})
			} else {
				fail(w, http.StatusBadRequest, "invalid starts_at", fieldError{badField, "RFC3339 required"})
			}
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	from := q.Get("from") // date or datetime; normalize below
	to := q.Get("to")
	from = normalizeWindowBound(from, false)
	to = normalizeWindowBound(to, true)
	if from == "" || to == "" {
		now := time.Now().UTC()
		if from == "" {
			from = now.AddDate(0, 0, -7).Format("2006-01-02")
		}
		if to == "" {
			to = now.AddDate(0, 0, 30).Format("2006-01-02")
		}
	}
	items, err := p.OccurrencesBetween(from, to)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid from/to", fieldError{"from,to", "YYYY-MM-DD or RFC3339"})
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

// normalizeWindowBound accepts YYYY-MM-DD or RFC3339 and returns YYYY-MM-DD.
// For `to`, a bare date stays inclusive as-is.
func normalizeWindowBound(v string, isTo bool) string {
	if v == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.UTC().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", v); err == nil {
		return v
	}
	return "\x00invalid" // forces ErrInvalid downstream
}

func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	e, err := p.GetEvent(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, e)
}

type eventPatch struct {
	Title       *string   `json:"title"`
	Description *string   `json:"description"`
	StartsAt    *string   `json:"starts_at"`
	EndsAt      **string  `json:"ends_at"`
	Location    *string   `json:"location"`
	Recurrence  **string  `json:"recurrence_rule"`
	Tags        *[]string `json:"tags"`
}

func (s *Server) handleUpdateEvent(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	var req eventPatch
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	e, err := p.UpdateEvent(chiURLParam(r, "id"), store.EventUpdate{
		Title: req.Title, Description: req.Description, StartsAt: req.StartsAt,
		EndsAt: req.EndsAt, Location: req.Location, Recurrence: req.Recurrence, Tags: req.Tags,
	})
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid event fields",
				fieldError{"starts_at/recurrence_rule", "RFC3339 / RRULE-lite"})
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) handleDeleteEvent(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	if err := p.DeleteEvent(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Bundles ----

func (s *Server) handlePlannerToday(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	b, err := p.Today(time.Now())
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *Server) handlePlannerUpcoming(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	days := 7
	if n, ok := queryInt(r, "days"); ok {
		days = int(n)
	}
	items, err := p.Upcoming(time.Now(), days)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handlePlannerOverview(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	b, err := p.Overview(r.URL.Query().Get("date"), time.Now())
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid date", fieldError{"date", "YYYY-MM-DD"})
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, b)
}
