package server

import (
	"net/http"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/store"
)

// GET /v1/activity — per-pillar latest-change timestamps for the live board.
func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	if s.activity == nil {
		fail(w, http.StatusServiceUnavailable, "activity unavailable")
		return
	}
	out, err := store.LatestActivity(s.activity.DB)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
