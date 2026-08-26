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

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/store"
)

//go:embed openapi.json
var openapiSpec []byte

// Server holds dependencies for all handlers.
type Server struct {
	db        *sql.DB
	logger    zerolog.Logger
	finance   *store.Finance
	planner   *store.Planner
	knowledge *store.Knowledge
	items     *store.Items
	health    *store.Health
	activity  *store.ActivityStore
}

func New(sqlDB *sql.DB, logger zerolog.Logger, apiToken string) http.Handler {
	s := &Server{db: sqlDB, logger: logger}
	if sqlDB != nil {
		s.finance = &store.Finance{DB: sqlDB}
		s.planner = &store.Planner{DB: sqlDB}
		s.knowledge = &store.Knowledge{DB: sqlDB}
		s.items = &store.Items{DB: sqlDB}
		s.health = &store.Health{DB: sqlDB}
		s.activity = &store.ActivityStore{DB: sqlDB}
	}

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

		if s.finance != nil {
			s.mountFinance(r)
		}
		if s.planner != nil {
			s.mountPlanner(r)
		}
		if s.knowledge != nil {
			s.mountKnowledge(r)
		}
		if s.items != nil {
			s.mountItems(r)
		}
		if s.health != nil {
			s.mountHealth(r)
		}
		if s.activity != nil {
			r.Get("/activity", s.handleActivity)
			r.Get("/activity/feed", s.handleActivityFeed)
		}
		s.mountExtras(r)
		s.mountSavedSearches(r)
		r.Get("/export", s.handleExport)
	})

	return r
}

func (s *Server) mountFinance(r chi.Router) {
	r.Route("/accounts", func(r chi.Router) {
		r.Post("/", s.handleCreateAccount)
		r.Get("/", s.handleListAccounts)
		r.Get("/{id}", s.handleGetAccount)
		r.Patch("/{id}", s.handleUpdateAccount)
		r.Delete("/{id}", s.handleDeleteAccount)
	})
	r.Route("/transactions", func(r chi.Router) {
		r.Post("/", s.handleCreateTransaction)
		r.Get("/", s.handleListTransactions)
		r.Post("/import", s.handleImportTransactions)
		r.Get("/{id}", s.handleGetTransaction)
		r.Patch("/{id}", s.handleUpdateTransaction)
		r.Delete("/{id}", s.handleDeleteTransaction)
	})
	r.Route("/categories", func(r chi.Router) {
		r.Post("/", s.handleCreateCategory)
		r.Get("/", s.handleListCategories)
		r.Patch("/{id}", s.handleUpdateCategory)
		r.Delete("/{id}", s.handleDeleteCategory)
	})
	r.Route("/rules", func(r chi.Router) {
		r.Post("/", s.handleCreateRule)
		r.Get("/", s.handleListRules)
		r.Patch("/{id}", s.handleUpdateRule)
		r.Delete("/{id}", s.handleDeleteRule)
		r.Post("/{id}/apply", s.handleApplyRule)
	})
	r.Route("/budgets", func(r chi.Router) {
		r.Post("/", s.handleUpsertBudget)
		r.Get("/", s.handleListBudgets)
		r.Delete("/{id}", s.handleDeleteBudget)
	})
	r.Get("/finance/summary", s.handleFinanceSummary)
	r.Get("/finance/spending", s.handleFinanceSpending)
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
