package server

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

//go:embed openapi.json
var openapiSpec []byte

func New(sqlDB *sql.DB, logger zerolog.Logger, apiToken string) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))
	r.Use(requestLogger(logger))

	r.Get("/healthz", healthz(sqlDB))
	r.Get("/openapi.json", serveOpenAPI)

	r.Route("/v1", func(r chi.Router) {
		if strings.TrimSpace(apiToken) != "" {
			r.Use(bearerAuth(apiToken))
		}
		r.Get("/health", healthz(sqlDB))
	})

	return r
}

func serveOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(openapiSpec)
}

func healthz(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "ok"
		dbStatus := "ok"
		code := http.StatusOK

		if db != nil {
			if err := db.PingContext(r.Context()); err != nil {
				dbStatus = "unavailable"
				status = "degraded"
				code = http.StatusServiceUnavailable
			}
		} else {
			dbStatus = "unconfigured"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": status,
			"db":     dbStatus,
		})
	}
}

func bearerAuth(expected string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				writeAuthError(w)
				return
			}
			token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
			if token != expected {
				writeAuthError(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeAuthError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "unauthorized",
		"code":  "unauthorized",
	})
}

func requestLogger(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			logger.Info().
				Str("request_id", middleware.GetReqID(r.Context())).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.Status()).
				Int("bytes", ww.BytesWritten()).
				Dur("duration_ms", time.Since(start)).
				Msg("request")
		})
	}
}
