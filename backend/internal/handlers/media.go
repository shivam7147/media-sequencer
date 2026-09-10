package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

type createMediaRequest struct {
	Label                  string `json:"label"`
	Kind                   string `json:"kind"`
	URL                    string `json:"url"`
	DefaultDurationSeconds int    `json:"default_duration_seconds"`
}

var validMediaKinds = map[string]bool{"image": true, "video": true, "blank": true}

// CreateMedia adds an entry to the media library.
func (h *Handlers) CreateMedia(w http.ResponseWriter, r *http.Request) {
	var req createMediaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Label == "" {
		writeError(w, http.StatusBadRequest, "label is required")
		return
	}
	if !validMediaKinds[req.Kind] {
		writeError(w, http.StatusBadRequest, "kind must be one of image, video, blank")
		return
	}
	// url is only allowed to be empty for a blank item — an image or
	// video with nothing to render would silently show nothing on the
	// client with no indication why.
	if req.URL == "" && req.Kind != "blank" {
		writeError(w, http.StatusBadRequest, "url is required unless kind is blank")
		return
	}
	if req.DefaultDurationSeconds <= 0 {
		writeError(w, http.StatusBadRequest, "default_duration_seconds must be positive")
		return
	}

	media, err := h.Store.CreateMedia(r.Context(), req.Label, req.Kind, req.URL, req.DefaultDurationSeconds)
	if err != nil {
		log.Printf("create media: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create media")
		return
	}

	h.broadcast("state_changed")

	writeJSON(w, http.StatusCreated, mediaJSON{
		ID:                     media.ID,
		Label:                  media.Label,
		Kind:                   media.Kind,
		URL:                    media.URL,
		DefaultDurationSeconds: media.DefaultDurationSeconds,
	})
}
