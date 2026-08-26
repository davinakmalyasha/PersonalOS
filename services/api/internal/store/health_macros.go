package store

import (
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
)

// ---- Health settings (singleton row) ----

type HealthSettings struct {
	CalorieTarget        *int64 `json:"calorie_target"`
	ProteinTargetG       *int64 `json:"protein_target_g"`
	CarbsTargetG         *int64 `json:"carbs_target_g"`
	FatTargetG           *int64 `json:"fat_target_g"`
	WaterTargetMl        *int64 `json:"water_target_ml"`
	WeeklyWorkoutTarget  *int64 `json:"weekly_workout_target"` // 1..14 sessions/week
	UpdatedAt            string `json:"updated_at"`
}

const healthSettingsID = "default"

// GetSettings returns the stored targets; all-nil defaults before first PUT.
func (h *Health) GetSettings() (HealthSettings, error) {
	var s HealthSettings
	var cal, prot, carbs, fat, water, wk sql.NullInt64
	err := h.DB.QueryRow(`
		SELECT calorie_target,protein_target_g,carbs_target_g,fat_target_g,
		       water_target_ml,weekly_workout_target,updated_at
		FROM health_settings WHERE id=?`, healthSettingsID).
		Scan(&cal, &prot, &carbs, &fat, &water, &wk, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return HealthSettings{}, nil // fresh install: defaults, no row yet
	}
	if err != nil {
		return HealthSettings{}, err
	}
	s.CalorieTarget = nullIntPtr(cal)
	s.ProteinTargetG = nullIntPtr(prot)
	s.CarbsTargetG = nullIntPtr(carbs)
	s.FatTargetG = nullIntPtr(fat)
	s.WaterTargetMl = nullIntPtr(water)
	s.WeeklyWorkoutTarget = nullIntPtr(wk)
	return s, nil
}

func nullIntPtr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

// UpdateSettings upserts the singleton row; omitted fields are left unchanged
// via COALESCE-style merge on pointers.
func (h *Health) UpdateSettings(u HealthSettings) (HealthSettings, error) {
	if u.WeeklyWorkoutTarget != nil && (*u.WeeklyWorkoutTarget < 1 || *u.WeeklyWorkoutTarget > 14) {
		return HealthSettings{}, ErrInvalid
	}
	for _, t := range []*int64{u.CalorieTarget, u.ProteinTargetG, u.CarbsTargetG, u.FatTargetG, u.WaterTargetMl} {
		if t != nil && *t < 0 {
			return HealthSettings{}, ErrInvalid
		}
	}
	now := NowRFC3339()
	_, err := h.DB.Exec(`
		INSERT INTO health_settings (id,calorie_target,protein_target_g,carbs_target_g,fat_target_g,water_target_ml,weekly_workout_target,updated_at)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			calorie_target=COALESCE(?,calorie_target),
			protein_target_g=COALESCE(?,protein_target_g),
			carbs_target_g=COALESCE(?,carbs_target_g),
			fat_target_g=COALESCE(?,fat_target_g),
			water_target_ml=COALESCE(?,water_target_ml),
			weekly_workout_target=COALESCE(?,weekly_workout_target),
			updated_at=?`,
		healthSettingsID, u.CalorieTarget, u.ProteinTargetG, u.CarbsTargetG,
		u.FatTargetG, u.WaterTargetMl, u.WeeklyWorkoutTarget, now,
		u.CalorieTarget, u.ProteinTargetG, u.CarbsTargetG,
		u.FatTargetG, u.WaterTargetMl, u.WeeklyWorkoutTarget, now)
	if err != nil {
		return HealthSettings{}, err
	}
	logChange(h.DB, "health_settings", healthSettingsID, "update", "health targets")
	return h.GetSettings()
}

// ---- Macro totals (summary rings) ----

type MacroTotals struct {
	Calories *int64   `json:"calories,omitempty"`
	ProteinG *float64 `json:"protein_g,omitempty"`
	CarbsG   *float64 `json:"carbs_g,omitempty"`
	FatG     *float64 `json:"fat_g,omitempty"`
}

func (h *Health) macroTotals(from, to string) (MacroTotals, error) {
	where, args := h.buildDayRange("eaten_at", from, to)
	var cal sql.NullInt64
	var p, c, f sql.NullFloat64
	err := h.DB.QueryRow(
		`SELECT SUM(calories),SUM(protein_g),SUM(carbs_g),SUM(fat_g) FROM meals WHERE `+where,
		args...).Scan(&cal, &p, &c, &f)
	if err != nil {
		return MacroTotals{}, err
	}
	out := MacroTotals{}
	if cal.Valid {
		v := cal.Int64
		out.Calories = &v
	}
	if p.Valid {
		v := p.Float64
		out.ProteinG = &v
	}
	if c.Valid {
		v := c.Float64
		out.CarbsG = &v
	}
	if f.Valid {
		v := f.Float64
		out.FatG = &v
	}
	return out, nil
}

// ---- Weekly tonnage (/health/volume) ----

type VolumeRow struct {
	Exercise    string  `json:"exercise"`
	Sets        int     `json:"sets"`
	RepsTotal   int     `json:"reps_total"`
	VolumeKg    float64 `json:"volume_kg"`
	MaxWeightKg float64 `json:"max_weight_kg"`
}

// WeeklyVolume aggregates training volume per exercise within [from,to]:
// volume = sum(weight_kg x reps) across logged sets.
func (h *Health) WeeklyVolume(from, to string) ([]VolumeRow, error) {
	where, args := h.buildDayRange("performed_at", from, to)
	rows, err := h.DB.Query(
		`SELECT exercises FROM workouts WHERE `+where+` ORDER BY performed_at ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type set struct {
		Name     string   `json:"name"`
		WeightKg *float64 `json:"weight_kg"`
		Reps     *int     `json:"reps"`
	}
	agg := map[string]*VolumeRow{}
	names := map[string]string{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var sets []set
		if err := json.Unmarshal([]byte(raw), &sets); err != nil {
			continue // malformed row — skip
		}
		for _, s := range sets {
			name := strings.TrimSpace(s.Name)
			if name == "" {
				continue
			}
			key := strings.ToLower(name)
			row, ok := agg[key]
			if !ok {
				row = &VolumeRow{Exercise: name}
				agg[key] = row
				names[key] = name
			}
			reps := 0
			if s.Reps != nil {
				reps = *s.Reps
			}
			w := 0.0
			if s.WeightKg != nil {
				w = *s.WeightKg
			}
			row.Sets++
			row.RepsTotal += reps
			row.VolumeKg += w * float64(reps)
			if w > row.MaxWeightKg {
				row.MaxWeightKg = w
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]VolumeRow, 0, len(agg))
	for _, row := range agg {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].VolumeKg > out[j].VolumeKg })
	return out, nil
}

// ---- Measurement trends ----

type TrendPoint struct {
	Date  string  `json:"date"` // YYYY-MM-DD
	Value float64 `json:"value"`
}

// MeasurementTrends turns free-form measurements objects into per-key time
// series (ascending) over [from,to], e.g. chest/waist/hip cm.
func (h *Health) MeasurementTrends(from, to string) (map[string][]TrendPoint, error) {
	where, args := h.buildDayRange("measured_at", from, to)
	rows, err := h.DB.Query(
		`SELECT substr(measured_at,1,10), measurements FROM body_metrics WHERE `+where+` ORDER BY substr(measured_at,1,10) ASC`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string][]TrendPoint{}
	for rows.Next() {
		var day, raw string
		if err := rows.Scan(&day, &raw); err != nil {
			return nil, err
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			continue
		}
		for k, v := range m {
			n, ok := v.(float64)
			if !ok {
				continue
			}
			out[k] = append(out[k], TrendPoint{Date: day, Value: n})
		}
	}
	return out, rows.Err()
}
