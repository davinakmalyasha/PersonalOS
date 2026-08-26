package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func icsMultipart(t *testing.T, ics string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "calendar.ics")
	_, _ = fw.Write([]byte(ics))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/v1/events/import.ics", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestImportICSAndDedupe(t *testing.T) {
	h := newTestAPI(t)
	ics := "BEGIN:VCALENDAR\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:a@x\r\n" +
		"DTSTART:20260910T090000Z\r\n" +
		"SUMMARY:Standup\r\n" +
		"END:VEVENT\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:b@x\r\n" +
		"DTSTART;VALUE=DATE:20260915\r\n" +
		"SUMMARY:Offsite\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, icsMultipart(t, ics))
	if rec.Code != http.StatusOK {
		t.Fatalf("import: %d %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Imported int `json:"imported"`
		Skipped  int `json:"skipped"`
		Total    int `json:"total"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Imported != 2 || res.Skipped != 0 || res.Total != 2 {
		t.Fatalf("first import counts: %+v", res)
	}

	// Re-import the same file: everything skipped (idempotent).
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, icsMultipart(t, ics))
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Imported != 0 || res.Skipped != 2 {
		t.Fatalf("second import counts: %+v", res)
	}

	// Events exist and carry the ics tag; occurrence window shows them.
	rec = doJSON(t, h, http.MethodGet, "/v1/events?from=2026-09-01&to=2026-09-30", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var res2 struct {
		Items []struct {
			Title string   `json:"title"`
			Tags  []string `json:"tags"`
		} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res2)
	if len(res2.Items) < 2 {
		t.Fatalf("occurrences: %+v", res2)
	}

	// Bad content rejected.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, icsMultipart(t, "hello world"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad ics should 400, got %d", rec.Code)
	}
}
