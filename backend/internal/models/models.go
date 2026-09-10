// Package models holds the plain data types that mirror the database schema.
package models

import "time"

// Media is one entry in the media library.
type Media struct {
	ID                     int64
	Label                  string
	Kind                   string // "image" | "video" | "blank"
	URL                    string
	DefaultDurationSeconds int
}

// WindowItem is one entry in a window's ordered playlist.
type WindowItem struct {
	ID              int64
	WindowID        int64
	MediaID         int64
	Position        int
	DurationSeconds int
}

// Window is a single display, with its playlist already loaded in order.
type Window struct {
	ID       int64
	Name     string
	Position int
	Items    []WindowItem
}

// SyncEvent is a one-off overlay that every window shows while it's active.
type SyncEvent struct {
	ID              int64
	MediaID         int64
	StartAt         time.Time
	DurationSeconds int
	CreatedAt       time.Time
}

// Settings holds the two schedule-wide values stored in the settings table.
type Settings struct {
	Anchor       time.Time
	CycleSeconds int
}
