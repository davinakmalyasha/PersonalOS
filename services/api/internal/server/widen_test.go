package server

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func seedSubscription(t *testing.T, h http.Handler, merchant string, amount int64, nextDue string) map[string]interface{} {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/subscriptions", map[string]interface{}{
		"merchant": merchant, "amount_minor": amount, "next_due": nextDue,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create subscription: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

// ---- Managed subscriptions: sync + lifecycle ----

func TestSubscriptionsSyncAndLifecycle(t *testing.T) {
	h := newTestAPI(t)
	account := mustCreateAccount(t, h)
	for _, d := range []string{"2026-06-05", "2026-07-05", "2026-08-05"} {
		doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
			"account_id": account, "amount_minor": -186000, "date": d, "merchant": "NETFLIX",
		})
	}

	// Sync upserts detection into managed rows.
	rec := doJSON(t, h, http.MethodPost, "/v1/finance/subscriptions/sync", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: %d %s", rec.Code, rec.Body.String())
	}
	var synced struct {
		Detected int                      `json:"detected"`
		Created  int                      `json:"created"`
		Items    []map[string]interface{} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &synced)
	if synced.Detected != 1 || synced.Created != 1 || len(synced.Items) != 1 {
		t.Fatalf("sync result wrong: %+v", synced)
	}
	if synced.Items[0]["status"] != "active" {
		t.Fatalf("new sub should be active: %+v", synced.Items[0])
	}

	// Idempotent second sync: no new rows.
	rec = doJSON(t, h, http.MethodPost, "/v1/finance/subscriptions/sync", nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &synced)
	if synced.Created != 0 {
		t.Fatalf("second sync should not create, got %+v", synced)
	}

	id := synced.Items[0]["id"].(string)

	// Mute then cancel.
	rec = doJSON(t, h, http.MethodPatch, "/v1/subscriptions/"+id, map[string]interface{}{"status": "muted"})
	if rec.Code != http.StatusOK {
		t.Fatalf("mute: %d %s", rec.Code, rec.Body.String())
	}
	var sub map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &sub)
	if sub["status"] != "muted" {
		t.Fatalf("expected muted, got %v", sub["status"])
	}

	// Status filter.
	rec = doJSON(t, h, http.MethodGet, "/v1/subscriptions?status=active", nil)
	var active struct {
		Items []map[string]interface{} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &active)
	if len(active.Items) != 0 {
		t.Fatal("muted sub must not appear under status=active")
	}

	// Manual creation + duplicate conflict.
	seedSubscription(t, h, "Rent", -3500000, "2026-09-01")
	rec = doJSON(t, h, http.MethodPost, "/v1/subscriptions", map[string]interface{}{
		"merchant": "Rent", "amount_minor": -3500000,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate subscription should 409, got %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/subscriptions", map[string]interface{}{
		"merchant": "Bad", "amount_minor": -5, "cadence": "biweekly",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad cadence should 400, got %d", rec.Code)
	}
}

// ---- Safe to spend ----

func TestSafeToSpendMath(t *testing.T) {
	h := newTestAPI(t)
	account := mustCreateAccount(t, h)
	cat := doJSON(t, h, http.MethodPost, "/v1/categories", map[string]interface{}{"name": "Food"})
	var c struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(cat.Body.Bytes(), &c)

	month := time.Now().UTC().Format("2006-01")
	day := func(n int) string { return month + "-" + n2(n) }

	// Income +5000k, spend -800k in the category with a 2000k budget.
	doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": account, "amount_minor": 5000000, "date": day(3), "merchant": "SALARY",
	})
	doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": account, "amount_minor": -800000, "date": day(10), "merchant": "Groceries", "category_id": c.ID,
	})
	doJSON(t, h, http.MethodPost, "/v1/budgets", map[string]interface{}{
		"category_id": c.ID, "month": month, "amount_minor": 2000000,
	})

	// No active subscriptions seeded → BillsAhead = 0.
	rec := doJSON(t, h, http.MethodGet, "/v1/finance/safe-to-spend?month="+month, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("safe-to-spend: %d %s", rec.Code, rec.Body.String())
	}
	var sts struct {
		IncomeMinor int64 `json:"income_mtd_minor"`
		SpendMinor  int64 `json:"spend_mtd_minor"`
		BudgetLeft  int64 `json:"budget_left_minor"`
		BillsAhead  int64 `json:"bills_ahead_minor"`
		Safe        int64 `json:"safe_to_spend_minor"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sts)
	if sts.IncomeMinor != 5000000 || sts.SpendMinor != 800000 {
		t.Fatalf("mtd wrong: %+v", sts)
	}
	if sts.BudgetLeft != 1200000 {
		t.Fatalf("budget left wrong: %+v", sts)
	}
	if sts.Safe != 5000000-800000-1200000 {
		t.Fatalf("safe math wrong: %+v", sts)
	}
}

func n2(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// ---- Forecast + low-balance alert ----

func TestForecastProjectsSubscriptionCharges(t *testing.T) {
	h := newTestAPI(t)
	mustCreateAccountWithOpening(t, h, 10000000)

	in30 := time.Now().UTC().AddDate(0, 0, 30).Format("2006-01-02")
	seedSubscription(t, h, "Rent", -3000000, in30)

	rec := doJSON(t, h, http.MethodGet, "/v1/finance/forecast?days=45&alert_below=7000001", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("forecast: %d %s", rec.Code, rec.Body.String())
	}
	var fc struct {
		Start    int64  `json:"start_minor"`
		AvgDaily int64  `json:"avg_daily_net_minor"`
		Lowest   *struct {
			Date           string `json:"date"`
			ProjectedMinor int64  `json:"projected_minor"`
		} `json:"lowest"`
		Points []struct {
			Date           string `json:"date"`
			ProjectedMinor int64  `json:"projected_minor"`
		} `json:"points"`
		Alert bool `json:"alert"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &fc)
	if fc.Start != 10000000 {
		t.Fatalf("opening balance not used as start: %v", fc.Start)
	}
	if len(fc.Points) != 45 {
		t.Fatalf("want 45 points, got %d", len(fc.Points))
	}
	// Flat before the charge, dropped after it.
	if fc.Points[0].ProjectedMinor != 10000000 {
		t.Fatalf("day 1 should be unchanged (no avg flow), got %v", fc.Points[0].ProjectedMinor)
	}
	last := fc.Points[len(fc.Points)-1].ProjectedMinor
	if last != 10000000-3000000 {
		t.Fatalf("charge missing from projection: last=%v want %v", last, 10000000-3000000)
	}
	if fc.Lowest == nil || fc.Lowest.ProjectedMinor != 7000000 {
		t.Fatalf("lowest wrong: %+v", fc.Lowest)
	}
	if !fc.Alert {
		t.Fatal("alert_below=7,000,001 should trigger when lowest hits 7M")
	}
}

func mustCreateAccountWithOpening(t *testing.T, h http.Handler, opening int64) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/accounts", map[string]interface{}{
		"name": "BCA Checking", "type": "checking", "currency": "IDR",
		"opening_balance_minor": opening,
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

// ---- Rules: amount windows + backfill ----

func TestRuleAmountWindowAndBackfill(t *testing.T) {
	h := newTestAPI(t)
	account := mustCreateAccount(t, h)
	catFood := doJSON(t, h, http.MethodPost, "/v1/categories", map[string]interface{}{"name": "Food"})
	var cf struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(catFood.Body.Bytes(), &cf)
	catBig := doJSON(t, h, http.MethodPost, "/v1/categories", map[string]interface{}{"name": "Big Spend"})
	var cb struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(catBig.Body.Bytes(), &cb)

	// Two transactions matching pattern; only the big one is inside the window.
	doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": account, "amount_minor": -2500000, "date": "2026-08-01",
		"raw_description": "KULKAS STORE PURCHASE", "merchant": "Kulkas Store",
	})
	doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": account, "amount_minor": -45000, "date": "2026-08-02",
		"raw_description": "kulkas store snack", "merchant": "Kulkas Snack",
	})

	rule := doJSON(t, h, http.MethodPost, "/v1/rules", map[string]interface{}{
		"pattern": "kulkas", "category_id": cb.ID,
		"amount_min": 1000000, // only |amount| >= 1M matches (the -2.5M row)
	})
	if rule.Code != http.StatusCreated {
		t.Fatalf("rule create: %d %s", rule.Code, rule.Body.String())
	}
	var rr struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rule.Body.Bytes(), &rr)

	rec := doJSON(t, h, http.MethodPost, "/v1/rules/"+rr.ID+"/apply", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Updated int64 `json:"updated"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Updated != 1 {
		t.Fatalf("backfill should move exactly 1 txn, moved %d", out.Updated)
	}

	// Verify the right one moved (the -2.5M row).
	rec = doJSON(t, h, http.MethodGet, "/v1/transactions?from=2026-08-01&to=2026-08-31&page_size=50", nil)
	var lr struct {
		Items []struct {
			Merchant     string `json:"merchant"`
			CategoryName string `json:"category_name"`
		} `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &lr)
	for _, it := range lr.Items {
		wantCat := ""
		if it.Merchant == "Kulkas Store" {
			wantCat = "Big Spend"
		}
		if it.CategoryName != wantCat {
			t.Fatalf("%s categorized as %q, want %q", it.Merchant, it.CategoryName, wantCat)
		}
	}
}

// ---- Transaction tags ----

func TestTransactionTagsLifecycle(t *testing.T) {
	h := newTestAPI(t)
	account := mustCreateAccount(t, h)

	rec := doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": account, "amount_minor": -75000, "date": "2026-08-15",
		"merchant": "Toko Kopi", "tags": []string{"work", "coffee"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var t1 struct {
		ID   string   `json:"id"`
		Tags []string `json:"tags"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &t1)
	if len(t1.Tags) != 2 {
		t.Fatalf("tags not stored: %+v", t1.Tags)
	}

	// Filter by tag.
	rec = doJSON(t, h, http.MethodGet, "/v1/transactions?tag=coffee", nil)
	var lr struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &lr)
	if lr.Total != 1 {
		t.Fatalf("tag filter total = %d, want 1", lr.Total)
	}

	// Replace tags on update.
	rec = doJSON(t, h, http.MethodPatch, "/v1/transactions/"+t1.ID, map[string]interface{}{
		"tags": []string{"personal"},
	})
	_ = json.Unmarshal(rec.Body.Bytes(), &t1)
	if len(t1.Tags) != 1 || t1.Tags[0] != "personal" {
		t.Fatalf("tag update failed: %+v", t1.Tags)
	}
}

// ---- Net worth v2: opening balances + liabilities ----

func TestNetWorthAssetsLiabilities(t *testing.T) {
	h := newTestAPI(t)
	asset := mustCreateAccountWithOpening(t, h, 10000000)
	rec := doJSON(t, h, http.MethodPost, "/v1/accounts", map[string]interface{}{
		"name": "Car Loan", "type": "card", "kind": "liability", "opening_balance_minor": 4000000,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("liability account: %d %s", rec.Code, rec.Body.String())
	}
	var loan struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &loan)

	// Pay down the loan by 500k (money out of asset into liability reduction).
	doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": loan.ID, "amount_minor": -500000, "date": "2026-08-20", "merchant": "Loan payment",
	})

	rec = doJSON(t, h, http.MethodGet, "/v1/finance/net-worth", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("net-worth: %d %s", rec.Code, rec.Body.String())
	}
	var nw struct {
		Assets      int64                       `json:"assets_minor"`
		Liabilities int64                       `json:"liabilities_minor"`
		Net         int64                       `json:"net_minor"`
		Accounts    []map[string]interface{}    `json:"accounts"`
		Points      []map[string]interface{}    `json:"points"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &nw)
	// Loan balance: 4,000,000 − 500,000 = 3,500,000 owed.
	if nw.Liabilities != 3500000 {
		t.Fatalf("liabilities = %v, want 3500000", nw.Liabilities)
	}
	if nw.Assets != 10000000 {
		t.Fatalf("assets = %v, want 10000000", nw.Assets)
	}
	if nw.Net != 6500000 {
		t.Fatalf("net = %v, want 6500000", nw.Net)
	}
	if len(nw.Points) == 0 {
		t.Fatal("points missing")
	}
	foundAsset := false
	for _, a := range nw.Accounts {
		if a["account_id"] == asset {
			foundAsset = true
			if a["kind"] != "asset" {
				t.Fatalf("asset kind wrong: %v", a["kind"])
			}
		}
	}
	if !foundAsset {
		t.Fatal("asset account missing from net-worth breakdown")
	}
}
