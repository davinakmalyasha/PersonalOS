// Package notify fans a digest out to the configured webhook channels
// (Telegram and/or Discord). Failures are returned, never fatal.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Notifier delivers one text message per channel.
type Notifier interface {
	Notify(ctx context.Context, text string) error
	Name() string
}

// Multi fans out to every configured notifier; first error wins (all still attempted).
type Multi []Notifier

func (m Multi) Notify(ctx context.Context, text string) error {
	var firstErr error
	for _, n := range m {
		if err := n.Notify(ctx, text); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m Multi) Name() string {
	names := make([]string, len(m))
	for i, n := range m {
		names[i] = n.Name()
	}
	return strings.Join(names, "+")
}

// ---- Telegram ----

type Telegram struct {
	BotToken string
	ChatID   string
	BaseURL  string // override for tests; default https://api.telegram.org
	Client   *http.Client
}

func (t Telegram) Name() string { return "telegram" }

func (t Telegram) Notify(ctx context.Context, text string) error {
	base := t.BaseURL
	if base == "" {
		base = "https://api.telegram.org"
	}
	url := fmt.Sprintf("%s/bot%s/sendMessage", strings.TrimRight(base, "/"), t.BotToken)
	body, _ := json.Marshal(map[string]string{
		"chat_id": t.ChatID,
		"text":    text,
	})
	return postJSON(ctx, t.client(), url, body)
}

// ---- Discord ----

type Discord struct {
	WebhookURL string
	Client     *http.Client
}

func (d Discord) Name() string { return "discord" }

func (d Discord) Notify(ctx context.Context, text string) error {
	body, _ := json.Marshal(map[string]string{"content": text})
	return postJSON(ctx, d.client(), d.WebhookURL, body)
}

// ---- shared ----

func (t Telegram) client() *http.Client {
	if t.Client != nil {
		return t.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (d Discord) client() *http.Client {
	if d.Client != nil {
		return d.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func postJSON(ctx context.Context, client *http.Client, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("webhook %s: status %d: %s", url, res.StatusCode, strings.TrimSpace(string(b)))
	}
	_, _ = io.Copy(io.Discard, res.Body)
	return nil
}
