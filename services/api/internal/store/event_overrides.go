package store

import (
	"database/sql"
	"errors"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/domain/planner"
)

// ---- Event exceptions (per-occurrence cancel/edit) ----

type EventOverride struct {
	ID        string  `json:"id"`
	EventID   string  `json:"event_id"`
	Date      string  `json:"date"` // occurrence date being overridden
	Action    string  `json:"action"` // cancel | edit
	Title     *string `json:"title,omitempty"`
	StartsAt  *string `json:"starts_at,omitempty"`
	EndsAt    *string `json:"ends_at,omitempty"`
	Location  *string `json:"location,omitempty"`
	CreatedAt string  `json:"created_at"`
}

func validOverrideAction(a string) bool { return a == "cancel" || a == "edit" }

// SetEventOverride upserts the exception for (event, date).
func (p *Planner) SetEventOverride(eventID string, o EventOverride) (EventOverride, error) {
	if _, err := p.GetEvent(eventID); err != nil {
		return EventOverride{}, err
	}
	if !validOverrideAction(o.Action) {
		return EventOverride{}, ErrInvalid
	}
	if _, err := planner.ParseDateStrict(o.Date); err != nil {
		return EventOverride{}, ErrInvalid
	}
	if o.Action == "edit" {
		if o.StartsAt != nil && !validEventTime(*o.StartsAt) {
			return EventOverride{}, ErrInvalid
		}
		if o.EndsAt != nil && *o.EndsAt != "" && !validEventTime(*o.EndsAt) {
			return EventOverride{}, ErrInvalid
		}
	}
	now := NowRFC3339()
	var existing string
	err := p.DB.QueryRow(`SELECT id FROM event_overrides WHERE event_id=? AND date=?`, eventID, o.Date).Scan(&existing)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		id := NewID()
		_, err = p.DB.Exec(`
			INSERT INTO event_overrides (id,event_id,date,action,title,starts_at,ends_at,location,created_at)
			VALUES (?,?,?,?,?,?,?,?,?)`,
			id, eventID, o.Date, o.Action, o.Title, o.StartsAt, o.EndsAt, o.Location, now)
		if err != nil {
			return EventOverride{}, err
		}
		return p.GetEventOverride(id)
	case err != nil:
		return EventOverride{}, err
	default:
		_, err = p.DB.Exec(`
			UPDATE event_overrides SET action=?, title=?, starts_at=?, ends_at=?, location=? WHERE id=?`,
			o.Action, o.Title, o.StartsAt, o.EndsAt, o.Location, existing)
		if err != nil {
			return EventOverride{}, err
		}
		return p.GetEventOverride(existing)
	}
}

func (p *Planner) GetEventOverride(id string) (EventOverride, error) {
	var o EventOverride
	err := p.DB.QueryRow(
		`SELECT id,event_id,date,action,title,starts_at,ends_at,location,created_at FROM event_overrides WHERE id=?`, id).
		Scan(&o.ID, &o.EventID, &o.Date, &o.Action, &o.Title, &o.StartsAt, &o.EndsAt, &o.Location, &o.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return EventOverride{}, ErrNotFound
	}
	return o, err
}

func (p *Planner) ListEventOverrides(eventID string) ([]EventOverride, error) {
	rows, err := p.DB.Query(
		`SELECT id,event_id,date,action,title,starts_at,ends_at,location,created_at FROM event_overrides WHERE event_id=? ORDER BY date`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EventOverride{}
	for rows.Next() {
		var o EventOverride
		if err := rows.Scan(&o.ID, &o.EventID, &o.Date, &o.Action, &o.Title, &o.StartsAt, &o.EndsAt, &o.Location, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (p *Planner) DeleteEventOverride(id string) error {
	res, err := p.DB.Exec(`DELETE FROM event_overrides WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// overridesBetween loads exceptions applicable to [from,to].
func (p *Planner) overridesBetween(from, to string) (map[string]EventOverride, error) {
	rows, err := p.DB.Query(
		`SELECT id,event_id,date,action,title,starts_at,ends_at,location FROM event_overrides WHERE date>=? AND date<=?`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]EventOverride{}
	for rows.Next() {
		var o EventOverride
		if err := rows.Scan(&o.ID, &o.EventID, &o.Date, &o.Action, &o.Title, &o.StartsAt, &o.EndsAt, &o.Location); err != nil {
			return nil, err
		}
		out[o.EventID+"|"+o.Date] = o
	}
	return out, rows.Err()
}

// applyOverride mutates an occurrence per its exception (cancel handled by
// caller dropping it).
func applyOverride(o *Occurrence, ov EventOverride) {
	if ov.Action == "edit" {
		if ov.Title != nil {
			o.Title = *ov.Title
		}
		if ov.Location != nil {
			o.Location = *ov.Location
		}
		if ov.StartsAt != nil {
			o.StartsAt = *ov.StartsAt
		}
		if ov.EndsAt != nil {
			o.EndsAt = ov.EndsAt
		}
	}
}
