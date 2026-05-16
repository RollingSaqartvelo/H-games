import { useEffect, useRef } from 'react'
import { useGame } from '../store/game'

export function RunningVideo() {
  const roundState = useGame((s) => s.roundState)
  const videoRef   = useRef<HTMLVideoElement>(null)

  const visible = roundState === 'RUNNING'

  useEffect(() => {
    const vid = videoRef.current
    if (!vid) return
    if (visible) {
      vid.currentTime = 0
      vid.play().catch(() => {})
    } else {
      vid.pause()
    }
  }, [visible])

  // Seamless loop — jump back 80ms before natural end
  useEffect(() => {
    const vid = videoRef.current
    if (!vid) return
    const onTime = () => {
      if (vid.duration && vid.currentTime >= vid.duration - 0.08) {
        vid.currentTime = 0
      }
    }
    vid.addEventListener('timeupdate', onTime)
    return () => vid.removeEventListener('timeupdate', onTime)
  }, [])

  return (
    <video
      ref={videoRef}
      src="/video/comp1.mp4"
      style={{
        position: 'absolute',
        inset: 0,
        width: '100%',
        height: '100%',
        objectFit: 'cover',
        zIndex: 2,
        opacity: visible ? 1 : 0,
        transition: 'opacity 0.4s ease',
        pointerEvents: 'none',
      }}
      muted
      loop
      playsInline
      preload="auto"
      {...{ 'webkit-playsinline': 'true' }}
    />
  )
}
