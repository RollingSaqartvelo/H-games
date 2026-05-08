import { useState, useCallback } from 'react'
import { useGame } from '../store/game'
import { api, ApiError } from '../api/http'
import { haptic } from '../lib/telegram'
import { CashoutButton } from './CashoutButton'

interface Props {
  token: string
}

const QUICK_AMOUNTS  = ['0.50', '1.00', '5.00', '10.00']
const QUICK_CASHOUTS = ['1.5', '2', '3', '5', '10']

export function BetPanel({ token }: Props) {
  const roundState     = useGame((s) => s.roundState)
  const activeBet      = useGame((s) => s.activeBet)
  const cashedOut      = useGame((s) => s.cashedOut)
  const cashoutMult    = useGame((s) => s.cashoutMultiplier)
  const crashPoint     = useGame((s) => s.crashPoint)
  const payout         = useGame((s) => s.payout)
  const betAmount      = useGame((s) => s.betAmount)
  const autoCashout    = useGame((s) => s.autoCashout)
  const setBetAmount   = useGame((s) => s.setBetAmount)
  const setAutoCashout = useGame((s) => s.setAutoCashout)
  const setBetActive   = useGame((s) => s.setBetActive)

  const [loading, setLoading]     = useState(false)
  const [error, setError]         = useState<string | null>(null)
  const [acEnabled, setAcEnabled] = useState(false)

  // ── Derived ─────────────────────────────────────────────────────────────────

  const canBet =
    roundState === 'STARTING' &&
    activeBet === null &&
    !loading

  const canCashout =
    roundState === 'RUNNING' &&
    activeBet !== null &&
    !cashedOut &&
    !loading

  const isBusted =
    roundState === 'CRASHED' &&
    activeBet !== null &&
    !cashedOut

  // ── Actions ─────────────────────────────────────────────────────────────────

  const handleBet = useCallback(async () => {
    if (!canBet) return
    setError(null)
    setLoading(true)
    haptic('tap')

    try {
      const acValue = acEnabled && autoCashout ? parseFloat(autoCashout) : undefined
      const resp = await api.placeBet(token, {
        amount:       betAmount,
        currency:     'USD',
        auto_cashout: acValue,
      })
      setBetActive({
        betId:       resp.bet_id,
        amount:      betAmount,
        currency:    'USD',
        autoCashout: acValue ?? 0,
      })
      haptic('success')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to place bet')
      haptic('error')
    } finally {
      setLoading(false)
    }
  }, [canBet, token, betAmount, autoCashout, acEnabled, setBetActive])

  const handleCashout = useCallback(async () => {
    if (!canCashout || !activeBet) return
    setError(null)
    setLoading(true)
    haptic('tap')

    try {
      await api.cashout(token, { bet_id: activeBet.betId })
      haptic('success')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to cashout')
      haptic('error')
    } finally {
      setLoading(false)
    }
  }, [canCashout, activeBet, token])

  // ── Render ──────────────────────────────────────────────────────────────────

  return (
    <div className="bet-panel">
      {error && <div className="bet-panel__error">{error}</div>}

      {/* Result row */}
      {cashedOut && cashoutMult !== null && (
        <div className="bet-panel__result bet-panel__result--win">
          Escaped at <strong>{cashoutMult.toFixed(2)}×</strong>
          {payout && <> — robbed <strong>${payout}</strong></>}
        </div>
      )}
      {isBusted && (
        <div className="bet-panel__result bet-panel__result--loss">
          Caught by sheriffs at {crashPoint?.toFixed(2)}×
        </div>
      )}

      {/* Bet amount row — hidden while bet is in flight */}
      {!canCashout && (
        <div className="bet-panel__row">
          <label className="bet-panel__label">Bet amount</label>
          <div className="bet-panel__amount-wrap">
            <span className="bet-panel__currency">$</span>
            <input
              className="bet-panel__input"
              type="number"
              min="0.01"
              step="0.01"
              value={betAmount}
              onChange={(e) => setBetAmount(e.target.value)}
              disabled={activeBet !== null || loading}
              inputMode="decimal"
            />
          </div>
          <div className="bet-panel__quick">
            {QUICK_AMOUNTS.map((v) => (
              <button
                key={v}
                className="bet-panel__quick-btn"
                onClick={() => setBetAmount(v)}
                disabled={activeBet !== null || loading}
              >
                {v}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Auto-cashout row — hidden while bet is in flight */}
      {!canCashout && (
        <div className="bet-panel__row">
          <label className="bet-panel__label">
            <input
              type="checkbox"
              checked={acEnabled}
              onChange={(e) => setAcEnabled(e.target.checked)}
              disabled={activeBet !== null || loading}
            />
            {' '}Auto cashout
          </label>
          {acEnabled && (
            <>
              <div className="bet-panel__amount-wrap">
                <input
                  className="bet-panel__input bet-panel__input--ac"
                  type="number"
                  min="1.01"
                  step="0.01"
                  placeholder="2.00"
                  value={autoCashout}
                  onChange={(e) => setAutoCashout(e.target.value)}
                  disabled={activeBet !== null || loading}
                  inputMode="decimal"
                />
                <span className="bet-panel__currency">×</span>
              </div>
              <div className="bet-panel__quick">
                {QUICK_CASHOUTS.map((v) => (
                  <button
                    key={v}
                    className="bet-panel__quick-btn"
                    onClick={() => setAutoCashout(v)}
                    disabled={activeBet !== null || loading}
                  >
                    {v}×
                  </button>
                ))}
              </div>
            </>
          )}
        </div>
      )}

      {/* ── Action buttons ─────────────────────────────────────────────────── */}
      <div className="bet-panel__actions">
        {canCashout ? (
          <CashoutButton onCashout={handleCashout} loading={loading} />
        ) : (
          <button
            className={`btn btn--bet ${canBet ? '' : 'btn--disabled'}`}
            onClick={handleBet}
            disabled={!canBet || loading}
          >
            {loading
              ? 'Joining heist…'
              : activeBet && !cashedOut && roundState === 'RUNNING'
                ? `Riding… $${activeBet.amount}`
                : 'JOIN THE HEIST'}
          </button>
        )}
      </div>

      {/* Active bet info — only shown in bet view */}
      {activeBet && !cashedOut && !isBusted && !canCashout && (
        <div className="bet-panel__active-info">
          Bet: <strong>${activeBet.amount}</strong>
          {activeBet.autoCashout > 0 && (
            <> · Auto {activeBet.autoCashout}×</>
          )}
        </div>
      )}
    </div>
  )
}
