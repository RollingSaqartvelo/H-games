import { useEffect, useRef, useState } from 'react'
import { useGame } from '../store/game'

const SHERIFF_IDLE = '/assets/sheriff/%D1%88%D0%B5%D1%80%D0%B8%D1%84.gif'
const SHERIFF_SHOT = '/assets/sheriff/%D0%92%D1%8B%D1%81%D1%82%D1%80%D0%B5%D0%BB.gif'
const HERO_SRC     = '/assets/hero/%D0%B3%D0%B5%D1%80%D0%BE%D0%B9.gif'

const SHOT_MS      = 5000
const SHOT_SHOW_MS = 800

function useCharSize(): string {
  const [size, setSize] = useState(() =>
    window.innerWidth < 768 ? '375px' : 'min(600px, 38vw)'
  )
  useEffect(() => {
    const update = () =>
      setSize(window.innerWidth < 768 ? '375px' : 'min(600px, 38vw)')
    window.addEventListener('resize', update)
    return () => window.removeEventListener('resize', update)
  }, [])
  return size
}

export function GifCharacters() {
  const roundState = useGame((s) => s.roundState)
  const [firing, setFiring]   = useState(false)
  const [shotKey, setShotKey] = useState(0)
  const intervalRef = useRef<number | undefined>(undefined)
  const hideRef     = useRef<number | undefined>(undefined)
  const size = useCharSize()

  // Only visible during active game round
  const running = roundState === 'RUNNING'

  useEffect(() => {
    window.clearInterval(intervalRef.current)
    window.clearTimeout(hideRef.current)
    setFiring(false)

    if (!running) return

    const fire = () => {
      setShotKey((k) => k + 1)
      setFiring(true)
      hideRef.current = window.setTimeout(() => setFiring(false), SHOT_SHOW_MS)
    }

    fire()
    intervalRef.current = window.setInterval(fire, SHOT_MS)

    return () => {
      window.clearInterval(intervalRef.current)
      window.clearTimeout(hideRef.current)
    }
  }, [running])

  // Hidden during betting — video plays instead
  if (!running) return null

  const charStyle: React.CSSProperties = {
    position: 'absolute',
    bottom: 0,
    width: size,
    height: size,
    objectFit: 'contain',
    objectPosition: 'bottom center',
    display: 'block',
  }

  return (
    <div
      aria-hidden="true"
      style={{
        position: 'absolute',
        inset: 0,
        pointerEvents: 'none',
        zIndex: 50,
        overflow: 'hidden',
      }}
    >
      <img
        key={firing ? `shot-${shotKey}` : 'idle'}
        src={firing ? SHERIFF_SHOT : SHERIFF_IDLE}
        alt=""
        style={{ ...charStyle, left: 0 }}
      />
      <img
        src={HERO_SRC}
        alt=""
        style={{ ...charStyle, left: '55%' }}
      />
    </div>
  )
}
