package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port     string
	DBPath   string
	LogLevel string
	APIToken string
}

func Load() Config {
	_ = godotenv.Load()
	_ = godotenv.Load(".env")

	cfg := Config{
		Port:     envOr("PORT", "8080"),
		DBPath:   envOr("DB_PATH", "./data/personal-os.db"),
		LogLevel: strings.ToLower(envOr("LOG_LEVEL", "info")),
		APIToken: os.Getenv("API_TOKEN"),
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	return cfg
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
