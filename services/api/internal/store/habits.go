package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/domain/planner"
)

// ---- Habits + checkoffs ----

type Habit struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Cadence       string  `json:"cadence"`
	TargetPerWeek int     `json:"target_per_week"`
	Weekdays      string  `json:"weekdays"` // Mon-first schedule mask, e.g. 1110100
	PausedUntil   *string `json:"paused_until"` // YYYY-MM-DD; not due while paused
	Color         *string `json:"color"`
	CreatedAt     string  `json:"created_at"`
	ArchivedAt    *string `json:"archived_at"`

	// Computed (present when fetched via GetHabit/ListHabits with streaks).
	Dates   []string        `json:"dates,omitempty"` // recent checkoff dates
	Streaks planner.Streaks `json:"streaks"`
}

type HabitUpdate struct {
	Name          *string
	Description   *string
	Cadence       *string
	TargetPerWeek *int
	Weekdays      *string
	PausedUntil   **string
	Color         **string
	Archived      *bool
}

func validCadence(c string) bool { return c == "daily" || c == "weekly" }

func validWeekdays(w string) bool {
	if len(w) != 7 {
		return false
	}
	for _, c := range w {
		if c != '0' && c != '1' {
			return false
		}
	}
	return true
}

// weekdaysOf returns the habit's schedule mask, defaulting for unset/legacy rows.
func weekdaysOf(h *Habit) string {
	if len(h.Weekdays) == 7 {
		return h.Weekdays
	}
	return "1111111"
}

func (p *Planner) CreateHabit(name, description, cadence string, targetPerWeek int, weekdays *string, color *string) (Habit, error) {
	if cadence == "" {
		cadence = "daily"
	}
	if !validCadence(cadence) {
		return Habit{}, ErrInvalid
	}
	if cadence == "daily" {
		targetPerWeek = 7
	}
	if targetPerWeek < 1 || targetPerWeek > 7 {
		return Habit{}, ErrInvalid
	}
	wd := "1111111"
	if weekdays != nil && *weekdays != "" {
		if !validWeekdays(*weekdays) {
			return Habit{}, ErrInvalid
		}
		wd = *weekdays
	}
	h := Habit{
		ID: NewID(), Name: name, Description: description, Cadence: cadence,
		TargetPerWeek: targetPerWeek, Weekdays: wd, Color: color, CreatedAt: NowRFC3339(),
	}
	var col interface{}
	if color != nil && *color != "" {
		col = *color
	} else {
		h.Color = nil
	}
	_, err := p.DB.Exec(`
		INSERT INTO habits (id,name,description,cadence,target_per_week,weekdays,paused_until,color,created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		h.ID, h.Name, h.Description, h.Cadence, h.TargetPerWeek, h.Weekdays, nil, col, h.CreatedAt)
	if err != nil {
		return Habit{}, err
	}
	logChange(p.DB, "habit", h.ID, "create", h.Name)
	return h, nil
}

const habitCols = `id,name,description,cadence,target_per_week,weekdays,paused_until,color,created_at,archived_at`

func habitScan(h *Habit) []interface{} {
	return []interface{}{&h.ID, &h.Name, &h.Description, &h.Cadence,
		&h.TargetPerWeek, &h.Weekdays, &h.PausedUntil, &h.Color, &h.CreatedAt, &h.ArchivedAt}
}

func (p *Planner) GetHabit(id string) (Habit, error) {
	var h Habit
	err := p.DB.QueryRow(`SELECT `+habitCols+` FROM habits WHERE id=?`, id).
		Scan(habitScan(&h)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Habit{}, ErrNotFound
	}
	if err != nil {
		return Habit{}, err
	}
	p.attachStreaks(&h, time.Now().UTC())
	return h, nil
}

// ListHabits returns habits; archived included only when requested. Streaks
// computed for all returned rows.
func (p *Planner) ListHabits(includeArchived bool) ([]Habit, error) {
	q := `SELECT ` + habitCols + ` FROM habits`
	if !includeArchived {
		q += ` WHERE archived_at IS NULL`
	}
	q += ` ORDER BY created_at`
	rows, err := p.DB.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Habit{}
	for rows.Next() {
		var h Habit
		if err := rows.Scan(habitScan(&h)...); err != nil {
			return nil, err
		}
		p.attachStreaks(&h, time.Now().UTC())
		out = append(out, h)
	}
	return out, rows.Err()
}

func (p *Planner) attachStreaks(h *Habit, today time.Time) {
	dates, err := p.allCheckoffDates(h.ID)
	if err != nil {
		return
	}
	h.Dates = dates
	h.Streaks = planner.ComputeStreaks(dates, h.Cadence, h.TargetPerWeek, today)
	h.Streaks.Consistency30 = planner.Consistency30(dates, weekdaysOf(h), today, h.PausedUntil)
}

func (p *Planner) allCheckoffDates(habitID string) ([]string, error) {
	rows, err := p.DB.Query(`SELECT date FROM habit_checkoffs WHERE habit_id=? ORDER BY date`, habitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (p *Planner) UpdateHabit(id string, u HabitUpdate) (Habit, error) {
	cur, err := p.GetHabit(id)
	if err != nil {
		return Habit{}, err
	}
	if u.Name != nil && *u.Name != "" {
		cur.Name = *u.Name
	}
	if u.Description != nil {
		cur.Description = *u.Description
	}
	cadenceChanged := false
	if u.Cadence != nil {
		if !validCadence(*u.Cadence) {
			return Habit{}, ErrInvalid
		}
		cadenceChanged = cur.Cadence != *u.Cadence
		cur.Cadence = *u.Cadence
	}
	if u.TargetPerWeek != nil {
		if *u.TargetPerWeek < 1 || *u.TargetPerWeek > 7 {
			return Habit{}, ErrInvalid
		}
		cur.TargetPerWeek = *u.TargetPerWeek
	} else if cadenceChanged && cur.Cadence == "weekly" && cur.TargetPerWeek == 7 {
		cur.TargetPerWeek = 3 // sensible default when flipping dailyÃ¢â€ â€™weekly
	}
	if cur.Cadence == "daily" {
		cur.TargetPerWeek = 7
	}
	if u.Weekdays != nil {
		if !validWeekdays(*u.Weekdays) {
			return Habit{}, ErrInvalid
		}
		cur.Weekdays = *u.Weekdays
	}
	if u.PausedUntil != nil {
		if *u.PausedUntil == nil || **u.PausedUntil == "" {
			cur.PausedUntil = nil
		} else {
			if _, perr := time.Parse("2006-01-02", **u.PausedUntil); perr != nil {
				return Habit{}, ErrInvalid
			}
			v := **u.PausedUntil
			cur.PausedUntil = &v
		}
	}
	if u.Color != nil {
		cur.Color = *u.Color // double pointer: nil means "not sent"; ptr-to-nil clears
	}
	if u.Archived != nil {
		if *u.Archived {
			now := NowRFC3339()
			cur.ArchivedAt = &now
		} else {
			cur.ArchivedAt = nil
		}
	}

	var col, archived, paused interface{}
	if cur.Color != nil {
		col = *cur.Color
	}
	if cur.ArchivedAt != nil {
		archived = *cur.ArchivedAt
	}
	if cur.PausedUntil != nil {
		paused = *cur.PausedUntil
	}
	_, err = p.DB.Exec(`
		UPDATE habits SET name=?, description=?, cadence=?, target_per_week=?, weekdays=?, paused_until=?, color=?, archived_at=?
		WHERE id=?`,
		cur.Name, cur.Description, cur.Cadence, cur.TargetPerWeek, cur.Weekdays, paused, col, archived, id)
	if err != nil {
		return Habit{}, err
	}
	logChange(p.DB, "habit", id, "update", cur.Name)
	return p.GetHabit(id)
}

func (p *Planner) DeleteHabit(id string) error {
	h, err := p.getHabitRaw(id)
	if err != nil {
		return err
	}
	res, err := p.DB.Exec(`DELETE FROM habits WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	logChange(p.DB, "habit", id, "delete", h.Name)
	return nil
}

// ToggleCheckoff inserts the (habit,date) row when missing and removes it when
// present Ã¢â‚¬â€ one call is always idempotent-safe to repeat in either state.
// Returns whether the habit is checked after the toggle.
func (p *Planner) ToggleCheckoff(habitID, date string) (bool, error) {
	if _, err := p.getHabitRaw(habitID); err != nil {
		return false, err
	}
	res, err := p.DB.Exec(`
		INSERT INTO habit_checkoffs (id,habit_id,date,created_at) VALUES (?,?,?,?)
		ON CONFLICT(habit_id, date) DO NOTHING`,
		NewID(), habitID, date, NowRFC3339())
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		if h, herr := p.getHabitRaw(habitID); herr == nil {
			logChange(p.DB, "habit", habitID, "update", "checked off: "+h.Name)
		}
		return true, nil // inserted Ã¢â€ â€™ now done
	}
	// Already existed Ã¢â€ â€™ remove it.
	res, err = p.DB.Exec(`DELETE FROM habit_checkoffs WHERE habit_id=? AND date=?`, habitID, date)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 0, nil // nothing deleted (concurrent re-insert?) Ã¢â€ â€™ still done
}

// SetCheckoff forces a state instead of toggling; used by tests/tools that
// need explicit on/off semantics.
func (p *Planner) SetCheckoff(habitID, date string, done bool) error {
	if _, err := p.getHabitRaw(habitID); err != nil {
		return err
	}
	if done {
		_, err := p.DB.Exec(`
			INSERT INTO habit_checkoffs (id,habit_id,date,created_at) VALUES (?,?,?,?)
			ON CONFLICT(habit_id, date) DO NOTHING`,
			NewID(), habitID, date, NowRFC3339())
		return err
	}
	_, err := p.DB.Exec(`DELETE FROM habit_checkoffs WHERE habit_id=? AND date=?`, habitID, date)
	return err
}

// UpsertCheckoff logs (or replaces) a measurable entry: presence counts as a
// done day for streaks; value is the quantity (e.g. 8 glasses), note free text.
func (p *Planner) UpsertCheckoff(habitID, date string, value *float64, note string) error {
	h, err := p.getHabitRaw(habitID)
	if err != nil {
		return err
	}
	var v interface{}
	if value != nil {
		if *value < 0 {
			return ErrInvalid
		}
		v = *value
	}
	_, err = p.DB.Exec(`
		INSERT INTO habit_checkoffs (id,habit_id,date,value,note,created_at)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(habit_id, date) DO UPDATE SET value=COALESCE(?,value), note=?`,
		NewID(), habitID, date, v, note, NowRFC3339(), v, note)
	if err == nil {
		logChange(p.DB, "habit", habitID, "update", "logged: "+h.Name)
	}
	return err
}

func (p *Planner) getHabitRaw(id string) (Habit, error) {
	var h Habit
	err := p.DB.QueryRow(`SELECT `+habitCols+` FROM habits WHERE id=?`, id).
		Scan(habitScan(&h)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Habit{}, ErrNotFound
	}
	return h, err
}

// CheckoffsBetween lists checkoff dates for a habit within [from,to].
func (p *Planner) CheckoffsBetween(habitID, from, to string) ([]string, error) {
	if _, err := p.getHabitRaw(habitID); err != nil {
		return nil, err
	}
	where := []string{"habit_id=?"}
	args := []interface{}{habitID}
	if from != "" {
		where = append(where, "date>=?")
		args = append(args, from)
	}
	if to != "" {
		where = append(where, "date<=?")
		args = append(args, to)
	}
	rows, err := p.DB.Query(
		`SELECT date FROM habit_checkoffs WHERE `+strings.Join(where, " AND ")+` ORDER BY date DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CheckoffsForDate maps habit_id Ã¢â€ â€™ checked for one calendar day.
func (p *Planner) CheckoffsForDate(date string) (map[string]bool, error) {
	rows, err := p.DB.Query(`SELECT habit_id FROM habit_checkoffs WHERE date=?`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
