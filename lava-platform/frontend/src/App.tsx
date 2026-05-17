import { useEffect, useRef } from 'react'
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

  const pixiMount = useRef<HTMLDivElement>(null)
  usePixi(pixiMount)
  useBgMusic()

  const { token, playerId, firstName, loading } = useTMAAuth()
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

        {/* z:50 — Splash overlay on first load; sits above Pixi so the mount
            div stays in game-area and Pixi can initialise into the correct element */}
        {loading && (
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
