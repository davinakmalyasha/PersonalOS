package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ---- Highlights as first-class records (phase 12d) ----
//
// Quotes stop being a replace-only JSON blob: each highlight is searchable,
// linkable and scheduled through an SM-2-lite review queue.

type Highlight struct {
	ID           string  `json:"id"`
	ReadingID    string  `json:"reading_id"`
	Quote        string  `json:"quote"`
	Note         string  `json:"note,omitempty"`
	Location     string  `json:"location,omitempty"`
	ReviewCount  int     `json:"review_count"`
	IntervalDays int     `json:"interval_days"`
	NextReviewAt *string `json:"next_review_at"` // nil = due now
	CreatedAt    string  `json:"created_at"`
}

const highlightCols = `id,reading_id,quote,note,location,review_count,interval_days,next_review_at,created_at`

func highlightScan(h *Highlight) []interface{} {
	return []interface{}{&h.ID, &h.ReadingID, &h.Quote, &h.Note, &h.Location,
		&h.ReviewCount, &h.IntervalDays, &h.NextReviewAt, &h.CreatedAt}
}

var sm2Ladder = []int{1, 3, 7, 14, 30, 60}

// CreateHighlight attaches a quote to a reading and mirrors it into the
// universal items FTS so highlights surface in /v1/search.
func (k *Knowledge) CreateHighlight(readingID, quote, note, location string) (Highlight, error) {
	if strings.TrimSpace(quote) == "" {
		return Highlight{}, ErrInvalid
	}
	rd, err := k.GetReading(readingID)
	if err != nil {
		return Highlight{}, err
	}
	h := Highlight{ID: NewID(), ReadingID: readingID, Quote: strings.TrimSpace(quote),
		Note: note, Location: location, CreatedAt: NowRFC3339()}
	tx, err := k.DB.Begin()
	if err != nil {
		return Highlight{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`INSERT INTO highlights (`+highlightCols+`) VALUES (?,?,?,?,?,0,0,NULL,?)`,
		h.ID, h.ReadingID, h.Quote, h.Note, h.Location, h.CreatedAt); err != nil {
		return Highlight{}, err
	}
	title := "“" + firstN(h.Quote, 80) + "”"
	if err := mirrorItem(tx, "highlight", h.ID, title, rd.Title+" — "+h.Quote, rd.Tags,
		map[string]interface{}{"reading_id": readingID}); err != nil {
		return Highlight{}, err
	}
	logChange(tx, "highlight", h.ID, "create", title)
	if err := tx.Commit(); err != nil {
		return Highlight{}, err
	}
	k.touchReading(readingID)
	return k.GetHighlight(h.ID)
}

// touchReading bumps updated_at so the reading board reorders naturally.
func (k *Knowledge) touchReading(id string) {
	_, _ = k.DB.Exec(`UPDATE reading_list SET updated_at=? WHERE id=?`, NowRFC3339(), id)
}

func (k *Knowledge) GetHighlight(id string) (Highlight, error) {
	var h Highlight
	err := k.DB.QueryRow(`SELECT `+highlightCols+` FROM highlights WHERE id=?`, id).
		Scan(highlightScan(&h)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Highlight{}, ErrNotFound
	}
	return h, err
}

// HighlightsFor lists one reading's quotes newest-first.
func (k *Knowledge) HighlightsFor(readingID string) ([]Highlight, error) {
	rows, err := k.DB.Query(
		`SELECT `+highlightCols+` FROM highlights WHERE reading_id=? ORDER BY created_at DESC`, readingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Highlight{}
	for rows.Next() {
		var h Highlight
		if err := rows.Scan(highlightScan(&h)...); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// DeleteHighlight removes the row and its search mirror.
func (k *Knowledge) DeleteHighlight(id string) error {
	cur, err := k.GetHighlight(id)
	if err != nil {
		return err
	}
	tx, err := k.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM highlights WHERE id=?`, id); err != nil {
		return err
	}
	if err := unmirrorItem(tx, "highlight", id); err != nil {
		return err
	}
	logChange(tx, "highlight", id, "delete", firstN(cur.Quote, 60))
	return tx.Commit()
}

// DueHighlights returns quotes whose review date has passed (nil = due now).
func (k *Knowledge) DueHighlights(limit int) ([]Highlight, error) {
	if limit < 1 || limit > 50 {
		limit = 10
	}
	now := NowRFC3339()
	rows, err := k.DB.Query(`
		SELECT `+highlightCols+` FROM highlights
		WHERE next_review_at IS NULL OR next_review_at <= ?
		ORDER BY COALESCE(next_review_at, created_at) ASC LIMIT ?`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Highlight{}
	for rows.Next() {
		var h Highlight
		if err := rows.Scan(highlightScan(&h)...); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// ReviewHighlight advances (remembered=true) or resets (false) the SM-2-lite
// schedule: intervals climb 1→3→7→14→30→60 days; a miss drops back to due-now.
func (k *Knowledge) ReviewHighlight(id string, remembered bool) (Highlight, error) {
	cur, err := k.GetHighlight(id)
	if err != nil {
		return Highlight{}, err
	}
	interval := 0
	var nextDue *string
	if remembered {
		next := sm2Ladder[len(sm2Ladder)-1]
		for _, step := range sm2Ladder {
			if step > cur.IntervalDays {
				next = step
				break
			}
		}
		interval = next
		d := time.Now().UTC().AddDate(0, 0, interval).Format(time.RFC3339)
		nextDue = &d
	}
	now := NowRFC3339()
	if _, err := k.DB.Exec(`
		UPDATE highlights SET review_count=review_count+1, interval_days=?, last_reviewed=?, next_review_at=?
		WHERE id=?`,
		interval, now, now2Ptr(nextDue), id); err != nil {
		return Highlight{}, err
	}
	logChange(k.DB, "highlight", id, "update", "reviewed")
	return k.GetHighlight(id)
}

func now2Ptr(p *string) interface{} {
	if p == nil {
		return nil
	}
	return *p
}
