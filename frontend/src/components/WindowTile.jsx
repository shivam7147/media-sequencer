import MediaFrame from './MediaFrame'

export default function WindowTile({ name, resolved, media }) {
  const remainingSeconds = Math.ceil(resolved.remainingMs / 1000)
  const isActive = !resolved.blank

  return (
    <div className={`window-tile ${isActive ? 'active' : ''}`}>
      <div className="window-tile-header">
        <span className="window-tile-name">{name}</span>
        {resolved.isSync && <span className="sync-badge">SYNC</span>}
        <span className="window-tile-remaining">{remainingSeconds}s</span>
      </div>
      <div className="window-tile-media">
        <MediaFrame resolved={resolved} media={media} />
      </div>
    </div>
  )
}
