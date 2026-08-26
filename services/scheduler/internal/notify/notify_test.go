package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscordPostsContent(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	d := Discord{WebhookURL: srv.URL, Client: srv.Client()}
	if err := d.Notify(context.Background(), "hello nightly"); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if got["content"] != "hello nightly" {
		t.Fatalf("content = %q", got["content"])
	}
}

func TestTelegramSendsMessage(t *testing.T) {
	var got struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	tg := Telegram{BotToken: "tok", ChatID: "42", BaseURL: srv.URL, Client: srv.Client()}
	if err := tg.Notify(context.Background(), "nightly digest"); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if gotPath != "/bottok/sendMessage" {
		t.Fatalf("path = %q", gotPath)
	}
	if got.ChatID != "42" || got.Text != "nightly digest" {
		t.Fatalf("payload = %+v", got)
	}
}

func TestMultiFansOutAndCollectsFirstError(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ok.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadRequest)
	}))
	defer bad.Close()

	m := Multi{
		Discord{WebhookURL: ok.URL, Client: ok.Client()},
		Discord{WebhookURL: bad.URL + "/hook", Client: bad.Client()},
	}
	err := m.Notify(context.Background(), "digest")
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("want first error 400, got %v", err)
	}
	if m.Name() != "discord+discord" {
		t.Fatalf("name = %q", m.Name())
	}
}
