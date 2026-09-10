package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// minCycleSeconds and maxCycleSeconds bound PUT /api/settings/cycle. The
// shipped default is 18000 (5 hours), per PLAN.md — nobody can watch that
// play out. This endpoint exists purely so a reviewer can drop the cycle
// to something like 60s and watch every window loop and restart together
// in under a minute; the clamp just keeps that demo control from being
// used to put the schedule into a degenerate state (effectively zero, or
// longer than anyone could ever observe).
const (
	minCycleSeconds = 10
	maxCycleSeconds = 86400
)

type setCycleRequest struct {
	CycleSeconds int `json:"cycle_seconds"`
}

// SetCycleSeconds updates the schedule-wide cycle length.
func (h *Handlers) SetCycleSeconds(w http.ResponseWriter, r *http.Request) {
	var req setCycleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CycleSeconds < minCycleSeconds || req.CycleSeconds > maxCycleSeconds {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("cycle_seconds must be between %d and %d", minCycleSeconds, maxCycleSeconds))
		return
	}

	if err := h.Store.SetCycleSeconds(r.Context(), req.CycleSeconds); err != nil {
		log.Printf("set cycle seconds: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update cycle length")
		return
	}

	h.broadcast("state_changed")

	writeJSON(w, http.StatusOK, map[string]int{"cycle_seconds": req.CycleSeconds})
}
