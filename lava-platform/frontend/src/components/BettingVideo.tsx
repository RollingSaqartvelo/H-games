/**
 * BettingVideo — cinematic full-area background for WAITING / BETTING states.
 *
 * DEBUG MODE: red block confirms wrapper renders; video element has full debug logging.
 * Remove DEBUG_MODE flag and red block once video confirmed working.
 */

import { useEffect, useRef, useState } from 'react'
import { useGame } from '../store/game'

const VIDEO_SRC    = '/video/betting-loop2.mp4'
const FADE_OUT_MS  = 600
const SIREN_THRESH = 2

type Phase = 'visible' | 'fading-out' | 'hidden'

export function BettingVideo() {
  const roundState = useGame((s) => s.roundState)
  const startingAt = useGame((s) => s.startingAt)

  const videoRef = useRef<HTMLVideoElement>(null)
  const timerRef = useRef<number | undefined>(undefined)
  const tickRef  = useRef<number | undefined>(undefined)

  const [phase, setPhase] = useState<Phase>('hidden')
  const [siren, setSiren] = useState(false)
  const [, setDbgInfo] = useState('init')

  // ── Phase state machine ───────────────────────────────────────────────────

  useEffect(() => {
    window.clearTimeout(timerRef.current)

    if (roundState === 'RUNNING') {
      setPhase('fading-out')
      timerRef.current = window.setTimeout(() => {
        setPhase('hidden')
        videoRef.current?.pause()
      }, FADE_OUT_MS + 50)
      setSiren(false)
      return
    }

    if (roundState === 'STARTING' || roundState === 'CREATED') {
      setPhase('visible')

      const vid = videoRef.current
      if (vid) {
        console.log('[VIDEO] ref ok — src:', vid.src || VIDEO_SRC,
          'readyState:', vid.readyState, 'networkState:', vid.networkState,
          'dimensions:', vid.videoWidth, 'x', vid.videoHeight)
        vid.currentTime = 0
        vid.play()
          .then(() => {
            console.log('[VIDEO] play() resolved OK')
            setDbgInfo(`playing  readyState:${vid.readyState}  ${vid.videoWidth}×${vid.videoHeight}`)
          })
          .catch((e: unknown) => {
            console.error('[VIDEO] play() rejected:', e)
            setDbgInfo(`play BLOCKED: ${String(e)}`)
          })
      } else {
        console.warn('[VIDEO] videoRef.current is null at STARTING')
        setDbgInfo('REF NULL at STARTING')
      }
      return
    }

    return () => window.clearTimeout(timerRef.current)
  }, [roundState])

  // ── Siren tick ────────────────────────────────────────────────────────────

  useEffect(() => {
    window.clearInterval(tickRef.current)
    setSiren(false)
    if (roundState !== 'STARTING' || !startingAt) return

    tickRef.current = window.setInterval(() => {
      const remaining = Math.max(0, 10 - (Date.now() - startingAt) / 1000)
      setSiren(remaining > 0 && remaining <= SIREN_THRESH)
    }, 100)

    return () => window.clearInterval(tickRef.current)
  }, [roundState, startingAt])

  // ── Visibility ────────────────────────────────────────────────────────────

  useEffect(() => {
    const onVis = () => {
      if (document.hidden) {
        videoRef.current?.pause()
      } else if (phase === 'visible') {
        videoRef.current?.play().catch(() => {})
      }
    }
    document.addEventListener('visibilitychange', onVis)
    return () => document.removeEventListener('visibilitychange', onVis)
  }, [phase])

  // ── Seamless loop (prevent black frame at natural end) ───────────────────

  useEffect(() => {
    const vid = videoRef.current
    if (!vid) return
    const onTimeUpdate = () => {
      if (vid.duration && vid.currentTime >= vid.duration - 0.08) {
        vid.currentTime = 0
      }
    }
    vid.addEventListener('timeupdate', onTimeUpdate)
    return () => vid.removeEventListener('timeupdate', onTimeUpdate)
  }, [])

  // ── CSS ───────────────────────────────────────────────────────────────────

  const wrapClass = [
    'bvb-wrap',
    phase === 'fading-out' ? 'bvb-wrap--out'   : '',
    phase === 'hidden'     ? 'bvb-wrap--hidden' : '',
  ].filter(Boolean).join(' ')

  return (
    <>
      <div className={wrapClass} aria-hidden="true">
        <video
          ref={videoRef}
          src={VIDEO_SRC}
          style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover', objectPosition: 'center center', display: 'block' }}
          autoPlay
          loop
          muted
          playsInline
          preload="auto"
          onLoadedMetadata={(e) => {
            const v = e.currentTarget
            console.log('[VIDEO] loadedmetadata:', v.videoWidth, 'x', v.videoHeight, 'duration:', v.duration)
            setDbgInfo(`meta ${v.videoWidth}×${v.videoHeight}`)
          }}
          onCanPlay={(e) => {
            const v = e.currentTarget
            console.log('[VIDEO] canplay — readyState:', v.readyState)
            setDbgInfo(`canplay rs:${v.readyState} ${v.videoWidth}×${v.videoHeight}`)
          }}
          onPlay={() => console.log('[VIDEO] onPlay fired')}
          onPlaying={() => {
            const v = videoRef.current
            console.log('[VIDEO] onPlaying — actual frames:', v?.videoWidth, 'x', v?.videoHeight)
            setDbgInfo(`PLAYING ${v?.videoWidth}×${v?.videoHeight}`)
          }}
          onError={(e) => {
            const v = e.currentTarget as HTMLVideoElement
            const err = v.error
            console.error('[VIDEO] onError code:', err?.code, 'msg:', err?.message)
            setDbgInfo(`ERR code:${err?.code}`)
          }}
          onStalled={() => console.warn('[VIDEO] stalled')}
          onWaiting={() => console.warn('[VIDEO] waiting for data')}
          {...{ 'webkit-playsinline': 'true' }}
        />
        <div className="bvb-vignette" />
      </div>

      {siren && (
        <div className="bvb-siren" aria-hidden="true">
          <div className="bvb-siren__label">⚡ LAST SECONDS!</div>
        </div>
      )}
    </>
  )
}
