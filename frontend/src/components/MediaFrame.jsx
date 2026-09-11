import { useRef, useState } from 'react'

// Renders whatever a window is currently scheduled to show. Used by both
// the Wall tiles and the single-window route, so this is the one place
// that decides how each media kind actually renders.
export default function MediaFrame({ resolved, media }) {
  // resolved.blank means "nothing valid to show" (empty playlist, every
  // item invalid) — distinct from a configured blank media item, which
  // has its own real id/label and is handled by the kind === 'blank'
  // branch below. Both render the same deliberate placeholder, since
  // from the viewer's side there's no meaningful difference.
  if (resolved.blank || !media) {
    return <BlankPanel label="blank" />
  }

  if (media.kind === 'image') {
    return <ImageFrame key={media.url} url={media.url} label={media.label} />
  }

  if (media.kind === 'video') {
    return (
      <VideoFrame key={media.url} url={media.url} label={media.label} elapsedMs={resolved.elapsedMs} />
    )
  }

  return <BlankPanel label={media.label} />
}

function BlankPanel({ label }) {
  return <div className="media-blank">{label}</div>
}

function ImageFrame({ url, label }) {
  const [failed, setFailed] = useState(false)

  // A missing or broken file should read as "this needs attention," not
  // as a native broken-image icon that looks like a rendering bug.
  if (failed) {
    return <BlankPanel label={`${label} (missing)`} />
  }

  return <img className="media-image" src={url} alt={label} onError={() => setFailed(true)} />
}

function VideoFrame({ url, label, elapsedMs }) {
  const videoRef = useRef(null)
  const seekedRef = useRef(false)
  const [failed, setFailed] = useState(false)

  if (failed) {
    return <BlankPanel label={`${label} (missing)`} />
  }

  function handleLoadedMetadata() {
    const video = videoRef.current
    if (!video || seekedRef.current) return
    seekedRef.current = true

    // Resume at the point the clock says this item is already at,
    // rather than restarting from 0 — this is what makes video obey the
    // same clock-derived "nothing is stored" rule as the rest of the
    // schedule: a reload landing 6s into a 15s slot should show 6s in,
    // not rewind it. Modulo the video's own duration in case its
    // intrinsic length is shorter than its scheduled slot (it plays on
    // loop for the rest of the slot either way).
    const elapsedSeconds = elapsedMs / 1000
    if (video.duration > 0) {
      video.currentTime = elapsedSeconds % video.duration
    }
  }

  return (
    <video
      ref={videoRef}
      className="media-video"
      src={url}
      // All three are required, not just autoPlay: browsers block
      // autoplay entirely unless the video is muted, and without
      // playsInline iOS Safari forces fullscreen playback instead of
      // playing inline in this tile — this wall has no user interaction
      // to hang a "click to play" prompt on, so both matter.
      muted
      autoPlay
      playsInline
      loop
      onLoadedMetadata={handleLoadedMetadata}
      onError={() => setFailed(true)}
    >
      <track kind="captions" />
    </video>
  )
}
