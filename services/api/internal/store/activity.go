package store

import (
	"database/sql"
	"strings"
)

// ---- Activity: cheap per-pillar latest-change timestamps ----

// Activity powers the dashboard's live pulse. Clients poll it and only
// re-fetch pillar data when a timestamp moves.

type Activity struct {
	Pillars map[string]string `json:"pillars"` // pillar -> RFC3339 latest change
	Latest  string            `json:"latest"`  // max across pillars ("" when empty DB)
}

// ActivityStore reads change timestamps; separate struct so any handler can
// depend on just this.
type ActivityStore struct {
	DB *sql.DB
}

// pillarSources maps pillar name → timestamp column queries. Keep in one
// place; new tables must register here or the board won't react to them.
var pillarSources = []struct {
	pillar  string
	queries []string // each returns 0..1 rows with one TEXT ts column
}{
	{"finance", []string{
		`SELECT MAX(created_at) FROM transactions`,
		`SELECT MAX(created_at) FROM accounts`,
		`SELECT MAX(created_at) FROM categories`,
		`SELECT MAX(created_at) FROM categorization_rules`,
		`SELECT MAX(created_at) FROM budgets`,
	}},
	{"planner", []string{
		`SELECT MAX(updated_at) FROM tasks`,
		`SELECT MAX(created_at) FROM tasks`,
		`SELECT MAX(created_at) FROM habit_checkoffs`,
		`SELECT MAX(updated_at) FROM events`,
		`SELECT MAX(created_at) FROM habits`,
	}},
	{"knowledge", []string{
		`SELECT MAX(updated_at) FROM notes`,
		`SELECT MAX(updated_at) FROM bookmarks`,
		`SELECT MAX(updated_at) FROM reading_list`,
	}},
	{"universal", []string{
		`SELECT MAX(updated_at) FROM items`,
		`SELECT MAX(created_at) FROM item_links`,
	}},
	{"health", []string{
		`SELECT MAX(updated_at) FROM meals`,
		`SELECT MAX(updated_at) FROM workouts`,
		`SELECT MAX(updated_at) FROM body_metrics`,
		`SELECT MAX(updated_at) FROM grocery_items`,
		`SELECT MAX(updated_at) FROM recipes`,
	}},
}

func LatestActivity(db *sql.DB) (Activity, error) {
	out := Activity{Pillars: map[string]string{}}

	var b strings.Builder
	b.WriteString(`SELECT pillar, MAX(ts) AS latest FROM (`)

	for i, src := range pillarSources {
		for j, q := range src.queries {
			if i > 0 || j > 0 {
				b.WriteString(" UNION ALL ")
			}
			b.WriteString(`SELECT '` + src.pillar + `' AS pillar, (` + q + `) AS ts`)
		}
	}
	b.WriteString(`) WHERE ts IS NOT NULL GROUP BY pillar`)

	rows, err := db.Query(b.String())
	if err != nil {
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		var pillar, ts string
		if err := rows.Scan(&pillar, &ts); err != nil {
			return out, err
		}
		out.Pillars[pillar] = ts
		if ts > out.Latest {
			out.Latest = ts
		}
	}
	return out, rows.Err()
}
