package store

import (
	"database/sql"
	"strings"
	"time"
)

// ---- Daily note: one scratch note per day ----

// DailyNoteTitle is the canonical title for a day's scratch note.
func DailyNoteTitle(day string) string { return "Daily " + day }

// DailyNote returns the day's scratch note, creating it (tagged "daily") on
// first touch. Second return is true when freshly created.
func (k *Knowledge) DailyNote(day string) (Note, bool, error) {
	if _, err := time.Parse("2006-01-02", day); err != nil {
		return Note{}, false, ErrInvalid
	}
	var id string
	err := k.DB.QueryRow(
		`SELECT id FROM notes WHERE title=? ORDER BY created_at LIMIT 1`,
		DailyNoteTitle(day)).Scan(&id)
	if err == nil {
		n, gerr := k.GetNote(id)
		return n, false, gerr
	}
	if err != sql.ErrNoRows {
		return Note{}, false, err
	}
	n, cerr := k.CreateNote(DailyNoteTitle(day), "", []string{"daily"}, false)
	return n, cerr == nil, cerr
}

// AppendDailyNote adds a bullet line to the day's note (creating it when
// missing) and returns the updated note.
func (k *Knowledge) AppendDailyNote(day, text string) (Note, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Note{}, ErrInvalid
	}
	n, _, err := k.DailyNote(day)
	if err != nil {
		return Note{}, err
	}
	body := strings.TrimRight(n.Body, "\n")
	if body != "" {
		body += "\n"
	}
	body += "- " + text
	return k.UpdateNote(n.ID, NoteUpdate{Body: &body})
}
