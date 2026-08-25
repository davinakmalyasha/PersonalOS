package store

import (
	"database/sql"
)

// Planner owns all planner-table queries (tasks, habits, checkoffs, events).
type Planner struct {
	DB *sql.DB
}
