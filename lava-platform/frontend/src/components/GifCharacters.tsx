import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useGame } from '../store/game'
import { playWastedSound } from '../audio/WastedSound'

const SHERIFF_IDLE  = '/assets/sheriff/%D1%88%D0%B5%D1%80%D0%B8%D1%84.gif'
const SHERIFF_SHOT  = '/assets/sheriff/%D0%92%D1%8B%D1%81%D1%82%D1%80%D0%B5%D0%BB.gif'
const SHERIFF_CRASH     = '/assets/sheriff/sherif%20crash.gif'
const SHERIFF_CRASH_END = '/assets/sheriff/crash%20end.png'
const HERO_SRC      = '/assets/hero/%D0%B3%D0%B5%D1%80%D0%BE%D0%B9.gif'
const CRASH_SRC     = '/assets/hero/Newcrash.gif'
const WASTED_SRC    = '/assets/ui/Wasted/newwasted.png'

// Duration of crash GIFs — wasted appears 500ms before this, then game transitions to betting
const CRASH_GIF_MS      = 1700
// Hero crash GIF starts this many ms after sheriff crash GIF
const HERO_CRASH_DELAY  = 100

const SHOT_MS      = 5000
const SHOT_SHOW_MS = 800

function useCharLayout(): { size: string; sizeNum: number; isMobile: boolean } {
  // App container is always ≤480px — always use compact mobile character layout
  const [w, setW] = useState(() => Math.min(window.innerWidth, 480))
  useEffect(() => {
    const update = () => setW(Math.min(window.innerWidth, 480))
    window.addEventListener('resize', update)
    return () => window.removeEventListener('resize', update)
  }, [])
  const sizeNum = Math.round(w * 0.85)
  return { size: sizeNum + 'px', sizeNum, isMobile: true }
}

export function GifCharacters() {
  const roundState           = useGame((s) => s.roundState)
  const preCrash             = useGame((s) => s.preCrash)
  const setCrashSequenceDone = useGame((s) => s.setCrashSequenceDone)

  const [firing, setFiring]             = useState(false)
  const [shotKey, setShotKey]           = useState(0)
  const [heroState, setHeroState]       = useState<'run' | 'crash-gif' | 'done'>('run')
  const [sheriffCrashing, setSheriffCrashing]     = useState(false)
  const [sheriffCrashEnded, setSheriffCrashEnded] = useState(false)
  const [showWasted, setShowWasted]               = useState(false)

  const intervalRef    = useRef<number | undefined>(undefined)
  const hideRef        = useRef<number | undefined>(undefined)
  const heroTimer      = useRef<number | undefined>(undefined)
  const wastedTimer    = useRef<number | undefined>(undefined)
  const sheriffTimer    = useRef<number | undefined>(undefined)
  const sheriffOffTimer = useRef<number | undefined>(undefined)
  const sheriffEndTimer = useRef<number | undefined>(undefined)
  const heroDelayTimer  = useRef<number | undefined>(undefined)

  const { size, sizeNum, isMobile } = useCharLayout()

  const running = roundState === 'RUNNING'
  const crashed = roundState === 'CRASHED'
  const visible = (running || crashed) && heroState !== 'done'

  // Sheriff shooting — only while running; first shot delayed so hero establishes scene
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
    // First shot after 1500ms, then every SHOT_MS
    hideRef.current = window.setTimeout(() => {
      fire()
      intervalRef.current = window.setInterval(fire, SHOT_MS)
    }, 1500)
    return () => {
      window.clearInterval(intervalRef.current)
      window.clearTimeout(hideRef.current)
    }
  }, [running])

  // Hero + crash sequence state machine
  useEffect(() => {
    window.clearTimeout(heroTimer.current)
    window.clearTimeout(wastedTimer.current)
    window.clearTimeout(sheriffTimer.current)
    window.clearTimeout(sheriffOffTimer.current)
    window.clearTimeout(sheriffEndTimer.current)
    window.clearTimeout(heroDelayTimer.current)

    if (running && !preCrash) {
      setHeroState('run')
      setSheriffCrashing(false)
      setSheriffCrashEnded(false)
      setShowWasted(false)
      setCrashSequenceDone(false)
      return
    }

    if (running && preCrash) {
      // Sheriff starts immediately, hero GIF 100ms later
      setSheriffCrashing(true)
      heroDelayTimer.current = window.setTimeout(() => setHeroState('crash-gif'), HERO_CRASH_DELAY)
      return
    }

    if (crashed) {
      setSheriffCrashEnded(false)

      // Pause bg video immediately
      const vid = document.getElementById('running-bg-video') as HTMLVideoElement | null
      if (vid) vid.pause()

      // Sheriff crash GIF starts immediately
      setSheriffCrashing(true)
      // Hero crash GIF starts 100ms later
      heroDelayTimer.current  = window.setTimeout(() => setHeroState('crash-gif'), HERO_CRASH_DELAY)
      // 1100ms after sheriff GIF starts → freeze frame (crash end.png)
      sheriffEndTimer.current = window.setTimeout(() => setSheriffCrashEnded(true), 1100)
      // Sheriff hidden 100ms before sequence ends
      sheriffOffTimer.current = window.setTimeout(() => setSheriffCrashing(false), CRASH_GIF_MS - 100)

      // Wasted appears 500ms before GIFs end — play wasted.mp3 at same moment
      wastedTimer.current = window.setTimeout(() => {
        setShowWasted(true)
        document.body.classList.add('wasted-showing')
        playWastedSound()
      }, CRASH_GIF_MS - 500)

      // When GIFs finish: hide crash overlay, signal betting panel to appear
      heroTimer.current = window.setTimeout(() => {
        setHeroState('done')
        setSheriffCrashing(false)
        setSheriffCrashEnded(false)
        setShowWasted(false)
        document.body.classList.remove('wasted-showing')
        setCrashSequenceDone(true)
      }, CRASH_GIF_MS)

      return
    }

    // STARTING / CREATED / null
    setHeroState('run')
    setSheriffCrashing(false)
    setSheriffCrashEnded(false)
    setShowWasted(false)
    document.body.classList.remove('wasted-showing')
    setCrashSequenceDone(false)

    return () => {
      window.clearTimeout(heroTimer.current)
      window.clearTimeout(wastedTimer.current)
      window.clearTimeout(sheriffTimer.current)
      window.clearTimeout(sheriffOffTimer.current)
      window.clearTimeout(sheriffEndTimer.current)
      window.clearTimeout(heroDelayTimer.current)
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

  const sheriffStyle: React.CSSProperties = {
    ...charStyle,
  }

  return (
    <>
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
        {/* Sheriff — hidden during crash delay; crash GIF → freeze frame after 1100ms */}
        {(heroState !== 'crash-gif' || sheriffCrashing) && (
          <img
            key={sheriffCrashEnded ? 'sheriff-crash-end' : sheriffCrashing ? 'sheriff-crash' : (firing ? `shot-${shotKey}` : 'idle')}
            src={sheriffCrashEnded ? SHERIFF_CRASH_END : sheriffCrashing ? SHERIFF_CRASH : (firing ? SHERIFF_SHOT : SHERIFF_IDLE)}
            alt=""
            style={{
              ...sheriffStyle,
              left: sheriffCrashEnded ? (isMobile ? '-10%' : 0) : (isMobile ? '-35%' : 0),
              // crash end PNG (255×196) has less padding than GIF (500×500) — scale down to match silhouette
              ...(sheriffCrashEnded && {
                width:  Math.round(sizeNum * 0.54) + 'px',
                height: Math.round(sizeNum * 0.54) + 'px',
                objectFit: 'contain' as const,
                bottom: '10%',
              }),
            }}
          />
        )}

        {/* Hero */}
        <img
          key={heroState}
          src={heroState === 'run' ? HERO_SRC : CRASH_SRC}
          alt=""
          style={{ ...charStyle, left: isMobile ? '25%' : '55%' }}
        />
      </div>

      {/* Wasted — full-screen fixed overlay via portal; flexbox center is immune to ancestor transforms */}
      {showWasted && createPortal(
        <div
          aria-hidden="true"
          style={{
            position: 'fixed',
            inset: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            paddingBottom: '40vh',
            pointerEvents: 'none',
            zIndex: 9999,
          }}
        >
          <img
            src={WASTED_SRC}
            alt="WASTED"
            style={{
              width: 'min(88vw, 420px)',
              height: 'auto',
              display: 'block',
              animation: 'wasted-slam 320ms cubic-bezier(0.15, 1.35, 0.4, 1) both',
            }}
          />
        </div>,
        document.body
      )}
    </>
  )
}
