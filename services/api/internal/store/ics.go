package store

import (
	"database/sql"
	"errors"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/domain/planner"
)

// ---- ICS import (phase 13c) ----

// ImportEvents inserts calendar events deduped by their ICS UID. Existing
// UIDs are skipped (idempotent re-imports); events without a UID always
// import. Returns counts.
func (p *Planner) ImportEvents(drafts []planner.ICSEvent, tags []string) (imported, skipped int, err error) {
	for _, d := range drafts {
		if d.UID != "" {
			var exists string
			qerr := p.DB.QueryRow(`SELECT id FROM events WHERE external_uid=?`, d.UID).Scan(&exists)
			if qerr == nil {
				skipped++
				continue
			}
			if !errors.Is(qerr, sql.ErrNoRows) {
				return imported, skipped, qerr
			}
		}
		var uid interface{}
		if d.UID != "" {
			uid = d.UID
		}
		var end interface{}
		if d.EndsAt != nil {
			end = *d.EndsAt
		}
		if _, ierr := p.DB.Exec(`
			INSERT INTO events (id,title,description,starts_at,ends_at,location,recurrence_rule,tags,external_uid,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			NewID(), d.Title, d.Description, d.StartsAt, end, d.Location, nil,
			joinTags(normalizeTagList(tags)), uid, NowRFC3339(), NowRFC3339()); ierr != nil {
			return imported, skipped, ierr
		}
		imported++
	}
	return imported, skipped, nil
}
