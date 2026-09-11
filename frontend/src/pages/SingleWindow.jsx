import { Link, useParams } from 'react-router-dom'
import { useSequencerState } from '../useSequencerState'
import { resolveWindowMedia } from '../windowSchedule'
import MediaFrame from '../components/MediaFrame'

// Full-bleed view of a single window, e.g. for a physical display that
// should only ever show one window. Uses the same useSequencerState hook
// (one fetch, one clock sync, one shared ticker) and the same MediaFrame
// as the Wall grid, so a window here and the same window on the Wall are
// guaranteed to agree — including across a refresh, since nothing about
// what's shown is stored anywhere but derived fresh from the clock.
export default function SingleWindow() {
  const { id } = useParams()
  const { state, error, now } = useSequencerState()

  if (error) {
    return <div className="wall-message">Failed to load: {error}</div>
  }
  if (!state) {
    return <div className="wall-message">Loading…</div>
  }

  const window = state.windows.find((w) => String(w.id) === id)
  if (!window) {
    return <div className="wall-message">No window with id {id}</div>
  }

  const { resolved, media } = resolveWindowMedia(state, window, now)

  return (
    <div className="single-window">
      <div className="single-window-media">
        <MediaFrame resolved={resolved} media={media} />
      </div>
      <div className="single-window-caption">
        <Link to="/" className="single-window-back">
          ← Wall
        </Link>
        <span>{window.name}</span>
        {resolved.isSync && <span className="sync-badge">SYNC</span>}
      </div>
    </div>
  )
}
