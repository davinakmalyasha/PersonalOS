package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func chiURLParam(r *http.Request, name string) string { return chi.URLParam(r, name) }

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type fieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func fail(w http.ResponseWriter, status int, msg string, details ...fieldError) {
	writeJSON(w, status, map[string]interface{}{
		"error":   msg,
		"code":    errCode(status),
		"details": detailsOrNil(details),
	})
}

func errCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "validation_error"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	default:
		return "internal_error"
	}
}

func detailsOrNil(d []fieldError) interface{} {
	if len(d) == 0 {
		return nil
	}
	return d
}

// mapStoreErr converts store sentinel errors to responses; returns handled=true.
func mapStoreErr(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, store.ErrNotFound):
		fail(w, http.StatusNotFound, "not found")
	case errors.Is(err, store.ErrConflict):
		fail(w, http.StatusConflict, "conflict with existing data")
	case errors.Is(err, store.ErrInvalid):
		fail(w, http.StatusBadRequest, "invalid value")
	default:
		return false
	}
	return true
}

func decodeJSON(r *http.Request, dst interface{}, maxBytes int64) error {
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func queryInt(r *http.Request, key string) (int64, bool) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func strPtr(s string) *string { return &s }

// ---- Accounts ----

type accountReq struct {
	Name     *string `json:"name"`
	Type     *string `json:"type"`
	Currency *string `json:"currency"`
}

var accountTypes = map[string]bool{"checking": true, "savings": true, "cash": true, "card": true, "wallet": true}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var req accountReq
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if req.Name == nil || *req.Name == "" {
		fail(w, http.StatusBadRequest, "name required", fieldError{"name", "required"})
		return
	}
	typ := "checking"
	if req.Type != nil {
		typ = *req.Type
	}
	if !accountTypes[typ] {
		fail(w, http.StatusBadRequest, "invalid type", fieldError{"type", "one of checking|savings|cash|card|wallet"})
		return
	}
	cur := ""
	if req.Currency != nil {
		cur = *req.Currency
	}
	a, err := s.finance.CreateAccount(*req.Name, typ, cur)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	out, err := s.finance.ListAccounts()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if out == nil {
		out = []store.AccountWithBalance{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": out})
}

func (s *Server) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	a, err := s.finance.GetAccount(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleUpdateAccount(w http.ResponseWriter, r *http.Request) {
	var req accountReq
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if req.Type != nil && !accountTypes[*req.Type] {
		fail(w, http.StatusBadRequest, "invalid type", fieldError{"type", "one of checking|savings|cash|card|wallet"})
		return
	}
	a, err := s.finance.UpdateAccount(chiURLParam(r, "id"), req.Name, req.Type)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	err := s.finance.DeleteAccount(chiURLParam(r, "id"))
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			fail(w, http.StatusConflict, "account has transactions; move or delete them first")
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Categories ----

func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string  `json:"name"`
		ParentID *string `json:"parent_id"`
		Color    *string `json:"color"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if req.Name == "" {
		fail(w, http.StatusBadRequest, "name required", fieldError{"name", "required"})
		return
	}
	c, err := s.finance.CreateCategory(req.Name, req.ParentID, req.Color)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	flat, err := s.finance.ListCategories()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": buildTree(flat)})
}

type catNode struct {
	store.Category
	Children []*catNode `json:"children"`
}

func buildTree(flat []store.Category) []*catNode {
	byID := map[string]*catNode{}
	for _, c := range flat {
		byID[c.ID] = &catNode{Category: c, Children: []*catNode{}}
	}
	var roots []*catNode
	for _, n := range byID {
		if n.ParentID != nil {
			if p, ok := byID[*n.ParentID]; ok {
				p.Children = append(p.Children, n)
				continue
			}
		}
		roots = append(roots, n)
	}
	sortNodes(roots)
	for _, n := range byID {
		sortNodes(n.Children)
	}
	return roots
}

func sortNodes(ns []*catNode) {
	for i := 1; i < len(ns); i++ {
		for j := i; j > 0 && ns[j-1].Name > ns[j].Name; j-- {
			ns[j-1], ns[j] = ns[j], ns[j-1]
		}
	}
}

func (s *Server) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     *string  `json:"name"`
		ParentID **string `json:"parent_id"`
		Color    **string `json:"color"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	c, err := s.finance.UpdateCategory(chiURLParam(r, "id"), req.Name, req.ParentID, req.Color)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	reassign := r.URL.Query().Get("reassign_to")
	var rp *string
	if reassign != "" {
		rp = &reassign
	}
	err := s.finance.DeleteCategory(chiURLParam(r, "id"), rp)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			fail(w, http.StatusConflict, "category is referenced (child categories or transactions). Delete children or pass ?reassign_to=<categoryId>")
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Rules ----

func (s *Server) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pattern    string `json:"pattern"`
		CategoryID string `json:"category_id"`
		Priority   *int   `json:"priority"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if req.Pattern == "" || req.CategoryID == "" {
		fail(w, http.StatusBadRequest, "pattern and category_id required",
			fieldError{"pattern", condMsg(req.Pattern == "")},
			fieldError{"category_id", condMsg(req.CategoryID == "")})
		return
	}
	prio := 100
	if req.Priority != nil {
		prio = *req.Priority
	}
	rule, err := s.finance.CreateRule(req.Pattern, req.CategoryID, prio)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func condMsg(cond bool) string {
	if cond {
		return "required"
	}
	return ""
}

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	out, err := s.finance.ListRules()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": out})
}

func (s *Server) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pattern    *string `json:"pattern"`
		CategoryID *string `json:"category_id"`
		Priority   *int    `json:"priority"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	rule, err := s.finance.UpdateRule(chiURLParam(r, "id"), req.Pattern, req.CategoryID, req.Priority)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	if err := s.finance.DeleteRule(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Budgets ----

func (s *Server) handleUpsertBudget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CategoryID string `json:"category_id"`
		Month      string `json:"month"`
		Amount     *int64 `json:"amount_minor"`
		Rollover   *bool  `json:"rollover"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	details := []fieldError{}
	if req.CategoryID == "" {
		details = append(details, fieldError{"category_id", "required"})
	}
	if len(req.Month) != 7 {
		details = append(details, fieldError{"month", "format YYYY-MM"})
	}
	if req.Amount == nil || *req.Amount < 0 {
		details = append(details, fieldError{"amount_minor", "required >= 0"})
	}
	if len(details) > 0 {
		fail(w, http.StatusBadRequest, "invalid budget", details...)
		return
	}
	b, err := s.finance.UpsertBudget(req.CategoryID, req.Month, *req.Amount, req.Rollover != nil && *req.Rollover)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *Server) handleListBudgets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	out, err := s.finance.ListBudgets(q.Get("from"), q.Get("to"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": out})
}

func (s *Server) handleDeleteBudget(w http.ResponseWriter, r *http.Request) {
	if err := s.finance.DeleteBudget(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
