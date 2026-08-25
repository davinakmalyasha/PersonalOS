package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ---- Workouts ----

type Workout struct {
	ID              string          `json:"id"`
	PerformedAt     string          `json:"performed_at"` // RFC3339 UTC
	Title           *string         `json:"title"`
	Notes           string          `json:"notes"`
	DurationMinutes *int64          `json:"duration_minutes"`
	Exercises       json.RawMessage `json:"exercises"` // JSON array of {name, sets, reps, weight_kg, ...}
	Tags            []string        `json:"tags"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`

	tagsRaw string
}

const workoutCols = `id,performed_at,title,notes,duration_minutes,exercises,tags,created_at,updated_at`

func workoutScan(w *Workout, tagsRaw *string) []interface{} {
	return []interface{}{&w.ID, &w.PerformedAt, &w.Title, &w.Notes,
		&w.DurationMinutes, &w.Exercises, tagsRaw, &w.CreatedAt, &w.UpdatedAt}
}

func (h *Health) CreateWorkout(performedAt string, title *string, notes, exercisesJSON string, durationMinutes *int64, tags []string) (Workout, error) {
	if !validRFC3339(performedAt) {
		return Workout{}, ErrInvalid
	}
	if !validJSONArray(exercisesJSON) {
		return Workout{}, ErrInvalid
	}
	if durationMinutes != nil && *durationMinutes < 0 {
		return Workout{}, ErrInvalid
	}
	now := NowRFC3339()
	w := Workout{
		ID: NewID(), PerformedAt: performedAt, Title: title, Notes: notes,
		DurationMinutes: durationMinutes, Exercises: rawJSONArray(exercisesJSON),
		Tags: normalizeTagList(tags), CreatedAt: now, UpdatedAt: now,
	}
	if title != nil && *title == "" {
		w.Title = nil
	}
	var titleV, dur interface{}
	if w.Title != nil {
		titleV = *w.Title
	}
	if durationMinutes != nil {
		dur = *durationMinutes
	}
	_, err := h.DB.Exec(
		`INSERT INTO workouts (`+workoutCols+`) VALUES (?,?,?,?,?,?,?,?,?)`,
		w.ID, w.PerformedAt, titleV, w.Notes, dur, w.Exercises, joinTags(w.Tags), now, now)
	if err != nil {
		return Workout{}, err
	}
	logChange(h.DB, "workout", w.ID, "create", workoutLogTitle(w))
	return h.GetWorkout(w.ID)
}

func workoutLogTitle(w Workout) string {
	if w.Title != nil && *w.Title != "" {
		return *w.Title
	}
	return "workout"
}

func (h *Health) GetWorkout(id string) (Workout, error) {
	var w Workout
	err := h.DB.QueryRow(`SELECT `+workoutCols+` FROM workouts WHERE id=?`, id).
		Scan(workoutScan(&w, &w.tagsRaw)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Workout{}, ErrNotFound
	}
	if err != nil {
		return Workout{}, err
	}
	w.Tags = splitTags(w.tagsRaw)
	return w, nil
}

type WorkoutFilter struct {
	From, To string // YYYY-MM-DD on performed_at date
	Q        string
	Page     int
	PageSize int
}

func (h *Health) ListWorkouts(f WorkoutFilter) ([]Workout, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 50
	}
	where, args := h.buildDayRange("performed_at", f.From, f.To)
	extra := []string{}
	if f.Q != "" {
		extra = append(extra, "(LOWER(COALESCE(title,'')) LIKE ? OR LOWER(notes) LIKE ?)")
		pat := "%" + strings.ToLower(f.Q) + "%"
		args = append(args, pat, pat)
	}
	where = strings.Join(append([]string{where}, extra...), " AND ")

	var total int
	if err := h.DB.QueryRow(`SELECT COUNT(*) FROM workouts WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT ` + workoutCols + ` FROM workouts WHERE ` + where +
		` ORDER BY performed_at DESC LIMIT ? OFFSET ?`
	pageArgs := append(append([]interface{}{}, args...), f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := h.DB.Query(q, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Workout{}
	for rows.Next() {
		var w Workout
		if err := rows.Scan(workoutScan(&w, &w.tagsRaw)...); err != nil {
			return nil, 0, err
		}
		w.Tags = splitTags(w.tagsRaw)
		out = append(out, w)
	}
	return out, total, rows.Err()
}

type WorkoutUpdate struct {
	PerformedAt     *string
	Title           **string
	Notes           *string
	DurationMinutes **int64
	Exercises       *string
	Tags            *[]string
}

func (h *Health) UpdateWorkout(id string, u WorkoutUpdate) (Workout, error) {
	cur, err := h.GetWorkout(id)
	if err != nil {
		return Workout{}, err
	}
	if u.PerformedAt != nil {
		if !validRFC3339(*u.PerformedAt) {
			return Workout{}, ErrInvalid
		}
		cur.PerformedAt = *u.PerformedAt
	}
	if u.Title != nil {
		cur.Title = *u.Title // ptr-to-nil clears
		if *u.Title != nil && **u.Title == "" {
			cur.Title = nil
		}
	}
	if u.Notes != nil {
		cur.Notes = *u.Notes
	}
	if u.DurationMinutes != nil {
		if *u.DurationMinutes == nil || **u.DurationMinutes >= 0 {
			cur.DurationMinutes = *u.DurationMinutes
		} else {
			return Workout{}, ErrInvalid
		}
	}
	if u.Exercises != nil {
		if !validJSONArray(*u.Exercises) {
			return Workout{}, ErrInvalid
		}
		cur.Exercises = rawJSONArray(*u.Exercises)
	}
	if u.Tags != nil {
		cur.Tags = normalizeTagList(*u.Tags)
	}
	cur.UpdatedAt = NowRFC3339()

	var titleV, dur interface{}
	if cur.Title != nil {
		titleV = *cur.Title
	}
	if cur.DurationMinutes != nil {
		dur = *cur.DurationMinutes
	}
	_, err = h.DB.Exec(
		`UPDATE workouts SET performed_at=?, title=?, notes=?, duration_minutes=?, exercises=?, tags=?, updated_at=? WHERE id=?`,
		cur.PerformedAt, titleV, cur.Notes, dur, cur.Exercises, joinTags(cur.Tags), cur.UpdatedAt, id)
	if err != nil {
		return Workout{}, err
	}
	logChange(h.DB, "workout", id, "update", workoutLogTitle(cur))
	return h.GetWorkout(id)
}

func (h *Health) DeleteWorkout(id string) error {
	cur, err := h.GetWorkout(id)
	if err != nil {
		return err
	}
	res, err := h.DB.Exec(`DELETE FROM workouts WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	logChange(h.DB, "workout", id, "delete", workoutLogTitle(cur))
	return nil
}

// ---- Body metrics ----

// UpsertBodyMetric inserts or REPLACES the row for the same calendar day
// (unique index on substr(measured_at,1,10)) ÃƒÂ¢Ã¢â€šÂ¬Ã¢â‚¬Â the day's latest measurement
// wins. Returns the stored row.
func (h *Health) UpsertBodyMetric(measuredAt string, weightKg, bodyFatPct *float64, notes string) (BodyMetric, error) {
	if !validRFC3339(measuredAt) {
		return BodyMetric{}, ErrInvalid
	}
	if weightKg != nil && *weightKg <= 0 {
		return BodyMetric{}, ErrInvalid
	}
	if bodyFatPct != nil && (*bodyFatPct <= 0 || *bodyFatPct >= 100) {
		return BodyMetric{}, ErrInvalid
	}

	tx, err := h.DB.Begin()
	if err != nil {
		return BodyMetric{}, err
	}
	defer func() { _ = tx.Rollback() }()

	day := measuredAt[:10]
	var existingID string
	err = tx.QueryRow(`SELECT id FROM body_metrics WHERE substr(measured_at,1,10)=?`, day).Scan(&existingID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		now := NowRFC3339()
		id := NewID()
		var w, f interface{}
		if weightKg != nil {
			w = *weightKg
		}
		if bodyFatPct != nil {
			f = *bodyFatPct
		}
		if _, err := tx.Exec(
			`INSERT INTO body_metrics (id,measured_at,weight_kg,body_fat_pct,notes,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`,
			id, measuredAt, w, f, notes, now, now); err != nil {
			return BodyMetric{}, err
		}
		logChange(tx, "body_metric", id, "create", "body metrics "+day)
	case err != nil:
		return BodyMetric{}, err
	default:
		now := NowRFC3339()
		var w, f interface{}
		if weightKg != nil {
			w = *weightKg
		}
		if bodyFatPct != nil {
			f = *bodyFatPct
		}
		if _, err := tx.Exec(
			`UPDATE body_metrics SET measured_at=?, weight_kg=COALESCE(?,weight_kg), body_fat_pct=COALESCE(?,body_fat_pct), notes=?, updated_at=? WHERE id=?`,
			measuredAt, w, f, notes, now, existingID); err != nil {
			return BodyMetric{}, err
		}
		logChange(tx, "body_metric", existingID, "update", "body metrics "+day)
	}
	if err := tx.Commit(); err != nil {
		return BodyMetric{}, err
	}
	return h.BodyMetricForDay(day)
}

func (h *Health) BodyMetricForDay(day string) (BodyMetric, error) {
	var m BodyMetric
	err := h.DB.QueryRow(`SELECT `+bodyMetricCols+` FROM body_metrics WHERE substr(measured_at,1,10)=?`, day).
		Scan(bodyMetricScan(&m)...)
	if errors.Is(err, sql.ErrNoRows) {
		return BodyMetric{}, ErrNotFound
	}
	return m, err
}

type BodyMetric struct {
	ID         string   `json:"id"`
	MeasuredAt string   `json:"measured_at"` // RFC3339 UTC
	WeightKg   *float64 `json:"weight_kg"`
	BodyFatPct *float64 `json:"body_fat_pct"`
	Notes      string   `json:"notes"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

const bodyMetricCols = `id,measured_at,weight_kg,body_fat_pct,notes,created_at,updated_at`

func bodyMetricScan(m *BodyMetric) []interface{} {
	return []interface{}{&m.ID, &m.MeasuredAt, &m.WeightKg, &m.BodyFatPct, &m.Notes, &m.CreatedAt, &m.UpdatedAt}
}

// ListBodyMetrics returns daily metrics newest-first within [from,to].
func (h *Health) ListBodyMetrics(from, to string) ([]BodyMetric, error) {
	where, args := h.buildDayRange("measured_at", from, to)
	q := `SELECT ` + bodyMetricCols + ` FROM body_metrics WHERE ` + where +
		` ORDER BY measured_at DESC`
	rows, err := h.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BodyMetric{}
	for rows.Next() {
		var m BodyMetric
		if err := rows.Scan(bodyMetricScan(&m)...); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (h *Health) DeleteBodyMetric(id string) error {
	res, err := h.DB.Exec(`DELETE FROM body_metrics WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// WeightSeries returns daily-bucketed weight points ascending; days without a
// measurement are omitted (chart connects the rest).
func (h *Health) WeightSeries(from, to string) ([]WeightPoint, error) {
	where, args := h.buildDayRange("measured_at", from, to)
	q := `SELECT substr(measured_at,1,10) AS day, weight_kg FROM body_metrics
		WHERE ` + where + ` AND weight_kg IS NOT NULL ORDER BY day ASC`
	rows, err := h.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WeightPoint{}
	for rows.Next() {
		var p WeightPoint
		var kg float64
		if err := rows.Scan(&p.Date, &kg); err != nil {
			return nil, err
		}
		p.WeightKg = kg
		out = append(out, p)
	}
	return out, rows.Err()
}

type WeightPoint struct {
	Date     string  `json:"date"` // YYYY-MM-DD
	WeightKg float64 `json:"weight_kg"`
}

// ---- Health summary ----

type HealthSummary struct {
	From     string         `json:"from"`
	To       string         `json:"to"`
	Workouts WorkoutsRollup `json:"workouts"`
	Meals    MealsRollup    `json:"meals"`
	Weight   WeightRollup   `json:"weight"`
	Grocery  GroceryRollup  `json:"grocery"`

	CalorieGoal   *int64 `json:"calorie_goal,omitempty"`
	CaloriesToday *int64 `json:"calories_today,omitempty"`
	WaterTodayMl  *int64 `json:"water_today_ml,omitempty"`
}

type WorkoutsRollup struct {
	Count        int   `json:"count"`
	TotalMinutes int64 `json:"total_minutes"`
}

type MealsRollup struct {
	Count         int    `json:"count"`
	CaloriesTotal *int64 `json:"calories_total"`
}

type WeightRollup struct {
	LatestKg   *float64 `json:"latest_kg"`
	FirstKg    *float64 `json:"first_kg"`
	ChangeKg   *float64 `json:"change_kg"`
	MeasuredOn *string  `json:"measured_on"`
}

type GroceryRollup struct {
	Total   int `json:"total"`
	Checked int `json:"checked"`
}

func (h *Health) Summary(from, to string) (HealthSummary, error) {
	s := HealthSummary{From: from, To: to}
	where, args := h.buildDayRange("performed_at", from, to)
	if err := h.DB.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(duration_minutes),0) FROM workouts WHERE `+where,
		args...).Scan(&s.Workouts.Count, &s.Workouts.TotalMinutes); err != nil {
		return s, err
	}

	mWhere, mArgs := h.buildDayRange("eaten_at", from, to)
	var cal sql.NullInt64
	if err := h.DB.QueryRow(
		`SELECT COUNT(*), SUM(calories) FROM meals WHERE `+mWhere,
		mArgs...).Scan(&s.Meals.Count, &cal); err != nil {
		return s, err
	}
	if cal.Valid {
		v := cal.Int64
		s.Meals.CaloriesTotal = &v
	}

	// Weight rollup over window.
	wq := `SELECT weight_kg, substr(measured_at,1,10) FROM body_metrics
		WHERE weight_kg IS NOT NULL`
	wArgs := []interface{}{}
	if from != "" || to != "" {
		clauses := []string{}
		if from != "" {
			clauses = append(clauses, "substr(measured_at,1,10)>=?")
			wArgs = append(wArgs, from)
		}
		if to != "" {
			clauses = append(clauses, "substr(measured_at,1,10)<=?")
			wArgs = append(wArgs, to)
		}
		wq += " AND " + strings.Join(clauses, " AND ")
	}
	wq += " ORDER BY substr(measured_at,1,10) ASC"
	rows, err := h.DB.Query(wq, wArgs...)
	if err != nil {
		return s, err
	}
	type kv struct {
		day string
		kg  float64
	}
	var pts []kv
	for rows.Next() {
		var p kv
		if err := rows.Scan(&p.kg, &p.day); err != nil {
			rows.Close()
			return s, err
		}
		pts = append(pts, p)
	}
	rows.Close()
	if len(pts) > 0 {
		first, last := pts[0].kg, pts[len(pts)-1].kg
		change := last - first
		s.Weight.FirstKg = &first
		s.Weight.LatestKg = &last
		s.Weight.ChangeKg = &change
		s.Weight.MeasuredOn = &pts[len(pts)-1].day
	}

	gq := `SELECT COUNT(*), COALESCE(SUM(checked),0) FROM grocery_items`
	if err := h.DB.QueryRow(gq).Scan(&s.Grocery.Total, &s.Grocery.Checked); err != nil {
		return s, err
	}

	// Calorie goal (single 'calorie' goal row) + today's consumed.
	var target sql.NullInt64
	if err := h.DB.QueryRow(`SELECT target_minor FROM goals WHERE kind='calorie' LIMIT 1`).Scan(&target); err == nil && target.Valid {
		v := target.Int64
		s.CalorieGoal = &v
		var consumed sql.NullInt64
		today := time.Now().UTC().Format("2006-01-02")
		_ = h.DB.QueryRow(
			`SELECT SUM(calories) FROM meals WHERE substr(eaten_at,1,10)=?`, today).Scan(&consumed)
		if consumed.Valid {
			c := consumed.Int64
			s.CaloriesToday = &c
		} else {
			z := int64(0)
			s.CaloriesToday = &z
		}
	}

	// Water today.
	var water sql.NullInt64
	today := time.Now().UTC().Format("2006-01-02")
	_ = h.DB.QueryRow(
		`SELECT water_ml FROM body_metrics WHERE substr(measured_at,1,10)=?`, today).Scan(&water)
	if water.Valid {
		w := water.Int64
		s.WaterTodayMl = &w
	}
	return s, nil
}
