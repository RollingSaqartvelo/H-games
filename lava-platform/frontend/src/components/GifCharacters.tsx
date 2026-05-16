import { useEffect, useRef, useState } from 'react'
import { useGame } from '../store/game'

// URL-encoded because Cyrillic in img src can fail on some WebViews
const HERO_SRC    = '/assets/hero/%D0%B3%D0%B5%D1%80%D0%BE%D0%B9.gif'
const SHERIFF_SRC = '/assets/sheriff/%D1%88%D0%B5%D1%80%D0%B8%D1%84.gif'
const SHOT_SRC    = '/assets/sheriff/%D0%92%D1%8B%D1%81%D1%82%D1%80%D0%B5%D0%BB.gif'

const CHAR_SIZE    = 120   // px — all three GIFs same size
const SHOT_MS      = 5000  // interval between shots
const SHOT_SHOW_MS = 1200  // how long shot GIF stays visible

export function GifCharacters() {
  const roundState = useGame((s) => s.roundState)
  const [shotKey, setShotKey]         = useState(0)
  const [shotVisible, setShotVisible] = useState(false)
  const intervalRef = useRef<number | undefined>(undefined)
  const hideRef     = useRef<number | undefined>(undefined)

  const running = roundState === 'RUNNING'

  useEffect(() => {
    window.clearInterval(intervalRef.current)
    window.clearTimeout(hideRef.current)
    setShotVisible(false)

    if (!running) return

    const fire = () => {
      setShotKey((k) => k + 1)
      setShotVisible(true)
      hideRef.current = window.setTimeout(() => setShotVisible(false), SHOT_SHOW_MS)
    }

    fire()
    intervalRef.current = window.setInterval(fire, SHOT_MS)

    return () => {
      window.clearInterval(intervalRef.current)
      window.clearTimeout(hideRef.current)
    }
  }, [running])

  // Always render characters (not just during RUNNING)
  // so they are visible even in STARTING/CRASHED states
  if (roundState === null) return null

  return (
    <div
      aria-hidden="true"
      style={{
        position: 'absolute',
        inset: 0,
        pointerEvents: 'none',
        zIndex: 50,
      }}
    >
      {/* Sheriff — far left */}
      <img
        src={SHERIFF_SRC}
        alt=""
        style={{
          position: 'absolute',
          bottom: 0,
          left: 0,
          width: CHAR_SIZE,
          height: CHAR_SIZE,
          objectFit: 'contain',
          display: 'block',
        }}
      />

      {/* Shot — fires from sheriff's hand (right edge of sheriff sprite) */}
      {shotVisible && (
        <img
          key={shotKey}
          src={SHOT_SRC}
          alt=""
          style={{
            position: 'absolute',
            bottom: Math.round(CHAR_SIZE * 0.35),
            left: Math.round(CHAR_SIZE * 0.75),
            width: CHAR_SIZE,
            height: CHAR_SIZE,
            objectFit: 'contain',
            display: 'block',
          }}
        />
      )}

      {/* Hero — slightly past centre */}
      <img
        src={HERO_SRC}
        alt=""
        style={{
          position: 'absolute',
          bottom: 0,
          left: '55%',
          width: CHAR_SIZE,
          height: CHAR_SIZE,
          objectFit: 'contain',
          display: 'block',
        }}
      />
    </div>
  )
}
