// Adapts the snake_case shapes from GET /api/state into the plain shapes
// schedule.js expects, and resolves one window's currently-scheduled
// media. Kept separate from schedule.js so that file stays a clean port
// of the Go reference with no knowledge of the API's JSON shape.

import { resolve } from './schedule'

export function toScheduleItems(window) {
  return window.items.map((item) => ({
    mediaId: item.media_id,
    durationSeconds: item.duration_seconds,
  }))
}

export function toScheduleSync(activeSync) {
  if (!activeSync) return null
  return {
    mediaId: activeSync.media_id,
    startAt: new Date(activeSync.start_at).getTime(),
    durationSeconds: activeSync.duration_seconds,
  }
}

// Resolves what `window` shows right now, given a full /api/state
// snapshot and the current clock-derived time (ms since epoch). Used by
// both the Wall grid and the single-window route so the two never drift
// apart in how they interpret the API response.
export function resolveWindowMedia(state, window, now) {
  const anchor = new Date(state.anchor).getTime()
  const items = toScheduleItems(window)
  const sync = toScheduleSync(state.active_sync)
  const resolved = resolve(anchor, items, state.cycle_seconds, sync, now)
  const media = resolved.blank ? null : (state.media.find((m) => m.id === resolved.mediaId) ?? null)
  return { resolved, media }
}
