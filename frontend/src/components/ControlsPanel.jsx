import { useState } from 'react'
import { addWindowItem, createSync, setCycleSeconds } from '../api'

export default function ControlsPanel({ state, onRefresh }) {
  const media = state.media ?? []
  const windows = state.windows ?? []

  const [windowId, setWindowId] = useState(() => windows[0]?.id ?? '')
  const [addMediaId, setAddMediaId] = useState(() => media[0]?.id ?? '')
  const [addDuration, setAddDuration] = useState('')
  const [syncMediaId, setSyncMediaId] = useState(() => media[0]?.id ?? '')
  const [syncDuration, setSyncDuration] = useState(15)
  const [cycle, setCycle] = useState(state.cycle_seconds)
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')

  const run = async (action) => {
    setBusy(true)
    setMessage('')
    try {
      await action()
      await onRefresh()
    } catch (err) {
      setMessage(err.message)
    } finally {
      setBusy(false)
    }
  }

  const handleAdd = (event) => {
    event.preventDefault()
    const durationSeconds = addDuration === '' ? undefined : Number(addDuration)
    run(() =>
      addWindowItem(Number(windowId), {
        mediaId: Number(addMediaId),
        durationSeconds,
      }),
    )
  }

  const handleSync = (event) => {
    event.preventDefault()
    run(() =>
      createSync({
        mediaId: Number(syncMediaId),
        durationSeconds: Number(syncDuration),
      }),
    )
  }

  const handleCycle = (event) => {
    event.preventDefault()
    run(() => setCycleSeconds(Number(cycle)))
  }

  return (
    <aside className="controls">
      <h2>Controls</h2>
      <p className="controls-hint">
        Edits persist on the server. Every open window refetches and recomputes
        from the same clock — nothing about “where we are” is stored.
      </p>

      <section>
        <h3>Media library</h3>
        <ul className="controls-library">
          {media.map((item) => (
            <li key={item.id}>
              <strong>{item.label}</strong>
              <span>
                {item.kind} · {item.default_duration_seconds}s
              </span>
            </li>
          ))}
        </ul>
      </section>

      <form onSubmit={handleAdd}>
        <h3>Add to window</h3>
        <label>
          Window
          <select value={windowId} onChange={(e) => setWindowId(e.target.value)}>
            {windows.map((window) => (
              <option key={window.id} value={window.id}>
                {window.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Media
          <select value={addMediaId} onChange={(e) => setAddMediaId(e.target.value)}>
            {media.map((item) => (
              <option key={item.id} value={item.id}>
                {item.label}
              </option>
            ))}
          </select>
        </label>
        <label>
          Duration (seconds, blank = default)
          <input
            type="number"
            min="1"
            placeholder="default"
            value={addDuration}
            onChange={(e) => setAddDuration(e.target.value)}
          />
        </label>
        <button type="submit" disabled={busy}>
          Add item
        </button>
      </form>

      <form onSubmit={handleSync}>
        <h3>Trigger sync</h3>
        <label>
          Media
          <select value={syncMediaId} onChange={(e) => setSyncMediaId(e.target.value)}>
            {media.map((item) => (
              <option key={item.id} value={item.id}>
                {item.label}
              </option>
            ))}
          </select>
        </label>
        <label>
          Duration (seconds)
          <input
            type="number"
            min="1"
            value={syncDuration}
            onChange={(e) => setSyncDuration(e.target.value)}
            required
          />
        </label>
        <button type="submit" disabled={busy}>
          Sync now
        </button>
      </form>

      <form onSubmit={handleCycle}>
        <h3>Cycle length</h3>
        <p className="controls-hint">
          Default is 18000s (5h). Set 60 to watch a full cycle in a minute.
        </p>
        <label>
          Seconds
          <input
            type="number"
            min="10"
            max="86400"
            value={cycle}
            onChange={(e) => setCycle(e.target.value)}
            required
          />
        </label>
        <div className="controls-row">
          <button type="submit" disabled={busy}>
            Set cycle
          </button>
          <button
            type="button"
            disabled={busy}
            onClick={() => {
              setCycle(60)
              run(() => setCycleSeconds(60))
            }}
          >
            60s demo
          </button>
          <button
            type="button"
            disabled={busy}
            onClick={() => {
              setCycle(18000)
              run(() => setCycleSeconds(18000))
            }}
          >
            5h default
          </button>
        </div>
      </form>

      {message ? <p className="controls-error">{message}</p> : null}
    </aside>
  )
}
