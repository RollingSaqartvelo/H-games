import { useEffect, useRef, useState } from 'react'
import { TopBar }            from './components/TopBar'
import { MultiplierHistory } from './components/MultiplierHistory'
import { Multiplier }        from './components/Multiplier'
import { RoundCountdown }    from './components/RoundCountdown'
import { SocialTicker }      from './components/SocialTicker'
import { DualBetPanel }      from './components/DualBetPanel'
import { BetsViewer }        from './components/BetsViewer'
import { BettingVideo }      from './components/BettingVideo'
import { WastedOverlay }    from './components/WastedOverlay'
import { GifCharacters }   from './components/GifCharacters'
import { RunningVideo }    from './components/RunningVideo'
import { RoundLoader }     from './components/RoundLoader'
import { useSocket }         from './hooks/useSocket'
import { usePixi }           from './hooks/usePixi'
import { useTMAAuth }        from './hooks/useTMAAuth'
import { useBgMusic }        from './hooks/useBgMusic'
import { initTMA }           from './lib/telegram'

/**
 * Layout (flex-column, 100dvh):
 *
 *   TopBar            48px  — logo / balance / state badge
 *   MultiplierHistory 36px  — last 10 crash point pills
 *   .game-area        flex:1 — PixiJS canvas + absolute overlays
 *     pixi-mount        (absolute fill, z:1)
 *     BettingVideo      (absolute fill, z:2)
 *     SocialTicker      (absolute top-right, z:6)
 *     Multiplier        (absolute center, z:5)
 *     RoundCountdown    (absolute bottom, z:6)
 *   app__footer       auto  — DualBetPanel (~280px)
 *
 * CRITICAL: pixi-mount ref must point to the SAME div for the entire lifetime
 * of the App component. Using a conditional early-return with a different div
 * causes React to detach the Pixi canvas from the DOM when loading ends.
 * The splash screen is an absolute overlay (z:50) so the Pixi mount stays
 * stable underneath it from the very first render.
 */
// On the first user touch, unlock all media elements in the DOM.
// iOS WKWebView (Telegram) blocks even muted video autoplay until a gesture.
function unlockAllMedia(): void {
  document.querySelectorAll<HTMLMediaElement>('video, audio').forEach((el) => {
    if (el.paused) {
      void el.play().then(() => {
        // For audio elements that shouldn't auto-start: pause immediately after unlock
        if (el.tagName === 'AUDIO' && !el.classList.contains('music')) {
          el.pause()
          el.currentTime = 0
        }
      }).catch(() => {})
    }
  })
}

export function App() {
  useEffect(() => {
    initTMA()
    window.addEventListener('touchstart', unlockAllMedia, { once: true, passive: true })
    window.addEventListener('click',      unlockAllMedia, { once: true })
  }, [])

  // Desktop only: publish the live .game-area rect as CSS vars so the fixed
  // WASTED / 3·2·1 overlays center on the game broadcast (not the viewport,
  // which is off because the stats column shifts the game screen right).
  useEffect(() => {
    const root = document.documentElement
    const update = () => {
      const ga = document.querySelector('.game-area') as HTMLElement | null
      if (!ga || window.innerWidth < 1000) {
        root.style.removeProperty('--ga-left')
        root.style.removeProperty('--ga-top')
        root.style.removeProperty('--ga-w')
        root.style.removeProperty('--ga-h')
        return
      }
      const r = ga.getBoundingClientRect()
      root.style.setProperty('--ga-left', `${r.left}px`)
      root.style.setProperty('--ga-top',  `${r.top}px`)
      root.style.setProperty('--ga-w',    `${r.width}px`)
      root.style.setProperty('--ga-h',    `${r.height}px`)
    }
    update()
    window.addEventListener('resize', update)
    window.addEventListener('scroll', update, true)
    const ga = document.querySelector('.game-area')
    const ro = ga ? new ResizeObserver(update) : null
    if (ga && ro) ro.observe(ga)
    return () => {
      window.removeEventListener('resize', update)
      window.removeEventListener('scroll', update, true)
      ro?.disconnect()
    }
  }, [])

  const pixiMount = useRef<HTMLDivElement>(null)
  usePixi(pixiMount)

  const { token, playerId, firstName, loading } = useTMAAuth()

  // Keep the splash up until the heavy media (background videos + character GIFs)
  // are actually loaded — otherwise the game shows before they're ready.
  const [mediaReady, setMediaReady] = useState(false)
  useEffect(() => {
    let done = false
    const finish = () => { if (!done) { done = true; setMediaReady(true) } }
    const imgs = [
      '/assets/sheriff/%D1%88%D0%B5%D1%80%D0%B8%D1%84.gif',
      '/assets/sheriff/%D0%92%D1%8B%D1%81%D1%82%D1%80%D0%B5%D0%BB.gif',
      '/assets/sheriff/sherif%20crash.gif',
      '/assets/sheriff/crash%20end.png',
      '/assets/hero/%D0%B3%D0%B5%D1%80%D0%BE%D0%B9.gif',
      '/assets/hero/Newcrash.gif',
      '/assets/ui/Wasted/wasted1.png',
      '/assets/ui/wins/you_win_combo_full.png',
    ].map(src => new Promise<void>(res => {
      const im = new Image(); im.onload = () => res(); im.onerror = () => res(); im.src = src
    }))
    const vids = [
      '/video/betting-loop2.mp4',
      '/video/outlawbgnew.mp4?v=2',
    ].map(src => fetch(src).then(r => r.blob()).then(() => {}).catch(() => {}))
    Promise.all([...imgs, ...vids]).then(finish)
    const cap = window.setTimeout(finish, 14000)   // never block forever
    return () => window.clearTimeout(cap)
  }, [])

  const ready = !loading && mediaReady
  useBgMusic(ready)
  useSocket(playerId)

  return (
    <div className="app">
      <TopBar />
      <MultiplierHistory />

      {/* Game canvas — grows to fill all available space */}
      <div className="game-area">
        {/* z:1 — PixiJS canvas mount (stable ref, never swapped) */}
        <div ref={pixiMount} className="pixi-mount" aria-hidden="true" />

        {/* z:2 — betting-loop2.mp4 during WAITING/BETTING; fades out on RUNNING */}
        <BettingVideo />

        {/* z:2 — comp1.mp4 background during RUNNING */}
        <RunningVideo />

        {/* z:5-6 — Absolute overlays on top of canvas / video */}
        <div className="game-overlay game-overlay--top-right">
          <SocialTicker />
        </div>

        <div className="game-overlay game-overlay--center">
          <Multiplier />
        </div>

        <div className="game-overlay game-overlay--bottom">
          <RoundCountdown />
        </div>

        {/* z:10 — GIF characters: sheriff (left) + hero (center-right) + shot */}
        <GifCharacters />

        {/* z:99 — Wasted cinematic slam on crash */}
        <WastedOverlay />

        {/* z:90 — Inter-round logo loader (2s) after crash, before betting */}
        <RoundLoader />

        {/* z:50 — Splash overlay on first load; sits above Pixi so the mount
            div stays in game-area and Pixi can initialise into the correct element */}
        {!ready && (
          <div className="splash">
            <div className="splash__logo">OUTLAW</div>
            <div className="splash__spinner" />
            <div className="splash__text">Loading…</div>
          </div>
        )}
      </div>

      {/* Fixed bottom controls */}
      <footer className="app__footer">
        {firstName && (
          <div className="player-badge">🤠 {firstName}</div>
        )}
        <DualBetPanel token={token} playerId={playerId} />
        <BetsViewer playerId={playerId ?? ''} />
      </footer>
    </div>
  )
}
