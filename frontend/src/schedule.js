// Direct port of backend/internal/schedule/schedule.go. That Go file is
// the reference implementation — if the two ever disagree, Go is right
// and this file should be fixed to match, not the other way around.
// Structure is kept close enough to read side by side: same floorMod,
// same non-positive-duration guard, same minimum-remaining clamp, same
// `min(item's natural remaining, time to cycle end)` line, same sync
// overlay.
//
// Unlike the Go version, times and durations here are plain milliseconds
// (numbers) rather than time.Time / time.Duration — anchor and now are
// ms-since-epoch (what serverNow() and `new Date(...).getTime()` return),
// which is the native unit of a JS clock. Every comparison and formula
// below has the same shape as the Go code, just in that unit.
//
// One JS-only addition beyond the port: `resolved.elapsedMs`, how far
// into its current slot the resolved item already is. The Go side never
// needs this (the backend doesn't play video), but the frontend does —
// see MediaFrame's video-seeking logic — so it's computed here rather
// than reconstructed from remainingMs by every caller.

// Mirrors minRemaining = time.Second in schedule.go.
export const MIN_REMAINING_MS = 1000

/**
 * floorMod returns a mod n in the mathematical sense: always in [0, n),
 * never negative, for n > 0.
 *
 * JavaScript's % has the same problem Go's does: it's remainder, not
 * modulo, so for a negative dividend it returns a negative result
 * (-5 % 100 === -5, not 95). That shows up whenever now is before anchor
 * — clock skew, a wrong client clock, or an anchor in the future — and
 * without this helper it would silently produce a negative offset and
 * garbage output instead of a valid wrapped position.
 */
export function floorMod(a, n) {
  if (n <= 0) return 0
  const m = a % n
  return m < 0 ? m + n : m
}

function clampMin(d) {
  return d < MIN_REMAINING_MS ? MIN_REMAINING_MS : d
}

function blankResolved(remainingMs) {
  return { mediaId: 0, remainingMs, elapsedMs: 0, isSync: false, blank: true }
}

/**
 * currentItem computes what a window's own playlist shows at now, with no
 * regard to any sync overlay. Returns { resolved, ok }: ok is false when
 * there is nothing valid to show (empty playlist, or every item invalid)
 * — callers should render blank in that case; resolved.remainingMs still
 * tells them how long until the cycle restarts and it's worth checking
 * again.
 *
 * @param {number} anchor - ms since epoch
 * @param {{mediaId: number, durationSeconds: number}[]} items
 * @param {number} cycleSeconds
 * @param {number} now - ms since epoch
 */
export function currentItem(anchor, items, cycleSeconds, now) {
  const cycleDur = cycleSeconds * 1000
  if (cycleDur <= 0) {
    return { resolved: blankResolved(MIN_REMAINING_MS), ok: false }
  }

  // Position within the current cycle. Always in [0, cycleDur).
  const elapsed = floorMod(now - anchor, cycleDur)
  // Always in (0, cycleDur]: the time left before every window restarts
  // its cycle together, regardless of what item is currently playing.
  const toCycleEnd = cycleDur - elapsed

  // Guard against non-positive durations: an item with durationSeconds
  // <= 0 must never be treated as playable. Sum only the valid items,
  // and skip invalid ones below rather than letting them corrupt the
  // walk (a negative duration would shrink the running total and throw
  // off every comparison after it).
  let playlistDur = 0
  for (const item of items) {
    if (item.durationSeconds > 0) {
      playlistDur += item.durationSeconds * 1000
    }
  }
  if (playlistDur <= 0) {
    // Empty playlist, or every item is invalid: nothing to show. Report
    // blank, with remaining set to when the cycle restarts — that's the
    // next moment this could possibly change.
    return { resolved: blankResolved(clampMin(toCycleEnd)), ok: false }
  }

  // Position within the looping playlist. Naturally truncates a
  // playlist longer than the cycle: elapsed never reaches cycleDur, so
  // offset never reaches past wherever elapsed capped out, and any item
  // positioned beyond that point in the playlist is simply never
  // reached — it never plays, as intended.
  const offset = floorMod(elapsed, playlistDur)

  let acc = 0
  for (const item of items) {
    if (item.durationSeconds <= 0) continue
    const d = item.durationSeconds * 1000
    if (offset < acc + d) {
      // How far into this item's own slot we already are. Unaffected by
      // the cycle-boundary clamp below — that only shortens how much is
      // left, not how much has already played — so this is always in
      // [0, d) by construction (acc only grows, and offset < acc + d is
      // exactly the condition that put us here).
      const elapsedInItem = offset - acc

      let remaining = acc + d - offset
      if (remaining > toCycleEnd) {
        // This item would run past the cycle boundary — cut it there so
        // every window restarts its cycle at the same instant. This is
        // also what cuts the final repetition short when the playlist
        // doesn't evenly divide the cycle: intended, it keeps cycles
        // aligned.
        remaining = toCycleEnd
      }
      return {
        resolved: {
          mediaId: item.mediaId,
          remainingMs: clampMin(remaining),
          elapsedMs: elapsedInItem,
          isSync: false,
          blank: false,
        },
        ok: true,
      }
    }
    acc += d
  }

  // Unreachable when playlistDur and offset are consistent (offset is
  // always < playlistDur, and playlistDur is the sum of every valid
  // item's duration), but this never trusts that from the outside: fall
  // back to blank rather than throw.
  return { resolved: blankResolved(clampMin(toCycleEnd)), ok: false }
}

/**
 * resolve applies the sync overlay on top of currentItem. If sync is
 * active — startAt <= now < startAt + durationSeconds*1000 — every
 * window shows its media instead, with the remaining sync time.
 * Otherwise each window falls back to its own computed item.
 *
 * The base schedule is never touched by this: a sync is purely an
 * overlay evaluated fresh each call, so when it ends every window's own
 * schedule is exactly where it would have been anyway.
 *
 * @param {number} anchor - ms since epoch
 * @param {{mediaId: number, durationSeconds: number}[]} items
 * @param {number} cycleSeconds
 * @param {{mediaId: number, startAt: number, durationSeconds: number}|null} sync - startAt in ms since epoch
 * @param {number} now - ms since epoch
 */
export function resolve(anchor, items, cycleSeconds, sync, now) {
  if (sync && sync.durationSeconds > 0) {
    const start = sync.startAt
    const end = start + sync.durationSeconds * 1000
    if (now >= start && now < end) {
      return {
        mediaId: sync.mediaId,
        remainingMs: clampMin(end - now),
        elapsedMs: now - start,
        isSync: true,
        blank: false,
      }
    }
  }

  const { resolved } = currentItem(anchor, items, cycleSeconds, now)
  return resolved
}
