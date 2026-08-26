// Package jobs runs the nightly maintenance passes against the Go API over
// HTTP (the scheduler never touches the DB directly — same rule as MCP).
package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a thin typed client over the Personal OS API.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 256))
		return fmt.Errorf("%s %s: status %d: %s", method, path, res.StatusCode, strings.TrimSpace(string(b)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// PairResult is POST /finance/transfers/detect.
type PairResult struct {
	Paired int `json:"paired"`
}

// DetectTransfers re-runs transfer pairing; returns how many pairs were linked.
func (c *Client) DetectTransfers(ctx context.Context) (int, error) {
	var r PairResult
	if err := c.do(ctx, http.MethodPost, "/v1/finance/transfers/detect", &r); err != nil {
		return 0, err
	}
	return r.Paired, nil
}

// Subscription is one detected recurring charge.
type Subscription struct {
	Merchant    string  `json:"merchant"`
	AmountMinor int64   `json:"amount_minor"`
	Occurrences int     `json:"occurrences"`
	NextGuess   *string `json:"next_guess,omitempty"`
	DaysLeft    *int    `json:"days_left,omitempty"`
}

// Recurring recomputes the subscription detection (derived at read time).
func (c *Client) Recurring(ctx context.Context) ([]Subscription, error) {
	var r struct {
		Items []Subscription `json:"items"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/finance/recurring", &r); err != nil {
		return nil, err
	}
	return r.Items, nil
}

// ExpiringItem is a capture nearing its date-like data field.
type ExpiringItem struct {
	Title    string `json:"title"`
	Date     string `json:"date"`
	DaysLeft int    `json:"days_left"`
}

// Expiring lists items expiring within the next `days`.
func (c *Client) Expiring(ctx context.Context, days int) ([]ExpiringItem, error) {
	var r struct {
		Items []ExpiringItem `json:"items"`
	}
	path := fmt.Sprintf("/v1/items/expiring?days=%d", days)
	if err := c.do(ctx, http.MethodGet, path, &r); err != nil {
		return nil, err
	}
	return r.Items, nil
}

// BudgetLine is one monthly budget with spend progress.
type BudgetLine struct {
	CategoryName string `json:"category_name"`
	BudgetMinor  int64  `json:"budget_minor"`
	SpentMinor   int64  `json:"spent_minor"`
	Over         bool   `json:"over"`
}

// OverBudgets returns the categories over budget for the given month.
func (c *Client) OverBudgets(ctx context.Context, month string) ([]BudgetLine, error) {
	var r struct {
		Budgets []BudgetLine `json:"budgets"`
	}
	path := "/v1/finance/summary?month=" + month
	if err := c.do(ctx, http.MethodGet, path, &r); err != nil {
		return nil, err
	}
	out := make([]BudgetLine, 0, len(r.Budgets))
	for _, b := range r.Budgets {
		if b.Over {
			out = append(out, b)
		}
	}
	return out, nil
}
