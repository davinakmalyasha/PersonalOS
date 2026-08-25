package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
	ErrInvalid  = errors.New("invalid")
)

// Finance owns all finance-table queries.
type Finance struct {
	DB *sql.DB
}

func NewID() string { return ulid.Make().String() }

func NowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }
