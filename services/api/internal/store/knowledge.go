package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/domain/knowledge"
)

// ---- Knowledge pillar: notes, bookmarks, reading_list ----

// Knowledge owns the three knowledge tables; every write mirrors to items in
// the same transaction so unified FTS stays consistent.
type Knowledge struct {
	DB *sql.DB
}

// ---- Notes ----

type Note struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Tags       []string `json:"tags"`
	Pinned     bool     `json:"pinned"`
	ArchivedAt *string  `json:"archived_at"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`

	tagsRaw string
}

type NoteFilter struct {
	Tag      string
	Q        string
	Pinned   bool
	Archived bool
	Page     int
	PageSize int
}

const noteCols = `id,title,body,tags,pinned,archived_at,created_at,updated_at`

func noteScan(n *Note, tagsRaw *string) []interface{} {
	return []interface{}{&n.ID, &n.Title, &n.Body, tagsRaw,
		&n.Pinned, &n.ArchivedAt, &n.CreatedAt, &n.UpdatedAt}
}

func (k *Knowledge) noteMirror(n Note) error {
	return mirrorItem(k.DB, "note", n.ID, n.Title, n.Body, n.Tags,
		map[string]interface{}{"pinned": n.Pinned, "archived": n.ArchivedAt != nil})
}

func (k *Knowledge) CreateNote(title, body string, tags []string, pinned bool) (Note, error) {
	if strings.TrimSpace(title) == "" {
		return Note{}, ErrInvalid
	}
	now := NowRFC3339()
	n := Note{ID: NewID(), Title: title, Body: body, Tags: normalizeTagList(tags),
		Pinned: pinned, CreatedAt: now, UpdatedAt: now}
	tx, err := k.DB.Begin()
	if err != nil {
		return Note{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`INSERT INTO notes (`+noteCols+`) VALUES (?,?,?,?,?,?,?,?)`,
		n.ID, n.Title, n.Body, joinTags(n.Tags), n.Pinned, nil, n.CreatedAt, n.UpdatedAt); err != nil {
		return Note{}, err
	}
	if err := mirrorItem(tx, "note", n.ID, n.Title, n.Body, n.Tags,
		map[string]interface{}{"pinned": n.Pinned, "archived": false}); err != nil {
		return Note{}, err
	}
	logChange(tx, "note", n.ID, "create", n.Title)
	if err := tx.Commit(); err != nil {
		return Note{}, err
	}
	return k.GetNote(n.ID)
}

func (k *Knowledge) GetNote(id string) (Note, error) {
	var n Note
	err := k.DB.QueryRow(`SELECT `+noteCols+` FROM notes WHERE id=?`, id).
		Scan(noteScan(&n, &n.tagsRaw)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, ErrNotFound
	}
	if err != nil {
		return Note{}, err
	}
	n.Tags = splitTags(n.tagsRaw)
	return n, nil
}

func (k *Knowledge) buildNoteWhere(f NoteFilter) (string, []interface{}) {
	where := []string{"1=1"}
	args := []interface{}{}
	if f.Archived {
		where = append(where, "archived_at IS NOT NULL")
	} else {
		where = append(where, "archived_at IS NULL")
	}
	if f.Pinned {
		where = append(where, "pinned=1")
	}
	if f.Tag != "" {
		where = append(where, "tags LIKE ?")
		args = append(args, `%"`+f.Tag+`"%`)
	}
	if f.Q != "" {
		where = append(where, "(LOWER(title) LIKE ? OR LOWER(body) LIKE ?)")
		pat := "%" + strings.ToLower(f.Q) + "%"
		args = append(args, pat, pat)
	}
	return strings.Join(where, " AND "), args
}

func (k *Knowledge) ListNotes(f NoteFilter) ([]Note, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 50
	}
	whereSQL, args := k.buildNoteWhere(f)
	var total int
	if err := k.DB.QueryRow(`SELECT COUNT(*) FROM notes WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := `ORDER BY pinned DESC, created_at DESC`
	q := `SELECT ` + noteCols + ` FROM notes WHERE ` + whereSQL + ` ` + order + ` LIMIT ? OFFSET ?`
	pageArgs := append(append([]interface{}{}, args...), f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := k.DB.Query(q, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Note{}
	for rows.Next() {
		var n Note
		if err := rows.Scan(noteScan(&n, &n.tagsRaw)...); err != nil {
			return nil, 0, err
		}
		n.Tags = splitTags(n.tagsRaw)
		out = append(out, n)
	}
	return out, total, rows.Err()
}

type NoteUpdate struct {
	Title   *string
	Body    *string
	Tags    *[]string
	Pinned  *bool
	Archive *bool // true â†’ set archived_at; false â†’ clear
}

func (k *Knowledge) UpdateNote(id string, u NoteUpdate) (Note, error) {
	cur, err := k.GetNote(id)
	if err != nil {
		return Note{}, err
	}
	if u.Title != nil && strings.TrimSpace(*u.Title) != "" {
		cur.Title = *u.Title
	}
	if u.Body != nil {
		cur.Body = *u.Body
	}
	if u.Tags != nil {
		cur.Tags = normalizeTagList(*u.Tags)
	}
	if u.Pinned != nil {
		cur.Pinned = *u.Pinned
	}
	if u.Archive != nil {
		if *u.Archive {
			now := NowRFC3339()
			cur.ArchivedAt = &now
		} else {
			cur.ArchivedAt = nil
		}
	}
	cur.UpdatedAt = NowRFC3339()

	tx, err := k.DB.Begin()
	if err != nil {
		return Note{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var archived interface{}
	if cur.ArchivedAt != nil {
		archived = *cur.ArchivedAt
	}
	if _, err := tx.Exec(
		`UPDATE notes SET title=?, body=?, tags=?, pinned=?, archived_at=?, updated_at=? WHERE id=?`,
		cur.Title, cur.Body, joinTags(cur.Tags), cur.Pinned, archived, cur.UpdatedAt, id); err != nil {
		return Note{}, err
	}
	if err := mirrorItem(tx, "note", cur.ID, cur.Title, cur.Body, cur.Tags,
		map[string]interface{}{"pinned": cur.Pinned, "archived": cur.ArchivedAt != nil}); err != nil {
		return Note{}, err
	}
	logChange(tx, "note", id, "update", cur.Title)
	if err := tx.Commit(); err != nil {
		return Note{}, err
	}
	return k.GetNote(id)
}

func (k *Knowledge) DeleteNote(id string) error {
	n, err := k.GetNote(id)
	if err != nil {
		return err
	}
	tx, err := k.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM notes WHERE id=?`, id); err != nil {
		return err
	}
	if err := unmirrorItem(tx, "note", id); err != nil {
		return err
	}
	logChange(tx, "note", id, "delete", n.Title)
	return tx.Commit()
}

// ---- Bookmarks ----

type Bookmark struct {
	ID          string   `json:"id"`
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`

	tagsRaw string
}

const bookmarkCols = `id,url,title,description,tags,created_at,updated_at`

func bookmarkScan(b *Bookmark, tagsRaw *string) []interface{} {
	return []interface{}{&b.ID, &b.URL, &b.Title, &b.Description, tagsRaw, &b.CreatedAt, &b.UpdatedAt}
}

// CreateBookmark normalizes the URL and dedupes: when the canonical URL
// already exists the existing row is returned with Duplicate=true (200 at the
// API layer).
func (k *Knowledge) CreateBookmark(rawURL, title, description string, tags []string) (Bookmark, bool, error) {
	canonical, err := knowledge.NormalizeURL(rawURL)
	if err != nil {
		return Bookmark{}, false, ErrInvalid
	}
	existing, err := k.bookmarkByURL(canonical)
	if err == nil {
		return existing, true, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Bookmark{}, false, err
	}
	if strings.TrimSpace(title) == "" {
		title = canonicalHost(canonical)
	}
	now := NowRFC3339()
	b := Bookmark{ID: NewID(), URL: canonical, Title: title, Description: description,
		Tags: normalizeTagList(tags), CreatedAt: now, UpdatedAt: now}
	tx, err := k.DB.Begin()
	if err != nil {
		return Bookmark{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`INSERT INTO bookmarks (`+bookmarkCols+`) VALUES (?,?,?,?,?,?,?)`,
		b.ID, b.URL, b.Title, b.Description, joinTags(b.Tags), b.CreatedAt, b.UpdatedAt); err != nil {
		return Bookmark{}, false, err
	}
	if err := mirrorItem(tx, "bookmark", b.ID, b.Title, b.Description, b.Tags,
		map[string]interface{}{"url": b.URL}); err != nil {
		return Bookmark{}, false, err
	}
	logChange(tx, "bookmark", b.ID, "create", b.Title)
	if err := tx.Commit(); err != nil {
		return Bookmark{}, false, err
	}
	return b, false, nil
}

func canonicalHost(u string) string {
	s := strings.TrimPrefix(u, "https://")
	s = strings.TrimPrefix(s, "http://")
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return s
}

func (k *Knowledge) bookmarkByURL(u string) (Bookmark, error) {
	var b Bookmark
	err := k.DB.QueryRow(`SELECT `+bookmarkCols+` FROM bookmarks WHERE url=?`, u).
		Scan(bookmarkScan(&b, &b.tagsRaw)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Bookmark{}, ErrNotFound
	}
	if err != nil {
		return Bookmark{}, err
	}
	b.Tags = splitTags(b.tagsRaw)
	return b, nil
}

func (k *Knowledge) GetBookmark(id string) (Bookmark, error) {
	var b Bookmark
	err := k.DB.QueryRow(`SELECT `+bookmarkCols+` FROM bookmarks WHERE id=?`, id).
		Scan(bookmarkScan(&b, &b.tagsRaw)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Bookmark{}, ErrNotFound
	}
	if err != nil {
		return Bookmark{}, err
	}
	b.Tags = splitTags(b.tagsRaw)
	return b, nil
}

type BookmarkFilter struct {
	Tag      string
	Q        string
	Page     int
	PageSize int
}

func (k *Knowledge) ListBookmarks(f BookmarkFilter) ([]Bookmark, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 50
	}
	where := []string{"1=1"}
	args := []interface{}{}
	if f.Tag != "" {
		where = append(where, "tags LIKE ?")
		args = append(args, `%"`+f.Tag+`"%`)
	}
	if f.Q != "" {
		where = append(where, "(LOWER(title) LIKE ? OR LOWER(url) LIKE ? OR LOWER(description) LIKE ?)")
		pat := "%" + strings.ToLower(f.Q) + "%"
		args = append(args, pat, pat, pat)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := k.DB.QueryRow(`SELECT COUNT(*) FROM bookmarks WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT ` + bookmarkCols + ` FROM bookmarks WHERE ` + whereSQL +
		` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	pageArgs := append(append([]interface{}{}, args...), f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := k.DB.Query(q, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Bookmark{}
	for rows.Next() {
		var b Bookmark
		if err := rows.Scan(bookmarkScan(&b, &b.tagsRaw)...); err != nil {
			return nil, 0, err
		}
		b.Tags = splitTags(b.tagsRaw)
		out = append(out, b)
	}
	return out, total, rows.Err()
}

type BookmarkUpdate struct {
	Title       *string
	Description *string
	Tags        *[]string
}

func (k *Knowledge) UpdateBookmark(id string, u BookmarkUpdate) (Bookmark, error) {
	cur, err := k.GetBookmark(id)
	if err != nil {
		return Bookmark{}, err
	}
	if u.Title != nil && strings.TrimSpace(*u.Title) != "" {
		cur.Title = *u.Title
	}
	if u.Description != nil {
		cur.Description = *u.Description
	}
	if u.Tags != nil {
		cur.Tags = normalizeTagList(*u.Tags)
	}
	cur.UpdatedAt = NowRFC3339()

	tx, err := k.DB.Begin()
	if err != nil {
		return Bookmark{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`UPDATE bookmarks SET title=?, description=?, tags=?, updated_at=? WHERE id=?`,
		cur.Title, cur.Description, joinTags(cur.Tags), cur.UpdatedAt, id); err != nil {
		return Bookmark{}, err
	}
	if err := mirrorItem(tx, "bookmark", cur.ID, cur.Title, cur.Description, cur.Tags,
		map[string]interface{}{"url": cur.URL}); err != nil {
		return Bookmark{}, err
	}
	logChange(tx, "bookmark", id, "update", cur.Title)
	if err := tx.Commit(); err != nil {
		return Bookmark{}, err
	}
	return k.GetBookmark(id)
}

func (k *Knowledge) DeleteBookmark(id string) error {
	b, err := k.GetBookmark(id)
	if err != nil {
		return err
	}
	tx, err := k.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM bookmarks WHERE id=?`, id); err != nil {
		return err
	}
	if err := unmirrorItem(tx, "bookmark", id); err != nil {
		return err
	}
	logChange(tx, "bookmark", id, "delete", b.Title)
	return tx.Commit()
}

// ---- Reading list ----

type Reading struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Author     *string  `json:"author"`
	URL        *string  `json:"url"`
	Status     string   `json:"status"`
	Rating     *int     `json:"rating"`
	Notes      string   `json:"notes"`
	Tags       []string `json:"tags"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
	FinishedAt *string  `json:"finished_at"`

	Highlights json.RawMessage `json:"highlights"` // [{quote, note?, at}]

	tagsRaw       string
	highlightsRaw string
}

const readingCols = `id,title,author,url,status,rating,notes,tags,highlights,created_at,updated_at,finished_at`

func readingScan(rd *Reading, tagsRaw *string) []interface{} {
	return []interface{}{&rd.ID, &rd.Title, &rd.Author, &rd.URL, &rd.Status, &rd.Rating,
		&rd.Notes, tagsRaw, &rd.highlightsRaw, &rd.CreatedAt, &rd.UpdatedAt, &rd.FinishedAt}
}

// hydrateReading converts raw scans into exported JSON fields.
func hydrateReading(rd *Reading) {
	rd.Tags = splitTags(rd.tagsRaw)
	rd.Highlights = json.RawMessage(defaultJSON([]byte(rd.highlightsRaw)))
}

var readingStatuses = map[string]bool{"to-read": true, "reading": true, "done": true}

func (k *Knowledge) CreateReading(title string, author, url *string, status string, rating *int, notes string, tags []string) (Reading, error) {
	if strings.TrimSpace(title) == "" {
		return Reading{}, ErrInvalid
	}
	if status == "" {
		status = "to-read"
	}
	if !readingStatuses[status] {
		return Reading{}, ErrInvalid
	}
	if rating != nil && (*rating < 1 || *rating > 5) {
		return Reading{}, ErrInvalid
	}
	var finished *string
	if status == "done" {
		now := NowRFC3339()
		finished = &now
	}
	rd := Reading{
		ID: NewID(), Title: title, Author: author, URL: url, Status: status,
		Rating: rating, Notes: notes, Tags: normalizeTagList(tags),
		CreatedAt: NowRFC3339(), UpdatedAt: "", FinishedAt: finished,
	}
	rd.UpdatedAt = rd.CreatedAt

	tx, err := k.DB.Begin()
	if err != nil {
		return Reading{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var authorV, urlV, ratingV, finV interface{}
	if author != nil && *author != "" {
		authorV = *author
	} else {
		rd.Author = nil
	}
	if url != nil && *url != "" {
		urlV = *url
	} else {
		rd.URL = nil
	}
	if rating != nil {
		ratingV = *rating
	}
	if finished != nil {
		finV = *finished
	}
	if _, err := tx.Exec(
		`INSERT INTO reading_list (`+readingCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		rd.ID, rd.Title, authorV, urlV, rd.Status, ratingV, rd.Notes,
		joinTags(rd.Tags), defaultJSON(rd.Highlights), rd.CreatedAt, rd.UpdatedAt, finV); err != nil {
		return Reading{}, err
	}
	if err := mirrorReading(tx, rd); err != nil {
		return Reading{}, err
	}
	logChange(tx, "reading", rd.ID, "create", rd.Title)
	if err := tx.Commit(); err != nil {
		return Reading{}, err
	}
	return k.GetReading(rd.ID)
}

func mirrorReading(db dbtx, rd Reading) error {
	return mirrorItem(db, "reading", rd.ID, rd.Title, rd.Notes, rd.Tags, map[string]interface{}{
		"author": rd.Author, "url": rd.URL, "status": rd.Status,
		"rating": rd.Rating, "finished_at": rd.FinishedAt,
		"highlights": rd.Highlights,
	})
}

func (k *Knowledge) GetReading(id string) (Reading, error) {
	var rd Reading
	err := k.DB.QueryRow(`SELECT `+readingCols+` FROM reading_list WHERE id=?`, id).
		Scan(readingScan(&rd, &rd.tagsRaw)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Reading{}, ErrNotFound
	}
	if err != nil {
		return Reading{}, err
	}
	hydrateReading(&rd)
	return rd, nil
}

type ReadingFilter struct {
	Status   string
	Tag      string
	Q        string
	Page     int
	PageSize int
}

func (k *Knowledge) ListReadings(f ReadingFilter) ([]Reading, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 100
	}
	where := []string{"1=1"}
	args := []interface{}{}
	if f.Status != "" {
		if !readingStatuses[f.Status] {
			return nil, 0, ErrInvalid
		}
		where = append(where, "status=?")
		args = append(args, f.Status)
	}
	if f.Tag != "" {
		where = append(where, "tags LIKE ?")
		args = append(args, `%"`+f.Tag+`"%`)
	}
	if f.Q != "" {
		where = append(where, "(LOWER(title) LIKE ? OR LOWER(COALESCE(author,'')) LIKE ?)")
		pat := "%" + strings.ToLower(f.Q) + "%"
		args = append(args, pat, pat)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := k.DB.QueryRow(`SELECT COUNT(*) FROM reading_list WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT ` + readingCols + ` FROM reading_list WHERE ` + whereSQL +
		` ORDER BY updated_at DESC LIMIT ? OFFSET ?`
	pageArgs := append(append([]interface{}{}, args...), f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := k.DB.Query(q, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Reading{}
	for rows.Next() {
		var rd Reading
		if err := rows.Scan(readingScan(&rd, &rd.tagsRaw)...); err != nil {
			return nil, 0, err
		}
		hydrateReading(&rd)
		out = append(out, rd)
	}
	return out, total, rows.Err()
}

type ReadingUpdate struct {
	Title      *string
	Author     **string
	URL        **string
	Status     *string
	Rating     **int
	Notes      *string
	Tags       *[]string
	Highlights *json.RawMessage // replacement array of {quote, note?, at}
}

func (k *Knowledge) UpdateReading(id string, u ReadingUpdate) (Reading, error) {
	cur, err := k.GetReading(id)
	if err != nil {
		return Reading{}, err
	}
	if u.Title != nil && strings.TrimSpace(*u.Title) != "" {
		cur.Title = *u.Title
	}
	if u.Author != nil {
		cur.Author = *u.Author // ptr-to-nil clears
	}
	if u.URL != nil {
		cur.URL = *u.URL
	}
	statusChanged := false
	if u.Status != nil {
		if !readingStatuses[*u.Status] {
			return Reading{}, ErrInvalid
		}
		statusChanged = cur.Status != *u.Status
		cur.Status = *u.Status
	}
	if u.Rating != nil {
		if *u.Rating == nil {
			cur.Rating = nil
		} else if **u.Rating < 1 || **u.Rating > 5 {
			return Reading{}, ErrInvalid
		} else {
			v := **u.Rating
			cur.Rating = &v
		}
	}
	if u.Notes != nil {
		cur.Notes = *u.Notes
	}
	if u.Highlights != nil {
		hl := strings.TrimSpace(string(*u.Highlights))
		if hl != "" && (!json.Valid([]byte(hl)) || !strings.HasPrefix(hl, "[")) {
			return Reading{}, ErrInvalid
		}
		if hl == "" {
			hl = "[]"
		}
		cur.Highlights = json.RawMessage(hl)
	}
	if u.Tags != nil {
		cur.Tags = normalizeTagList(*u.Tags)
	}
	if statusChanged {
		if cur.Status == "done" && cur.FinishedAt == nil {
			now := NowRFC3339()
			cur.FinishedAt = &now
		} else if cur.Status != "done" {
			cur.FinishedAt = nil
		}
	}
	cur.UpdatedAt = NowRFC3339()

	tx, err := k.DB.Begin()
	if err != nil {
		return Reading{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var authorV, urlV, ratingV, finV interface{}
	if cur.Author != nil {
		authorV = *cur.Author
	}
	if cur.URL != nil {
		urlV = *cur.URL
	}
	if cur.Rating != nil {
		ratingV = *cur.Rating
	}
	if cur.FinishedAt != nil {
		finV = *cur.FinishedAt
	}
	if _, err := tx.Exec(
		`UPDATE reading_list SET title=?, author=?, url=?, status=?, rating=?, notes=?, tags=?, highlights=?, updated_at=?, finished_at=? WHERE id=?`,
		cur.Title, authorV, urlV, cur.Status, ratingV, cur.Notes, joinTags(cur.Tags), defaultJSON(cur.Highlights), cur.UpdatedAt, finV, id); err != nil {
		return Reading{}, err
	}
	if err := mirrorReading(tx, cur); err != nil {
		return Reading{}, err
	}
	logChange(tx, "reading", id, "update", cur.Title)
	if err := tx.Commit(); err != nil {
		return Reading{}, err
	}
	return k.GetReading(id)
}

func (k *Knowledge) DeleteReading(id string) error {
	rd, err := k.GetReading(id)
	if err != nil {
		return err
	}
	tx, err := k.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM reading_list WHERE id=?`, id); err != nil {
		return err
	}
	if err := unmirrorItem(tx, "reading", id); err != nil {
		return err
	}
	logChange(tx, "reading", id, "delete", rd.Title)
	return tx.Commit()
}
