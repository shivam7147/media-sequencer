package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/shivam7147/media-sequencer/backend/internal/models"
)

// ErrNotFound is returned by write methods when a referenced row (a
// window, a media item, a window item) doesn't exist. Handlers translate
// this into a 404; anything else becomes a 500.
var ErrNotFound = errors.New("not found")

// AppendWindowItem adds mediaID to the end of windowID's playlist. If
// durationSeconds is nil, it falls back to the media's own
// default_duration_seconds. Position is max(existing position)+1 within
// the window, computed and inserted in one transaction so two concurrent
// appends to the same window can't collide on position.
func (s *Store) AppendWindowItem(ctx context.Context, windowID, mediaID int64, durationSeconds *int) (models.WindowItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.WindowItem{}, fmt.Errorf("appending window item: %w", err)
	}
	defer tx.Rollback(ctx) // no-op once committed

	var windowExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM windows WHERE id = $1)`, windowID).Scan(&windowExists); err != nil {
		return models.WindowItem{}, fmt.Errorf("checking window %d: %w", windowID, err)
	}
	if !windowExists {
		return models.WindowItem{}, fmt.Errorf("window %d: %w", windowID, ErrNotFound)
	}

	var defaultDuration int
	err = tx.QueryRow(ctx, `SELECT default_duration_seconds FROM media WHERE id = $1`, mediaID).Scan(&defaultDuration)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.WindowItem{}, fmt.Errorf("media %d: %w", mediaID, ErrNotFound)
		}
		return models.WindowItem{}, fmt.Errorf("checking media %d: %w", mediaID, err)
	}

	duration := defaultDuration
	if durationSeconds != nil {
		duration = *durationSeconds
	}

	var position int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(position), 0) FROM window_items WHERE window_id = $1`, windowID,
	).Scan(&position); err != nil {
		return models.WindowItem{}, fmt.Errorf("computing position for window %d: %w", windowID, err)
	}
	position++

	var item models.WindowItem
	err = tx.QueryRow(ctx,
		`INSERT INTO window_items (window_id, media_id, position, duration_seconds)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, window_id, media_id, position, duration_seconds`,
		windowID, mediaID, position, duration,
	).Scan(&item.ID, &item.WindowID, &item.MediaID, &item.Position, &item.DurationSeconds)
	if err != nil {
		return models.WindowItem{}, fmt.Errorf("inserting window item: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return models.WindowItem{}, fmt.Errorf("appending window item: %w", err)
	}
	return item, nil
}

// DeleteWindowItem removes one item from one window. Deleting doesn't
// renumber the remaining items' positions — ordering only ever depends on
// the relative order of position values, never their being contiguous, so
// a gap left behind is harmless. Renumbering on every delete would just be
// extra writes (and extra opportunities to race a concurrent append) for
// no behavioural benefit.
func (s *Store) DeleteWindowItem(ctx context.Context, windowID, itemID int64) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM window_items WHERE id = $1 AND window_id = $2`, itemID, windowID)
	if err != nil {
		return fmt.Errorf("deleting window item %d: %w", itemID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("window item %d in window %d: %w", itemID, windowID, ErrNotFound)
	}
	return nil
}

// CreateMedia adds a new entry to the media library.
func (s *Store) CreateMedia(ctx context.Context, label, kind, url string, defaultDurationSeconds int) (models.Media, error) {
	var m models.Media
	err := s.pool.QueryRow(ctx,
		`INSERT INTO media (label, kind, url, default_duration_seconds)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, label, kind, url, default_duration_seconds`,
		label, kind, url, defaultDurationSeconds,
	).Scan(&m.ID, &m.Label, &m.Kind, &m.URL, &m.DefaultDurationSeconds)
	if err != nil {
		return models.Media{}, fmt.Errorf("creating media: %w", err)
	}
	return m, nil
}

// CreateSync inserts a new sync event starting at startAt. It doesn't need
// to touch any existing sync event: ActiveSync always picks the most
// recent one by start_at, so a newer sync simply outranks whatever was
// active before it the moment it's inserted. That's the deliberate answer
// to two people triggering sync at nearly the same time — no locking, no
// "already syncing" error, just last write wins.
func (s *Store) CreateSync(ctx context.Context, mediaID int64, durationSeconds int, startAt time.Time) (models.SyncEvent, error) {
	var mediaExists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM media WHERE id = $1)`, mediaID).Scan(&mediaExists); err != nil {
		return models.SyncEvent{}, fmt.Errorf("checking media %d: %w", mediaID, err)
	}
	if !mediaExists {
		return models.SyncEvent{}, fmt.Errorf("media %d: %w", mediaID, ErrNotFound)
	}

	var e models.SyncEvent
	err := s.pool.QueryRow(ctx,
		`INSERT INTO sync_events (media_id, start_at, duration_seconds)
		 VALUES ($1, $2, $3)
		 RETURNING id, media_id, start_at, duration_seconds, created_at`,
		mediaID, startAt, durationSeconds,
	).Scan(&e.ID, &e.MediaID, &e.StartAt, &e.DurationSeconds, &e.CreatedAt)
	if err != nil {
		return models.SyncEvent{}, fmt.Errorf("creating sync event: %w", err)
	}
	return e, nil
}

// SetCycleSeconds updates the schedule-wide cycle length. It never touches
// the anchor: changing the cycle length recomputes every window's position
// on the next read, from the same anchor — no separate "restart" step.
func (s *Store) SetCycleSeconds(ctx context.Context, cycleSeconds int) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO settings (key, value) VALUES ('cycle_seconds', $1)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		strconv.Itoa(cycleSeconds),
	)
	if err != nil {
		return fmt.Errorf("setting cycle_seconds: %w", err)
	}
	return nil
}
