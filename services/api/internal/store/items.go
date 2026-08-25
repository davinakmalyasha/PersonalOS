package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/domain/knowledge"
)

// ---- Universal capture core: items ----

// Items owns queries against the universal capture core.
type Items struct {
	DB *sql.DB
}

// dbtx is satisfied by both *sql.DB and *sql.Tx; mirror helpers work inside
// caller-managed transactions.
type dbtx interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

type Item struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	Data         string   `json:"data"` // raw JSON object
	Tags         []string `json:"tags"`
	Source       string   `json:"source"`
	SourceItemID *string  `json:"source_item_id"` // native record id when mirrored
	Pinned       bool     `json:"pinned"`
	Archived     bool     `json:"archived"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`

	tagsRaw string
}

const itemCols = `id,type,title,body,data,tags,source,source_item_id,pinned,archived,created_at,updated_at`

func itemScan(i *Item, tagsRaw *string) []interface{} {
	return []interface{}{&i.ID, &i.Type, &i.Title, &i.Body, &i.Data,
		tagsRaw, &i.Source, &i.SourceItemID, &i.Pinned, &i.Archived, &i.CreatedAt, &i.UpdatedAt}
}

func (i *Item) hydrate() { i.Tags = splitTags(i.tagsRaw) }

var itemTypes = map[string]bool{
	"note": true, "bookmark": true, "reading": true,
}

// KnownItemType reports whether t is one of the pillar mirror types.
func KnownItemType(t string) bool { return itemTypes[t] }

// ValidItemType allows any short slug so future types don't need migrations;
// the type column is open vocabulary per spec ("random personal data lands
// here").
func ValidItemType(t string) bool { return t != "" && len(t) <= 40 }

func (it *Items) CreateItem(typ, title, body, data string, tags []string, source string, sourceItemID *string) (Item, error) {
	if !ValidItemType(typ) {
		return Item{}, ErrInvalid
	}
	if strings.TrimSpace(title) == "" || len(title) > 300 {
		return Item{}, ErrInvalid
	}
	if data == "" {
		data = "{}"
	}
	if !json.Valid([]byte(data)) {
		return Item{}, ErrInvalid
	}
	if source == "" {
		source = "manual"
	}
	switch source {
	case "manual", "api", "mcp", "import", "promotion":
	default:
		return Item{}, ErrInvalid
	}
	now := NowRFC3339()
	item := Item{
		ID: NewID(), Type: typ, Title: title, Body: body, Data: data,
		Tags: normalizeTagList(tags), Source: source, SourceItemID: sourceItemID,
		CreatedAt: now, UpdatedAt: now,
	}
	var srcID interface{}
	if sourceItemID != nil && *sourceItemID != "" {
		srcID = *sourceItemID
	} else {
		item.SourceItemID = nil
	}
	_, err := it.DB.Exec(
		`INSERT INTO items (`+itemCols+`) VALUES (?,?,?,?,?,?,?,?,0,0,?,?)`,
		item.ID, item.Type, item.Title, item.Body, item.Data, joinTags(item.Tags),
		item.Source, srcID, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return Item{}, err
	}
	logChange(it.DB, "item", item.ID, "create", item.Title)
	return it.GetItem(item.ID)
}

func (it *Items) GetItem(id string) (Item, error) {
	var i Item
	err := it.DB.QueryRow(`SELECT `+itemCols+` FROM items WHERE id=?`, id).
		Scan(itemScan(&i, &i.tagsRaw)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	i.hydrate()
	return i, nil
}

type ItemFilter struct {
	Type            string
	Tag             string
	Q               string // sanitized at call site or here
	Pinned          bool
	IncludeArchived bool
	Page            int
	PageSize        int
}

func (it *Items) buildItemWhere(f ItemFilter) (string, []interface{}) {
	where := []string{"1=1"}
	args := []interface{}{}
	if f.Type != "" {
		where = append(where, "i.type=?")
		args = append(args, f.Type)
	}
	if f.Tag != "" {
		where = append(where, "i.tags LIKE ?")
		args = append(args, `%"`+f.Tag+`"%`)
	}
	if f.Pinned {
		where = append(where, "i.pinned=1")
	}
	// Archived items are hidden unless explicitly requested via IncludeArchived.
	if !f.IncludeArchived {
		where = append(where, "i.archived=0")
	}
	return strings.Join(where, " AND "), args
}

// ListItems returns paged items; when Q is set it routes through FTS ranked
// search instead of a plain scan.
func (it *Items) ListItems(f ItemFilter) ([]Item, int, error) {
	if f.Q != "" {
		items, err := it.SearchItems(f.Q, nil, f.Type, f.Tag, 100, f.IncludeArchived)
		return items, len(items), err
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 50
	}
	whereSQL, args := it.buildItemWhere(f)

	var total int
	if err := it.DB.QueryRow(`SELECT COUNT(*) FROM items i WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT ` + prefixedCols("i") + ` FROM items i WHERE ` + whereSQL +
		` ORDER BY i.pinned DESC, i.created_at DESC LIMIT ? OFFSET ?`
	pageArgs := append(append([]interface{}{}, args...), f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := it.DB.Query(q, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var i Item
		if err := rows.Scan(itemScan(&i, &i.tagsRaw)...); err != nil {
			return nil, 0, err
		}
		i.hydrate()
		out = append(out, i)
	}
	return out, total, rows.Err()
}

func prefixedCols(p string) string {
	cols := strings.Split(itemCols, ",")
	for i, c := range cols {
		cols[i] = p + "." + c
	}
	return strings.Join(cols, ",")
}

// SearchItems runs an FTS5 MATCH query over items_fts, optionally restricted
// to types/tag, ranked by bm25 (pinned first). Archived rows are hidden unless
// includeArchived is set. Unsearchable/empty input degrades to recent items
// (created DESC).
func (it *Items) SearchItems(q string, types []string, typ, tag string, limit int, includeArchived bool) ([]Item, error) {
	archiveWhere := "i.archived=0"
	if includeArchived {
		archiveWhere = "1=1"
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	match := knowledge.SanitizeFTSQuery(q)
	if match == "" {
		// Recent-captures fallback keeps the search-first UI useful pre-query.
		where := []string{"1=1", archiveWhere}
		args := []interface{}{}
		if typ != "" {
			where = append(where, "i.type=?")
			args = append(args, typ)
		} else if len(types) > 0 {
			where = append(where, placeholders("i.type", types)...)
			args = append(args, toArgs(types)...)
		}
		if tag != "" {
			where = append(where, "i.tags LIKE ?")
			args = append(args, `%"`+tag+`"%`)
		}
		qry := `SELECT ` + prefixedCols("i") + ` FROM items i WHERE ` +
			strings.Join(where, " AND ") + ` ORDER BY i.pinned DESC, i.created_at DESC LIMIT ?`
		args = append(args, limit)
		return it.queryItems(qry, args...)
	}

	where := []string{"items_fts MATCH ?", archiveWhere}
	args := []interface{}{match}
	if typ != "" {
		where = append(where, "i.type=?")
		args = append(args, typ)
	} else if len(types) > 0 {
		where = append(where, placeholders("i.type", types)...)
		args = append(args, toArgs(types)...)
	}
	if tag != "" {
		where = append(where, "i.tags LIKE ?")
		args = append(args, `%"`+tag+`"%`)
	}
	qry := `SELECT ` + prefixedCols("i") + `, bm25(items_fts) AS rank
		FROM items_fts JOIN items i ON i.rowid = items_fts.rowid
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY i.pinned DESC, rank LIMIT ?`
	args = append(args, limit)
	return it.queryItemsRanked(qry, args...)
}
// queryItemsRanked scans the same 12 item columns plus a trailing bm25 rank.
func (it *Items) queryItemsRanked(q string, args ...interface{}) ([]Item, error) {
	rows, err := it.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var i Item
		var rank float64
		dest := append(itemScan(&i, &i.tagsRaw), &rank)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		i.hydrate()
		out = append(out, i)
	}
	return out, rows.Err()
}

func placeholders(col string, values []string) []string {
	if len(values) == 0 {
		return []string{"0=1"}
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(values)), ",")
	return []string{col + " IN (" + marks + ")"}
}

func toArgs(values []string) []interface{} {
	out := make([]interface{}, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

func (it *Items) queryItems(q string, args ...interface{}) ([]Item, error) {
	rows, err := it.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var i Item
		if err := rows.Scan(itemScan(&i, &i.tagsRaw)...); err != nil {
			return nil, err
		}
		i.hydrate()
		out = append(out, i)
	}
	return out, rows.Err()
}

type ItemUpdate struct {
	Title    *string
	Body     *string
	Data     *string
	Tags     *[]string
	Pinned   *bool
	Archived *bool
}

func (it *Items) UpdateItem(id string, u ItemUpdate) (Item, error) {
	cur, err := it.GetItem(id)
	if err != nil {
		return Item{}, err
	}
	if u.Title != nil && strings.TrimSpace(*u.Title) != "" && len(*u.Title) <= 300 {
		cur.Title = *u.Title
	}
	if u.Body != nil {
		cur.Body = *u.Body
	}
	if u.Data != nil {
		if !json.Valid([]byte(*u.Data)) {
			return Item{}, ErrInvalid
		}
		cur.Data = *u.Data
	}
	if u.Tags != nil {
		cur.Tags = normalizeTagList(*u.Tags)
	}
	if u.Pinned != nil {
		cur.Pinned = *u.Pinned
	}
	if u.Archived != nil {
		cur.Archived = *u.Archived
	}
	cur.UpdatedAt = NowRFC3339()
	_, err = it.DB.Exec(
		`UPDATE items SET title=?, body=?, data=?, tags=?, pinned=?, archived=?, updated_at=? WHERE id=?`,
		cur.Title, cur.Body, cur.Data, joinTags(cur.Tags), cur.Pinned, cur.Archived, cur.UpdatedAt, id)
	if err != nil {
		return Item{}, err
	}
	logChange(it.DB, "item", id, "update", cur.Title)
	return it.GetItem(id)
}

func (it *Items) DeleteItem(id string) error {
	cur, err := it.GetItem(id)
	if err != nil {
		return err
	}
	res, err := it.DB.Exec(`DELETE FROM items WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	logChange(it.DB, "item", id, "delete", cur.Title)
	return nil
}

// ---- Tag inventory ----

type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// TagsWithCounts aggregates distinct tags across items, optionally restricted
// to a set of types and/or a prefix.
func (it *Items) TagsWithCounts(types []string, prefix string, limit int) ([]TagCount, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	where := []string{"1=1"}
	args := []interface{}{}
	if len(types) > 0 {
		where = append(where, placeholders("i.type", types)...)
		args = append(args, toArgs(types)...)
	}
	if prefix != "" {
		where = append(where, "je.value LIKE ?")
		args = append(args, prefix+"%")
	}
	q := `SELECT je.value AS tag, COUNT(*) AS n
		FROM items i, json_each(i.tags) je
		WHERE ` + strings.Join(where, " AND ") + `
		GROUP BY je.value ORDER BY n DESC, tag ASC LIMIT ?`
	args = append(args, limit)

	rows, err := it.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TagCount{}
	for rows.Next() {
		var tc TagCount
		if err := rows.Scan(&tc.Tag, &tc.Count); err != nil {
			return nil, err
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}

// ---- Links ----

type Link struct {
	FromID    string `json:"from_id"`
	ToID      string `json:"to_id"`
	ToType    string `json:"to_type,omitempty"`
	ToTitle   string `json:"to_title,omitempty"`
	FromType  string `json:"from_type,omitempty"`
	FromTitle string `json:"from_title,omitempty"`
	Kind      string `json:"kind"`
	CreatedAt string `json:"created_at"`
}

func (it *Items) CreateLink(fromID, toID, kind string) (Link, error) {
	if fromID == toID {
		return Link{}, ErrInvalid
	}
	if kind == "" {
		kind = "related"
	}
	if _, err := it.GetItem(fromID); err != nil {
		return Link{}, err
	}
	if _, err := it.GetItem(toID); err != nil {
		return Link{}, err
	}
	now := NowRFC3339()
	_, err := it.DB.Exec(
		`INSERT OR IGNORE INTO item_links (from_id,to_id,kind,created_at) VALUES (?,?,?,?)`,
		fromID, toID, kind, now)
	if err != nil {
		return Link{}, err
	}
	return Link{FromID: fromID, ToID: toID, Kind: kind, CreatedAt: now}, nil
}

// LinksFor returns outgoing and incoming links with peer titles resolved.
func (it *Items) LinksFor(id string) (outgoing, incoming []Link, err error) {
	outgoing, err = it.linksSide(`
		SELECT l.to_id, i.type, i.title, l.kind, l.created_at
		FROM item_links l JOIN items i ON i.id = l.to_id
		WHERE l.from_id=? ORDER BY l.created_at`, id)
	if err != nil {
		return nil, nil, err
	}
	incoming, err = it.linksSide(`
		SELECT l.from_id, i.type, i.title, l.kind, l.created_at
		FROM item_links l JOIN items i ON i.id = l.from_id
		WHERE l.to_id=? ORDER BY l.created_at`, id)
	return outgoing, incoming, err
}

func (it *Items) linksSide(q, id string) ([]Link, error) {
	rows, err := it.DB.Query(q, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Link{}
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ToID, &l.ToType, &l.ToTitle, &l.Kind, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (it *Items) DeleteLink(fromID, toID, kind string) error {
	res, err := it.DB.Exec(`DELETE FROM item_links WHERE from_id=? AND to_id=? AND kind=?`, fromID, toID, kind)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Mirror helpers (used by Knowledge writes) ----

// mirrorItem upserts an items row projecting a typed record into universal
// search. Mirrors are located by (type, source_item_id).
func mirrorItem(db dbtx, typ, nativeID, title, body string, tags []string, data map[string]interface{}) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}
	now := NowRFC3339()
	var existing string
	err = db.QueryRow(`SELECT id FROM items WHERE type=? AND source_item_id=?`, typ, nativeID).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = db.Exec(
			`INSERT INTO items (`+itemCols+`) VALUES (?,?,?,?,?,?,?,?,0,0,?,?)`,
			NewID(), typ, title, body, string(dataJSON), joinTags(normalizeTagList(tags)),
			"api", nativeID, now, now)
		return err
	}
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE items SET title=?, body=?, data=?, tags=?, updated_at=? WHERE id=?`,
		title, body, string(dataJSON), joinTags(normalizeTagList(tags)), now, existing)
	return err
}

// unmirrorItem removes the projection of a deleted typed record.
func unmirrorItem(db dbtx, typ, nativeID string) error {
	_, err := db.Exec(`DELETE FROM items WHERE type=? AND source_item_id=?`, typ, nativeID)
	return err
}

// defaultJSON normalizes an empty RawMessage to the given default literal.
func defaultJSON(v json.RawMessage) string {
	if len(strings.TrimSpace(string(v))) == 0 {
		return "[]"
	}
	return string(v)
}

// ---- Expiry surfacing ----

type ExpiringItem struct {
	Item
	DateKey  string `json:"date_key"` // which data field carried the date
	Date     string `json:"date"`     // YYYY-MM-DD
	DaysLeft int    `json:"days_left"`
}

// expiryKeys are data-JSON keys scanned for upcoming dates.
var expiryKeys = []string{"expires", "expires_at", "expiry", "due", "due_date", "until", "warranty_end", "end_date", "valid_until"}

// ExpiringItems scans recent items for date-like data fields within the next
// `days` (or already expired within a 30d grace). Personal-scale scan.
func (it *Items) ExpiringItems(days int) ([]ExpiringItem, error) {
	if days < 1 || days > 365 {
		days = 30
	}
	rows, err := it.DB.Query(
		`SELECT ` + itemCols + ` FROM items WHERE archived=0 ORDER BY created_at DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	today := time.Now().UTC()
	out := []ExpiringItem{}
	for rows.Next() {
		var i Item
		if err := rows.Scan(itemScan(&i, &i.tagsRaw)...); err != nil {
			return nil, err
		}
		i.hydrate()
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(i.Data), &data); err != nil {
			continue
		}
		for _, key := range expiryKeys {
			raw, ok := data[key]
			if !ok {
				continue
			}
			s, _ := raw.(string)
			if s == "" {
				continue
			}
			d, perr := time.Parse("2006-01-02", strings.TrimSpace(s))
			if perr != nil {
				continue
			}
			daysLeft := int(d.Sub(today).Hours() / 24)
			if daysLeft > days || daysLeft < -30 {
				continue
			}
			out = append(out, ExpiringItem{Item: i, DateKey: key, Date: d.Format("2006-01-02"), DaysLeft: daysLeft})
			break
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Date < out[b].Date })
	return out, rows.Err()
}
