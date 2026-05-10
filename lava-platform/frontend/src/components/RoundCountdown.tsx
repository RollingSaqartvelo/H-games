import { useEffect, useRef, useState } from 'react'
import { useGame } from '../store/game'
import { playCountdownSound, stopCountdownSound } from '../audio/CountdownSound'

const BETTING_DURATION = 10 // seconds, must match backend BettingDuration
const COUNTDOWN_TRIGGER = 3.0 // seconds — when to start 321.mp3

export function RoundCountdown() {
  const roundState = useGame((s) => s.roundState)
  const startingAt = useGame((s) => s.startingAt)

  const [seconds, setSeconds]   = useState(BETTING_DURATION)
  const [progress, setProgress] = useState(1)   // 1 → 0 as time runs out
  const [showRide, setShowRide] = useState(false)
  const rideTimer    = useRef<number | undefined>(undefined)
  const tickTimer    = useRef<number | undefined>(undefined)
  const countdownFired = useRef(false)

  // "RIDE!" flash when round starts
  useEffect(() => {
    if (roundState === 'RUNNING') {
      stopCountdownSound()
      setShowRide(true)
      window.clearTimeout(rideTimer.current)
      rideTimer.current = window.setTimeout(() => setShowRide(false), 900)
    }
    return () => window.clearTimeout(rideTimer.current)
  }, [roundState])

  // Countdown tick
  useEffect(() => {
    window.clearInterval(tickTimer.current)
    countdownFired.current = false

    if (roundState !== 'STARTING' || !startingAt) {
      setSeconds(BETTING_DURATION)
      setProgress(1)
      return
    }

    const tick = () => {
      const elapsed = Date.now() - startingAt
      const remaining = Math.max(0, BETTING_DURATION - elapsed / 1000)
      setSeconds(Math.ceil(remaining))
      setProgress(remaining / BETTING_DURATION)

      // Fire 321.mp3 exactly once when remaining first drops to ≤ 3.0s
      if (!countdownFired.current && remaining <= COUNTDOWN_TRIGGER) {
        countdownFired.current = true
        playCountdownSound()
      }
    }

    tick()
    tickTimer.current = window.setInterval(tick, 100)
    return () => {
      window.clearInterval(tickTimer.current)
      stopCountdownSound()
    }
  }, [roundState, startingAt])

  if (showRide) {
    return (
      <div className="rcd rcd--ride" aria-live="assertive">
        <span className="rcd__ride-text">LET'S GO</span>
      </div>
    )
  }

  if (roundState !== 'STARTING') return null

  const urgency = seconds <= 3 ? 'urgent' : seconds <= 6 ? 'warn' : 'normal'
  const isFinal = seconds <= 3 && seconds > 0

  return (
    <>
      {isFinal && (
        <div key={seconds} className="rcd__final-number" aria-live="assertive">
          {seconds}
        </div>
      )}

      <div className={`rcd rcd--${urgency}`} aria-live="polite">
        {isFinal && (
          <img
            className="rcd__sheriff-warning"
            src="/assets/betting/Warnings/sheriff_warning.png.png"
            alt=""
            aria-hidden="true"
          />
        )}
        {!isFinal && <div className="rcd__label">HEIST STARTS IN</div>}
        {!isFinal && <div className="rcd__number">{seconds}</div>}
        <div className="rcd__bar-wrap">
          <div
            className="rcd__bar-fill"
            style={{ width: `${progress * 100}%` }}
          />
        </div>
      </div>
    </>
  )
}
