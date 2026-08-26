package server

import (
	"errors"
	"io"
	"net/http"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/domain/finance"
	"github.com/davinakmalyasha/PersonalOS/services/api/internal/store"
)

// ---- Transactions CRUD ----

func (s *Server) handleCreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID      string   `json:"account_id"`
		AmountMinor    *int64   `json:"amount_minor"`
		Date           string   `json:"date"`
		Merchant       string   `json:"merchant"`
		RawDescription string   `json:"raw_description"`
		CategoryID     *string  `json:"category_id"`
		Tags           []string `json:"tags"`
		Currency       string   `json:"currency"`
		Notes          string   `json:"notes"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	details := []fieldError{}
	if req.AccountID == "" {
		details = append(details, fieldError{"account_id", "required"})
	}
	if req.AmountMinor == nil || *req.AmountMinor == 0 {
		details = append(details, fieldError{"amount_minor", "required non-zero integer"})
	}
	if len(req.Date) != 10 {
		details = append(details, fieldError{"date", "format YYYY-MM-DD"})
	}
	if req.RawDescription == "" && req.Merchant == "" {
		details = append(details, fieldError{"raw_description", "required when merchant empty"})
	}
	if len(details) > 0 {
		fail(w, http.StatusBadRequest, "invalid transaction", details...)
		return
	}
	t, err := s.finance.CreateTransaction(req.AccountID, *req.AmountMinor, req.Date,
		req.Merchant, req.RawDescription, req.Notes, req.CategoryID, req.Tags, req.Currency)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			fail(w, http.StatusConflict, "duplicate transaction (same date+amount+description) already exists")
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := queryInt(r, "page")
	size, _ := queryInt(r, "page_size")
	fl := store.TxnFilter{
		AccountID:  q.Get("account_id"),
		CategoryID: q.Get("category_id"),
		Uncat:      q.Get("category_id") == "none",
		From:       q.Get("from"),
		To:         q.Get("to"),
		Tag:        q.Get("tag"),
		Q:          q.Get("q"),
		Page:       int(page),
		PageSize:   int(size),
	}
	if v, ok := queryInt(r, "min"); ok {
		fl.Min = &v
	}
	if v, ok := queryInt(r, "max"); ok {
		fl.Max = &v
	}
	if fl.CategoryID == "none" {
		fl.CategoryID = ""
	}
	items, total, err := s.finance.ListTransactions(fl)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items, "total": total, "page": fl.Page, "page_size": fl.PageSize,
	})
}

func (s *Server) handleGetTransaction(w http.ResponseWriter, r *http.Request) {
	t, err := s.finance.GetTransaction(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleUpdateTransaction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date           *string  `json:"date"`
		AmountMinor    *int64   `json:"amount_minor"`
		Merchant       *string  `json:"merchant"`
		RawDescription *string  `json:"raw_description"`
		Notes          *string  `json:"notes"`
		CategoryID     **string `json:"category_id"`
		Tags           *[]string `json:"tags"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	upd := store.TransactionUpdate{Date: req.Date, Amount: req.AmountMinor, Merchant: req.Merchant,
		RawDescription: req.RawDescription, Notes: req.Notes, Tags: req.Tags}
	if req.CategoryID != nil {
		upd.CategoryID = *req.CategoryID // **string -> *string (empty string clears)
	}
	t, err := s.finance.UpdateTransaction(chiURLParam(r, "id"), upd)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			fail(w, http.StatusConflict, "update collides with an existing transaction natural key")
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleDeleteTransaction(w http.ResponseWriter, r *http.Request) {
	if err := s.finance.DeleteTransaction(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- CSV Import ----

const maxImportBytes = 16 << 20

func (s *Server) handleImportTransactions(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes+1<<20)
	if err := r.ParseMultipartForm(maxImportBytes); err != nil {
		fail(w, http.StatusBadRequest, "multipart form required: file + account_id", fieldError{"form", err.Error()})
		return
	}
	accountID := r.FormValue("account_id")
	if accountID == "" {
		fail(w, http.StatusBadRequest, "account_id required", fieldError{"account_id", "required"})
		return
	}
	if _, err := s.finance.GetAccount(accountID); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		fail(w, http.StatusBadRequest, "file field required", fieldError{"file", err.Error()})
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(file)
	if err != nil {
		fail(w, http.StatusBadRequest, "cannot read file", fieldError{"file", err.Error()})
		return
	}

	override := parseMappingOverride(r)
	dateFormat := r.FormValue("date_format")

	// No explicit mapping → fall back to the account's saved import profile.
	if override == nil && dateFormat == "" {
		if profile, perr := s.finance.GetImportProfile(accountID); perr == nil && profile != nil {
			override = profileToOverride(profile)
			dateFormat = profile.DateFormat
		}
	}

	rows, rowErrs, resolvedMapping, resolvedLayout, err := finance.ParseCSV(bytesReader(data), override, dateFormat)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error(), fieldError{"csv", err.Error()})
		return
	}

	existing, err := s.finance.ExistingKeys(accountID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ruleRows, err := s.finance.ListRules()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	rules := make([]finance.Rule, len(ruleRows))
	for i, rr := range ruleRows {
		rules[i] = finance.Rule{ID: rr.ID, Pattern: rr.Pattern, CategoryID: rr.CategoryID,
			Priority: rr.Priority, AmountMin: rr.AmountMin, AmountMax: rr.AmountMax}
	}

	currency := r.FormValue("currency")
	if currency == "" {
		currency = "IDR"
	}
	drafts, res := finance.Prepare(rows, rowErrs, existing, rules)

	inserted, err := s.finance.ImportTransactions(accountID, currency, drafts)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Persist the resolved mapping as the account's import profile so future
	// imports skip detection (profile is overridden by explicit form values).
	if resolvedMapping != nil {
		_ = s.finance.SaveImportProfile(accountID, store.ImportProfile{
			Mapping: map[string]int{
				"date_col": resolvedMapping.Date, "desc_col": resolvedMapping.Description,
				"amount_col": resolvedMapping.Amount, "debit_col": resolvedMapping.Debit,
				"credit_col": resolvedMapping.Credit, "merchant_col": resolvedMapping.Merchant,
			},
			DateFormat: resolvedLayout,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"imported":         inserted,
		"skipped":          res.Skipped,
		"skipped_invalid":  res.SkippedInvalid,
		"auto_categorized": res.AutoCategorized,
		"errors":           res.Errors,
	})
}

// profileToOverride converts a saved import profile into a column mapping.
func profileToOverride(p *store.ImportProfile) *finance.ColumnMapping {
	get := func(k string) int {
		if v, ok := p.Mapping[k]; ok && v >= 0 {
			return v
		}
		return -1
	}
	m := &finance.ColumnMapping{
		Date:        get("date_col"),
		Description: get("desc_col"),
		Amount:      get("amount_col"),
		Debit:       get("debit_col"),
		Credit:      get("credit_col"),
		Merchant:    get("merchant_col"),
	}
	if m.Date < 0 || m.Description < 0 || (m.Amount < 0 && m.Debit < 0 && m.Credit < 0) {
		return nil // incomplete profile → fall back to auto-detect
	}
	return m
}

// parseMappingOverride reads explicit 0-based column indexes when provided.
func parseMappingOverride(r *http.Request) *finance.ColumnMapping {
	get := func(k string) int {
		v := r.FormValue(k)
		if v == "" {
			return -1
		}
		n, err := atoi(v)
		if err != nil || n < 0 {
			return -1
		}
		return n
	}
	date := get("date_col")
	desc := get("desc_col")
	amount := get("amount_col")
	debit := get("debit_col")
	credit := get("credit_col")
	merchant := get("merchant_col")
	if date < 0 && desc < 0 && amount < 0 && debit < 0 && credit < 0 {
		return nil // nothing overridden; auto-detect
	}
	if date < 0 || desc < 0 || (amount < 0 && debit < 0 && credit < 0) {
		return nil // incomplete override → fall back to auto-detect
	}
	return &finance.ColumnMapping{
		Date: date, Description: desc, Amount: amount,
		Debit: debit, Credit: credit, Merchant: merchant,
	}
}

// ---- Summaries ----

func (s *Server) handleFinanceSummary(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	sum, err := s.finance.SummaryMonth(month)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "month must be YYYY-MM", fieldError{"month", "format YYYY-MM"})
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

func (s *Server) handleFinanceSpending(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	groupBy := q.Get("group_by")
	from := q.Get("from")
	to := q.Get("to")
	if from == "" {
		from = "0000-01-01"
	}
	points, err := s.finance.SpendingSeries(groupBy, from, to)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "group_by must be month|category", fieldError{"group_by", "month|category"})
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"group_by": orDefault(groupBy, "month"), "points": points})
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
