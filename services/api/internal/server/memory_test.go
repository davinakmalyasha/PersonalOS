package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/db"
)

// newTestAPIWithDB exposes the raw handle for direct seeding (resurface).

func createItemViaAPI(t *testing.T, h http.Handler, typ, title, body string) map[string]interface{} {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/items", map[string]interface{}{
		"type": typ, "title": title, "body": body,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create item: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

// ---- Pin + archive lifecycle ----

func TestItemPinArchiveLifecycle(t *testing.T) {
	h := newTestAPI(t)
	item := createItemViaAPI(t, h, "warranty", "Fridge warranty", "expires soon")
	id := item["id"].(string)

	// Archive it.
	rec := doJSON(t, h, http.MethodPatch, "/v1/items/"+id, map[string]interface{}{
		"archived": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("archive: %d %s", rec.Code, rec.Body.String())
	}

	// Default list hides archived.
	rec = doJSON(t, h, http.MethodGet, "/v1/items", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	var list struct {
		Items []map[string]interface{} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	for _, i := range list.Items {
		if i["id"] == id {
			t.Fatal("archived item leaked into default list")
		}
	}

	// include_archived=true shows it; search hides it.
	rec = doJSON(t, h, http.MethodGet, "/v1/items?include_archived=true", nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	found := false
	for _, i := range list.Items {
		if i["id"] == id {
			found = true
			if i["archived"] != true {
				t.Fatal("archived flag not persisted")
			}
		}
	}
	if !found {
		t.Fatal("include_archived list should contain the item")
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/search?q=Fridge", nil)
	var sr struct {
		Items []map[string]interface{} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sr)
	if len(sr.Items) != 0 {
		t.Fatal("search must exclude archived items")
	}

	// Unarchive + pin: pinned ordering puts it first.
	rec = doJSON(t, h, http.MethodPatch, "/v1/items/"+id, map[string]interface{}{
		"archived": false, "pinned": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("unarchive+pin: %d %s", rec.Code, rec.Body.String())
	}
	createItemViaAPI(t, h, "note-ish", "Newer unpinned", "")
	rec = doJSON(t, h, http.MethodGet, "/v1/items", nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Items) < 2 || list.Items[0]["title"] != "Fridge warranty" {
		t.Fatalf("pinned item should sort first, got %v", list.Items)
	}
}

// ---- Changelog feed ----

func TestActivityFeedRecordsMutations(t *testing.T) {
	h := newTestAPI(t)

	createNote(t, h, "feed note one", "", nil)
	task := createTask(t, h, map[string]interface{}{"title": "feed task"})
	rec := doJSON(t, h, http.MethodPatch, "/v1/tasks/"+task["id"].(string), map[string]interface{}{
		"status": "done",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("complete task: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/activity/feed?limit=50", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("feed: %d %s", rec.Code, rec.Body.String())
	}
	var feed struct {
		Items []struct {
			Entity   string `json:"entity"`
			Action   string `json:"action"`
			Title    string `json:"title"`
			EntityID string `json:"entity_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &feed); err != nil {
		t.Fatalf("decode feed: %v", err)
	}
	if len(feed.Items) < 3 { // note create + task create + task complete (+ mirror updates are not logged)
		t.Fatalf("expected >=3 entries, got %d: %+v", len(feed.Items), feed.Items)
	}
	has := func(entity, action, title string) bool {
		for _, c := range feed.Items {
			if c.Entity == entity && c.Action == action && c.Title == title {
				return true
			}
		}
		return false
	}
	if !has("note", "create", "feed note one") {
		t.Fatalf("missing note create entry: %+v", feed.Items)
	}
	if !has("task", "create", "feed task") || !has("task", "complete", "feed task") {
		t.Fatalf("missing task create/complete entries: %+v", feed.Items)
	}

	// Entity filter narrows the feed.
	rec = doJSON(t, h, http.MethodGet, "/v1/activity/feed?entity=task", nil)
	feed.Items = nil
	_ = json.Unmarshal(rec.Body.Bytes(), &feed)
	if len(feed.Items) == 0 {
		t.Fatal("entity=task filter returned nothing")
	}
	for _, c := range feed.Items {
		if c.Entity != "task" {
			t.Fatalf("filter leaked %s entry", c.Entity)
		}
	}
}

// ---- Saved searches ----

func TestSavedSearchCRUDAndRun(t *testing.T) {
	h := newTestAPI(t)
	createNote(t, h, "grocery hacks", "oat milk forever", []string{"kitchen"})

	rec := doJSON(t, h, http.MethodPost, "/v1/saved_searches", map[string]interface{}{
		"name":  "milk finder",
		"query": map[string]interface{}{"q": "oat milk"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create saved search: %d %s", rec.Code, rec.Body.String())
	}
	var ss struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ss)

	// Duplicate name conflicts.
	rec = doJSON(t, h, http.MethodPost, "/v1/saved_searches", map[string]interface{}{
		"name": "milk finder", "query": map[string]interface{}{},
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate name should conflict, got %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/saved_searches/"+ss.ID+"/run", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("run: %d %s", rec.Code, rec.Body.String())
	}
	var run struct {
		Items []searchResult `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &run)
	if len(run.Items) != 1 || run.Items[0].Title != "grocery hacks" {
		t.Fatalf("run should find the note, got %+v", run.Items)
	}

	// Update renames.
	rec = doJSON(t, h, http.MethodPatch, "/v1/saved_searches/"+ss.ID, map[string]interface{}{
		"name": "milk radar",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update saved search: %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodDelete, "/v1/saved_searches/"+ss.ID, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/saved_searches/"+ss.ID, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete: %d", rec.Code)
	}
}

// ---- Daily note ----

func TestDailyNoteCreateAppendIdempotent(t *testing.T) {
	h := newTestAPI(t)
	day := time.Now().UTC().Format("2006-01-02")

	rec := doJSON(t, h, http.MethodGet, "/v1/knowledge/daily", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("daily: %d %s", rec.Code, rec.Body.String())
	}
	var first struct {
		Created bool                   `json:"created"`
		Note    map[string]interface{} `json:"note"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &first)
	if !first.Created {
		t.Fatal("first touch should create")
	}
	if first.Note["title"] != "Daily "+day {
		t.Fatalf("wrong title %v", first.Note["title"])
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/knowledge/daily", nil)
	var second struct {
		Created bool                   `json:"created"`
		Note    map[string]interface{} `json:"note"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &second)
	if second.Created || second.Note["id"] != first.Note["id"] {
		t.Fatalf("second GET must reuse the same note (created=%v)", second.Created)
	}

	// Board action: append a line.
	rec = doJSON(t, h, http.MethodPatch, "/v1/knowledge/daily", map[string]interface{}{
		"text": "called the dentist",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("append: %d %s", rec.Code, rec.Body.String())
	}
	var appended map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &appended)
	if body, _ := appended["body"].(string); !contains(body, "- called the dentist") {
		t.Fatalf("append missing from body %q", body)
	}

	// Invalid date rejected.
	rec = doJSON(t, h, http.MethodGet, "/v1/knowledge/daily?date=nope", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad date should 400, got %d", rec.Code)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(needle) == 0 || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// ---- Resurface ----

func TestResurfaceOnThisDay(t *testing.T) {
	dir := t.TempDir()
	sqlDB, err := db.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Migrate(sqlDB, "../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	lastYear := time.Now().UTC().AddDate(-1, 0, 0).Format(time.RFC3339)
	if _, err := sqlDB.Exec(`INSERT INTO items (id,type,title,body,data,tags,source,source_item_id,pinned,archived,created_at,updated_at)
		VALUES ('resurface-me','warranty','Old warranty','x','{}','[]','manual',NULL,0,0,?,?)`,
		lastYear, lastYear); err != nil {
		t.Fatalf("seed: %v", err)
	}
	h := New(sqlDB, zerolog.Nop(), "")

	rec := doJSON(t, h, http.MethodGet, "/v1/knowledge/resurface", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("resurface: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Items []struct {
			Kind string `json:"kind"`
			ID   string `json:"id"`
			Year int    `json:"year"`
		} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.Items) != 1 || out.Items[0].ID != "resurface-me" || out.Items[0].Kind != "item" {
		t.Fatalf("expected the seeded on-this-day item, got %+v", out.Items)
	}
	if out.Items[0].Year >= time.Now().UTC().Year() {
		t.Fatalf("year should be in the past: %d", out.Items[0].Year)
	}
}

// ---- Search v2 typed hits + export ----

func TestSearchV2TypedHitsAndExport(t *testing.T) {
	h := newTestAPI(t)

	createTask(t, h, map[string]interface{}{"title": "Buy oat milk"})
	createItemViaAPI(t, h, "note-ish", "Pantry list", "oat milk and rice")
	account := mustCreateAccount(t, h)
	rec := doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": account, "amount_minor": -45000, "date": "2026-08-20",
		"merchant": "Oat Milk Depot",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("txn: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/search?q=oat+milk", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("search v2: %d %s", rec.Code, rec.Body.String())
	}
	var sr struct {
		Hits []struct {
			Kind  string `json:"kind"`
			Title string `json:"title"`
		} `json:"hits"`
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &sr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	kinds := map[string]bool{}
	for _, hit := range sr.Hits {
		kinds[hit.Kind] = true
	}
	for _, want := range []string{"task", "transaction"} {
		if !kinds[want] {
			t.Fatalf("search v2 missing %s hit, got %+v", want, sr.Hits)
		}
	}
	if len(sr.Items) == 0 {
		t.Fatal("compat items array should still surface the pantry item")
	}

	// Export covers every pillar table with data present.
	rec = doJSON(t, h, http.MethodGet, "/v1/export", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("export: %d %s", rec.Code, rec.Body.String())
	}
	var dump struct {
		Data map[string][]map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dump); err != nil {
		t.Fatalf("export decode: %v", err)
	}
	for _, table := range []string{"tasks", "transactions", "items"} {
		rows, ok := dump.Data[table]
		if !ok || len(rows) == 0 {
			t.Fatalf("export missing rows for %s: %+v", table, dump.Data)
		}
	}
}
