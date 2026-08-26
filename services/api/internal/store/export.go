package store

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// ---- Markdown vault export (phase 13d) ----

// AllItemsByTypes returns every item of the given types (archived included),
// oldest first, no paging — export wants the full set.
func (it *Items) AllItemsByTypes(types []string) ([]Item, error) {
	if len(types) == 0 {
		return nil, nil
	}
	ph := strings.TrimRight(strings.Repeat("?,", len(types)), ",")
	args := make([]interface{}, 0, len(types))
	for _, t := range types {
		args = append(args, t)
	}
	rows, err := it.DB.Query(
		`SELECT `+itemCols+` FROM items WHERE type IN (`+ph+`) ORDER BY created_at ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var i Item
		if err := rows.Scan(itemScan(&i, &i.tagsRaw)...); err != nil {
			return nil, err
		}
		i.Tags = splitTags(i.tagsRaw)
		out = append(out, i)
	}
	return out, rows.Err()
}

// AllHighlights returns every highlight (review state included), oldest first.
func (k *Knowledge) AllHighlights() ([]Highlight, error) {
	rows, err := k.DB.Query(`SELECT ` + highlightCols + ` FROM highlights ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Highlight
	for rows.Next() {
		var h Highlight
		if err := rows.Scan(highlightScan(&h)...); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// ReadingTitles returns id→title for readings (used to label highlights).
func (k *Knowledge) ReadingTitles() map[string]string {
	out := map[string]string{}
	rows, err := k.DB.Query(`SELECT id, title FROM reading_list`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id, title string
		if rows.Scan(&id, &title) == nil {
			out[id] = title
		}
	}
	return out
}

// Slugify makes a filesystem-safe file name fragment from a title.
func Slugify(title string) string {
	var b strings.Builder
	lastDash := true // swallow leading dashes
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
		if b.Len() >= 40 {
			break
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "untitled"
	}
	return out
}

// FlattenData turns an item's JSON data object into sorted key/value lines
// for front matter (scalars only; unknown shapes are ignored safely).
func FlattenData(data string) [][2]string {
	data = strings.TrimSpace(data)
	if data == "" || data == "{}" || data == "null" {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(data), &m); err != nil || m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([][2]string, 0, len(keys))
	for _, k := range keys {
		switch v := m[k].(type) {
		case string:
			out = append(out, [2]string{k, v})
		case float64:
			out = append(out, [2]string{k, strconv.FormatFloat(v, 'f', -1, 64)})
		case bool:
			out = append(out, [2]string{k, strconv.FormatBool(v)})
		}
	}
	return out
}
