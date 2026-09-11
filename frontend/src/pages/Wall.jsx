import { Link } from 'react-router-dom'
import { useSequencerState } from '../useSequencerState'
import { resolveWindowMedia } from '../windowSchedule'
import WindowTile from '../components/WindowTile'
import ControlsPanel from '../components/ControlsPanel'
import Footer from '../components/Footer'

export default function Wall() {
  const { state, error, now, refresh } = useSequencerState()

  if (error && !state) {
    return (
      <div className="wall-container">
        <div className="wall-message">Failed to load: {error}</div>
        <Footer />
      </div>
    )
  }
  if (!state) {
    return (
      <div className="wall-container">
        <div className="wall-message">Loading…</div>
        <Footer />
      </div>
    )
  }

  return (
    <div className="wall-container">
      <div className="wall">
        <header className="wall-header">
          <h1>Media Sequencer</h1>
          <p>
            Cycle {state.cycle_seconds}s
            {state.active_sync ? ' · sync overlay active' : ''}
          </p>
        </header>
        <div className="wall-body">
          <div className="wall-grid">
            {state.windows.map((window) => {
              const { resolved, media } = resolveWindowMedia(state, window, now)

              return (
                <Link key={window.id} to={`/window/${window.id}`} className="window-tile-link">
                  <WindowTile name={window.name} resolved={resolved} media={media} />
                </Link>
              )
            })}
          </div>
          <ControlsPanel state={state} onRefresh={refresh} />
        </div>
      </div>
      <Footer />
    </div>
  )
}
