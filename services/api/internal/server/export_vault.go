package server

import (
	"archive/zip"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/store"
)

// ---- Markdown vault export (phase 13d) ----

// GET /export/vault.zip — the knowledge base as a folder of Markdown files:
// notes/, bookmarks/, readings/ (mirrored items) + highlights/ (native
// records), each with YAML-ish front matter. Wiki-links in note bodies are
// preserved verbatim so the folder works in Obsidian-style tools.
func (s *Server) handleExportVault(w http.ResponseWriter, r *http.Request) {
	items, err := s.items.AllItemsByTypes([]string{"note", "bookmark", "reading"})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	highlights, err := s.knowledge.AllHighlights()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	titles := s.knowledge.ReadingTitles()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="personal-os-vault.zip"`)

	zw := zip.NewWriter(w)
	defer zw.Close()
	counts := map[string]int{}

	writeItem := func(dir string, i store.Item) {
		name := fmt.Sprintf("%s/%s-%s.md", dir, store.Slugify(i.Title), shortID(i.ID))
		f, err := zw.Create(name)
		if err != nil {
			return
		}
		var b strings.Builder
		b.WriteString("---\n")
		fmt.Fprintf(&b, "id: %s\n", i.ID)
		fmt.Fprintf(&b, "type: %s\n", i.Type)
		fmt.Fprintf(&b, "title: %q\n", i.Title)
		fmt.Fprintf(&b, "tags: [%s]\n", strings.Join(i.Tags, ", "))
		fmt.Fprintf(&b, "pinned: %t\n", i.Pinned)
		fmt.Fprintf(&b, "archived: %t\n", i.Archived)
		if i.Source != "" {
			fmt.Fprintf(&b, "source: %q\n", i.Source)
		}
		for _, kv := range store.FlattenData(i.Data) {
			fmt.Fprintf(&b, "%s: %q\n", kv[0], kv[1])
		}
		fmt.Fprintf(&b, "created: %s\n", i.CreatedAt)
		fmt.Fprintf(&b, "updated: %s\n", i.UpdatedAt)
		b.WriteString("---\n\n")
		b.WriteString(strings.TrimRight(i.Body, "\n"))
		b.WriteString("\n")
		_, _ = f.Write([]byte(b.String()))
		counts[dir]++
	}

	for _, i := range items {
		switch i.Type {
		case "note":
			writeItem("notes", i)
		case "bookmark":
			writeItem("bookmarks", i)
		case "reading":
			writeItem("readings", i)
		}
	}

	for _, h := range highlights {
		name := fmt.Sprintf("highlights/%s-%s.md", store.Slugify(titles[h.ReadingID]), shortID(h.ID))
		f, err := zw.Create(name)
		if err != nil {
			continue
		}
		var b strings.Builder
		b.WriteString("---\n")
		fmt.Fprintf(&b, "id: %s\n", h.ID)
		fmt.Fprintf(&b, "reading_id: %s\n", h.ReadingID)
		fmt.Fprintf(&b, "reading: %q\n", titles[h.ReadingID])
		if h.Location != "" {
			fmt.Fprintf(&b, "location: %q\n", h.Location)
		}
		fmt.Fprintf(&b, "review_count: %d\n", h.ReviewCount)
		fmt.Fprintf(&b, "interval_days: %d\n", h.IntervalDays)
		if h.NextReviewAt != nil {
			fmt.Fprintf(&b, "next_review_at: %s\n", *h.NextReviewAt)
		}
		fmt.Fprintf(&b, "created: %s\n", h.CreatedAt)
		b.WriteString("---\n\n")
		b.WriteString("> ")
		b.WriteString(strings.ReplaceAll(h.Quote, "\n", "\n> "))
		b.WriteString("\n")
		if h.Note != "" {
			b.WriteString("\n")
			b.WriteString(h.Note)
			b.WriteString("\n")
		}
		_, _ = f.Write([]byte(b.String()))
		counts["highlights"]++
	}

	var idx strings.Builder
	fmt.Fprintf(&idx, "# Personal OS vault\n\nExported %s.\n\n", time.Now().UTC().Format(time.RFC3339))
	for _, dir := range []string{"notes", "bookmarks", "readings", "highlights"} {
		fmt.Fprintf(&idx, "- %s/: %d\n", dir, counts[dir])
	}
	if f, err := zw.Create("INDEX.md"); err == nil {
		_, _ = f.Write([]byte(idx.String()))
	}
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
