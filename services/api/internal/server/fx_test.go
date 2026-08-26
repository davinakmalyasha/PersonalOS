package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

func mustCreateAccountFull(t *testing.T, h http.Handler, name, typ, currency, kind string, opening int64) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/accounts", map[string]interface{}{
		"name": name, "type": typ, "currency": currency, "kind": kind,
		"opening_balance_minor": opening,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %s: %d %s", name, rec.Code, rec.Body.String())
	}
	var a struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &a)
	return a.ID
}

// ---- FX rates: net worth + summary convert across currencies ----

func TestFXConvertsNetWorthAndSummary(t *testing.T) {
	h := newTestAPI(t)

	// IDR account (base) 10M; USD account opening $1,000 at rate 16,000.
	idr := mustCreateAccountWithOpening(t, h, 10000000)
	usd := mustCreateAccountFull(t, h, "USD Wallet", "cash", "USD", "asset", 100000) // $1,000.00

	// Spend $50 from the USD account.
	doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": usd, "amount_minor": -5000, "date": "2026-08-10", "merchant": "Coffee USD", "currency": "USD",
	})

	// Set rate AFTER transactions: conversion happens at report time.
	// Minor-unit semantics: USD cents -> IDR ≈ 160 when 1 USD = 16,000 IDR.
	rec := doJSON(t, h, http.MethodPut, "/v1/finance/fx", map[string]interface{}{"code": "USD", "rate_to_base": 160})
	if rec.Code != http.StatusOK {
		t.Fatalf("upsert rate: %d %s", rec.Code, rec.Body.String())
	}

	// Net worth: both USD rows convert at report time:
	// (100000 − 5000) minor × 160 = 15,200,000 → assets = 25,200,000.
	rec = doJSON(t, h, http.MethodGet, "/v1/finance/net-worth", nil)
	var nw struct {
		Assets int64                    `json:"assets_minor"`
		Net    int64                    `json:"net_minor"`
		Points []map[string]interface{} `json:"points"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &nw)
	if nw.Assets != 25200000 {
		t.Fatalf("assets not fx-converted: %v", nw.Assets)
	}
	if len(nw.Points) == 0 || nw.Points[len(nw.Points)-1]["total_minor"].(float64) != float64(nw.Assets) {
		t.Fatalf("points should end at converted total: %+v", nw.Points)
	}

	// Month summary converts the $50 spend (5000 minor × 160 = 800,000).
	rec = doJSON(t, h, http.MethodGet, "/v1/finance/summary?month=2026-08", nil)
	var sum struct {
		Outcome    int64 `json:"outcome_minor"`
		ByCategory []struct {
			Name       string `json:"name"`
			SpentMinor int64  `json:"spent_minor"`
		} `json:"by_category"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sum)
	if sum.Outcome != 800000 {
		t.Fatalf("outcome not converted: %v", sum.Outcome)
	}
	if sum.ByCategory[0].SpentMinor != 800000 || sum.ByCategory[0].Name != "Uncategorized" {
		t.Fatalf("category rollup not converted: %+v", sum.ByCategory)
	}

	// Rates listing includes the base.
	rec = doJSON(t, h, http.MethodGet, "/v1/finance/fx", nil)
	var fx struct {
		Base  string `json:"base"`
		Rates []struct {
			Code       string  `json:"code"`
			RateToBase float64 `json:"rate_to_base"`
		} `json:"rates"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &fx)
	if fx.Base != "IDR" || len(fx.Rates) != 1 || fx.Rates[0].Code != "USD" {
		t.Fatalf("rates listing wrong: %+v", fx)
	}

	_ = idr
}
