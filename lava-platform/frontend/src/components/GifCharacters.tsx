import { useEffect, useRef, useState } from 'react'
import { useGame } from '../store/game'

const SHERIFF_IDLE  = '/assets/sheriff/%D1%88%D0%B5%D1%80%D0%B8%D1%84.gif'
const SHERIFF_SHOT  = '/assets/sheriff/%D0%92%D1%8B%D1%81%D1%82%D1%80%D0%B5%D0%BB.gif'
const SHERIFF_CRASH = '/assets/sheriff/sherif%20crash.gif'
const HERO_SRC      = '/assets/hero/%D0%B3%D0%B5%D1%80%D0%BE%D0%B9.gif'
const CRASH_SRC     = '/assets/hero/Newcrash.gif'
const WASTED_SRC    = '/assets/ui/Wasted/newwasted.png'

// Duration of crash GIFs — wasted appears 250ms before this, then game transitions to betting
const CRASH_GIF_MS = 1700

const SHOT_MS      = 5000
const SHOT_SHOW_MS = 800

function useCharLayout(): { size: string; isMobile: boolean } {
  // App container is always ≤480px — always use compact mobile character layout
  const [w, setW] = useState(() => Math.min(window.innerWidth, 480))
  useEffect(() => {
    const update = () => setW(Math.min(window.innerWidth, 480))
    window.addEventListener('resize', update)
    return () => window.removeEventListener('resize', update)
  }, [])
  return { size: w + 'px', isMobile: true }
}

export function GifCharacters() {
  const roundState          = useGame((s) => s.roundState)
  const preCrash            = useGame((s) => s.preCrash)
  const setCrashSequenceDone = useGame((s) => s.setCrashSequenceDone)

  const [firing, setFiring]       = useState(false)
  const [shotKey, setShotKey]     = useState(0)
  const [heroState, setHeroState] = useState<'run' | 'crash-gif' | 'done'>('run')
  const [showWasted, setShowWasted] = useState(false)

  const intervalRef  = useRef<number | undefined>(undefined)
  const hideRef      = useRef<number | undefined>(undefined)
  const heroTimer    = useRef<number | undefined>(undefined)
  const wastedTimer  = useRef<number | undefined>(undefined)

  const { size, isMobile } = useCharLayout()

  const running = roundState === 'RUNNING'
  const crashed = roundState === 'CRASHED'
  const visible = (running || crashed) && heroState !== 'done'

  // Sync opacity with bg video timestamps: fade to 30% at 0–3s and 6.25s+
  const [bgOpacity, setBgOpacity] = useState(1)
  useEffect(() => {
    if (!visible) return
    let raf: number
    const tick = () => {
      const vid = document.getElementById('running-bg-video') as HTMLVideoElement | null
      if (vid && vid.duration) {
        const t = vid.currentTime
        let op = 1
        if      (t <= 0.05)                  op = 0.15  // 0.00–0.05s: 85% transparent
        else if (t <= 0.06)                  op = 0.60  // 0.05–0.06s: 40% transparent
        else if (t >= 6.25 && t <= 6.27)     op = 0.15  // 6.25–6.27s: 85% transparent
        else if (t >= 6.27 && t <= 6.28)     op = 0.60  // 6.27–6.28s: 40% transparent
        setBgOpacity(op)
      }
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [visible])

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

  // Hero + crash sequence state machine
  useEffect(() => {
    window.clearTimeout(heroTimer.current)
    window.clearTimeout(wastedTimer.current)

    if (running && !preCrash) {
      setHeroState('run')
      setShowWasted(false)
      setCrashSequenceDone(false)
      return
    }

    if (running && preCrash) {
      setHeroState('crash-gif')
      return
    }

    if (crashed) {
      setHeroState('crash-gif')

      // Pause bg video immediately
      const vid = document.getElementById('running-bg-video') as HTMLVideoElement | null
      if (vid) vid.pause()

      // Wasted appears 250ms before GIFs end
      wastedTimer.current = window.setTimeout(() => setShowWasted(true), CRASH_GIF_MS - 250)

      // When GIFs finish: hide crash overlay, signal betting panel to appear
      heroTimer.current = window.setTimeout(() => {
        setHeroState('done')
        setShowWasted(false)
        setCrashSequenceDone(true)
      }, CRASH_GIF_MS)

      return
    }

    // STARTING / CREATED / null
    setHeroState('run')
    setShowWasted(false)
    setCrashSequenceDone(false)

    return () => {
      window.clearTimeout(heroTimer.current)
      window.clearTimeout(wastedTimer.current)
    }
  }, [running, preCrash, crashed, setCrashSequenceDone])

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
        opacity: bgOpacity,
        transition: 'opacity 0.03s linear',
      }}
    >
      {/* Sheriff */}
      <img
        key={heroState === 'crash-gif' ? 'sheriff-crash' : (firing ? `shot-${shotKey}` : 'idle')}
        src={heroState === 'crash-gif' ? SHERIFF_CRASH : (firing ? SHERIFF_SHOT : SHERIFF_IDLE)}
        alt=""
        style={{ ...charStyle, left: isMobile ? '-35%' : 0 }}
      />

      {/* Hero */}
      <img
        key={heroState}
        src={heroState === 'run' ? HERO_SRC : CRASH_SRC}
        alt=""
        style={{ ...charStyle, left: isMobile ? '25%' : '55%' }}
      />

      {/* Wasted — flies in 100ms before crash GIFs end */}
      {showWasted && (
        <img
          src={WASTED_SRC}
          alt="WASTED"
          style={{
            position: 'absolute',
            top: '40%',
            left: '50%',
            transform: 'translate(-50%, -50%)',
            width: 'min(88vw, 420px)',
            height: 'auto',
            zIndex: 101,
            animation: 'wasted-slam 320ms cubic-bezier(0.15, 1.35, 0.4, 1) both',
          }}
        />
      )}
    </div>
  )
}
