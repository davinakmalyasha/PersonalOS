package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DB_PATH", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("API_TOKEN", "")

	// Unset to test fallback (godotenv may have loaded .env — override explicitly)
	if err := os.Unsetenv("PORT"); err != nil {
		t.Fatalf("unset PORT: %v", err)
	}
	if err := os.Unsetenv("DB_PATH"); err != nil {
		t.Fatalf("unset DB_PATH: %v", err)
	}
	if err := os.Unsetenv("LOG_LEVEL"); err != nil {
		t.Fatalf("unset LOG_LEVEL: %v", err)
	}

	cfg := Load()
	if cfg.Port != "8080" {
		t.Fatalf("expected Port=8080, got %q", cfg.Port)
	}
	if cfg.DBPath != "./data/personal-os.db" {
		t.Fatalf("expected default DBPath, got %q", cfg.DBPath)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("expected LogLevel info, got %q", cfg.LogLevel)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DB_PATH", "./data/test.db")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("API_TOKEN", "secret123")

	cfg := Load()
	if cfg.Port != "9090" {
		t.Fatalf("expected 9090, got %q", cfg.Port)
	}
	if cfg.DBPath != "./data/test.db" {
		t.Fatalf("expected ./data/test.db, got %q", cfg.DBPath)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("expected debug, got %q", cfg.LogLevel)
	}
	if cfg.APIToken != "secret123" {
		t.Fatalf("expected secret123, got %q", cfg.APIToken)
	}
}

func TestLoadLogLevelLowercases(t *testing.T) {
	t.Setenv("LOG_LEVEL", "DEBUG")
	cfg := Load()
	if cfg.LogLevel != "debug" {
		t.Fatalf("expected lowercased debug, got %q", cfg.LogLevel)
	}
}
