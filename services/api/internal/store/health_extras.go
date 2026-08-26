package store

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// ---- Water intake (on the body-metrics day row) ----

// LogWater adds ml to today's (or a given day's) body-metric row, creating it
// when absent. Returns the stored total for the day.
func (h *Health) LogWater(day string, ml int) (int, error) {
	if ml <= 0 || ml > 10000 {
		return 0, ErrInvalid
	}
	if day == "" {
		day = time.Now().UTC().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", day); err != nil {
		return 0, ErrInvalid
	}
	// Ensure a row exists for the day (measured_at = noon UTC keeps the day key).
	var id string
	err := h.DB.QueryRow(`SELECT id FROM body_metrics WHERE substr(measured_at,1,10)=?`, day).Scan(&id)
	if err == nil {
		if _, err := h.DB.Exec(`UPDATE body_metrics SET water_ml = COALESCE(water_ml,0)+?, updated_at=? WHERE id=?`,
			ml, NowRFC3339(), id); err != nil {
			return 0, err
		}
	} else {
		now := NowRFC3339()
		if _, err := h.DB.Exec(
			`INSERT INTO body_metrics (id,measured_at,water_ml,created_at,updated_at) VALUES (?,?,?,?,?)`,
			NewID(), day+"T12:00:00Z", ml, now, now); err != nil {
			return 0, err
		}
	}
	var total int
	if err := h.DB.QueryRow(`SELECT COALESCE(water_ml,0) FROM body_metrics WHERE substr(measured_at,1,10)=?`, day).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// ---- Exercise PRs (computed from workout exercises JSON) ----

type ExercisePR struct {
	Exercise      string  `json:"exercise"`
	MaxWeightKg   float64 `json:"max_weight_kg"`
	BestRepsAtMax int     `json:"best_reps_at_max"`
	EstOneRMKg    float64 `json:"est_one_rm_kg"` // Epley: w × (1 + reps/30)
	LastDate      string  `json:"last_date"`
}

// EstimateOneRM computes the Epley estimated one-rep max.
func EstimateOneRM(weight float64, reps int) float64 {
	if reps <= 0 || weight <= 0 {
		return weight
	}
	if reps == 1 {
		return weight
	}
	return weight * (1 + float64(reps)/30.0)
}

// ExercisePRs computes the heaviest set per exercise name within [from,to]
// (dates on performed_at). Ties keep the earliest date.
func (h *Health) ExercisePRs(from, to string) ([]ExercisePR, error) {
	where, args := h.buildDayRange("performed_at", from, to)
	rows, err := h.DB.Query(
		`SELECT performed_at, exercises FROM workouts WHERE `+where+` ORDER BY performed_at ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type set struct {
		Name     string   `json:"name"`
		WeightKg *float64 `json:"weight_kg"`
		Reps     *int     `json:"reps"`
	}
	best := map[string]ExercisePR{}
	for rows.Next() {
		var performed, raw string
		if err := rows.Scan(&performed, &raw); err != nil {
			return nil, err
		}
		day := performed[:10]
		var sets []set
		if err := json.Unmarshal([]byte(raw), &sets); err != nil {
			continue // malformed row — skip
		}
		for _, s := range sets {
			name := strings.TrimSpace(s.Name)
			if name == "" || s.WeightKg == nil {
				continue
			}
			key := strings.ToLower(name)
			w := *s.WeightKg
			reps := 0
			if s.Reps != nil {
				reps = *s.Reps
			}
			oneRM := EstimateOneRM(w, reps)
			cur, ok := best[key]
			if !ok || w > cur.MaxWeightKg {
				best[key] = ExercisePR{Exercise: name, MaxWeightKg: w, BestRepsAtMax: reps, EstOneRMKg: oneRM, LastDate: day}
			} else if w == cur.MaxWeightKg {
				if reps > cur.BestRepsAtMax {
					cur.BestRepsAtMax = reps
				}
				if oneRM > cur.EstOneRMKg {
					cur.EstOneRMKg = oneRM
				}
				cur.LastDate = day
				best[key] = cur
			} else if oneRM > cur.EstOneRMKg {
				cur.EstOneRMKg = oneRM
				cur.LastDate = day
				best[key] = cur
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]ExercisePR, 0, len(best))
	for _, pr := range best {
		out = append(out, pr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MaxWeightKg > out[j].MaxWeightKg })
	return out, nil
}
