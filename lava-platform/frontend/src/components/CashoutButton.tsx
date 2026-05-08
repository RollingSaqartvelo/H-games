import { useEffect, useRef, useState } from 'react'
import { useGame } from '../store/game'
import { useRealtimeMult, multTier } from '../hooks/useRealtimeMult'

interface Props {
  onCashout: () => void
  loading: boolean
}

/**
 * Animated cashout button with live payout and profit display.
 *
 * Animation layers:
 *   1. Button glow pulse — CSS keyframe, speed scales with multiplier tier
 *   2. Amount breathe   — subtle scale oscillation, speeds up at higher tiers
 *   3. Tier pop         — amount pops 20% when crossing a tier boundary
 *   4. Profit display   — below button, color tracks tier
 */
export function CashoutButton({ onCashout, loading }: Props) {
  const activeBet = useGame((s) => s.activeBet)
  const mult      = useRealtimeMult()

  const betAmt = parseFloat(activeBet?.amount ?? '0')
  const payout = betAmt * mult
  const profit = payout - betAmt
  const tier   = multTier(mult)

  // Trigger a CSS pop animation whenever we cross a tier boundary
  const [popKey, setPopKey] = useState(0)
  const prevTierRef = useRef(tier)
  useEffect(() => {
    if (tier !== prevTierRef.current) {
      prevTierRef.current = tier
      setPopKey((k) => k + 1)
    }
  }, [tier])

  const profitColor =
    tier === 'red'    ? '#f87171' :
    tier === 'purple' ? '#c084fc' :
    tier === 'orange' ? '#fb923c' :
    tier === 'green'  ? '#4ade80' :
    '#00d4ff'  // cyan

  return (
    <div className="co-wrap">
      <button
        className={`co-btn co-btn--${tier}`}
        onClick={onCashout}
        disabled={loading}
        aria-label={`Cash out for ${payout.toFixed(2)} USD`}
      >
        <span className="co-btn__label">
          {loading ? 'Escaping…' : 'ESCAPE NOW'}
        </span>

        {/* Amount remounts on tier change → triggers pop CSS animation */}
        <span key={popKey} className="co-btn__amount">
          ${payout.toFixed(2)}
        </span>
      </button>

      <div className="co-profit" style={{ color: profitColor }}>
        Loot +${profit.toFixed(2)}
      </div>
    </div>
  )
}
