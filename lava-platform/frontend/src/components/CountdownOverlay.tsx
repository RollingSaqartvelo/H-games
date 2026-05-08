import { useEffect, useRef, useState } from 'react'
import { useGame } from '../store/game'

const BETTING_DURATION = 10  // seconds, matches backend BettingDuration

export function CountdownOverlay() {
  const roundState = useGame((s) => s.roundState)
  const startingAt = useGame((s) => s.startingAt)

  const [seconds, setSeconds]   = useState(BETTING_DURATION)
  const [animKey, setAnimKey]   = useState(0)
  const [showGo, setShowGo]     = useState(false)
  const intervalRef = useRef<ReturnType<typeof setInterval>>()

  // "GO!" flash when RUNNING begins
  useEffect(() => {
    if (roundState === 'RUNNING') {
      setShowGo(true)
      const t = setTimeout(() => setShowGo(false), 900)
      return () => clearTimeout(t)
    }
    setShowGo(false)
  }, [roundState])

  // Countdown tick during STARTING
  useEffect(() => {
    if (intervalRef.current !== undefined) clearInterval(intervalRef.current)
    if (roundState !== 'STARTING' || !startingAt) return

    const tick = () => {
      const elapsed = (Date.now() - startingAt) / 1000
      const remaining = Math.max(0, Math.ceil(BETTING_DURATION - elapsed))
      setSeconds((prev) => {
        if (prev !== remaining) setAnimKey((k) => k + 1)
        return remaining
      })
    }
    tick()
    intervalRef.current = setInterval(tick, 100)
    return () => {
      if (intervalRef.current !== undefined) clearInterval(intervalRef.current)
    }
  }, [roundState, startingAt])

  const visible = roundState === 'STARTING' || showGo
  if (!visible) return null

  const urgencyClass =
    seconds <= 3 ? 'countdown-number--urgent' :
    seconds <= 6 ? 'countdown-number--warn'   : ''

  return (
    <div className={`countdown-overlay${showGo ? ' countdown-overlay--go' : ''}`}>
      {showGo ? (
        <div className="countdown-go">RIDE!</div>
      ) : (
        <>
          <p className="countdown-label">HEIST STARTS IN</p>
          <div key={animKey} className={`countdown-number ${urgencyClass}`}>
            {seconds}
          </div>
          <div className="countdown-dots" aria-hidden="true">
            {Array.from({ length: BETTING_DURATION }, (_, i) => (
              <span
                key={i}
                className={`countdown-dot${BETTING_DURATION - seconds > i ? ' countdown-dot--done' : ''}`}
              />
            ))}
          </div>
          <p className="countdown-hint">Place your bet before the ride!</p>
        </>
      )}
    </div>
  )
}
