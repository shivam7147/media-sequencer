// Package handlers wires HTTP requests to the store and schedule packages.
package handlers

import (
	"net/http"
	"time"

	"github.com/shivam7147/media-sequencer/backend/internal/sse"
	"github.com/shivam7147/media-sequencer/backend/internal/store"
)

// Handlers holds the dependencies every route needs.
type Handlers struct {
	Store  *store.Store
	Broker *sse.Broker
}

// New wires a Handlers against an already-connected store and SSE broker.
func New(s *store.Store, b *sse.Broker) *Handlers {
	return &Handlers{Store: s, Broker: b}
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

// broadcast is the single place every write handler reports what changed.
// It publishes event ("state_changed" or "sync") to every client currently
// connected to GET /api/events.
func (h *Handlers) broadcast(event string) {
	h.Broker.Broadcast(event)
}
