import { useEffect, useRef, useState } from 'react'
import { useGame } from '../store/game'

const SHERIFF_IDLE  = '/assets/sheriff/%D1%88%D0%B5%D1%80%D0%B8%D1%84.gif'
const SHERIFF_SHOT  = '/assets/sheriff/%D0%92%D1%8B%D1%81%D1%82%D1%80%D0%B5%D0%BB.gif'
const HERO_SRC      = '/assets/hero/%D0%B3%D0%B5%D1%80%D0%BE%D0%B9.gif'
const CRASH_SRC     = '/assets/hero/crash.gif'

const CRASH_GIF_MS  = 1800 // exact one-loop duration of crash.gif

const SHOT_MS      = 5000
const SHOT_SHOW_MS = 800

function useCharLayout(): { size: string; isMobile: boolean } {
  const mobile = () => window.innerWidth < 768
  const [isMobile, setIsMobile] = useState(mobile)
  useEffect(() => {
    const update = () => setIsMobile(mobile())
    window.addEventListener('resize', update)
    return () => window.removeEventListener('resize', update)
  }, [])
  return {
    size: isMobile ? '375px' : 'min(600px, 38vw)',
    isMobile,
  }
}

export function GifCharacters() {
  const roundState = useGame((s) => s.roundState)
  const preCrash   = useGame((s) => s.preCrash)
  const [firing, setFiring]   = useState(false)
  const [shotKey, setShotKey] = useState(0)
  const [heroState, setHeroState] = useState<'run' | 'crash-gif' | 'hidden'>('run')
  const [flashKey, setFlashKey]   = useState(0)
  const intervalRef = useRef<number | undefined>(undefined)
  const hideRef     = useRef<number | undefined>(undefined)
  const heroTimer   = useRef<number | undefined>(undefined)
  const { size, isMobile } = useCharLayout()

  const running = roundState === 'RUNNING'
  const crashed = roundState === 'CRASHED'
  const visible = running || crashed

  // Sheriff shooting — only while running
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

  // Hero state machine
  useEffect(() => {
    window.clearTimeout(heroTimer.current)
    if (running && !preCrash) {
      setHeroState('run')
    } else if (running && preCrash) {
      setHeroState('crash-gif')
      setFlashKey((k) => k + 1)
    } else if (crashed) {
      setHeroState('crash-gif')
      setFlashKey((k) => k + 1)
      heroTimer.current = window.setTimeout(() => setHeroState('hidden'), CRASH_GIF_MS)
    } else {
      setHeroState('run')
    }
    return () => window.clearTimeout(heroTimer.current)
  }, [running, preCrash, crashed])

  // Hidden during betting — video plays instead
  if (!visible) return null

  const charStyle: React.CSSProperties = {
    position: 'absolute',
    bottom: 0,
    width: size,
    height: 'auto',
    display: 'block',
    transform: 'translateY(28%)',
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
        style={{ ...charStyle, left: isMobile ? '-35%' : 0 }}
      />
      {heroState !== 'hidden' && (
        <img
          key={heroState}
          src={heroState === 'run' ? HERO_SRC : CRASH_SRC}
          alt=""
          style={{ ...charStyle, left: isMobile ? '25%' : '55%' }}
        />
      )}
      {flashKey > 0 && (
        <div
          key={flashKey}
          style={{
            position: 'absolute',
            inset: 0,
            background: '#fff',
            animation: 'crash-flash 350ms ease-out forwards',
            zIndex: 100,
          }}
        />
      )}
    </div>
  )
}
