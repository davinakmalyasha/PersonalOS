package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fixtureAPI serves deterministic pillar data for the four nightly passes.
func fixtureAPI(t *testing.T, over bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/finance/transfers/detect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("transfers: want POST got %s", r.Method)
		}
		_, _ = w.Write([]byte(`{"paired":2}`))
	})
	mux.HandleFunc("/v1/finance/recurring", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[
			{"merchant":"Netflix","amount_minor":-186000,"occurrences":6,"next_guess":"2026-09-01","days_left":3},
			{"merchant":"Spotify","amount_minor":-54900,"occurrences":9,"next_guess":"2026-10-11","days_left":43}
		]}`))
	})
	mux.HandleFunc("/v1/items/expiring", func(w http.ResponseWriter, r *http.Request) {
		if q := r.URL.Query().Get("days"); q != "30" {
			t.Fatalf("expiring days param: %q", q)
		}
		_, _ = w.Write([]byte(`{"items":[{"title":"Passport","date":"2026-09-15","days_left":17}]}`))
	})
	mux.HandleFunc("/v1/finance/summary", func(w http.ResponseWriter, r *http.Request) {
		flag := "false"
		if over {
			flag = "true"
		}
		_, _ = w.Write([]byte(`{"budgets":[
			{"category_name":"Food","budget_minor":2000000,"spent_minor":2400000,"over":` + flag + `},
			{"category_name":"Transport","budget_minor":500000,"spent_minor":100000,"over":false}
		]}`))
	})
	return httptest.NewServer(mux)
}

func TestRunnerCollectsFindings(t *testing.T) {
	srv := fixtureAPI(t, true)
	defer srv.Close()

	r := &Runner{Client: NewClient(srv.URL, "")}
	d, errs := r.Run(context.Background(), time.Date(2026, 8, 26, 3, 0, 0, 0, time.UTC))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if d.Paired != 2 {
		t.Errorf("paired = %d, want 2", d.Paired)
	}
	if d.Subscriptions != 2 || len(d.BillsDue) != 1 {
		t.Fatalf("subs=%d billsDue=%d, want 2/1", d.Subscriptions, len(d.BillsDue))
	}
	if d.BillsDue[0].Merchant != "Netflix" {
		t.Errorf("only Netflix is due ≤7d, got %+v", d.BillsDue)
	}
	if len(d.Expiring) != 1 || d.Expiring[0].Title != "Passport" {
		t.Errorf("expiring wrong: %+v", d.Expiring)
	}
	if len(d.OverBudgetLines) != 1 || d.OverBudgetLines[0].CategoryName != "Food" {
		t.Errorf("over-budget wrong: %+v", d.OverBudgetLines)
	}
	if !d.HasFindings() {
		t.Fatal("digest should have findings")
	}

	text := d.Render(time.Date(2026, 8, 26, 3, 0, 0, 0, time.UTC))
	for _, want := range []string{"nightly digest — 2026-08-26", "transfers paired: 2", "Netflix", "Passport", "Food"} {
		if !strings.Contains(text, want) {
			t.Errorf("render missing %q in:\n%s", want, text)
		}
	}
}

func TestRunnerQuietWhenNothingOver(t *testing.T) {
	srv := fixtureAPI(t, false)
	defer srv.Close()

	r := &Runner{Client: NewClient(srv.URL, "")}
	d, errs := r.Run(context.Background(), time.Date(2026, 8, 26, 3, 0, 0, 0, time.UTC))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// Transfers paired>0 still counts as a finding; zero it to test the quiet path.
	d.Paired = 0
	d.BillsDue = nil
	d.Expiring = nil
	if d.HasFindings() {
		t.Fatal("no findings expected when budgets are under and lists empty")
	}
	if text := d.Render(time.Now()); text != "" {
		t.Fatalf("quiet digest should render empty, got %q", text)
	}
}

func TestClientSendsBearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"paired":0}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret-token")
	if _, err := c.DetectTransfers(context.Background()); err != nil {
		t.Fatalf("detect: %v", err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("auth header = %q", gotAuth)
	}
}

func TestClientSurfacesAPIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	_, err := c.Recurring(context.Background())
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("want status error, got %v", err)
	}
}
