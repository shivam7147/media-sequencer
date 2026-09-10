// Package schedule computes what a window shows at a given instant.
//
// The core idea (see PLAN.md § 0): what a window shows is a pure function
// of the current time, not stored state. Nothing here is mutated and
// nothing here is stateful — every call recomputes from scratch, which is
// exactly what makes a page reload, a second browser window, and a sync
// overlay all agree with each other for free.
//
// This package has no database and no HTTP imports on purpose: it must
// stay portable enough to port near line-for-line to the frontend's
// schedule.js, since both sides need to derive the identical answer from
// the identical clock.
package schedule

import "time"

// minRemaining floors every Remaining value returned by this package.
// Landing exactly on a boundary (or any of the clamping below) can produce
// a Remaining of zero or a small fraction of a second; handing that to a
// client that schedules a timer for "remaining seconds" makes it spin in a
// zero-delay loop. A small positive floor guarantees forward progress.
const minRemaining = time.Second

// Item is one entry in a window's playlist.
type Item struct {
	MediaID         int64
	DurationSeconds int
}

// SyncEvent is a one-off overlay: while it is active, every window shows
// its media regardless of what their own playlists say.
type SyncEvent struct {
	MediaID         int64
	StartAt         time.Time
	DurationSeconds int
}

// Resolved is what a window should render right now.
type Resolved struct {
	MediaID   int64
	Remaining time.Duration
	IsSync    bool
	// Blank is true when there is nothing valid to show — an empty
	// playlist, or every item invalid. It is distinct from an explicit
	// blank media item configured in a playlist (that has its own
	// MediaID and shows up like any other item); callers must key off
	// this field rather than inferring blankness from MediaID == 0.
	Blank bool
}

// floorMod returns a mod n in the mathematical sense: always in [0, n),
// never negative, for n > 0.
//
// Go's % is truncated division, so for negative a it returns a negative
// result (e.g. -5 % 100 == -5, not 95). That negative operand shows up
// whenever now is before anchor — clock skew, a client with a wrong
// clock, or an anchor that's simply in the future relative to now — and
// without this helper it would silently turn into a negative offset and
// garbage output instead of a valid wrapped position.
func floorMod(a, n time.Duration) time.Duration {
	if n <= 0 {
		return 0
	}
	m := a % n
	if m < 0 {
		m += n
	}
	return m
}

// clampMin floors d to minRemaining.
func clampMin(d time.Duration) time.Duration {
	if d < minRemaining {
		return minRemaining
	}
	return d
}

// CurrentItem computes what a window's own playlist shows at now, with no
// regard to any sync overlay. It reports false when there is nothing
// valid to show (empty playlist, or every item invalid) — callers should
// render blank in that case; Resolved.Remaining still tells them how long
// until the cycle restarts and it's worth checking again.
func CurrentItem(anchor time.Time, items []Item, cycleSeconds int, now time.Time) (Resolved, bool) {
	cycleDur := time.Duration(cycleSeconds) * time.Second
	if cycleDur <= 0 {
		return Resolved{Remaining: minRemaining, Blank: true}, false
	}

	// Position within the current cycle. Always in [0, cycleDur).
	elapsed := floorMod(now.Sub(anchor), cycleDur)
	// Always in (0, cycleDur]: the time left before every window restarts
	// its cycle together, regardless of what item is currently playing.
	toCycleEnd := cycleDur - elapsed

	// Guard against non-positive durations: an item with DurationSeconds
	// <= 0 must never be treated as playable. Sum only the valid items,
	// and skip invalid ones below rather than letting them corrupt the
	// walk (a negative duration would shrink the running total and throw
	// off every comparison after it).
	var playlistDur time.Duration
	for _, it := range items {
		if it.DurationSeconds > 0 {
			playlistDur += time.Duration(it.DurationSeconds) * time.Second
		}
	}
	if playlistDur <= 0 {
		// Empty playlist, or every item is invalid: nothing to show.
		// Report blank, with Remaining set to when the cycle restarts —
		// that's the next moment this could possibly change.
		return Resolved{Remaining: clampMin(toCycleEnd), Blank: true}, false
	}

	// Position within the looping playlist. Naturally truncates a
	// playlist longer than the cycle: elapsed never reaches cycleDur, so
	// offset never reaches past wherever elapsed capped out, and any
	// item positioned beyond that point in the playlist is simply never
	// reached — it never plays, as intended.
	offset := floorMod(elapsed, playlistDur)

	var acc time.Duration
	for _, it := range items {
		if it.DurationSeconds <= 0 {
			continue
		}
		d := time.Duration(it.DurationSeconds) * time.Second
		if offset < acc+d {
			remaining := acc + d - offset
			if remaining > toCycleEnd {
				// This item would run past the cycle boundary — cut it
				// there so every window restarts its cycle at the same
				// instant. This is also what cuts the final repetition
				// short when the playlist doesn't evenly divide the
				// cycle: intended, it keeps cycles aligned.
				remaining = toCycleEnd
			}
			return Resolved{MediaID: it.MediaID, Remaining: clampMin(remaining)}, true
		}
		acc += d
	}

	// Unreachable when playlistDur and offset are consistent (offset is
	// always < playlistDur, and playlistDur is the sum of every valid
	// item's duration), but this package never trusts that from the
	// outside: fall back to blank rather than panic.
	return Resolved{Remaining: clampMin(toCycleEnd), Blank: true}, false
}

// Resolve applies the sync overlay on top of CurrentItem. If sync is
// active — start_at <= now < start_at+duration — every window shows its
// media instead, with the remaining sync time. Otherwise each window
// falls back to its own computed item.
//
// The base schedule is never touched by this: a sync is purely an
// overlay evaluated fresh each call, so when it ends every window's own
// schedule is exactly where it would have been anyway. "Resume without
// losing playlist configuration" needs no bookkeeping because nothing was
// ever mutated to begin with.
func Resolve(anchor time.Time, items []Item, cycleSeconds int, sync *SyncEvent, now time.Time) Resolved {
	if sync != nil && sync.DurationSeconds > 0 {
		start := sync.StartAt
		end := start.Add(time.Duration(sync.DurationSeconds) * time.Second)
		if !now.Before(start) && now.Before(end) {
			return Resolved{
				MediaID:   sync.MediaID,
				Remaining: clampMin(end.Sub(now)),
				IsSync:    true,
			}
		}
	}

	resolved, _ := CurrentItem(anchor, items, cycleSeconds, now)
	return resolved
}
