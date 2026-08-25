package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ---- Search v2: one "find anything" call for agents ----

// SearchHit is a single typed result from the unified search union.
type SearchHit struct {
	Kind        string `json:"kind"` // item|task|meal|workout|transaction
	ID          string `json:"id"`
	Title       string `json:"title"`
	Sub         string `json:"sub,omitempty"`
	Date        string `json:"date,omitempty"` // YYYY-MM-DD when meaningful
	AmountMinor *int64 `json:"amount_minor,omitempty"`
}

// GlobalSearch unions ranked item-FTS hits with LIKE scans over the typed
// pillars so agents answer "where was that?" with one call. Empty queries
// degrade to the most recent row of every kind.
func GlobalSearch(db *sql.DB, q string, limit int) ([]SearchHit, error) {
	if limit < 1 || limit > 50 {
		limit = 10
	}
	items := &Items{DB: db}
	itemRows, err := items.SearchItems(q, nil, "", "", limit, false)
	if err != nil {
		return nil, err
	}
	out := make([]SearchHit, 0, len(itemRows)+4*limit)
	for _, i := range itemRows {
		out = append(out, SearchHit{
			Kind:  "item",
			ID:    i.ID,
			Title: i.Title,
			Sub:   firstN(oneLine(i.Body), 140),
			Date:  i.CreatedAt[:10],
		})
	}

	pat := "%" + strings.ToLower(strings.TrimSpace(q)) + "%"

	hits, err := scanHits(db, `
		SELECT id, title,
			CASE WHEN notes='' THEN status ELSE status || ' - ' || notes END,
			COALESCE(due_date,''), 'task'
		FROM tasks
		WHERE (?='' OR LOWER(title) LIKE ? OR LOWER(notes) LIKE ?)
		ORDER BY updated_at DESC LIMIT ?`, pat, limit)
	if err != nil {
		return nil, err
	}
	out = append(out, hits...)

	hits, err = scanHits(db, `
		SELECT id,title,notes,'',substr(eaten_at,1,10),'meal' FROM meals
		WHERE (?='' OR LOWER(title) LIKE ? OR LOWER(notes) LIKE ?)
		ORDER BY eaten_at DESC LIMIT ?`, pat, limit)
	if err != nil {
		return nil, err
	}
	out = append(out, hits...)

	hits, err = scanHits(db, `
		SELECT id,COALESCE(NULLIF(title,''),'workout'),notes,'',substr(performed_at,1,10),'workout' FROM workouts
		WHERE (?='' OR LOWER(COALESCE(title,'')) LIKE ? OR LOWER(notes) LIKE ?)
		ORDER BY performed_at DESC LIMIT ?`, pat, limit)
	if err != nil {
		return nil, err
	}
	out = append(out, hits...)

	txns, err := scanTxns(db, pat, limit)
	if err != nil {
		return nil, err
	}
	out = append(out, txns...)
	return out, nil
}

func scanHits(db *sql.DB, query, pat string, limit int) ([]SearchHit, error) {
	rows, err := db.Query(query, pat, pat, pat, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SearchHit{}
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(&h.ID, &h.Title, &h.Sub, &h.Date, &h.Kind); err != nil {
			return nil, err
		}
		h.Sub = firstN(oneLine(h.Sub), 140)
		out = append(out, h)
	}
	return out, rows.Err()
}

func scanTxns(db *sql.DB, pat string, limit int) ([]SearchHit, error) {
	rows, err := db.Query(`
		SELECT id,merchant,raw_description,date,amount FROM transactions
		WHERE (?='' OR LOWER(merchant) LIKE ? OR LOWER(raw_description) LIKE ?)
		ORDER BY date DESC, created_at DESC LIMIT ?`,
		pat, pat, pat, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SearchHit{}
	for rows.Next() {
		var h SearchHit
		var amount int64
		if err := rows.Scan(&h.ID, &h.Title, &h.Sub, &h.Date, &amount); err != nil {
			return nil, err
		}
		h.Kind = "transaction"
		h.AmountMinor = &amount
		out = append(out, h)
	}
	return out, rows.Err()
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// ---- Full export ----

// exportTables enumerates every user-data table; new tables must register here
// or /v1/export won't include them.
var exportTables = []string{
	"accounts", "categories", "transactions", "categorization_rules", "budgets",
	"merchant_aliases", "goals",
	"tasks", "habits", "habit_checkoffs", "events", "event_overrides",
	"notes", "bookmarks", "reading_list", "items", "item_links",
	"meals", "recipes", "grocery_items", "workouts", "body_metrics",
	"saved_searches",
}

// ExportAll dumps every table as JSON-ready rows keyed by table name.
func ExportAll(db *sql.DB) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	for _, table := range exportTables {
		rows, err := db.Query(`SELECT * FROM ` + table)
		if err != nil {
			return nil, fmt.Errorf("export %s: %w", table, err)
		}
		rowsOut := []map[string]interface{}{}
		cols, _ := rows.Columns()
		for rows.Next() {
			vals := make([]interface{}, len(cols))
			dests := make([]interface{}, len(cols))
			for i := range vals {
				dests[i] = &vals[i]
			}
			if err := rows.Scan(dests...); err != nil {
				rows.Close()
				return nil, fmt.Errorf("export %s: %w", table, err)
			}
			row := map[string]interface{}{}
			for i, c := range cols {
				row[c] = normalizeExportVal(vals[i])
			}
			rowsOut = append(rowsOut, row)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("export %s: %w", table, err)
		}
		out[table] = rowsOut
	}
	return out, nil
}

func normalizeExportVal(v interface{}) interface{} {
	switch t := v.(type) {
	case []byte:
		return string(t)
	case time.Time:
		return t.Format(time.RFC3339)
	default:
		return v
	}
}

// ---- Resurface: on-this-day memory ----

type Resurfaced struct {
	Kind    string  `json:"kind"` // note|bookmark|reading|item
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	URL     *string `json:"url,omitempty"`
	Year    int     `json:"year"`
	Snippet string  `json:"snippet,omitempty"`
}

// Resurface finds notes/bookmarks/readings plus standalone items created on
// the same month-day in earlier years ("on this day").
func Resurface(db *sql.DB, date string, limit int) ([]Resurfaced, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, ErrInvalid
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	mday := t.Format("01-02")
	// Compare years as zero-padded TEXT: in SQLite, TEXT never compares
	// less-than an INTEGER parameter.
	thisYear := t.Format("2006")

	rows, err := db.Query(`
		SELECT 'note', id, title, NULL, CAST(substr(created_at,1,4) AS INTEGER), substr(body,1,160)
		FROM notes
		WHERE substr(created_at,6,5)=? AND substr(created_at,1,4)<?
		UNION ALL
		SELECT 'bookmark', id, title, url, CAST(substr(created_at,1,4) AS INTEGER), substr(description,1,160)
		FROM bookmarks
		WHERE substr(created_at,6,5)=? AND substr(created_at,1,4)<?
		UNION ALL
		SELECT 'reading', id, title, url, CAST(substr(created_at,1,4) AS INTEGER), substr(notes,1,160)
		FROM reading_list
		WHERE substr(created_at,6,5)=? AND substr(created_at,1,4)<?
		UNION ALL
		SELECT 'item', id, title, NULL, CAST(substr(created_at,1,4) AS INTEGER), substr(body,1,160)
		FROM items
		WHERE type NOT IN ('note','bookmark','reading')
		  AND substr(created_at,6,5)=? AND substr(created_at,1,4)<?
		LIMIT ?`,
		mday, thisYear, mday, thisYear, mday, thisYear, mday, thisYear, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Resurfaced{}
	for rows.Next() {
		var r Resurfaced
		var snippet sql.NullString
		if err := rows.Scan(&r.Kind, &r.ID, &r.Title, &r.URL, &r.Year, &snippet); err != nil {
			return nil, err
		}
		r.Snippet = oneLine(snippet.String)
		out = append(out, r)
	}
	return out, rows.Err()
}
