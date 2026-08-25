package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

// ---- Activity: per-pillar timestamps move on writes ----

func TestActivityTracksPillarWrites(t *testing.T) {
	h := newTestAPI(t)

	before := doJSON(t, h, http.MethodGet, "/v1/activity", nil)
	if before.Code != http.StatusOK {
		t.Fatalf("activity: %d %s", before.Code, before.Body.String())
	}
	var b struct {
		Pillars map[string]string `json:"pillars"`
		Latest  string            `json:"latest"`
	}
	_ = json.Unmarshal(before.Body.Bytes(), &b)
	if _, ok := b.Pillars["planner"]; ok {
		t.Fatal("empty DB should not report planner activity")
	}

	createTask(t, h, map[string]interface{}{"title": "pulse me"})
	createNote(t, h, "note for activity", "", nil)

	afterRec := doJSON(t, h, http.MethodGet, "/v1/activity", nil)
	_ = json.Unmarshal(afterRec.Body.Bytes(), &b)

	if _, ok := b.Pillars["planner"]; !ok {
		t.Fatalf("planner missing after task write: %+v", b.Pillars)
	}
	if _, ok := b.Pillars["knowledge"]; !ok {
		t.Fatalf("knowledge missing after note write: %+v", b.Pillars)
	}
	if b.Latest == "" || b.Latest < b.Pillars["planner"] {
		t.Fatalf("latest should be max of pillars: %+v", b)
	}
}
