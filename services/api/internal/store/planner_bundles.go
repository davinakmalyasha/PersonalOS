package store

import (
	"time"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/domain/planner"
)

// ---- Bundles: today / upcoming / overview ----

type HabitWithStatus struct {
	Habit
	DueToday  bool `json:"due_today"`
	DoneToday bool `json:"done_today"`
}

type DayBundle struct {
	Date   string            `json:"date"` // YYYY-MM-DD
	Tasks  []Task            `json:"tasks"`
	Habits []HabitWithStatus `json:"habits"`
	Events []Occurrence      `json:"events"`
}

// TaskWithBlocks wraps a task with its dependency status for bundles.
type TaskWithBlocks struct {
	Task
	Blocked bool `json:"blocked"` // blocked_by target is not done
}

type TodayBundle struct {
	Date     string            `json:"date"`
	Overdue  []TaskWithBlocks  `json:"overdue"`
	DueToday []TaskWithBlocks  `json:"due_today"`
	Habits   []HabitWithStatus `json:"habits"`
	Events   []Occurrence      `json:"events"`

	// Day-load: sum of open-task estimates due today/overdue, and total
	// scheduled event minutes â€” the "how full is today" indicator.
	TaskLoadMinutes  int64 `json:"task_load_minutes"`
	EventLoadMinutes int64 `json:"event_load_minutes"`
}

type UpcomingDay struct {
	Date   string       `json:"date"`
	Tasks  []Task       `json:"tasks"`
	Events []Occurrence `json:"events"`
}

func todayStr(t time.Time) string { return t.UTC().Format("2006-01-02") }

// Today aggregates everything actionable on the given day (default: now).
func (p *Planner) Today(now time.Time) (TodayBundle, error) {
	day := todayStr(now)
	b := TodayBundle{Date: day}

	open, _, err := p.ListTasks(TaskFilter{Status: "open", DueBefore: day, PageSize: 100})
	if err != nil {
		return b, err
	}

	// Dependency map: id â†’ whether the blocker is done. Only open tasks can
	// block, so fetch their statuses.
	blockedStatus := map[string]bool{}
	for _, t := range open {
		if t.BlockedBy != nil {
			if blocker, err := p.GetTask(*t.BlockedBy); err == nil {
				blockedStatus[*t.BlockedBy] = blocker.Status != "done"
			}
		}
	}
	wrap := func(t Task) TaskWithBlocks {
		blocked := false
		if t.BlockedBy != nil {
			blocked = blockedStatus[*t.BlockedBy]
		}
		return TaskWithBlocks{Task: t, Blocked: blocked}
	}

	for _, t := range open {
		if t.DueDate != nil && *t.DueDate == day {
			b.DueToday = append(b.DueToday, wrap(t))
		} else {
			b.Overdue = append(b.Overdue, wrap(t))
		}
		if t.EstimateMin != nil {
			b.TaskLoadMinutes += int64(*t.EstimateMin)
		}
	}
	if b.Overdue == nil {
		b.Overdue = []TaskWithBlocks{}
	}
	if b.DueToday == nil {
		b.DueToday = []TaskWithBlocks{}
	}

	habits, err := p.ListHabits(false)
	if err != nil {
		return b, err
	}
	doneMap, err := p.CheckoffsForDate(day)
	if err != nil {
		return b, err
	}
	b.Habits = []HabitWithStatus{}
	for _, h := range habits {
		paused := h.PausedUntil != nil && *h.PausedUntil >= day
		b.Habits = append(b.Habits, HabitWithStatus{
			Habit: h,
			DueToday: !paused &&
				planner.HabitDueToday(h.Cadence, h.TargetPerWeek, h.Streaks.WeekDone, weekdaysOf(&h), now),
			DoneToday: doneMap[h.ID],
		})
	}

	events, err := p.OccurrencesBetween(day, day)
	if err != nil {
		return b, err
	}
	if events == nil {
		events = []Occurrence{}
	}
	b.Events = events
	for _, e := range events {
		if e.EndsAt != nil {
			if start, e1 := time.Parse(time.RFC3339, e.StartsAt); e1 == nil {
				if end, e2 := time.Parse(time.RFC3339, *e.EndsAt); e2 == nil && end.After(start) {
					b.EventLoadMinutes += int64(end.Sub(start).Minutes())
				}
			}
		}
	}
	return b, nil
}

// Upcoming builds a per-day agenda for N days starting at `start`.
func (p *Planner) Upcoming(start time.Time, days int) ([]UpcomingDay, error) {
	if days < 1 || days > 60 {
		days = 7
	}
	from := todayStr(start)
	toD := start.UTC().AddDate(0, 0, days-1)
	to := todayStr(toD)

	tasks, _, err := p.ListTasks(TaskFilter{Status: "open", DueBefore: to, PageSize: 100})
	if err != nil {
		return nil, err
	}
	events, err := p.OccurrencesBetween(from, to)
	if err != nil {
		return nil, err
	}

	out := make([]UpcomingDay, 0, days)
	for i := 0; i < days; i++ {
		day := start.UTC().AddDate(0, 0, i).Format("2006-01-02")
		ud := UpcomingDay{Date: day, Tasks: []Task{}, Events: []Occurrence{}}
		for _, t := range tasks {
			if t.DueDate != nil && *t.DueDate == day {
				ud.Tasks = append(ud.Tasks, t)
			}
		}
		for _, e := range events {
			if e.Date == day {
				ud.Events = append(ud.Events, e)
			}
		}
		out = append(out, ud)
	}
	return out, nil
}

// Overview is the unified daily bundle used by the planner dashboard.
func (p *Planner) Overview(date string, now time.Time) (DayBundle, error) {
	if date == "" {
		date = todayStr(now)
	} else if _, err := time.Parse("2006-01-02", date); err != nil {
		return DayBundle{}, ErrInvalid
	}
	b := DayBundle{Date: date}

	tasks, _, err := p.ListTasks(TaskFilter{Due: date, PageSize: 100})
	if err != nil {
		return b, err
	}
	b.Tasks = tasks

	habits, err := p.ListHabits(false)
	if err != nil {
		return b, err
	}
	doneMap, err := p.CheckoffsForDate(date)
	if err != nil {
		return b, err
	}
	b.Habits = []HabitWithStatus{}
	for _, h := range habits {
		b.Habits = append(b.Habits, HabitWithStatus{
			Habit:     h,
			DueToday:  planner.HabitDueToday(h.Cadence, h.TargetPerWeek, h.Streaks.WeekDone, weekdaysOf(&h), now),
			DoneToday: doneMap[h.ID],
		})
	}

	events, err := p.OccurrencesBetween(date, date)
	if err != nil {
		return b, err
	}
	if events == nil {
		events = []Occurrence{}
	}
	b.Events = events
	return b, nil
}

// ---- Weekly review bundle ----

type HabitWeek struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Scheduled    int    `json:"scheduled"`
	Done         int    `json:"done"`
	Consistency  int    `json:"consistency"` // done*100/scheduled (0 when nothing scheduled)
}

type ReviewBundle struct {
	WeekStart    string       `json:"week_start"` // Monday
	WeekEnd      string       `json:"week_end"`   // Sunday
	TasksDone    []Task       `json:"tasks_done"`
	Habits       []HabitWeek  `json:"habits"`
	EventsCount  int          `json:"events_count"`
	Month        string       `json:"month"`
	SpendMinor   int64        `json:"spend_minor"`
	BudgetLines  []BudgetLine `json:"budget_lines"`
}

// Review aggregates the Monâ€“Sun week containing `date` (default: current week).
func (p *Planner) Review(now time.Time, date string) (ReviewBundle, error) {
	base := now
	if date != "" {
		t, err := time.Parse("2006-01-02", date)
		if err != nil {
			return ReviewBundle{}, ErrInvalid
		}
		base = t
	}
	// Monday of the week (local-agnostic: UTC).
	wd := int(base.Weekday()+6) % 7
	weekStart := time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -wd)
	weekEnd := weekStart.AddDate(0, 0, 6)
	start, end := weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02")

	rb := ReviewBundle{WeekStart: start, WeekEnd: end}

	// Completed tasks in window.
	doneRows, _, err := p.ListTasks(TaskFilter{Status: "done", Page: 1, PageSize: 100})
	if err != nil {
		return rb, err
	}
	rb.TasksDone = []Task{}
	for _, t := range doneRows {
		if t.CompletedAt != nil {
			day := (*t.CompletedAt)[:10]
			if day >= start && day <= end {
				rb.TasksDone = append(rb.TasksDone, t)
			}
		}
	}

	// Habits: done vs scheduled within the 7 days.
	habits, err := p.ListHabits(false)
	if err != nil {
		return rb, err
	}
	rb.Habits = []HabitWeek{}
	for _, h := range habits {
		set := map[string]bool{}
		for _, d := range h.Dates {
			set[d] = true
		}
		sched, done := 0, 0
		for i := 0; i < 7; i++ {
			day := weekStart.AddDate(0, 0, i).Format("2006-01-02")
			// Pause excused only when the pause covers the whole week window so far.
			wd := int(weekStart.AddDate(0, 0, i).Weekday()+6) % 7
			if weekdaysOf(&h)[wd] != '1' {
				continue
			}
			if h.PausedUntil != nil && day <= *h.PausedUntil {
				continue
			}
			sched++
			if set[day] {
				done++
			}
		}
		consistency := 0
		if sched > 0 {
			consistency = done * 100 / sched
		}
		rb.Habits = append(rb.Habits, HabitWeek{
			ID: h.ID, Name: h.Name, Scheduled: sched, Done: done, Consistency: consistency,
		})
	}

	events, err := p.OccurrencesBetween(start, end)
	if err != nil {
		return rb, err
	}
	rb.EventsCount = len(events)
	rb.Month = weekStart.Format("2006-01")
	return rb, nil
}
