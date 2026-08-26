package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) mountExtras(r chi.Router) {
	// Goals (savings + calorie).
	r.Route("/goals", func(r chi.Router) {
		r.Post("/", s.handleCreateGoal)
		r.Get("/", s.handleListGoals)
		r.Get("/{id}", s.handleGetGoal)
		r.Patch("/{id}", s.handleUpdateGoal)
		r.Delete("/{id}", s.handleDeleteGoal)
		r.Post("/{id}/add", s.handleAddToGoal)
	})
	// Finance extras.
	r.Get("/finance/recurring", s.handleRecurring)
	r.Post("/finance/transfers/detect", s.handleDetectTransfers)
	// Health extras.
	r.Post("/body-metrics/water", s.handleLogWater)
	r.Get("/health/prs", s.handleExercisePRs)
	// Universal extras.
	r.Get("/items/expiring", s.handleExpiringItems)
	// Phase 10a: finance intelligence + planner depth.
	r.Get("/finance/net-worth", s.handleNetWorth)
	r.Get("/finance/bills", s.handleUpcomingBills)
	r.Route("/merchant_aliases", func(r chi.Router) {
		r.Post("/", s.handleCreateAlias)
		r.Get("/", s.handleListAliases)
		r.Delete("/{id}", s.handleDeleteAlias)
	})
	r.Post("/events/{id}/exceptions", s.handleSetEventOverride)
	r.Get("/events/{id}/exceptions", s.handleListEventOverrides)
	r.Delete("/events/exceptions/{id}", s.handleDeleteEventOverride)
	r.Get("/planner/review", s.handlePlannerReview)

	// Phase 12a: finance depth.
	r.Route("/subscriptions", func(r chi.Router) {
		r.Post("/", s.handleCreateSubscription)
		r.Get("/", s.handleListSubscriptions)
		r.Patch("/{id}", s.handleUpdateSubscriptionStatus)
	})
	r.Post("/finance/subscriptions/sync", s.handleSyncSubscriptions)
	r.Get("/finance/safe-to-spend", s.handleSafeToSpend)
	r.Get("/finance/forecast", s.handleForecast)
	r.Route("/finance/fx", func(r chi.Router) {
		r.Get("/", s.handleListRates)
		r.Put("/", s.handleUpsertRate)
		r.Put("/base", s.handleSetBaseCurrency)
	})
	r.Get("/transactions/export.csv", s.handleExportTransactionsCSV)

	// Phase 13b: receipt attachments.
	r.Post("/transactions/{id}/receipt", s.handleUploadReceipt)
	r.Get("/transactions/{id}/receipt", s.handleGetReceipt)
	r.Delete("/transactions/{id}/receipt", s.handleDeleteReceipt)

	// Phase 13c: ICS import.
	r.Post("/events/import.ics", s.handleImportICS)

	// Phase 13d: markdown vault export.
	r.Get("/export/vault.zip", s.handleExportVault)
}

// ---- Goals ----

func (s *Server) handleCreateGoal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind        string  `json:"kind"`
		Name        string  `json:"name"`
		TargetMinor *int64  `json:"target_minor"`
		Deadline    *string `json:"deadline"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	g, err := s.finance.CreateGoal(req.Kind, req.Name, req.TargetMinor, req.Deadline)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func (s *Server) handleListGoals(w http.ResponseWriter, r *http.Request) {
	items, err := s.finance.ListGoals(r.URL.Query().Get("kind"))
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid kind filter")
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleGetGoal(w http.ResponseWriter, r *http.Request) {
	g, err := s.finance.GetGoal(chiURLParam(r, "id"))
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleUpdateGoal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        *string  `json:"name"`
		TargetMinor **int64  `json:"target_minor"`
		SavedMinor  *int64   `json:"saved_minor"`
		Deadline    **string `json:"deadline"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	g, err := s.finance.UpdateGoal(chiURLParam(r, "id"), store.GoalUpdate{
		Name: req.Name, TargetMinor: req.TargetMinor,
		SavedMinor: req.SavedMinor, Deadline: req.Deadline,
	})
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleAddToGoal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AmountMinor int64 `json:"amount_minor"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	g, err := s.finance.AddToGoal(chiURLParam(r, "id"), req.AmountMinor)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleDeleteGoal(w http.ResponseWriter, r *http.Request) {
	if err := s.finance.DeleteGoal(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Finance extras ----

func (s *Server) handleRecurring(w http.ResponseWriter, r *http.Request) {
	items, err := s.finance.FindRecurring()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleDetectTransfers(w http.ResponseWriter, r *http.Request) {
	n, err := s.finance.PairTransfers()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"paired": n})
}

// ---- Health extras ----

func (s *Server) handleLogWater(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	var req struct {
		Ml  int    `json:"ml"`
		Day string `json:"day"` // optional YYYY-MM-DD, default today
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	total, err := h.LogWater(req.Day, req.Ml)
	if err != nil {
		fail(w, http.StatusBadRequest, "ml must be 1..10000; day YYYY-MM-DD")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"water_ml": total})
}

func (s *Server) handleExercisePRs(w http.ResponseWriter, r *http.Request) {
	h, ok := s.requireHealth(w)
	if !ok {
		return
	}
	q := r.URL.Query()
	prs, err := h.ExercisePRs(q.Get("from"), q.Get("to"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": prs})
}

// ---- Universal extras ----

func (s *Server) handleExpiringItems(w http.ResponseWriter, r *http.Request) {
	days := 30
	if n, ok := queryInt(r, "days"); ok {
		days = int(n)
	}
	items, err := s.items.ExpiringItems(days)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

// ---- Phase 10a: finance intelligence ----

func (s *Server) handleNetWorth(w http.ResponseWriter, r *http.Request) {
	out, err := s.finance.NetWorthSeries()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleUpcomingBills(w http.ResponseWriter, r *http.Request) {
	days := 7
	if n, ok := queryInt(r, "days"); ok {
		days = int(n)
	}
	items, err := s.finance.UpcomingBills(days)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleCreateAlias(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pattern   string `json:"pattern"`
		Canonical string `json:"canonical"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	a, err := s.finance.CreateAlias(req.Pattern, req.Canonical)
	if err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (s *Server) handleListAliases(w http.ResponseWriter, r *http.Request) {
	items, err := s.finance.ListAliases()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleDeleteAlias(w http.ResponseWriter, r *http.Request) {
	if err := s.finance.DeleteAlias(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Phase 10a: planner depth ----

func (s *Server) handleSetEventOverride(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date     string `json:"date"`
		Action   string `json:"action"`
		Title    *string `json:"title"`
		StartsAt *string `json:"starts_at"`
		EndsAt   *string `json:"ends_at"`
		Location *string `json:"location"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	o, err := s.planner.SetEventOverride(chiURLParam(r, "id"), store.EventOverride{
		EventID: chiURLParam(r, "id"), Date: req.Date, Action: req.Action,
		Title: req.Title, StartsAt: req.StartsAt, EndsAt: req.EndsAt, Location: req.Location,
	})
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid exception",
				fieldError{"date/action/starts_at", "YYYY-MM-DD; cancel|edit; RFC3339"})
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) handleListEventOverrides(w http.ResponseWriter, r *http.Request) {
	items, err := s.planner.ListEventOverrides(chiURLParam(r, "id"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleDeleteEventOverride(w http.ResponseWriter, r *http.Request) {
	if err := s.planner.DeleteEventOverride(chiURLParam(r, "id")); err != nil {
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePlannerReview(w http.ResponseWriter, r *http.Request) {
	rb, err := s.planner.Review(time.Now(), r.URL.Query().Get("date"))
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid date", fieldError{"date", "YYYY-MM-DD"})
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Compose the finance part (spend + budgets for the containing month).
	if sum, err := s.finance.SummaryMonth(rb.Month); err == nil && sum != nil {
		rb.SpendMinor = sum.Outcome
		rb.BudgetLines = sum.BudgetLines
	}
	writeJSON(w, http.StatusOK, rb)
}

// ---- Phase 12a: finance depth handlers ----

// POST /subscriptions {merchant, amount_minor, cadence?, next_due?}
func (s *Server) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Merchant    string `json:"merchant"`
		AmountMinor int64  `json:"amount_minor"`
		Cadence     string `json:"cadence"`
		NextDue     string `json:"next_due"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	sub, err := s.finance.CreateSubscription(req.Merchant, req.AmountMinor, req.Cadence, req.NextDue)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid subscription",
				fieldError{"merchant/amount/cadence/next_due", "non-empty; non-zero; weekly|monthly|yearly; YYYY-MM-DD"})
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, sub)
}

// GET /subscriptions?status=
func (s *Server) handleListSubscriptions(w http.ResponseWriter, r *http.Request) {
	items, err := s.finance.ListSubscriptions(r.URL.Query().Get("status"))
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid status", fieldError{"status", "active|muted|cancelled"})
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

// PATCH /subscriptions/{id} {status: active|muted|cancelled}
func (s *Server) handleUpdateSubscriptionStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	sub, err := s.finance.UpdateSubscriptionStatus(chiURLParam(r, "id"), req.Status)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid status", fieldError{"status", "active|muted|cancelled"})
			return
		}
		if !mapStoreErr(w, err) {
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, sub)
}

// POST /finance/subscriptions/sync — re-run detection and upsert managed rows.
func (s *Server) handleSyncSubscriptions(w http.ResponseWriter, r *http.Request) {
	detected, err := s.finance.FindRecurring()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	created, err := s.finance.SyncSubscriptions(detected)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	items, _ := s.finance.ListSubscriptions("")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"detected": len(detected), "created": created, "items": items,
	})
}

// GET /finance/safe-to-spend?month=YYYY-MM
func (s *Server) handleSafeToSpend(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().UTC().Format("2006-01")
	}
	sts, err := s.finance.SafeToSpend(month)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid month", fieldError{"month", "YYYY-MM"})
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sts)
}

// GET /finance/forecast?days=30&alert_below= (minor units)
func (s *Server) handleForecast(w http.ResponseWriter, r *http.Request) {
	days := 30
	if n, ok := queryInt(r, "days"); ok {
		days = int(n)
	}
	fc, err := s.finance.Forecast(days)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := map[string]interface{}{
		"days": fc.Days, "start_minor": fc.StartMinor,
		"avg_daily_net_minor": fc.AvgDailyNet, "lowest": fc.Lowest, "points": fc.Points,
	}
	if v, ok := queryInt(r, "alert_below"); ok {
		willDrop, _, aerr := s.finance.LowBalanceAlert(days, v)
		if aerr == nil {
			resp["alert"] = willDrop
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// GET /transactions/export.csv?account_id&from&to — CSV portability.
func (s *Server) handleExportTransactionsCSV(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	fl := store.TxnFilter{
		AccountID: q.Get("account_id"),
		From:      q.Get("from"),
		To:        q.Get("to"),
		PageSize:  100,
	}
	var all []store.Transaction
	for page := 1; ; page++ {
		fl.Page = page
		items, total, err := s.finance.ListTransactions(fl)
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		all = append(all, items...)
		if len(all) >= total || len(items) == 0 {
			break
		}
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="transactions.csv"`)
	_, _ = w.Write([]byte("id,date,amount_minor,currency,merchant,description,category,tags,notes,is_transfer\n"))
	for _, t := range all {
		cat := ""
		if t.CategoryName != nil {
			cat = *t.CategoryName
		}
		record := []string{
			t.ID, t.Date, strconv.FormatInt(t.AmountMinor, 10), t.Currency,
			csvEscape(t.Merchant), csvEscape(t.RawDescription), csvEscape(cat),
			csvEscape(strings.Join(t.Tags, ";")), csvEscape(t.Notes),
			boolStr(t.IsTransfer),
		}
		_, _ = w.Write([]byte(strings.Join(record, ",") + "\n"))
	}
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
// ---- Phase 13a: fx rates ----

func (s *Server) handleListRates(w http.ResponseWriter, r *http.Request) {
	out := map[string]interface{}{
		"base": s.finance.BaseCurrency(),
	}
	rates, err := s.finance.ListRates()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	out["rates"] = rates
	writeJSON(w, http.StatusOK, out)
}

// PUT /finance/fx {code, rate_to_base} — 1 unit of code = N base units.
func (s *Server) handleUpsertRate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code       string  `json:"code"`
		RateToBase float64 `json:"rate_to_base"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	rate, err := s.finance.UpsertRate(req.Code, req.RateToBase)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid rate", fieldError{"code/rate_to_base", "non-empty code; positive rate"})
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rate)
}

// PUT /finance/fx/base {code}
func (s *Server) handleSetBaseCurrency(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &req, 0); err != nil {
		fail(w, http.StatusBadRequest, "bad json", fieldError{"body", err.Error()})
		return
	}
	if err := s.finance.SetBaseCurrency(req.Code); err != nil {
		if errors.Is(err, store.ErrInvalid) {
			fail(w, http.StatusBadRequest, "invalid base currency code")
			return
		}
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"base": s.finance.BaseCurrency()})
}