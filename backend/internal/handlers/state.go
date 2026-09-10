package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/shivam7147/media-sequencer/backend/internal/models"
	"github.com/shivam7147/media-sequencer/backend/internal/schedule"
)

type mediaJSON struct {
	ID                     int64  `json:"id"`
	Label                  string `json:"label"`
	Kind                   string `json:"kind"`
	URL                    string `json:"url"`
	DefaultDurationSeconds int    `json:"default_duration_seconds"`
}

type itemJSON struct {
	ID              int64 `json:"id"`
	MediaID         int64 `json:"media_id"`
	Position        int   `json:"position"`
	DurationSeconds int   `json:"duration_seconds"`
}

// resolvedJSON is what a window is showing right now, computed server-side
// via internal/schedule. Blank is explicit and separate from MediaID: a
// configured blank playlist item has its own real id and kind "blank",
// while "nothing valid to show" (empty playlist, every item invalid) has
// Blank: true and MediaID 0. Clients must key off Blank, never infer it
// from MediaID == 0.
type resolvedJSON struct {
	MediaID          int64   `json:"media_id"`
	Kind             string  `json:"kind"`
	URL              string  `json:"url"`
	RemainingSeconds float64 `json:"remaining_seconds"`
	IsSync           bool    `json:"is_sync"`
	Blank            bool    `json:"blank"`
}

type windowJSON struct {
	ID       int64        `json:"id"`
	Name     string       `json:"name"`
	Items    []itemJSON   `json:"items"`
	Resolved resolvedJSON `json:"resolved"`
}

type syncJSON struct {
	MediaID         int64  `json:"media_id"`
	StartAt         string `json:"start_at"`
	DurationSeconds int    `json:"duration_seconds"`
}

type stateResponse struct {
	ServerTime   string       `json:"server_time"`
	Anchor       string       `json:"anchor"`
	CycleSeconds int          `json:"cycle_seconds"`
	Media        []mediaJSON  `json:"media"`
	Windows      []windowJSON `json:"windows"`
	ActiveSync   *syncJSON    `json:"active_sync"`
}

// State returns everything a client needs in one call: the server clock,
// the schedule inputs (anchor, cycle length, every window's playlist, the
// media library), and — deliberately — each window's already-resolved
// current item. Resolving server-side means anyone can `curl /api/state`
// and see exactly what every window should be showing at that instant,
// with no browser and no client-side maths required to check it; the
// frontend still recomputes locally between polls so playback doesn't
// depend on request latency, but this endpoint is the ground truth.
func (h *Handlers) State(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now().UTC()

	settings, err := h.Store.GetSettings(ctx)
	if err != nil {
		log.Printf("state: loading settings: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load settings")
		return
	}

	mediaList, err := h.Store.ListMedia(ctx)
	if err != nil {
		log.Printf("state: loading media: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load media")
		return
	}

	windows, err := h.Store.ListWindows(ctx)
	if err != nil {
		log.Printf("state: loading windows: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load windows")
		return
	}

	activeSync, err := h.Store.ActiveSync(ctx, now)
	if err != nil {
		log.Printf("state: loading active sync: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load active sync")
		return
	}

	mediaByID := make(map[int64]models.Media, len(mediaList))
	mediaJSONs := make([]mediaJSON, len(mediaList))
	for i, m := range mediaList {
		mediaByID[m.ID] = m
		mediaJSONs[i] = mediaJSON{
			ID:                     m.ID,
			Label:                  m.Label,
			Kind:                   m.Kind,
			URL:                    m.URL,
			DefaultDurationSeconds: m.DefaultDurationSeconds,
		}
	}

	var syncOverlay *schedule.SyncEvent
	if activeSync != nil {
		syncOverlay = &schedule.SyncEvent{
			MediaID:         activeSync.MediaID,
			StartAt:         activeSync.StartAt,
			DurationSeconds: activeSync.DurationSeconds,
		}
	}

	windowJSONs := make([]windowJSON, len(windows))
	for i, win := range windows {
		itemJSONs := make([]itemJSON, len(win.Items))
		scheduleItems := make([]schedule.Item, len(win.Items))
		for j, it := range win.Items {
			itemJSONs[j] = itemJSON{
				ID:              it.ID,
				MediaID:         it.MediaID,
				Position:        it.Position,
				DurationSeconds: it.DurationSeconds,
			}
			scheduleItems[j] = schedule.Item{MediaID: it.MediaID, DurationSeconds: it.DurationSeconds}
		}

		resolved := schedule.Resolve(settings.Anchor, scheduleItems, settings.CycleSeconds, syncOverlay, now)

		var rj resolvedJSON
		if resolved.Blank {
			rj = resolvedJSON{
				Kind:             "blank",
				RemainingSeconds: resolved.Remaining.Seconds(),
				IsSync:           resolved.IsSync,
				Blank:            true,
			}
		} else {
			media := mediaByID[resolved.MediaID]
			rj = resolvedJSON{
				MediaID:          resolved.MediaID,
				Kind:             media.Kind,
				URL:              media.URL,
				RemainingSeconds: resolved.Remaining.Seconds(),
				IsSync:           resolved.IsSync,
				Blank:            false,
			}
		}

		windowJSONs[i] = windowJSON{
			ID:       win.ID,
			Name:     win.Name,
			Items:    itemJSONs,
			Resolved: rj,
		}
	}

	var activeSyncJSON *syncJSON
	if activeSync != nil {
		activeSyncJSON = &syncJSON{
			MediaID:         activeSync.MediaID,
			StartAt:         activeSync.StartAt.Format(time.RFC3339Nano),
			DurationSeconds: activeSync.DurationSeconds,
		}
	}

	writeJSON(w, http.StatusOK, stateResponse{
		ServerTime:   now.Format(time.RFC3339Nano),
		Anchor:       settings.Anchor.Format(time.RFC3339Nano),
		CycleSeconds: settings.CycleSeconds,
		Media:        mediaJSONs,
		Windows:      windowJSONs,
		ActiveSync:   activeSyncJSON,
	})
}
