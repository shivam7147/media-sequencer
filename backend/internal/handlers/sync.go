package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/shivam7147/media-sequencer/backend/internal/store"
)

type createSyncRequest struct {
	MediaID         int64 `json:"media_id"`
	DurationSeconds int   `json:"duration_seconds"`
}

// CreateSync starts a sync overlay now.
func (h *Handlers) CreateSync(w http.ResponseWriter, r *http.Request) {
	var req createSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.MediaID <= 0 {
		writeError(w, http.StatusBadRequest, "media_id is required")
		return
	}
	if req.DurationSeconds <= 0 {
		writeError(w, http.StatusBadRequest, "duration_seconds must be positive")
		return
	}

	// start_at is always the server's own clock, never a value the client
	// could supply: clients can have arbitrary clock skew, and the whole
	// point of a sync is one moment every window can agree actually
	// happened "now".
	startAt := time.Now().UTC()

	sync, err := h.Store.CreateSync(r.Context(), req.MediaID, req.DurationSeconds, startAt)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		log.Printf("create sync: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create sync")
		return
	}

	h.broadcast("sync")

	writeJSON(w, http.StatusCreated, syncJSON{
		MediaID:         sync.MediaID,
		StartAt:         sync.StartAt.Format(time.RFC3339Nano),
		DurationSeconds: sync.DurationSeconds,
	})
}
