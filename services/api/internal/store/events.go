package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/domain/planner"
)

// ---- Events ----

type Event struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	StartsAt    string   `json:"starts_at"` // RFC3339 UTC
	EndsAt      *string  `json:"ends_at"`
	Location    string   `json:"location"`
	Recurrence  *string  `json:"recurrence_rule"`
	Tags        []string `json:"tags"`

	createdAt, updatedAt string
	tagsRaw              string
}

// Occurrence is one materialized instance of an event inside a window.
type Occurrence struct {
	EventID     string   `json:"event_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Location    string   `json:"location"`
	Tags        []string `json:"tags"`
	Date        string   `json:"date"`      // YYYY-MM-DD of occurrence
	StartsAt    string   `json:"starts_at"` // RFC3339 on occurrence day
	EndsAt      *string  `json:"ends_at,omitempty"`
	Series      bool     `json:"series"` // true when part of a recurrence
}

func validEventTime(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

func (p *Planner) CreateEvent(title, description string, startsAt string, endsAt *string, location string, recurrence *string, tags []string) (Event, error) {
	if !validEventTime(startsAt) {
		return Event{}, ErrInvalid
	}
	if recurrence != nil && strings.TrimSpace(*recurrence) != "" {
		rule := strings.TrimSpace(*recurrence)
		if _, err := planner.ParseRecurrence(rule); err != nil {
			return Event{}, ErrInvalid
		}
		recurrence = &rule
	} else {
		recurrence = nil
	}
	if endsAt != nil && *endsAt != "" && !validEventTime(*endsAt) {
		return Event{}, ErrInvalid
	}
	now := NowRFC3339()
	e := Event{
		ID: NewID(), Title: title, Description: description, StartsAt: startsAt,
		Location: location, Recurrence: recurrence,
		Tags: normalizeTagList(tags), createdAt: now, updatedAt: now,
	}
	if endsAt != nil && *endsAt != "" {
		e.EndsAt = endsAt
	}
	var end, recArg interface{}
	if e.EndsAt != nil {
		end = *e.EndsAt
	}
	if e.Recurrence != nil {
		recArg = *e.Recurrence
	}
	_, err := p.DB.Exec(`
		INSERT INTO events (id,title,description,starts_at,ends_at,location,recurrence_rule,tags,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		e.ID, e.Title, e.Description, e.StartsAt, end, e.Location, recArg, joinTags(e.Tags), now, now)
	if err != nil {
		return Event{}, err
	}
	return p.GetEvent(e.ID)
}

const eventCols = `id,title,description,starts_at,ends_at,location,recurrence_rule,tags,created_at,updated_at`

func eventScan(e *Event, tagsRaw *string) []interface{} {
	return []interface{}{&e.ID, &e.Title, &e.Description, &e.StartsAt, &e.EndsAt,
		&e.Location, &e.Recurrence, tagsRaw, &e.createdAt, &e.updatedAt}
}

func (p *Planner) GetEvent(id string) (Event, error) {
	var e Event
	err := p.DB.QueryRow(`SELECT `+eventCols+` FROM events WHERE id=?`, id).
		Scan(eventScan(&e, &e.tagsRaw)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	if err != nil {
		return Event{}, err
	}
	e.Tags = splitTags(e.tagsRaw)
	return e, nil
}

type EventUpdate struct {
	Title       *string
	Description *string
	StartsAt    *string
	EndsAt      **string // nil=not sent; ptr-to-nil clears; ptr-to-value sets
	Location    *string
	Recurrence  **string // same convention; validated when set
	Tags        *[]string
}

func (p *Planner) UpdateEvent(id string, u EventUpdate) (Event, error) {
	cur, err := p.GetEvent(id)
	if err != nil {
		return Event{}, err
	}
	if u.Title != nil && *u.Title != "" {
		cur.Title = *u.Title
	}
	if u.Description != nil {
		cur.Description = *u.Description
	}
	if u.Location != nil {
		cur.Location = *u.Location
	}
	if u.Tags != nil {
		cur.Tags = normalizeTagList(*u.Tags)
	}
	if u.StartsAt != nil {
		if !validEventTime(*u.StartsAt) {
			return Event{}, ErrInvalid
		}
		cur.StartsAt = *u.StartsAt
	}
	if u.EndsAt != nil {
		if *u.EndsAt == nil || **u.EndsAt == "" {
			cur.EndsAt = nil
		} else if !validEventTime(**u.EndsAt) {
			return Event{}, ErrInvalid
		} else {
			v := **u.EndsAt
			cur.EndsAt = &v
		}
	}
	if u.Recurrence != nil {
		if *u.Recurrence == nil || **u.Recurrence == "" {
			cur.Recurrence = nil
		} else {
			rule := **u.Recurrence
			if _, err := planner.ParseRecurrence(rule); err != nil {
				return Event{}, ErrInvalid
			}
			cur.Recurrence = &rule
		}
	}

	now := NowRFC3339()
	cur.updatedAt = now
	var end, rec interface{}
	if cur.EndsAt != nil {
		end = *cur.EndsAt
	}
	if cur.Recurrence != nil {
		rec = *cur.Recurrence
	}
	_, err = p.DB.Exec(`
		UPDATE events SET title=?, description=?, starts_at=?, ends_at=?, location=?, recurrence_rule=?, tags=?, updated_at=?
		WHERE id=?`,
		cur.Title, cur.Description, cur.StartsAt, end, cur.Location, rec, joinTags(cur.Tags), now, id)
	if err != nil {
		return Event{}, err
	}
	return p.GetEvent(id)
}

func (p *Planner) DeleteEvent(id string) error {
	res, err := p.DB.Exec(`DELETE FROM events WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// eventsStartingOnOrBefore loads all events whose base start is <= to-date;
// expansion handles reaching back for COUNT-bounded series.
func (p *Planner) eventsUpTo(toDate string) ([]Event, error) {
	rows, err := p.DB.Query(
		`SELECT `+eventCols+` FROM events WHERE substr(starts_at,1,10)<=? ORDER BY starts_at`, toDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(eventScan(&e, &e.tagsRaw)...); err != nil {
			return nil, err
		}
		e.Tags = splitTags(e.tagsRaw)
		out = append(out, e)
	}
	return out, rows.Err()
}

// OccurrencesBetween expands single + recurring events into concrete instances
// within [from,to] (YYYY-MM-DD inclusive). Non-recurring events whose date
// falls outside are dropped; recurring ones are expanded per RRULE-lite.
func (p *Planner) OccurrencesBetween(from, to string) ([]Occurrence, error) {
	if _, err := planner.ParseDateStrict(from); err != nil {
		return nil, ErrInvalid
	}
	if _, err := planner.ParseDateStrict(to); err != nil {
		return nil, ErrInvalid
	}
	events, err := p.eventsUpTo(to)
	if err != nil {
		return nil, err
	}
	overrides, err := p.overridesBetween(from, to)
	if err != nil {
		return nil, err
	}
	var out []Occurrence
	for _, e := range events {
		start, err := time.Parse(time.RFC3339, e.StartsAt)
		if err != nil {
			continue
		}
		var days []string
		if e.Recurrence != nil && strings.TrimSpace(*e.Recurrence) != "" {
			rule, err := planner.ParseRecurrence(*e.Recurrence)
			if err != nil {
				continue // stored data should always validate
			}
			days = rule.Expand(start, from, to)
		} else {
			day := start.Format("2006-01-02")
			if day >= from && day <= to {
				days = []string{day}
			}
		}
		for _, d := range days {
			// Per-occurrence exceptions win over the series definition.
			if ov, ok := overrides[e.ID+"|"+d]; ok {
				if ov.Action == "cancel" {
					continue
				}
				o := occurrenceFor(e, start, d)
				applyOverride(&o, ov)
				out = append(out, o)
				continue
			}
			out = append(out, occurrenceFor(e, start, d))
		}
	}
	return out, nil
}

func occurrenceFor(e Event, baseStart time.Time, day string) Occurrence {
	hh, mm, ss := baseStart.Clock()
	startUTC := time.UTC
	y, m, d := parseDay(day)
	inst := time.Date(y, m, d, hh, mm, ss, 0, startUTC)
	o := Occurrence{
		EventID:     e.ID,
		Title:       e.Title,
		Description: e.Description,
		Location:    e.Location,
		Tags:        e.Tags,
		Date:        day,
		StartsAt:    inst.Format(time.RFC3339),
		Series:      e.Recurrence != nil,
	}
	if e.EndsAt != nil {
		if end, err := time.Parse(time.RFC3339, *e.EndsAt); err == nil {
			dur := end.Sub(baseStart)
			if dur < 0 {
				dur = 0
			}
			endStr := inst.Add(dur).Format(time.RFC3339)
			o.EndsAt = &endStr
		}
	}
	return o
}

func parseDay(day string) (int, time.Month, int) {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return 1970, 1, 1
	}
	return t.Year(), t.Month(), t.Day()
}
