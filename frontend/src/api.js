// Thin fetch wrappers over the backend API. Nothing here holds state or
// makes decisions — every function is a single request in, parsed JSON
// (or nothing) out, or a thrown Error carrying the API's own message.

const BASE_URL = import.meta.env.VITE_API_BASE

async function request(path, options = {}) {
  const res = await fetch(`${BASE_URL}${path}`, {
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  })

  if (!res.ok) {
    // The backend's error shape is always {"error": "message"} (see
    // CLAUDE.md § API contract) — surface that message directly rather
    // than a generic "request failed", falling back to the HTTP status
    // line only if the body isn't the JSON we expect.
    let message = `${res.status} ${res.statusText}`
    try {
      const body = await res.json()
      if (body && body.error) message = body.error
    } catch {
      // not JSON — keep the status-line fallback above
    }
    throw new Error(message)
  }

  if (res.status === 204) return null
  return res.json()
}

// -- Reads, used starting this step --------------------------------------

export function getState() {
  return request('/api/state')
}

export function getTime() {
  return request('/api/time')
}

// -- Writes, used by the Wall controls panel -----------------------------

export function addWindowItem(windowId, { mediaId, durationSeconds } = {}) {
  return request(`/api/windows/${windowId}/items`, {
    method: 'POST',
    body: JSON.stringify({
      media_id: mediaId,
      ...(durationSeconds != null ? { duration_seconds: durationSeconds } : {}),
    }),
  })
}

export function deleteWindowItem(windowId, itemId) {
  return request(`/api/windows/${windowId}/items/${itemId}`, {
    method: 'DELETE',
  })
}

export function createMedia({ label, kind, url, defaultDurationSeconds }) {
  return request('/api/media', {
    method: 'POST',
    body: JSON.stringify({
      label,
      kind,
      url,
      default_duration_seconds: defaultDurationSeconds,
    }),
  })
}

export function createSync({ mediaId, durationSeconds }) {
  return request('/api/sync', {
    method: 'POST',
    body: JSON.stringify({ media_id: mediaId, duration_seconds: durationSeconds }),
  })
}

export function setCycleSeconds(cycleSeconds) {
  return request('/api/settings/cycle', {
    method: 'PUT',
    body: JSON.stringify({ cycle_seconds: cycleSeconds }),
  })
}

// Subscribe to GET /api/events. Named SSE events never fire `onmessage`
// (that's only the default unnamed type), so we listen for the names the
// backend actually sends. EventSource reconnects on its own, but a closed
// stream behind a proxy can sit in CLOSED without recovering — we close
// and reopen on error so a drop always becomes a new connection.
export function subscribeEvents(onEvent) {
  let source = null
  let retryTimer = null
  let stopped = false

  const open = () => {
    if (stopped) return
    source = new EventSource(`${BASE_URL}/api/events`)
    const handle = () => onEvent()
    source.addEventListener('connected', handle)
    source.addEventListener('state_changed', handle)
    source.addEventListener('sync', handle)
    source.onerror = () => {
      source.close()
      source = null
      if (stopped) return
      retryTimer = setTimeout(open, 2000)
    }
  }

  open()

  return () => {
    stopped = true
    if (retryTimer) clearTimeout(retryTimer)
    if (source) source.close()
  }
}
