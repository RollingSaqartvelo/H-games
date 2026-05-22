import { useEffect, useRef, useState } from 'react'
import { useGame } from '../store/game'

const SHERIFF_IDLE  = '/assets/sheriff/%D1%88%D0%B5%D1%80%D0%B8%D1%84.gif'
const SHERIFF_SHOT  = '/assets/sheriff/%D0%92%D1%8B%D1%81%D1%82%D1%80%D0%B5%D0%BB.gif'
const HERO_SRC      = '/assets/hero/%D0%B3%D0%B5%D1%80%D0%BE%D0%B9.gif'
const CRASH_SRC     = '/assets/hero/crash.gif'
const WASTED_SRC    = '/assets/ui/Wasted/newwasted.png'

const CRASH_GIF_MS   = 1800
const WASTED_DELAY_MS = 2000 // multi-flash + wasted appear 2s after crash-gif starts

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
  const [firing, setFiring]         = useState(false)
  const [shotKey, setShotKey]       = useState(0)
  const [heroState, setHeroState]   = useState<'run' | 'crash-gif' | 'hidden'>('run')
  const [preFlashKey, setPreFlashKey]     = useState(0)
  const [multiFlashKey, setMultiFlashKey] = useState(0)
  const [showWasted, setShowWasted]       = useState(false)

  const intervalRef  = useRef<number | undefined>(undefined)
  const hideRef      = useRef<number | undefined>(undefined)
  const heroTimer    = useRef<number | undefined>(undefined)
  const multiTimer   = useRef<number | undefined>(undefined)
  const crashStarted = useRef(false)
  const { size, isMobile } = useCharLayout()

  const running = roundState === 'RUNNING'
  const crashed = roundState === 'CRASHED'
  const visible = running || crashed

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
        if (t < 0.005)                     op = 0
        else if (t < 2.0)                  op = 0.3
        else if (t >= 6.2455 && t < 6.25) op = 0
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

  // Hero state machine
  useEffect(() => {
    window.clearTimeout(heroTimer.current)
    if (running && !preCrash) {
      setHeroState('run')
    } else if (running && preCrash) {
      setHeroState('crash-gif')
    } else if (crashed) {
      setHeroState('crash-gif')
      heroTimer.current = window.setTimeout(() => setHeroState('hidden'), CRASH_GIF_MS)
    } else {
      setHeroState('run')
    }
    return () => window.clearTimeout(heroTimer.current)
  }, [running, preCrash, crashed])

  // Flash + wasted sequence — fires once when crash-gif starts
  useEffect(() => {
    if (heroState === 'crash-gif' && !crashStarted.current) {
      crashStarted.current = true
      setPreFlashKey((k) => k + 1)
      multiTimer.current = window.setTimeout(() => {
        setMultiFlashKey((k) => k + 1)
        setShowWasted(true)
      }, WASTED_DELAY_MS)
    } else if (heroState !== 'crash-gif') {
      crashStarted.current = false
      window.clearTimeout(multiTimer.current)
      setShowWasted(false)
    }
    return () => window.clearTimeout(multiTimer.current)
  }, [heroState])

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
        transition: 'opacity 0.4s ease',
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

      {/* 2-3 flashes at crash-gif start */}
      {preFlashKey > 0 && (
        <div
          key={`pre-${preFlashKey}`}
          style={{
            position: 'absolute',
            inset: 0,
            background: '#fff',
            animation: 'crash-pre-flash 500ms ease-out forwards',
            zIndex: 98,
          }}
        />
      )}

      {/* 5-6 rapid flashes at 2s */}
      {multiFlashKey > 0 && (
        <div
          key={`multi-${multiFlashKey}`}
          style={{
            position: 'absolute',
            inset: 0,
            background: '#fff',
            animation: 'crash-multi-flash 700ms ease-out forwards',
            zIndex: 99,
          }}
        />
      )}

      {/* Wasted — on top of flashes */}
      {showWasted && (
        <img
          key={`wasted-${multiFlashKey}`}
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
