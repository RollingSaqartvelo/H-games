import { useEffect, useRef } from 'react'
import { useGame } from '../store/game'

const LOOP_GUARD_S  = 0.15
const CRASH_PAUSE_MS = 1700 // pause 100ms before crash GIF freezes (GIF = 1800ms)

export function RunningVideo() {
  const roundState = useGame((s) => s.roundState)
  const videoRef   = useRef<HTMLVideoElement>(null)
  const rafRef     = useRef<number>(0)
  const pauseTimer = useRef<number | undefined>(undefined)

  const visible = roundState === 'RUNNING' || roundState === 'CRASHED'
  const crashed = roundState === 'CRASHED'

  // rAF loop: poll currentTime every frame for tightest possible loop
  useEffect(() => {
    const vid = videoRef.current
    if (!vid) return

    const tick = () => {
      if (vid.duration && vid.currentTime >= vid.duration - LOOP_GUARD_S) {
        vid.currentTime = 0
      }
      rafRef.current = requestAnimationFrame(tick)
    }

    rafRef.current = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(rafRef.current)
  }, [])

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

  // Pause video 1700ms after crash so it freezes 100ms before crash GIF ends
  useEffect(() => {
    const vid = videoRef.current
    if (!vid) return
    window.clearTimeout(pauseTimer.current)
    if (crashed) {
      pauseTimer.current = window.setTimeout(() => vid.pause(), CRASH_PAUSE_MS)
    }
    return () => window.clearTimeout(pauseTimer.current)
  }, [crashed])

  const handleEnded = () => {
    const vid = videoRef.current
    if (vid && visible) {
      vid.currentTime = 0
      vid.play().catch(() => {})
    }
  }

  return (
    <video
      ref={videoRef}
      src="/video/Comp%201.mp4"
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
      playsInline
      preload="auto"
      onEnded={handleEnded}
      {...{ 'webkit-playsinline': 'true' }}
    />
  )
}
