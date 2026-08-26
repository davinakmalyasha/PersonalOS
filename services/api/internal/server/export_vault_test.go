package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestExportVaultZip(t *testing.T) {
	h := newTestAPI(t)

	// A note (with a wiki-link), a bookmark, a reading + highlight.
	note := createItemViaAPI(t, h, "note", "Zettelkasten idea", "Links to [[Second Note]] here")
	_ = note
	_ = createItemViaAPI(t, h, "bookmark", "Hacker News", "news for hackers")
	readingRec := doJSON(t, h, http.MethodPost, "/v1/reading", map[string]interface{}{
		"title": "The Pragmatic Programmer", "author": "Hunt & Thomas", "status": "reading",
	})
	if readingRec.Code != http.StatusCreated {
		t.Fatalf("reading: %d %s", readingRec.Code, readingRec.Body.String())
	}
	var reading struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(readingRec.Body.Bytes(), &reading)
	hlRec := doJSON(t, h, http.MethodPost, "/v1/reading/"+reading.ID+"/highlights", map[string]interface{}{
		"quote": "Care about your craft", "note": "craftsmanship first", "location": "preface",
	})
	if hlRec.Code != http.StatusCreated {
		t.Fatalf("highlight: %d %s", hlRec.Code, hlRec.Body.String())
	}

	rec := doJSON(t, h, http.MethodGet, "/v1/export/vault.zip", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("export: %d %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("content type: %q", ct)
	}

	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
		files[f.Name] = string(data)
	}

	var noteFile, bmFile, hlFile string
	for name := range files {
		if strings.HasPrefix(name, "notes/") {
			noteFile = name
		}
		if strings.HasPrefix(name, "bookmarks/") {
			bmFile = name
		}
		if strings.HasPrefix(name, "highlights/") {
			hlFile = name
		}
	}
	if noteFile == "" || bmFile == "" || hlFile == "" {
		t.Fatalf("missing files: %v", keysOf(files))
	}

	if body := files[noteFile]; !strings.Contains(body, "[[Second Note]]") || !strings.Contains(body, "title: \"Zettelkasten idea\"") {
		t.Fatalf("note content wrong:\n%s", body)
	}
	if body := files[bmFile]; !strings.Contains(body, "Hacker News") {
		t.Fatalf("bookmark content wrong:\n%s", body)
	}
	if body := files[hlFile]; !strings.Contains(body, "> Care about your craft") || !strings.Contains(body, "The Pragmatic Programmer") {
		t.Fatalf("highlight content wrong:\n%s", body)
	}
	if idx, ok := files["INDEX.md"]; !ok || !strings.Contains(idx, "notes/: 1") {
		t.Fatalf("INDEX.md wrong: %q", idx)
	}
}

func keysOf(m map[string]string) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}
