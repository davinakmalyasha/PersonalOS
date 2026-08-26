package server

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/domain/planner"
	"github.com/go-chi/chi/v5"
)

// ---- ICS import (phase 13c) ----

const maxICSBytes = 2 << 20 // 2 MiB

// POST /events/import.ics — multipart field "file" OR JSON {url}.
// Idempotent: events with an already-known UID are skipped.
func (s *Server) handleImportICS(w http.ResponseWriter, r *http.Request) {
	var text string

	ct := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "multipart/form-data"):
		r.Body = http.MaxBytesReader(w, r.Body, maxICSBytes+1<<20)
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			fail(w, http.StatusBadRequest, "file too large (max 2 MiB) or bad multipart", fieldError{"file", err.Error()})
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			fail(w, http.StatusBadRequest, "missing file field", fieldError{"file", "multipart field 'file' required"})
			return
		}
		defer file.Close()
		buf, err := io.ReadAll(io.LimitReader(file, maxICSBytes))
		if err != nil {
			fail(w, http.StatusBadRequest, "could not read file")
			return
		}
		text = string(buf)
	default:
		var req struct {
			URL  string `json:"url"`
			Text string `json:"text"`
		}
		if err := decodeJSON(r, &req, maxICSBytes); err != nil {
			fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
			return
		}
		if req.Text != "" {
			text = req.Text
		} else if req.URL != "" {
			if err := planner.ValidateICSURL(req.URL); err != nil {
				fail(w, http.StatusBadRequest, err.Error(), fieldError{"url", req.URL})
				return
			}
			fetchCtx := r.Context()
			client := &http.Client{Timeout: 10 * time.Second}
			req2, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, req.URL, nil)
			if err != nil {
				fail(w, http.StatusBadRequest, "bad url", fieldError{"url", err.Error()})
				return
			}
			resp, err := client.Do(req2)
			if err != nil {
				fail(w, http.StatusBadGateway, "could not fetch url", fieldError{"url", err.Error()})
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				fail(w, http.StatusBadGateway, "url fetch failed", fieldError{"status", http.StatusText(resp.StatusCode)})
				return
			}
			buf, err := io.ReadAll(io.LimitReader(resp.Body, maxICSBytes))
			if err != nil {
				fail(w, http.StatusBadGateway, "could not read url body")
				return
			}
			text = string(buf)
		} else {
			fail(w, http.StatusBadRequest, "provide multipart 'file', JSON text, or JSON url")
			return
		}
	}

	if !bytes.Contains([]byte(strings.ToUpper(text)), []byte("BEGIN:VCALENDAR")) &&
		!bytes.Contains([]byte(strings.ToUpper(text)), []byte("BEGIN:VEVENT")) {
		fail(w, http.StatusBadRequest, "not an iCalendar file")
		return
	}

	drafts := planner.ParseICS(text)
	imported, skipped, err := s.planner.ImportEvents(drafts, []string{"ics"})
	if err != nil {
		fail(w, http.StatusInternalServerError, "import failed", fieldError{"db", err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"imported": imported, "skipped": skipped, "total": len(drafts)})
}

// POST /events/{id}/... reserved; kept for symmetry with future sync helpers.
var _ = chi.URLParam
