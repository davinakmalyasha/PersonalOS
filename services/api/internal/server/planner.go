package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/domain/planner"
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
		r.Post("/{id}/skip", s.handleSkipTask)
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
	r.Get("/planner/parse-date", s.handleParseDate)
	r.Get("/planner/calendar.ics", s.handleCalendarICS)
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
	Title           string   `json:"title"`
	Notes           string   `json:"notes"`
	Status          string   `json:"status"`
	Priority        string   `json:"priority"`
	DueDate         *string  `json:"due_date"`
	DueTime         *string  `json:"due_time"` // HH:MM (12b)
	Project         *string  `json:"project"`
	RecurrenceRule  *string  `json:"recurrence_rule"`
	ParentID        *string  `json:"parent_id"`
	BlockedBy       *string  `json:"blocked_by"`
	EstimateMinutes *int     `json:"estimate_minutes"`
	Tags            []string `json:"tags"`
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
	t, err := p.CreateTask(req.Title, req.Notes, req.Status, req.Priority, req.DueDate, req.Project, req.Tags, req.RecurrenceRule, req.ParentID, req.BlockedBy, req.EstimateMinutes, req.DueTime, nil)
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
		ParentID:  q.Get("parent_id"),
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
	Title           *string   `json:"title"`
	Notes           *string   `json:"notes"`
	Status          *string   `json:"status"`
	Priority        *string   `json:"priority"`
	DueDate         *string   `json:"due_date"`
	DueTime         *string   `json:"due_time"`
	Project         **string  `json:"project"`
	RecurrenceRule  **string  `json:"recurrence_rule"`
	ParentID        *string   `json:"parent_id"`
	BlockedBy       *string   `json:"blocked_by"`
	EstimateMinutes **int     `json:"estimate_minutes"`
	Tags            *[]string `json:"tags"`
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
		Priority: req.Priority, DueDate: req.DueDate, DueTime: req.DueTime, Tags: req.Tags,
	}
	if req.Project != nil {
		u.Project = *req.Project
	}
	if req.RecurrenceRule != nil {
		u.RecurrenceRule = *req.RecurrenceRule
	}
	u.ParentID = req.ParentID
	u.BlockedBy = req.BlockedBy
	u.EstimateMin = req.EstimateMinutes
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
	PausedUntil   **string `json:"paused_until"`
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
		PausedUntil: req.PausedUntil, Color: req.Color, Archived: req.Archived,
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
		Date  string   `json:"date"`
		Value *float64 `json:"value"` // measurable quantity (12b)
		Note  string   `json:"note"`
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
	if req.Value != nil || req.Note != "" {
		if err := p.UpsertCheckoff(chiURLParam(r, "id"), req.Date, req.Value, req.Note); err != nil {
			if errors.Is(err, store.ErrInvalid) {
				fail(w, http.StatusBadRequest, "value must be >= 0")
				return
			}
			if !mapStoreErr(w, err) {
				fail(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		resp := map[string]interface{}{
			"habit_id": chiURLParam(r, "id"),
			"date":     req.Date,
			"done":     true,
		}
		if h, gerr := p.GetHabit(chiURLParam(r, "id")); gerr == nil {
			resp["streaks"] = h.Streaks
		}
		writeJSON(w, http.StatusOK, resp)
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

// ---- Phase 12b: planner depth ----

// POST /tasks/{id}/skip — advance a recurring task without completing it.
func (s *Server) handleSkipTask(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requirePlanner(w)
	if !ok {
		return
	}
	next, err := p.SkipTaskOccurrence(chiURLParam(r, "id"))
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "only recurring tasks can be skipped")
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, next)
}

// GET /planner/parse-date?q=tomorrow%20at%207pm — natural-language date.
func (s *Server) handleParseDate(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		fail(w, http.StatusBadRequest, "q required", fieldError{"q", "natural language date, e.g. 'fri at 7pm'"})
		return
	}
	parsed, err := planner.ParseNaturalDate(q, time.Now().UTC())
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error(), fieldError{"q", "unparseable"})
		return
	}
	writeJSON(w, http.StatusOK, parsed)
}

// GET /planner/calendar.ics?days=90 — read-only feed of events + dated tasks.
func (s *Server) handleCalendarICS(w http.ResponseWriter, r *http.Request) {
	p := s.planner
	days := 90
	if n, ok := queryInt(r, "days"); ok && int(n) > 0 && int(n) <= 365 {
		days = int(n)
	}
	now := time.Now().UTC()
	from := now.Format("2006-01-02")
	to := now.AddDate(0, 0, days).Format("2006-01-02")

	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//PersonalOS//EN\r\nCALSCALE:GREGORIAN\r\n")

	occurrences, _ := p.OccurrencesBetween(from, to)
	for _, o := range occurrences {
		b.WriteString("BEGIN:VEVENT\r\n")
		fmt.Fprintf(&b, "UID:%s-%s@personal-os\r\n", o.EventID, o.Date)
		b.WriteString(icsLine("SUMMARY", o.Title))
		b.WriteString(icsLine("LOCATION", o.Location))
		st, _ := time.Parse(time.RFC3339, o.StartsAt)
		fmt.Fprintf(&b, "DTSTART:%sZ\r\n", st.UTC().Format("20060102T150405"))
		if o.EndsAt != nil {
			if en, perr := time.Parse(time.RFC3339, *o.EndsAt); perr == nil {
				fmt.Fprintf(&b, "DTEND:%sZ\r\n", en.UTC().Format("20060102T150405"))
			}
		}
		b.WriteString("END:VEVENT\r\n")
	}

	if tasks, total, err := p.ListTasks(store.TaskFilter{Status: "open", DueBefore: to, PageSize: 100}); err == nil {
		_ = total
		for _, t := range tasks {
			if t.DueDate == nil || *t.DueDate < from {
				continue
			}
			b.WriteString("BEGIN:VEVENT\r\n")
			fmt.Fprintf(&b, "UID:%s@personal-os\r\n", t.ID)
			b.WriteString(icsLine("SUMMARY", "[task] "+t.Title))
			day, perr := time.Parse("2006-01-02", *t.DueDate)
			if perr == nil {
				fmt.Fprintf(&b, "DTSTART;VALUE=DATE:%s\r\n", day.Format("20060102"))
			}
			b.WriteString("END:VEVENT\r\n")
		}
	}

	b.WriteString("END:VCALENDAR\r\n")
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="personal-os.ics"`)
	_, _ = w.Write([]byte(b.String()))
}

func icsLine(key, val string) string {
	val = strings.ReplaceAll(val, "\\", "\\\\")
	val = strings.ReplaceAll(val, "\n", "\\n")
	val = strings.ReplaceAll(val, ",", "\\,")
	if val == "" {
		return ""
	}
	return key + ":" + val + "\r\n"
}