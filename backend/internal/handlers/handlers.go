// Package handlers wires HTTP requests to the store and schedule packages.
package handlers

import (
	"net/http"
	"time"

	"github.com/shivam7147/media-sequencer/backend/internal/store"
)

// Handlers holds the dependencies every route needs.
type Handlers struct {
	Store *store.Store
}

// New wires a Handlers against an already-connected store.
func New(s *store.Store) *Handlers {
	return &Handlers{Store: s}
}

// Time reports the server's clock and nothing else. Clients sample this
// repeatedly (several round trips, keeping the fastest) to estimate their
// offset from the server clock, so it has to stay as cheap as a handler
// can be — no database, no allocation beyond the response itself.
func (h *Handlers) Time(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"server_time": time.Now().UTC().Format(time.RFC3339Nano),
	})
}
