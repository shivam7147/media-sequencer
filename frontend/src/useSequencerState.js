import { useCallback, useEffect, useRef, useState } from 'react'
import { getState, subscribeEvents } from './api'
import { serverNow, startClockSync } from './clock'

const TICK_MS = 250

// Shared by both the Wall grid and the single-window route: fetches
// /api/state on mount and whenever SSE says something changed, keeps one
// clock-sync loop running, and drives one shared ~250ms ticker for as
// long as this hook stays mounted.
//
// What any window shows is derived fresh from serverNow() and the anchor
// every time it's computed — never stored, never incremented — so a
// coarse shared tick is self-correcting and there's no accumulating
// drift to manage. That holds whether the caller then resolves one
// window (the single-window route) or all of them (the Wall grid), which
// is why both pages share this exact hook instead of each rolling its
// own timer. The SSE subscription lives here for the same reason: both
// routes must refetch the same way, or a sync would update the Wall and
// leave an open /window/:id showing the old overlay.
export function useSequencerState() {
  const [state, setState] = useState(null)
  const [error, setError] = useState(null)
  const [now, setNow] = useState(() => serverNow())
  const loadedRef = useRef(false)

  const refresh = useCallback(() => {
    return getState()
      .then((data) => {
        loadedRef.current = true
        setState(data)
        setError(null)
      })
      .catch((err) => {
        if (!loadedRef.current) setError(err.message)
      })
  }, [])

  useEffect(() => startClockSync(), [])

  useEffect(() => {
    refresh()
    return subscribeEvents(refresh)
  }, [refresh])

  useEffect(() => {
    const intervalId = setInterval(() => setNow(serverNow()), TICK_MS)
    return () => clearInterval(intervalId)
  }, [])

  return { state, error, now, refresh }
}
