package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func createReadingForHighlights(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/reading", map[string]interface{}{
		"title": "The Pragmatic Programmer", "status": "reading",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create reading: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.ID
}

// ---- Highlights: create → due → review scheduling ----

func TestHighlightLifecycleAndSpacedRepetition(t *testing.T) {
	h := newTestAPI(t)
	readingID := createReadingForHighlights(t, h)

	rec := doJSON(t, h, http.MethodPost, "/v1/reading/"+readingID+"/highlights", map[string]interface{}{
		"quote": "Care about your craft", "note": "why: foundations", "location": "ch. 1",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create highlight: %d %s", rec.Code, rec.Body.String())
	}
	var hh struct {
		ID           string  `json:"id"`
		NextReviewAt *string `json:"next_review_at"`
		IntervalDays int     `json:"interval_days"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &hh)
	if hh.NextReviewAt != nil {
		t.Fatal("new highlight should be due immediately")
	}

	// Empty quote rejected.
	rec = doJSON(t, h, http.MethodPost, "/v1/reading/"+readingID+"/highlights", map[string]interface{}{"quote": "  "})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty quote should 400, got %d", rec.Code)
	}

	// Due queue includes it.
	rec = doJSON(t, h, http.MethodGet, "/v1/knowledge/highlights/due", nil)
	var due struct {
		Items []map[string]interface{} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &due)
	if len(due.Items) != 1 {
		t.Fatalf("due queue wrong: %+v", due.Items)
	}

	// Review remembered=true → climbs to 3-day interval (first was 0→1? ladder starts at 1).
	rec = doJSON(t, h, http.MethodPost, "/v1/highlights/"+hh.ID+"/review", map[string]interface{}{"remembered": true})
	_ = json.Unmarshal(rec.Body.Bytes(), &hh)
	if hh.IntervalDays < 1 || hh.NextReviewAt == nil {
		t.Fatalf("remembered review should schedule ahead: %+v", hh)
	}

	// No longer due.
	rec = doJSON(t, h, http.MethodGet, "/v1/knowledge/highlights/due", nil)
	due.Items = nil
	_ = json.Unmarshal(rec.Body.Bytes(), &due)
	if len(due.Items) != 0 {
		t.Fatalf("scheduled highlight must leave the due queue: %+v", due.Items)
	}

	// A miss resets it to due-now.
	rec = doJSON(t, h, http.MethodPost, "/v1/highlights/"+hh.ID+"/review", map[string]interface{}{"remembered": false})
	_ = json.Unmarshal(rec.Body.Bytes(), &hh)
	if hh.IntervalDays != 0 || hh.NextReviewAt != nil {
		t.Fatalf("miss should reset schedule: %+v", hh)
	}

	// Highlights mirror into universal search.
	rec = doJSON(t, h, http.MethodGet, "/v1/search?q=craft", nil)
	var sr struct {
		Items []searchResult `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sr)
	found := false
	for _, i := range sr.Items {
		if i.Type == "highlight" {
			found = true
		}
	}
	if !found {
		t.Fatal("highlight not mirrored into search")
	}

	// Delete removes mirror too.
	rec = doJSON(t, h, http.MethodDelete, "/v1/highlights/"+hh.ID, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete highlight: %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/search?q=craft", nil)
	sr.Items = nil
	_ = json.Unmarshal(rec.Body.Bytes(), &sr)
	if len(sr.Items) != 0 {
		t.Fatal("deleted highlight still searchable")
	}
}

// ---- Wiki links + graph + orphans ----

func TestWikiLinksGraphAndOrphans(t *testing.T) {
	h := newTestAPI(t)

	target := createItemViaAPI(t, h, "concept", "Zettelkasten", "method of linked notes")
	noteRec := doJSON(t, h, http.MethodPost, "/v1/notes", map[string]interface{}{
		"title": "Reading notes", "body": "Links back to [[Zettelkasten]] and [[Missing Idea]].",
	})
	if noteRec.Code != http.StatusCreated {
		t.Fatalf("note create: %d %s", noteRec.Code, noteRec.Body.String())
	}
	var note struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(noteRec.Body.Bytes(), &note)

	// Wiki link edge created for the existing target only. Clients address
	// links by the ITEM id (the note's search mirror), not the native note id.
	rec := doJSON(t, h, http.MethodGet, "/v1/search?q=reading+notes&type=note", nil)
	var found struct {
		Items []struct {
			ID           string  `json:"id"`
			SourceItemID *string `json:"source_item_id"`
		} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &found)
	if len(found.Items) != 1 {
		t.Fatalf("note mirror not found in search: %+v", found.Items)
	}
	noteItemID := found.Items[0].ID

	rec = doJSON(t, h, http.MethodGet, "/v1/items/"+noteItemID+"/links", nil)
	var links struct {
		Outgoing []struct {
			ToID string `json:"to_id"`
			Kind string `json:"kind"`
		} `json:"outgoing"`
		Incoming []struct {
			FromID string `json:"from_id"`
			Kind   string `json:"kind"`
		} `json:"incoming"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &links)
	if len(links.Outgoing) != 1 || links.Outgoing[0].ToID != target["id"] || links.Outgoing[0].Kind != "wiki" {
		t.Fatalf("wiki edge wrong: %+v", links.Outgoing)
	}

	// Graph depth 2 around the note's item reaches the target concept.
	rec = doJSON(t, h, http.MethodGet, "/v1/graph/"+noteItemID+"?depth=2", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("graph: %d %s", rec.Code, rec.Body.String())
	}
	var g struct {
		Root  string `json:"root"`
		Nodes []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"nodes"`
		Edges []struct {
			From string `json:"from"`
			To   string `json:"to"`
			Kind string `json:"kind"`
		} `json:"edges"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &g)
	if g.Root != noteItemID || len(g.Nodes) < 2 || len(g.Edges) == 0 {
		t.Fatalf("graph wrong: %+v", g)
	}
	reachedTarget := false
	for _, n := range g.Nodes {
		if n.ID == target["id"] {
			reachedTarget = true
		}
	}
	if !reachedTarget {
		t.Fatal("graph did not reach the wiki-linked item")
	}

	// Orphans: standalone items with no links surface; the two linked ones don't.
	doJSON(t, h, http.MethodPost, "/v1/items", map[string]interface{}{"type": "misc", "title": "Lone capture"})
	rec = doJSON(t, h, http.MethodGet, "/v1/items/orphans", nil)
	var orphans struct {
		Items []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &orphans)
	loneFound, linkedLeaked := false, false
	for _, o := range orphans.Items {
		if o.Title == "Lone capture" {
			loneFound = true
		}
		if o.ID == target["id"] || o.ID == noteItemID {
			linkedLeaked = true
		}
	}
	if !loneFound || linkedLeaked {
		t.Fatalf("orphan list wrong: %+v", orphans.Items)
	}
}

// ---- Mirror propagation fix: archived notes leave search ----

func TestNoteArchivePropagatesToSearchMirror(t *testing.T) {
	h := newTestAPI(t)
	note := createNote(t, h, "secret plans", "hide me from search", nil)
	id := note["id"].(string)

	rec := doJSON(t, h, http.MethodPatch, "/v1/notes/"+id, map[string]interface{}{"archived": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("archive note: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/search?q=hide+me", nil)
	var sr struct {
		Items []searchResult `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sr)
	if len(sr.Items) != 0 {
		t.Fatal("archived note leaked into search via mirror")
	}

	// Unarchive restores visibility.
	doJSON(t, h, http.MethodPatch, "/v1/notes/"+id, map[string]interface{}{"archived": false})
	rec = doJSON(t, h, http.MethodGet, "/v1/search?q=hide+me", nil)
	sr.Items = nil
	_ = json.Unmarshal(rec.Body.Bytes(), &sr)
	if len(sr.Items) != 1 {
		t.Fatal("unarchived note should reappear in search")
	}
}

// ---- Bookmark meta-fetch ----

// serveTitlePage spins an httptest server serving one HTML page and counting hits.
func serveTitlePage(t *testing.T, calls *int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		_, _ = w.Write([]byte(body))
	}))
}

func TestBookmarkFetchesTitleWhenMissing(t *testing.T) {
	var calls int
	titleSrv := serveTitlePage(t, &calls, "<html><title>Example Domain &amp; More</title></html>")
	defer titleSrv.Close()

	h := newTestAPI(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/bookmarks", map[string]interface{}{
		"url": titleSrv.URL,
	})
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("bookmark create: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Bookmark struct {
			Title string `json:"title"`
		} `json:"bookmark"`
		Duplicate bool `json:"duplicate"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if !strings.Contains(out.Bookmark.Title, "Example Domain") {
		t.Fatalf("title not fetched from page: %q", out.Bookmark.Title)
	}
	if calls != 1 {
		t.Fatalf("expected exactly one fetch, got %d", calls)
	}

	// Explicit titles skip the fetch entirely.
	var calls2 int
	srv2 := serveTitlePage(t, &calls2, "<title>Never Read</title>")
	defer srv2.Close()
	rec = doJSON(t, h, http.MethodPost, "/v1/bookmarks", map[string]interface{}{
		"url": srv2.URL, "title": "My own title",
	})
	out.Bookmark.Title = ""
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Bookmark.Title != "My own title" || calls2 != 0 {
		t.Fatalf("explicit title should win without fetch: %q calls=%d", out.Bookmark.Title, calls2)
	}
}
