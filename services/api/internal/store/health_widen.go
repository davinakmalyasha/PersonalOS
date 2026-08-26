package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
)

// ---- Exercise library (phase 12c) ----

type Exercise struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MuscleGroup string `json:"muscle_group"`
	Equipment   string `json:"equipment"`
}

// ListExercises searches the seeded library; empty filters return everything.
func (h *Health) ListExercises(q, muscle, equipment string, limit int) ([]Exercise, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	where := []string{"1=1"}
	args := []interface{}{}
	if q != "" {
		where = append(where, "LOWER(name) LIKE ?")
		args = append(args, "%"+strings.ToLower(q)+"%")
	}
	if muscle != "" {
		where = append(where, "muscle_group=?")
		args = append(args, muscle)
	}
	if equipment != "" {
		where = append(where, "equipment=?")
		args = append(args, equipment)
	}
	rows, err := h.DB.Query(
		`SELECT id,name,muscle_group,equipment FROM exercises WHERE `+strings.Join(where, " AND ")+
			` ORDER BY name LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Exercise{}
	for rows.Next() {
		var e Exercise
		if err := rows.Scan(&e.ID, &e.Name, &e.MuscleGroup, &e.Equipment); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ResolveExerciseID matches a logged set's free-text name against the library.
func (h *Health) ResolveExerciseID(name string) string {
	var id string
	_ = h.DB.QueryRow(`SELECT id FROM exercises WHERE LOWER(name)=LOWER(?)`, strings.TrimSpace(name)).Scan(&id)
	return id
}

// ---- Workout routines (templates) ----

type RoutineExercise struct {
	ID         string `json:"id"`
	RoutineID  string `json:"routine_id,omitempty"`
	Position   int    `json:"position"`
	Name       string `json:"name"`
	Sets       int    `json:"sets"`
	TargetReps int    `json:"target_reps"`
}

type Routine struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Notes        string             `json:"notes"`
	Tags         []string           `json:"tags"`
	Exercises    []RoutineExercise  `json:"exercises"`
	CreatedAt    string             `json:"created_at,omitempty"`
	UpdatedAt    string             `json:"updated_at,omitempty"`

	tagsRaw string
}

const routineCols = `id,name,notes,tags,created_at,updated_at`

func routineScan(r *Routine, tagsRaw *string) []interface{} {
	return []interface{}{&r.ID, &r.Name, &r.Notes, tagsRaw, &r.CreatedAt, &r.UpdatedAt}
}

func (h *Health) CreateRoutine(name, notes string, tags []string, exs []RoutineExercise) (Routine, error) {
	if strings.TrimSpace(name) == "" {
		return Routine{}, ErrInvalid
	}
	now := NowRFC3339()
	r := Routine{ID: NewID(), Name: name, Notes: notes, Tags: normalizeTagList(tags),
		CreatedAt: now, UpdatedAt: now, Exercises: []RoutineExercise{}}
	tx, err := h.DB.Begin()
	if err != nil {
		return Routine{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`INSERT INTO routines (`+routineCols+`) VALUES (?,?,?,?,?,?)`,
		r.ID, r.Name, r.Notes, joinTags(r.Tags), now, now); err != nil {
		return Routine{}, err
	}
	if err := insertRoutineExercises(tx, r.ID, exs); err != nil {
		return Routine{}, err
	}
	logChange(tx, "routine", r.ID, "create", r.Name)
	if err := tx.Commit(); err != nil {
		return Routine{}, err
	}
	return h.GetRoutine(r.ID)
}

func insertRoutineExercises(tx dbtx, routineID string, exs []RoutineExercise) error {
	for i, e := range exs {
		name := strings.TrimSpace(e.Name)
		if name == "" {
			continue
		}
		sets := e.Sets
		if sets <= 0 {
			sets = 3
		}
		reps := e.TargetReps
		if reps <= 0 {
			reps = 10
		}
		if _, err := tx.Exec(
			`INSERT INTO routine_exercises (id,routine_id,position,name,sets,target_reps) VALUES (?,?,?,?,?,?)`,
			NewID(), routineID, i, name, sets, reps); err != nil {
			return err
		}
	}
	return nil
}

func (h *Health) GetRoutine(id string) (Routine, error) {
	var r Routine
	err := h.DB.QueryRow(`SELECT `+routineCols+` FROM routines WHERE id=?`, id).
		Scan(routineScan(&r, &r.tagsRaw)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Routine{}, ErrNotFound
	}
	if err != nil {
		return Routine{}, err
	}
	r.Tags = splitTags(r.tagsRaw)
	exs, err := h.routineExercises(id)
	if err != nil {
		return Routine{}, err
	}
	r.Exercises = exs
	return r, nil
}

func (h *Health) routineExercises(id string) ([]RoutineExercise, error) {
	rows, err := h.DB.Query(
		`SELECT id,routine_id,position,name,sets,target_reps FROM routine_exercises WHERE routine_id=? ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RoutineExercise{}
	for rows.Next() {
		var e RoutineExercise
		if err := rows.Scan(&e.ID, &e.RoutineID, &e.Position, &e.Name, &e.Sets, &e.TargetReps); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (h *Health) ListRoutines(q string) ([]Routine, error) {
	where := "1=1"
	args := []interface{}{}
	if q != "" {
		where = "LOWER(name) LIKE ?"
		args = append(args, "%"+strings.ToLower(q)+"%")
	}
	rows, err := h.DB.Query(`SELECT `+routineCols+` FROM routines WHERE `+where+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	out := []Routine{}
	for rows.Next() {
		var r Routine
		if err := rows.Scan(routineScan(&r, &r.tagsRaw)...); err != nil {
			return nil, err
		}
		r.Tags = splitTags(r.tagsRaw)
		r.Exercises = []RoutineExercise{}
		out = append(out, r)
		ids = append(ids, r.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, id := range ids {
		exs, err := h.routineExercises(id)
		if err != nil {
			return nil, err
		}
		out[i].Exercises = exs
	}
	return out, nil
}

func (h *Health) DeleteRoutine(id string) error {
	res, err := h.DB.Exec(`DELETE FROM routines WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	logChange(h.DB, "routine", id, "delete", "routine")
	return nil
}

// StartRoutine copies a routine into a fresh workout. Sets become one logged
// set per target entry; the caller fills in weight/reps as they train.
func (h *Health) StartRoutine(routineID, performedAt string) (Workout, error) {
	r, err := h.GetRoutine(routineID)
	if err != nil {
		return Workout{}, err
	}
	type set struct {
		Name     string   `json:"name"`
		Sets     *int     `json:"sets,omitempty"`
		Target   *int     `json:"target_reps,omitempty"`
		WeightKg *float64 `json:"weight_kg"`
		Reps     *int     `json:"reps"`
	}
	setsJSON := make([]set, 0, len(r.Exercises))
	for _, e := range r.Exercises {
		target := e.TargetReps
		nSets := e.Sets
		if nSets < 1 {
			nSets = 1
		}
		for s := 0; s < nSets; s++ {
			setsJSON = append(setsJSON, set{Name: e.Name, Target: &target})
		}
	}
	raw, err := json.Marshal(setsJSON)
	if err != nil {
		return Workout{}, err
	}
	title := r.Name + " (routine)"
	return h.CreateWorkout(performedAt, &title, r.Notes, string(raw), nil, r.Tags)
}

// ---- Personal food database ----

type Food struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	ServingDesc string   `json:"serving_desc"`
	Calories    int64    `json:"calories"`
	ProteinG    float64  `json:"protein_g"`
	CarbsG      float64  `json:"carbs_g"`
	FatG        float64  `json:"fat_g"`
	CreatedAt   string   `json:"created_at,omitempty"`
}

func (h *Health) UpsertFood(f Food) (Food, error) {
	if strings.TrimSpace(f.Name) == "" || f.Calories < 0 || f.ProteinG < 0 || f.CarbsG < 0 || f.FatG < 0 {
		return Food{}, ErrInvalid
	}
	if strings.TrimSpace(f.ServingDesc) == "" {
		f.ServingDesc = "1 serving"
	}
	now := NowRFC3339()
	_, err := h.DB.Exec(`
		INSERT INTO foods (id,name,serving_desc,calories,protein_g,carbs_g,fat_g,created_at)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(name) DO UPDATE SET
			serving_desc=CASE WHEN excluded.serving_desc!='' THEN excluded.serving_desc ELSE foods.serving_desc END,
			calories=CASE WHEN excluded.calories>0 THEN excluded.calories ELSE foods.calories END,
			protein_g=CASE WHEN excluded.protein_g>0 THEN excluded.protein_g ELSE foods.protein_g END,
			carbs_g=CASE WHEN excluded.carbs_g>0 THEN excluded.carbs_g ELSE foods.carbs_g END,
			fat_g=CASE WHEN excluded.fat_g>0 THEN excluded.fat_g ELSE foods.fat_g END`,
		NewID(), f.Name, f.ServingDesc, f.Calories, f.ProteinG, f.CarbsG, f.FatG, now)
	if isUniqueErr(err) {
		return Food{}, ErrConflict
	}
	if err != nil {
		return Food{}, err
	}
	var out Food
	ferr := h.DB.QueryRow(
		`SELECT id,name,serving_desc,calories,protein_g,carbs_g,fat_g,created_at FROM foods WHERE name=?`, f.Name).
		Scan(&out.ID, &out.Name, &out.ServingDesc, &out.Calories, &out.ProteinG, &out.CarbsG, &out.FatG, &out.CreatedAt)
	if ferr != nil {
		return Food{}, ferr
	}
	logChange(h.DB, "food", out.ID, "create", out.Name)
	return out, nil
}

func (h *Health) ListFoods(q string, limit int) ([]Food, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	where := "1=1"
	args := []interface{}{}
	if q != "" {
		where = "LOWER(name) LIKE ?"
		args = append(args, "%"+strings.ToLower(q)+"%")
	}
	rows, err := h.DB.Query(
		`SELECT id,name,serving_desc,calories,protein_g,carbs_g,fat_g,created_at FROM foods WHERE `+where+
			` ORDER BY name LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Food{}
	for rows.Next() {
		var f Food
		if err := rows.Scan(&f.ID, &f.Name, &f.ServingDesc, &f.Calories, &f.ProteinG, &f.CarbsG, &f.FatG, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// LogMealFromFood creates a meal from library food × servings.
func (h *Health) LogMealFromFood(foodID string, servings float64, eatenAt, slot string) (Meal, error) {
	if servings <= 0 || servings > 50 {
		return Meal{}, ErrInvalid
	}
	var f Food
	err := h.DB.QueryRow(
		`SELECT id,name,serving_desc,calories,protein_g,carbs_g,fat_g FROM foods WHERE id=?`, foodID).
		Scan(&f.ID, &f.Name, &f.ServingDesc, &f.Calories, &f.ProteinG, &f.CarbsG, &f.FatG)
	if errors.Is(err, sql.ErrNoRows) {
		return Meal{}, ErrNotFound
	}
	if err != nil {
		return Meal{}, err
	}
	cal := int64(math.Round(float64(f.Calories) * servings))
	p := round1(f.ProteinG * servings)
	c := round1(f.CarbsG * servings)
	ft := round1(f.FatG * servings)
	title := f.Name
	if servings != 1 {
		title = f.Name + " x" + strconv.FormatFloat(servings, 'f', -1, 64)
	}
	return h.CreateMeal(eatenAt, title, "", "[]", &cal, &p, &c, &ft, nil, slot)
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// ---- Daily macro series + weekly-target progress ----

type MacroDay struct {
	Date     string   `json:"date"`
	Calories *int64   `json:"calories"`
	ProteinG *float64 `json:"protein_g"`
	CarbsG   *float64 `json:"carbs_g"`
	FatG     *float64 `json:"fat_g"`
}

// MacroSeries returns per-day macro totals ascending for the rings' history.
func (h *Health) MacroSeries(from, to string) ([]MacroDay, error) {
	where, args := h.buildDayRange("substr(eaten_at,1,10)", from, to)
	rows, err := h.DB.Query(`
		SELECT substr(eaten_at,1,10), SUM(calories), SUM(protein_g), SUM(carbs_g), SUM(fat_g)
		FROM meals WHERE `+where+`
		GROUP BY substr(eaten_at,1,10) ORDER BY 1 ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MacroDay{}
	for rows.Next() {
		var d MacroDay
		var cal sql.NullInt64
		var p, c, f sql.NullFloat64
		if err := rows.Scan(&d.Date, &cal, &p, &c, &f); err != nil {
			return nil, err
		}
		if cal.Valid {
			v := cal.Int64
			d.Calories = &v
		}
		if p.Valid {
			v := p.Float64
			d.ProteinG = &v
		}
		if c.Valid {
			v := c.Float64
			d.CarbsG = &v
		}
		if f.Valid {
			v := f.Float64
			d.FatG = &v
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
