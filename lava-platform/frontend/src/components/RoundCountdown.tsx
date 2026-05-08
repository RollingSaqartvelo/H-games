import { useEffect, useRef, useState } from 'react'
import { useGame } from '../store/game'

const BETTING_DURATION = 10 // seconds, must match backend BettingDuration

export function RoundCountdown() {
  const roundState = useGame((s) => s.roundState)
  const startingAt = useGame((s) => s.startingAt)

  const [seconds, setSeconds]   = useState(BETTING_DURATION)
  const [progress, setProgress] = useState(1)   // 1 → 0 as time runs out
  const [showRide, setShowRide] = useState(false)
  const rideTimer = useRef<number | undefined>(undefined)
  const tickTimer = useRef<number | undefined>(undefined)

  // "RIDE!" flash when round starts
  useEffect(() => {
    if (roundState === 'RUNNING') {
      setShowRide(true)
      window.clearTimeout(rideTimer.current)
      rideTimer.current = window.setTimeout(() => setShowRide(false), 900)
    }
    return () => window.clearTimeout(rideTimer.current)
  }, [roundState])

  // Countdown tick
  useEffect(() => {
    window.clearInterval(tickTimer.current)

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
    }

    tick()
    tickTimer.current = window.setInterval(tick, 100)
    return () => window.clearInterval(tickTimer.current)
  }, [roundState, startingAt])

  if (showRide) {
    return (
      <div className="rcd rcd--ride" aria-live="assertive">
        <span className="rcd__ride-text">RIDE!</span>
      </div>
    )
  }

  if (roundState !== 'STARTING') return null

  const urgency = seconds <= 3 ? 'urgent' : seconds <= 6 ? 'warn' : 'normal'

  return (
    <div className={`rcd rcd--${urgency}`} aria-live="polite">
      <div className="rcd__label">HEIST STARTS IN</div>
      <div className="rcd__number">{seconds}</div>
      <div className="rcd__bar-wrap">
        <div
          className="rcd__bar-fill"
          style={{ width: `${progress * 100}%` }}
        />
      </div>
    </div>
  )
}
