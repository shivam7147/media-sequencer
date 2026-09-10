package handlers

import (
	"fmt"
	"net/http"
	"time"
)

// heartbeatInterval must stay comfortably under any idle-connection
// timeout a proxy in front of this server might apply. Render (and most
// reverse proxies) will close a connection that goes quiet, and a closed
// SSE stream looks to the browser like nothing ever silently stopped
// updating — there's no error, the page just stops receiving events. The
// ": ping" comment line below is invisible to EventSource listeners
// (SSE comment lines start with ':') but keeps bytes flowing so the proxy
// never considers the connection idle.
const heartbeatInterval = 20 * time.Second

// Events streams state_changed and sync notifications over SSE.
//
// Payloads are deliberately empty (`data: {}`): an event here only ever
// means "something changed, go refetch /api/state" — clients always
// refetch rather than trusting a pushed value. That's what makes this
// stream simple to reason about: there's no ordering to preserve and no
// staleness to detect, because no state ever actually travels over it.
func (h *Handlers) Events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Render (and most reverse proxies) buffer responses by default, which
	// would hold every event until the buffer fills instead of streaming
	// them as they're written. This header turns that off.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ch, unsubscribe := h.Broker.Subscribe()
	defer unsubscribe()

	// Confirm the stream is actually live before anything real happens —
	// otherwise a client has no way to distinguish "connected, waiting for
	// the first change" from "still connecting".
	fmt.Fprint(w, "event: connected\ndata: {}\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: {}\n\n", event)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
