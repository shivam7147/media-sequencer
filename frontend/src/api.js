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

// -- Writes: thin wrappers, not yet called from any UI (Step 10 wires
// these into the controls panel) -----------------------------------------

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
