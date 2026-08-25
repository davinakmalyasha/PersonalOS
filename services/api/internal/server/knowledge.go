package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) mountKnowledge(r chi.Router) {
	r.Route("/notes", func(r chi.Router) {
		r.Post("/", s.handleCreateNote)
		r.Get("/", s.handleListNotes)
		r.Get("/{id}", s.handleGetNote)
		r.Patch("/{id}", s.handleUpdateNote)
		r.Delete("/{id}", s.handleDeleteNote)
	})
	r.Route("/bookmarks", func(r chi.Router) {
		r.Post("/", s.handleCreateBookmark)
		r.Get("/", s.handleListBookmarks)
		r.Get("/{id}", s.handleGetBookmark)
		r.Patch("/{id}", s.handleUpdateBookmark)
		r.Delete("/{id}", s.handleDeleteBookmark)
	})
	r.Route("/reading", func(r chi.Router) {
		r.Post("/", s.handleCreateReading)
		r.Get("/", s.handleListReadings)
		r.Get("/{id}", s.handleGetReading)
		r.Patch("/{id}", s.handleUpdateReading)
		r.Delete("/{id}", s.handleDeleteReading)
	})
	r.Get("/knowledge/search", s.handleKnowledgeSearch)
	r.Get("/knowledge/tags", s.handleKnowledgeTags)
}

func (s *Server) mountItems(r chi.Router) {
	r.Route("/items", func(r chi.Router) {
		r.Post("/", s.handleCreateItem)
		r.Get("/", s.handleListItems)
		r.Get("/{id}", s.handleGetItem)
		r.Patch("/{id}", s.handleUpdateItem)
		r.Delete("/{id}", s.handleDeleteItem)
		r.Post("/{id}/links", s.handleCreateLink)
		r.Get("/{id}/links", s.handleListLinks)
		r.Delete("/{id}/links/{toId}/{kind}", s.handleDeleteLink)
		r.Post("/{id}/promote", s.handlePromoteItem)
	})
	r.Get("/search", s.handleGlobalSearch)
	r.Get("/tags", s.handleItemTags)
}

// ---- Notes ----

type noteReq struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Tags   []string `json:"tags"`
	Pinned bool     `json:"pinned"`
}

func (s *Server) handleCreateNote(w http.ResponseWriter, r *http.Request) {
	var req noteReq
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		fail(w, http.StatusBadRequest, "title required", fieldError{"title", "required"})
		return
	}
	n, err := s.knowledge.CreateNote(req.Title, req.Body, req.Tags, req.Pinned)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (s *Server) handleListNotes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := queryInt(r, "page")
	size, _ := queryInt(r, "page_size")
	items, total, err := s.knowledge.ListNotes(store.NoteFilter{
		Tag:      q.Get("tag"),
		Q:        q.Get("q"),
		Pinned:   q.Get("pinned") == "true",
		Archived: q.Get("archived") == "true",
		Page:     int(page),
		PageSize: int(size),
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items, "total": total, "page": page, "page_size": size,
	})
}

func (s *Server) handleGetNote(w http.ResponseWriter, r *http.Request) {
	n, err := s.knowledge.GetNote(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, n)
}

type notePatch struct {
	Title   *string   `json:"title"`
	Body    *string   `json:"body"`
	Tags    *[]string `json:"tags"`
	Pinned  *bool     `json:"pinned"`
	Archive *bool     `json:"archived"`
}

func (s *Server) handleUpdateNote(w http.ResponseWriter, r *http.Request) {
	var req notePatch
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	n, err := s.knowledge.UpdateNote(chiURLParam(r, "id"), store.NoteUpdate{
		Title: req.Title, Body: req.Body, Tags: req.Tags, Pinned: req.Pinned, Archive: req.Archive,
	})
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request) {
	if err := s.knowledge.DeleteNote(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Bookmarks ----

type bookmarkReq struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

func (s *Server) handleCreateBookmark(w http.ResponseWriter, r *http.Request) {
	var req bookmarkReq
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		fail(w, http.StatusBadRequest, "url required", fieldError{"url", "required"})
		return
	}
	b, duplicate, err := s.knowledge.CreateBookmark(req.URL, req.Title, req.Description, req.Tags)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid url",
				fieldError{"url", "absolute http(s) URL required"})
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK // idempotent by canonical URL
	}
	writeJSON(w, status, map[string]interface{}{
		"bookmark": b, "duplicate": duplicate,
	})
}

func (s *Server) handleListBookmarks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := queryInt(r, "page")
	size, _ := queryInt(r, "page_size")
	items, total, err := s.knowledge.ListBookmarks(store.BookmarkFilter{
		Tag: q.Get("tag"), Q: q.Get("q"),
		Page: int(page), PageSize: int(size),
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items, "total": total, "page": page, "page_size": size,
	})
}

func (s *Server) handleGetBookmark(w http.ResponseWriter, r *http.Request) {
	b, err := s.knowledge.GetBookmark(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, b)
}

type bookmarkPatch struct {
	Title       *string   `json:"title"`
	Description *string   `json:"description"`
	Tags        *[]string `json:"tags"`
}

func (s *Server) handleUpdateBookmark(w http.ResponseWriter, r *http.Request) {
	var req bookmarkPatch
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	b, err := s.knowledge.UpdateBookmark(chiURLParam(r, "id"), store.BookmarkUpdate{
		Title: req.Title, Description: req.Description, Tags: req.Tags,
	})
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *Server) handleDeleteBookmark(w http.ResponseWriter, r *http.Request) {
	if err := s.knowledge.DeleteBookmark(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Reading list ----

type readingReq struct {
	Title  string   `json:"title"`
	Author *string  `json:"author"`
	URL    *string  `json:"url"`
	Status string   `json:"status"`
	Rating *int     `json:"rating"`
	Notes  string   `json:"notes"`
	Tags   []string `json:"tags"`
}

func (s *Server) handleCreateReading(w http.ResponseWriter, r *http.Request) {
	var req readingReq
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	details := []fieldError{}
	if strings.TrimSpace(req.Title) == "" {
		details = append(details, fieldError{"title", "required"})
	}
	if req.Status != "" && !validReadingStatus(req.Status) {
		details = append(details, fieldError{"status", "one of to-read|reading|done"})
	}
	if req.Rating != nil && (*req.Rating < 1 || *req.Rating > 5) {
		details = append(details, fieldError{"rating", "1..5 or null"})
	}
	if len(details) > 0 {
		fail(w, http.StatusBadRequest, "invalid reading entry", details...)
		return
	}
	rd, err := s.knowledge.CreateReading(req.Title, req.Author, req.URL, req.Status, req.Rating, req.Notes, req.Tags)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, rd)
}

func validReadingStatus(st string) bool { return st == "to-read" || st == "reading" || st == "done" }

func (s *Server) handleListReadings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := queryInt(r, "page")
	size, _ := queryInt(r, "page_size")
	items, total, err := s.knowledge.ListReadings(store.ReadingFilter{
		Status: q.Get("status"), Tag: q.Get("tag"), Q: q.Get("q"),
		Page: int(page), PageSize: int(size),
	})
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid status filter")
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items, "total": total, "page": page, "page_size": size,
	})
}

func (s *Server) handleGetReading(w http.ResponseWriter, r *http.Request) {
	rd, err := s.knowledge.GetReading(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, rd)
}

type readingPatch struct {
	Title      *string          `json:"title"`
	Author     **string         `json:"author"`
	URL        **string         `json:"url"`
	Status     *string          `json:"status"`
	Rating     **int            `json:"rating"`
	Notes      *string          `json:"notes"`
	Tags       *[]string        `json:"tags"`
	Highlights *json.RawMessage `json:"highlights"`
}

func (s *Server) handleUpdateReading(w http.ResponseWriter, r *http.Request) {
	var req readingPatch
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	rd, err := s.knowledge.UpdateReading(chiURLParam(r, "id"), store.ReadingUpdate{
		Title: req.Title, Author: req.Author, URL: req.URL, Status: req.Status,
		Rating: req.Rating, Notes: req.Notes, Tags: req.Tags, Highlights: req.Highlights,
	})
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid fields",
				fieldError{"status/rating", "to-read|reading|done; rating 1..5"})
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, rd)
}

func (s *Server) handleDeleteReading(w http.ResponseWriter, r *http.Request) {
	if err := s.knowledge.DeleteReading(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Cross-type search + tags ----

var knowledgeTypes = []string{"note", "bookmark", "reading"}

func (s *Server) handleKnowledgeSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := queryInt(r, "limit")
	items, err := s.items.SearchItems(q.Get("q"), knowledgeTypes, "", q.Get("tag"), int(limit))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items, "q": q.Get("q")})
}

func (s *Server) handleKnowledgeTags(w http.ResponseWriter, r *http.Request) {
	limit, _ := queryInt(r, "limit")
	tags, err := s.items.TagsWithCounts(knowledgeTypes, r.URL.Query().Get("prefix"), int(limit))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": tags})
}

// ---- Universal items ----

type itemReq struct {
	Type string   `json:"type"`
	Title string  `json:"title"`
	Body string   `json:"body"`
	Data string   `json:"data"` // raw JSON object string
	Tags []string `json:"tags"`
}

func (s *Server) handleCreateItem(w http.ResponseWriter, r *http.Request) {
	var req itemReq
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	details := []fieldError{}
	if !store.ValidItemType(req.Type) {
		details = append(details, fieldError{"type", "required, max 40 chars"})
	}
	if strings.TrimSpace(req.Title) == "" {
		details = append(details, fieldError{"title", "required (max 300)"})
	}
	if req.Data != "" && !isJSONObject(req.Data) {
		details = append(details, fieldError{"data", "must be a JSON object"})
	}
	if len(details) > 0 {
		fail(w, http.StatusBadRequest, "invalid item", details...)
		return
	}
	item, err := s.items.CreateItem(req.Type, req.Title, req.Body, req.Data, req.Tags, "api", nil)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func isJSONObject(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")
}

func (s *Server) handleListItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := queryInt(r, "page")
	size, _ := queryInt(r, "page_size")
	items, total, err := s.items.ListItems(store.ItemFilter{
		Type: q.Get("type"), Tag: q.Get("tag"), Q: q.Get("q"),
		Page: int(page), PageSize: int(size),
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items, "total": total, "page": page, "page_size": size,
	})
}

func (s *Server) handleGetItem(w http.ResponseWriter, r *http.Request) {
	item, err := s.items.GetItem(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, item)
}

type itemPatch struct {
	Title *string   `json:"title"`
	Body  *string   `json:"body"`
	Data  *string   `json:"data"`
	Tags  *[]string `json:"tags"`
}

func (s *Server) handleUpdateItem(w http.ResponseWriter, r *http.Request) {
	var req itemPatch
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if req.Data != nil && !isJSONObject(*req.Data) {
		fail(w, http.StatusBadRequest, "data must be a JSON object", fieldError{"data", "object required"})
		return
	}
	item, err := s.items.UpdateItem(chiURLParam(r, "id"), store.ItemUpdate{
		Title: req.Title, Body: req.Body, Data: req.Data, Tags: req.Tags,
	})
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	if err := s.items.DeleteItem(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Links ----

func (s *Server) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ToID string `json:"to_id"`
		Kind string `json:"kind"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if req.ToID == "" {
		fail(w, http.StatusBadRequest, "to_id required", fieldError{"to_id", "required"})
		return
	}
	link, err := s.items.CreateLink(chiURLParam(r, "id"), req.ToID, req.Kind)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

func (s *Server) handleListLinks(w http.ResponseWriter, r *http.Request) {
	outgoing, incoming, err := s.items.LinksFor(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"outgoing": outgoing, "incoming": incoming})
}

func (s *Server) handleDeleteLink(w http.ResponseWriter, r *http.Request) {
	err := s.items.DeleteLink(chiURLParam(r, "id"), chiURLParam(r, "toId"), chiURLParam(r, "kind"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Promote (universal item → typed pillar record) ----

func (s *Server) handlePromoteItem(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	var req struct {
		Target string `json:"target"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	switch req.Target {
	case "task":
		item, err := s.items.GetItem(id)
		if err != nil {
			if !mapStoreErr(w, err) {
				fail(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		t, err := s.planner.CreateTask(item.Title, item.Body, "", "med", nil, nil, item.Tags, nil)
		if err != nil {
			if !mapStoreErr(w, err) {
				fail(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{"target": "task", "record_id": t.ID, "item": item})
	case "note":
		item, err := s.items.GetItem(id)
		if err != nil {
			if !mapStoreErr(w, err) {
				fail(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		n, err := s.knowledge.CreateNote(item.Title, item.Body, item.Tags, false)
		if err != nil {
			if !mapStoreErr(w, err) {
				fail(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{"target": "note", "record_id": n.ID, "item": item})
	default:
		fail(w, http.StatusBadRequest, "unsupported promotion target",
			fieldError{"target", "one of task|note"})
	}
}

// ---- Global search + tags ----

func (s *Server) handleGlobalSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := queryInt(r, "limit")
	items, err := s.items.SearchItems(q.Get("q"), nil, q.Get("type"), q.Get("tag"), int(limit))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items, "q": q.Get("q")})
}

func (s *Server) handleItemTags(w http.ResponseWriter, r *http.Request) {
	limit, _ := queryInt(r, "limit")
	tags, err := s.items.TagsWithCounts(nil, r.URL.Query().Get("prefix"), int(limit))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": tags})
}
