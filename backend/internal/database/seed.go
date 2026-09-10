package database

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedAnchor is the fixed epoch every window's schedule is computed from.
// It must never change once chosen: moving it would shift every window's
// current item on every future boot. It only gets written once, on first
// seed — see the ON CONFLICT DO NOTHING below.
var seedAnchor = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

const defaultCycleSeconds = 18000 // 5 hours, per PLAN.md

type seedMediaRow struct {
	label           string
	kind            string
	url             string
	durationSeconds int
}

type seedItem struct {
	mediaLabel      string
	durationSeconds int
}

type seedWindowRow struct {
	name  string
	items []seedItem
}

var seedMediaRows = []seedMediaRow{
	{"M1", "image", "/media/m1.svg", 8},
	{"M2", "image", "/media/m2.svg", 10},
	{"M3", "image", "/media/m3.svg", 7},
	{"M4", "image", "/media/m4.svg", 9},
	{"M5", "video", "/media/m5.mp4", 15},
	{"M6", "blank", "", 5},
}

// seedWindowRows gives each window a different subset of media in a
// different order, so the wall is visibly out of phase until a sync fires.
var seedWindowRows = []seedWindowRow{
	{"Window 1", []seedItem{{"M1", 8}, {"M2", 10}, {"M3", 7}}},
	{"Window 2", []seedItem{{"M3", 7}, {"M4", 9}, {"M5", 15}}},
	{"Window 3", []seedItem{{"M5", 15}, {"M1", 8}, {"M6", 5}, {"M2", 10}}},
	{"Window 4", []seedItem{{"M6", 5}, {"M4", 9}, {"M2", 10}, {"M3", 7}}},
}

// Seed inserts the seed data exactly once. It checks the media table inside
// the same transaction as the inserts, so a boot that races another boot
// (or is retried after a crash) can never duplicate rows. It reports whether
// it actually inserted anything.
func Seed(ctx context.Context, pool *pgxpool.Pool) (seeded bool, err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("beginning seed transaction: %w", err)
	}
	defer tx.Rollback(ctx) // no-op once committed

	var mediaCount int
	if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM media").Scan(&mediaCount); err != nil {
		return false, fmt.Errorf("checking existing media: %w", err)
	}
	if mediaCount > 0 {
		return false, nil
	}

	mediaIDs := make(map[string]int64, len(seedMediaRows))
	for _, m := range seedMediaRows {
		var id int64
		err := tx.QueryRow(ctx,
			`INSERT INTO media (label, kind, url, default_duration_seconds)
			 VALUES ($1, $2, $3, $4) RETURNING id`,
			m.label, m.kind, m.url, m.durationSeconds,
		).Scan(&id)
		if err != nil {
			return false, fmt.Errorf("seeding media %s: %w", m.label, err)
		}
		mediaIDs[m.label] = id
	}

	for wi, w := range seedWindowRows {
		var windowID int64
		err := tx.QueryRow(ctx,
			`INSERT INTO windows (name, position) VALUES ($1, $2) RETURNING id`,
			w.name, wi+1,
		).Scan(&windowID)
		if err != nil {
			return false, fmt.Errorf("seeding window %s: %w", w.name, err)
		}

		for ii, item := range w.items {
			mediaID, ok := mediaIDs[item.mediaLabel]
			if !ok {
				return false, fmt.Errorf("seed window %s references unknown media %s", w.name, item.mediaLabel)
			}
			_, err := tx.Exec(ctx,
				`INSERT INTO window_items (window_id, media_id, position, duration_seconds)
				 VALUES ($1, $2, $3, $4)`,
				windowID, mediaID, ii+1, item.durationSeconds,
			)
			if err != nil {
				return false, fmt.Errorf("seeding item %s for window %s: %w", item.mediaLabel, w.name, err)
			}
		}
	}

	settings := map[string]string{
		"cycle_seconds": strconv.Itoa(defaultCycleSeconds),
		"anchor":        seedAnchor.Format(time.RFC3339Nano),
	}
	for key, value := range settings {
		_, err := tx.Exec(ctx,
			`INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO NOTHING`,
			key, value,
		)
		if err != nil {
			return false, fmt.Errorf("seeding setting %s: %w", key, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("committing seed transaction: %w", err)
	}
	return true, nil
}
