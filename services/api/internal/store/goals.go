package store

import (
	"database/sql"
	"errors"
	"strings"
)

// ---- Goals (savings + daily calorie target) ----

type Goal struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"` // savings | calorie
	Name        string  `json:"name"`
	TargetMinor *int64  `json:"target_minor"` // savings: amount; calorie: daily kcal target
	SavedMinor  int64   `json:"saved_minor"`
	Deadline    *string `json:"deadline"` // YYYY-MM-DD
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

const goalCols = `id,kind,name,target_minor,saved_minor,deadline,created_at,updated_at`

func goalScan(g *Goal) []interface{} {
	return []interface{}{&g.ID, &g.Kind, &g.Name, &g.TargetMinor, &g.SavedMinor,
		&g.Deadline, &g.CreatedAt, &g.UpdatedAt}
}

func validGoalKind(k string) bool { return k == "savings" || k == "calorie" }

func (f *Finance) CreateGoal(kind, name string, targetMinor *int64, deadline *string) (Goal, error) {
	if !validGoalKind(kind) || strings.TrimSpace(name) == "" {
		return Goal{}, ErrInvalid
	}
	if targetMinor != nil && *targetMinor < 0 {
		return Goal{}, ErrInvalid
	}
	// Only one active calorie goal — upsert semantics.
	if kind == "calorie" {
		if _, err := f.DB.Exec(`DELETE FROM goals WHERE kind='calorie'`); err != nil {
			return Goal{}, err
		}
	}
	now := NowRFC3339()
	g := Goal{ID: NewID(), Kind: kind, Name: name, TargetMinor: targetMinor,
		SavedMinor: 0, Deadline: deadline, CreatedAt: now, UpdatedAt: now}
	var target, dl interface{}
	if targetMinor != nil {
		target = *targetMinor
	}
	if deadline != nil && *deadline != "" {
		dl = *deadline
	} else {
		g.Deadline = nil
	}
	_, err := f.DB.Exec(
		`INSERT INTO goals (`+goalCols+`) VALUES (?,?,?,?,?,?,?,?)`,
		g.ID, g.Kind, g.Name, target, 0, dl, now, now)
	if err != nil {
		return Goal{}, err
	}
	return f.GetGoal(g.ID)
}

func (f *Finance) GetGoal(id string) (Goal, error) {
	var g Goal
	err := f.DB.QueryRow(`SELECT `+goalCols+` FROM goals WHERE id=?`, id).Scan(goalScan(&g)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Goal{}, ErrNotFound
	}
	return g, err
}

func (f *Finance) ListGoals(kind string) ([]Goal, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if kind != "" {
		if !validGoalKind(kind) {
			return nil, ErrInvalid
		}
		where = append(where, "kind=?")
		args = append(args, kind)
	}
	rows, err := f.DB.Query(
		`SELECT `+goalCols+` FROM goals WHERE `+strings.Join(where, " AND ")+` ORDER BY created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Goal{}
	for rows.Next() {
		var g Goal
		if err := rows.Scan(goalScan(&g)...); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

type GoalUpdate struct {
	Name        *string
	TargetMinor **int64
	SavedMinor  *int64
	Deadline    **string
}

func (f *Finance) UpdateGoal(id string, u GoalUpdate) (Goal, error) {
	cur, err := f.GetGoal(id)
	if err != nil {
		return Goal{}, err
	}
	if u.Name != nil && strings.TrimSpace(*u.Name) != "" {
		cur.Name = *u.Name
	}
	if u.TargetMinor != nil {
		if *u.TargetMinor == nil || **u.TargetMinor >= 0 {
			cur.TargetMinor = *u.TargetMinor
		} else {
			return Goal{}, ErrInvalid
		}
	}
	if u.SavedMinor != nil && *u.SavedMinor >= 0 {
		cur.SavedMinor = *u.SavedMinor
	}
	if u.Deadline != nil {
		cur.Deadline = *u.Deadline
	}
	cur.UpdatedAt = NowRFC3339()

	var target, dl interface{}
	if cur.TargetMinor != nil {
		target = *cur.TargetMinor
	}
	if cur.Deadline != nil {
		dl = *cur.Deadline
	}
	_, err = f.DB.Exec(
		`UPDATE goals SET name=?, target_minor=?, saved_minor=?, deadline=?, updated_at=? WHERE id=?`,
		cur.Name, target, cur.SavedMinor, dl, cur.UpdatedAt, id)
	if err != nil {
		return Goal{}, err
	}
	return f.GetGoal(id)
}

// AddToGoal increments saved_minor (negative allowed for corrections).
func (f *Finance) AddToGoal(id string, amountMinor int64) (Goal, error) {
	cur, err := f.GetGoal(id)
	if err != nil {
		return Goal{}, err
	}
	next := cur.SavedMinor + amountMinor
	if next < 0 {
		next = 0
	}
	return f.UpdateGoal(id, GoalUpdate{SavedMinor: &next})
}

func (f *Finance) DeleteGoal(id string) error {
	res, err := f.DB.Exec(`DELETE FROM goals WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
