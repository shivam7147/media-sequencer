// Estimates the offset between this browser's clock and the server's.
//
// Every window's playback is derived from `serverNow()` (see
// schedule.js): what a window shows is a pure function of the current
// time, not stored state. If this browser's clock were simply wrong —
// a few minutes off, a stale VM snapshot, a user with the wrong
// timezone/date set — it would confidently compute the wrong scheduled
// item while having no way to know it's wrong, because it would be
// consistently wrong with itself. The only fix is to stop trusting
// Date.now() directly and instead measure how far this clock is from the
// server's, the one clock every window actually needs to agree with.

import { getTime } from './api'

const SAMPLE_COUNT = 5
const RESAMPLE_INTERVAL_MS = 5 * 60 * 1000 // re-sample every few minutes

let offsetMs = 0

async function sampleOnce() {
  const localBefore = Date.now()
  const { server_time: serverTimeRaw } = await getTime()
  const localAfter = Date.now()

  const rtt = localAfter - localBefore
  const serverTime = new Date(serverTimeRaw).getTime()
  const offset = serverTime - (localBefore + rtt / 2)

  return { offset, rtt }
}

// Samples GET /api/time SAMPLE_COUNT times and keeps the lowest-RTT
// sample, not the average. A round trip can be slow on the way there, on
// the way back, or queued somewhere in between — there's no way to tell
// which from the client, so averaging just blends that distortion into
// the estimate. The fastest sample is the one where "the response landed
// roughly halfway through the round trip" is closest to true, so it's
// the least distorted estimate of the offset.
export async function syncClock() {
  const samples = []
  for (let i = 0; i < SAMPLE_COUNT; i++) {
    try {
      samples.push(await sampleOnce())
    } catch {
      // A single failed sample shouldn't abort the estimate; just skip it.
    }
  }

  if (samples.length === 0) return offsetMs // keep the last known offset

  const best = samples.reduce((a, b) => (b.rtt < a.rtt ? b : a))
  offsetMs = best.offset
  return offsetMs
}

// The clock every playback calculation should use instead of Date.now().
export function serverNow() {
  return Date.now() + offsetMs
}

// Runs an initial sync immediately, then re-samples periodically (clocks
// drift, and a laptop that sleeps and wakes can jump). Returns a cleanup
// function that stops the periodic re-sync.
export function startClockSync() {
  syncClock()
  const intervalId = setInterval(syncClock, RESAMPLE_INTERVAL_MS)
  return () => clearInterval(intervalId)
}
