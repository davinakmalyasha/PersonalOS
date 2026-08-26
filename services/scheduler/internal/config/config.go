package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config carries everything the scheduler loop needs. All optional; sane
// defaults keep local dev one-command.
type Config struct {
	APIURL   string // base URL of the Go API, e.g. http://api:8080
	APIToken string // bearer token when the API requires auth

	IntervalSeconds int  // tick period
	RunHourUTC      int  // -1 = run every tick; else run once per day at this UTC hour
	RunOnStart      bool // fire a pass immediately on boot

	TelegramBotToken string
	TelegramChatID   string
	DiscordWebhook   string

	LowBalanceDays      int    // forecast horizon for the low-balance check (default 14)
	LowBalanceThreshold int64  // minor units; 0 disables the check
	ICSUrl              string // remote calendar to import each pass; "" disables
}

func Load() Config {
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")

	return Config{
		APIURL:           strings.TrimRight(envOr("API_URL", "http://localhost:8080"), "/"),
		APIToken:         os.Getenv("API_TOKEN"),
		IntervalSeconds:  envInt("SCHED_INTERVAL_SECONDS", 6*3600),
		RunHourUTC:       envIntOr("SCHED_RUN_HOUR", -1),
		RunOnStart:       envBool("SCHED_RUN_ON_START", true),
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		DiscordWebhook:   os.Getenv("DISCORD_WEBHOOK_URL"),

		LowBalanceDays:      envInt("SCHED_LOW_BALANCE_DAYS", 14),
		LowBalanceThreshold: envIntOr64("SCHED_LOW_BALANCE_MINOR", 0),
		ICSUrl:              strings.TrimSpace(os.Getenv("EVENTS_ICS_URL")),
	}
}

// NotifyConfigured reports whether any webhook channel is set.
func (c Config) NotifyConfigured() bool {
	return (c.TelegramBotToken != "" && c.TelegramChatID != "") || c.DiscordWebhook != ""
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envIntOr64(key string, fallback int64) int64 {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}
