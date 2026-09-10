package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/shivam7147/media-sequencer/backend/internal/store"
)

type addItemRequest struct {
	MediaID         int64 `json:"media_id"`
	DurationSeconds *int  `json:"duration_seconds"`
}

// AddWindowItem appends a media item to a window's playlist.
func (h *Handlers) AddWindowItem(w http.ResponseWriter, r *http.Request) {
	windowID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid window id")
		return
	}

	var req addItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.MediaID <= 0 {
		writeError(w, http.StatusBadRequest, "media_id is required")
		return
	}
	if req.DurationSeconds != nil && *req.DurationSeconds <= 0 {
		writeError(w, http.StatusBadRequest, "duration_seconds must be positive")
		return
	}

	item, err := h.Store.AppendWindowItem(r.Context(), windowID, req.MediaID, req.DurationSeconds)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		log.Printf("add window item: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to add item")
		return
	}

	h.broadcast("state_changed")

	writeJSON(w, http.StatusCreated, itemJSON{
		ID:              item.ID,
		MediaID:         item.MediaID,
		Position:        item.Position,
		DurationSeconds: item.DurationSeconds,
	})
}

// DeleteWindowItem removes one item from one window's playlist.
func (h *Handlers) DeleteWindowItem(w http.ResponseWriter, r *http.Request) {
	windowID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid window id")
		return
	}
	itemID, err := strconv.ParseInt(chi.URLParam(r, "itemId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid item id")
		return
	}

	if err := h.Store.DeleteWindowItem(r.Context(), windowID, itemID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		log.Printf("delete window item: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete item")
		return
	}

	h.broadcast("state_changed")

	w.WriteHeader(http.StatusNoContent)
}
