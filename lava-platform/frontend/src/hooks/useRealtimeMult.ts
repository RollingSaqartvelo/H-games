import { useEffect, useRef, useState } from 'react'
import { useGame } from '../store/game'

const GROWTH_RATE = 0.06  // must match backend

/**
 * RAF-interpolated live multiplier.
 * Shared by Multiplier display and CashoutButton so neither duplicates
 * the tick-subscription + animation-frame logic.
 *
 * Returns the current interpolated multiplier value:
 *   • RUNNING  → smooth exp curve at 60fps
 *   • CRASHED  → frozen at crash point
 *   • other    → 1.0
 */
export function useRealtimeMult(): number {
  const roundState = useGame((s) => s.roundState)
  const crashPoint = useGame((s) => s.crashPoint)

  // Keep tick data in refs — updated via subscription, never cause re-renders
  const elapsedRef  = useRef(0)
  const lastTickRef = useRef(Date.now())

  useEffect(() =>
    useGame.subscribe(
      (s) => ({ elapsed: s.elapsedMs, lastTick: s.lastTickAt }),
      ({ elapsed, lastTick }) => {
        elapsedRef.current  = elapsed
        lastTickRef.current = lastTick
      },
    ),
  [])

  const [mult, setMult] = useState(1.0)
  const rafRef = useRef<number>()

  useEffect(() => {
    if (roundState !== 'RUNNING') {
      cancelAnimationFrame(rafRef.current!)
      setMult(crashPoint ?? 1.0)
      return
    }

    const frame = () => {
      const elapsed = elapsedRef.current + (Date.now() - lastTickRef.current)
      setMult(Math.exp(GROWTH_RATE * elapsed / 1000))
      rafRef.current = requestAnimationFrame(frame)
    }

    rafRef.current = requestAnimationFrame(frame)
    return () => cancelAnimationFrame(rafRef.current!)
  }, [roundState, crashPoint])

  return mult
}

/** CSS color tier for a given multiplier (5 tiers). */
export type MultTier = 'cyan' | 'green' | 'orange' | 'purple' | 'red'

export function multTier(m: number): MultTier {
  if (m >= 100) return 'red'
  if (m >= 20)  return 'purple'
  if (m >= 5)   return 'orange'
  if (m >= 2)   return 'green'
  return 'cyan'
}
