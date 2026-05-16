import { useGame } from '../store/game'
import { useRealtimeMult, multTier } from '../hooks/useRealtimeMult'

/**
 * Center-screen game multiplier — shows the GLOBAL round multiplier only.
 * Personal cashout state (ESCAPED / CAUGHT) lives in the bet panel buttons,
 * never here. The center must always reflect what the server round is doing.
 */
export function Multiplier() {
  const roundState = useGame((s) => s.roundState)
  const crashPoint = useGame((s) => s.crashPoint)

  const liveMult = useRealtimeMult()

  const isCrashed  = roundState === 'CRASHED'
  const isRunning  = roundState === 'RUNNING'
  const isStarting = roundState === 'STARTING'
  const isWaiting  = !roundState || roundState === 'CREATED'


  // Color tier: crash = red, running = dynamic tier, others = neutral
  const colorClass = isCrashed
    ? 'mult--crashed'
    : isRunning
      ? `mult--${multTier(liveMult)}`
      : ''

  // Hint text during non-running phases
  const label = isWaiting
    ? 'Preparing getaway…'
    : isStarting
      ? 'Join the heist…'
      : null

  // Display value: crash point when crashed, live otherwise
  const value = isCrashed && crashPoint !== null ? crashPoint : liveMult

  return (
    <div className={`mult ${colorClass}${label ? ' mult--label-only' : ''}`}>
      {label ? (
        <div className="mult__label">{label}</div>
      ) : (
        <>
          <div className="mult__value" aria-live="polite" aria-atomic="true">
            {value.toFixed(2)}
            <span className="mult__x mult__x-label">×</span>
          </div>
          {/* WASTED badge replaced by WastedOverlay cinematic PNG */}
        </>
      )}
    </div>
  )
}
