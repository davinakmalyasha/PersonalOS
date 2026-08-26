package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/config"
	"github.com/davinakmalyasha/PersonalOS/services/api/internal/db"
	"github.com/davinakmalyasha/PersonalOS/services/api/internal/logging"
	"github.com/davinakmalyasha/PersonalOS/services/api/internal/server"
)

func main() {
	cfg := config.Load()
	logger := logging.New(cfg.LogLevel)

	if err := os.MkdirAll(filepath.Dir(resolveDBPath(cfg.DBPath)), 0o755); err != nil {
		logger.Fatal().Err(err).Msg("create data dir")
	}

	sqlDB, err := db.Open(resolveDBPath(cfg.DBPath))
	if err != nil {
		logger.Fatal().Err(err).Msg("open db")
	}
	defer func() { _ = sqlDB.Close() }()

	migrationsDir := "services/api/migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		migrationsDir = "migrations"
	}
	if err := db.Migrate(sqlDB, migrationsDir); err != nil {
		logger.Fatal().Err(err).Str("dir", migrationsDir).Msg("migrate")
	}
	logger.Info().Str("dir", migrationsDir).Msg("migrations applied")

	handler := server.New(sqlDB, logger, cfg.APIToken, cfg.AttachmentsDir)

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info().Str("addr", addr).Str("db", resolveDBPath(cfg.DBPath)).Msg("api listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("listen")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("shutdown")
	}
}
