package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/db"
)

// Fixture: 4 unique transactions + 1 exact in-file duplicate.
const bankCSV = `Tanggal,Keterangan,Debit,Kredit
01/08/2026,STARBUCKS COFFEE PLAZA INDONESIA,85000,,
01/08/2026,STARBUCKS COFFEE PLAZA INDONESIA,85000,,
02/08/2026,GAJIAN BULANAN AGUSTUS,,12500000,
03/08/2026,TOKO BUKU GRAMEDIA,175500,,
05/08/2026,BIAYA ADMIN BANK,6500,,
`

func newTestAPI(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	sqlDB, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Migrate(sqlDB, "../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return New(sqlDB, zerolog.Nop(), "")
}

func doJSON(t *testing.T, h http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func mustCreateAccount(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/accounts", map[string]interface{}{
		"name": "BCA Checking", "type": "checking", "currency": "IDR",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create account: %d %s", rec.Code, rec.Body.String())
	}
	var a struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &a)
	return a.ID
}

func importCSV(t *testing.T, h http.Handler, accountID, csvText string) map[string]interface{} {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("account_id", accountID); err != nil {
		t.Fatal(err)
	}
	fw, err := mw.CreateFormFile("file", "statement.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(csvText)); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/transactions/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("import failed: %d %s", rec.Code, rec.Body.String())
	}
	var res map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	return res
}

func num(m map[string]interface{}, k string) int64 {
	v, _ := m[k].(float64)
	return int64(v)
}

// TestImportTwiceZeroDuplicates is THE Phase 2 acceptance criterion.
func TestImportTwiceZeroDuplicates(t *testing.T) {
	h := newTestAPI(t)
	accountID := mustCreateAccount(t, h)

	first := importCSV(t, h, accountID, bankCSV)
	if got := num(first, "imported"); got != 4 {
		t.Fatalf("first import imported = %d, want 4 (%v)", got, first)
	}
	if got := num(first, "skipped"); got != 1 {
		t.Fatalf("first import skipped = %d, want 1 (in-file dup)", got)
	}

	second := importCSV(t, h, accountID, bankCSV)
	if got := num(second, "imported"); got != 0 {
		t.Fatalf("ACCEPTANCE VIOLATION: second import inserted %d rows", got)
	}
	// All 5 rows are duplicates now: 4 already in DB + 1 in-file duplicate.
	if got := num(second, "skipped"); got != 5 {
		t.Fatalf("second import skipped = %d, want 5", got)
	}

	list := doJSON(t, h, http.MethodGet,
		fmt.Sprintf("/v1/transactions?account_id=%s&page_size=100", accountID), nil)
	var lr struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(list.Body.Bytes(), &lr)
	if lr.Total != 4 {
		t.Fatalf("total after double import = %d, want 4", lr.Total)
	}
}

func TestRulesAutoCategorizeOnImport(t *testing.T) {
	h := newTestAPI(t)
	accountID := mustCreateAccount(t, h)

	makeCat := func(name string) string {
		rec := doJSON(t, h, http.MethodPost, "/v1/categories", map[string]string{"name": name})
		if rec.Code != http.StatusCreated {
			t.Fatalf("category %s: %s", name, rec.Body.String())
		}
		var c struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &c)
		return c.ID
	}
	coffee := makeCat("Coffee")
	books := makeCat("Books")

	for _, ruleDef := range []struct {
		pattern, catID string
	}{
		{"starbucks", coffee},
		{"gramedia", books},
	} {
		rec := doJSON(t, h, http.MethodPost, "/v1/rules", map[string]interface{}{
			"pattern": ruleDef.pattern, "category_id": ruleDef.catID, "priority": 10,
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("rule %s: %s", ruleDef.pattern, rec.Body.String())
		}
	}

	res := importCSV(t, h, accountID, bankCSV)
	if got := num(res, "auto_categorized"); got != 2 {
		t.Fatalf("auto_categorized = %d, want 2 (starbucks + gramedia)", got)
	}

	for _, check := range []struct {
		catID string
		want  int64
	}{
		{coffee, 1}, {books, 1},
	} {
		list := doJSON(t, h, http.MethodGet,
			fmt.Sprintf("/v1/transactions?account_id=%s&category_id=%s", accountID, check.catID), nil)
		var lr struct {
			Total int `json:"total"`
		}
		_ = json.Unmarshal(list.Body.Bytes(), &lr)
		if int64(lr.Total) != check.want {
			t.Fatalf("txns in category = %d, want %d", lr.Total, check.want)
		}
	}
}

func TestSummaryMatchesSpotCheck(t *testing.T) {
	h := newTestAPI(t)
	accountID := mustCreateAccount(t, h)
	res := importCSV(t, h, accountID, bankCSV)
	if num(res, "imported") != 4 {
		t.Fatalf("setup import: %v", res)
	}

	sum := doJSON(t, h, http.MethodGet, "/v1/finance/summary?month=2026-08", nil)
	if sum.Code != http.StatusOK {
		t.Fatalf("summary: %d %s", sum.Code, sum.Body.String())
	}
	var s struct {
		Income  int64 `json:"income_minor"`
		Outcome int64 `json:"outcome_minor"`
		Net     int64 `json:"net_minor"`
	}
	_ = json.Unmarshal(sum.Body.Bytes(), &s)

	// Manual spot check from fixture minor units:
	// income = 12,500,000.00 -> 1,250,000,000 minor
	// outcome = 85,000 + 175,500 + 6,500 = 267,000.00 -> 26,700,000 minor
	if s.Income != 1250000000 {
		t.Fatalf("income = %d, want 1250000000", s.Income)
	}
	if s.Outcome != 26700000 {
		t.Fatalf("outcome = %d, want 26700000", s.Outcome)
	}
	if s.Net != s.Income-s.Outcome {
		t.Fatalf("net inconsistent: %+v", s)
	}
}

func TestAuthBlocksFinanceRoutes(t *testing.T) {
	dir := t.TempDir()
	sqlDB, err := db.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Migrate(sqlDB, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	h := New(sqlDB, zerolog.Nop(), "secret-token")

	rec := doJSON(t, h, http.MethodGet, "/v1/accounts", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/accounts", strings.NewReader(""))
	req.Header.Set("Authorization", "Bearer secret-token")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 with token, got %d", rec2.Code)
	}
}
