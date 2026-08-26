package store

import (
	"database/sql"
	"regexp"
	"strings"
)

// ---- Wiki links + graph traversal (phase 12d) ----

var wikiLinkRe = regexp.MustCompile(`\[\[([^\[\]]{1,120})\]\]`)

// ExtractWikiLinks returns the [[targets]] referenced by a note body.
func ExtractWikiLinks(body string) []string {
	matches := wikiLinkRe.FindAllStringSubmatch(body, -1)
	out := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, m := range matches {
		title := strings.TrimSpace(m[1])
		if title == "" {
			continue
		}
		key := strings.ToLower(title)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, title)
	}
	return out
}

// SyncWikiLinks creates item_links (kind "wiki") from a note's *mirror item*
// to every existing item whose title matches a [[target]]. Missing targets are
// ignored — the note simply has no edge yet. Returns linked titles.
func (k *Knowledge) SyncWikiLinks(noteNativeID, body string) ([]string, error) {
	var srcItem string
	err := k.DB.QueryRow(
		`SELECT id FROM items WHERE type='note' AND source_item_id=?`, noteNativeID).Scan(&srcItem)
	if err == sql.ErrNoRows {
		return nil, nil // mirror missing; nothing to link from
	}
	if err != nil {
		return nil, err
	}
	titles := ExtractWikiLinks(body)
	linked := []string{}
	for _, t := range titles {
		var targetID string
		err := k.DB.QueryRow(
			`SELECT id FROM items WHERE LOWER(title)=LOWER(?) AND archived=0 ORDER BY created_at LIMIT 1`, t).
			Scan(&targetID)
		if err != nil || targetID == srcItem {
			continue // no such item yet / self-link rejected
		}
		if _, err := k.DB.Exec(
			`INSERT OR IGNORE INTO item_links (from_id,to_id,kind,created_at) VALUES (?,?, 'wiki', ?)`,
			srcItem, targetID, NowRFC3339()); err != nil {
			return linked, err
		}
		linked = append(linked, t)
	}
	return linked, nil
}

// GraphNode is one node in the local knowledge graph.
type GraphNode struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

// GraphResult is a bounded BFS around an item.
type GraphResult struct {
	Root  string      `json:"root"`
	Nodes []GraphNode `json:"nodes"`
	Edges []struct {
		From string `json:"from"`
		To   string `json:"to"`
		Kind string `json:"kind"`
	} `json:"edges"`
}

const maxGraphDepth = 2

// Graph walks up to `depth` hops of item_links around id.
func Graph(db *sql.DB, id string, depth int) (GraphResult, error) {
	if depth < 1 || depth > maxGraphDepth {
		depth = 1
	}
	out := GraphResult{Root: id, Nodes: []GraphNode{}, Edges: []struct {
		From string `json:"from"`
		To   string `json:"to"`
		Kind string `json:"kind"`
	}{}}
	frontier := map[string]bool{id: true}
	visited := map[string]bool{}
	for d := 0; d < depth; d++ {
		next := map[string]bool{}
		for node := range frontier {
			rows, err := db.Query(`SELECT from_id,to_id,kind FROM item_links WHERE from_id=? OR to_id=?`, node, node)
			if err != nil {
				return out, err
			}
			for rows.Next() {
				var from, to, kind string
				if err := rows.Scan(&from, &to, &kind); err != nil {
					rows.Close()
					return out, err
				}
				out.Edges = append(out.Edges, struct {
					From string `json:"from"`
					To   string `json:"to"`
					Kind string `json:"kind"`
				}{from, to, kind})
				for _, n := range []string{from, to} {
					if !visited[n] {
						next[n] = true
					}
				}
			}
			rows.Close()
			visited[node] = true
		}
		frontier = next
	}
	visited[id] = true
	for node := range visited {
		var typ, title string
		if err := db.QueryRow(`SELECT type,title FROM items WHERE id=?`, node).Scan(&typ, &title); err != nil {
			continue
		}
		out.Nodes = append(out.Nodes, GraphNode{ID: node, Type: typ, Title: title})
	}
	return out, nil
}

// Orphans lists non-mirror items with zero links in either direction.
func (it *Items) Orphans(limit int) ([]Item, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return it.queryItems(`
		SELECT i.id,i.type,i.title,i.body,i.data,i.tags,i.source,i.source_item_id,i.pinned,i.archived,i.created_at,i.updated_at
		FROM items i
		WHERE i.archived=0 AND i.source_item_id IS NULL
		  AND NOT EXISTS (SELECT 1 FROM item_links l WHERE l.from_id=i.id OR l.to_id=i.id)
		ORDER BY i.created_at DESC LIMIT ?`, limit)
}
