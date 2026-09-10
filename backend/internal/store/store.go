// Package store is the only place in this codebase that writes SQL. Every
// query lives here and returns typed models — callers never see a row.
package store

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivam7147/media-sequencer/backend/internal/models"
)

// Store wraps the connection pool with the queries the app needs.
type Store struct {
	pool *pgxpool.Pool
}

// New wraps an already-connected pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// ListMedia returns the whole media library, ordered by id.
func (s *Store) ListMedia(ctx context.Context) ([]models.Media, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, label, kind, url, default_duration_seconds FROM media ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("listing media: %w", err)
	}
	defer rows.Close()

	media := []models.Media{}
	for rows.Next() {
		var m models.Media
		if err := rows.Scan(&m.ID, &m.Label, &m.Kind, &m.URL, &m.DefaultDurationSeconds); err != nil {
			return nil, fmt.Errorf("scanning media: %w", err)
		}
		media = append(media, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing media: %w", err)
	}
	return media, nil
}

// ListWindows returns every window with its playlist already loaded, items
// ordered by position. Two queries total regardless of window count.
func (s *Store) ListWindows(ctx context.Context) ([]models.Window, error) {
	windowRows, err := s.pool.Query(ctx,
		`SELECT id, name, position FROM windows ORDER BY position`)
	if err != nil {
		return nil, fmt.Errorf("listing windows: %w", err)
	}
	windows := []models.Window{}
	for windowRows.Next() {
		var w models.Window
		if err := windowRows.Scan(&w.ID, &w.Name, &w.Position); err != nil {
			windowRows.Close()
			return nil, fmt.Errorf("scanning window: %w", err)
		}
		w.Items = []models.WindowItem{}
		windows = append(windows, w)
	}
	windowRows.Close()
	if err := windowRows.Err(); err != nil {
		return nil, fmt.Errorf("listing windows: %w", err)
	}

	itemRows, err := s.pool.Query(ctx,
		`SELECT id, window_id, media_id, position, duration_seconds
		 FROM window_items ORDER BY window_id, position`)
	if err != nil {
		return nil, fmt.Errorf("listing window items: %w", err)
	}
	defer itemRows.Close()

	itemsByWindow := make(map[int64][]models.WindowItem)
	for itemRows.Next() {
		var it models.WindowItem
		if err := itemRows.Scan(&it.ID, &it.WindowID, &it.MediaID, &it.Position, &it.DurationSeconds); err != nil {
			return nil, fmt.Errorf("scanning window item: %w", err)
		}
		itemsByWindow[it.WindowID] = append(itemsByWindow[it.WindowID], it)
	}
	if err := itemRows.Err(); err != nil {
		return nil, fmt.Errorf("listing window items: %w", err)
	}

	for i := range windows {
		if items, ok := itemsByWindow[windows[i].ID]; ok {
			windows[i].Items = items
		}
	}
	return windows, nil
}

// GetSettings reads the schedule-wide anchor and cycle length.
func (s *Store) GetSettings(ctx context.Context) (models.Settings, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT key, value FROM settings WHERE key IN ('anchor', 'cycle_seconds')`)
	if err != nil {
		return models.Settings{}, fmt.Errorf("loading settings: %w", err)
	}
	defer rows.Close()

	values := make(map[string]string, 2)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return models.Settings{}, fmt.Errorf("scanning settings: %w", err)
		}
		values[key] = value
	}
	if err := rows.Err(); err != nil {
		return models.Settings{}, fmt.Errorf("loading settings: %w", err)
	}

	anchorRaw, ok := values["anchor"]
	if !ok {
		return models.Settings{}, fmt.Errorf("settings: missing \"anchor\"")
	}
	anchor, err := time.Parse(time.RFC3339Nano, anchorRaw)
	if err != nil {
		return models.Settings{}, fmt.Errorf("settings: parsing anchor: %w", err)
	}

	cycleRaw, ok := values["cycle_seconds"]
	if !ok {
		return models.Settings{}, fmt.Errorf("settings: missing \"cycle_seconds\"")
	}
	cycleSeconds, err := strconv.Atoi(cycleRaw)
	if err != nil {
		return models.Settings{}, fmt.Errorf("settings: parsing cycle_seconds: %w", err)
	}

	return models.Settings{Anchor: anchor, CycleSeconds: cycleSeconds}, nil
}

// ActiveSync returns the sync event covering now, or nil if none is active.
func (s *Store) ActiveSync(ctx context.Context, now time.Time) (*models.SyncEvent, error) {
	var e models.SyncEvent
	err := s.pool.QueryRow(ctx,
		`SELECT id, media_id, start_at, duration_seconds, created_at
		 FROM sync_events
		 WHERE start_at <= $1 AND start_at + (duration_seconds * interval '1 second') > $1
		 ORDER BY start_at DESC
		 LIMIT 1`,
		now,
	).Scan(&e.ID, &e.MediaID, &e.StartAt, &e.DurationSeconds, &e.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("loading active sync: %w", err)
	}
	return &e, nil
}
