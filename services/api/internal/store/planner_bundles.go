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

type TodayBundle struct {
	Date     string            `json:"date"`
	Overdue  []Task            `json:"overdue"`
	DueToday []Task            `json:"due_today"`
	Habits   []HabitWithStatus `json:"habits"`
	Events   []Occurrence      `json:"events"`
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
	for _, t := range open {
		if t.DueDate != nil && *t.DueDate == day {
			b.DueToday = append(b.DueToday, t)
		} else {
			b.Overdue = append(b.Overdue, t)
		}
	}
	if b.Overdue == nil {
		b.Overdue = []Task{}
	}
	if b.DueToday == nil {
		b.DueToday = []Task{}
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
		b.Habits = append(b.Habits, HabitWithStatus{
			Habit:     h,
			DueToday:  planner.HabitDueToday(h.Cadence, h.TargetPerWeek, h.Streaks.WeekDone, weekdaysOf(&h), now),
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
