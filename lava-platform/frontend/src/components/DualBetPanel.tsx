import { useState, useCallback, useEffect, useRef } from 'react'
import { useGame } from '../store/game'
import { api, ApiError } from '../api/http'
import { haptic } from '../lib/telegram'
import { useRealtimeMult } from '../hooks/useRealtimeMult'
import type { ActiveBet } from '../store/game'
import { YouWinOverlay } from './YouWinOverlay'
import { playWinSound } from '../audio/WinSound'

// ─── Shared constants ────────────────────────────────────────────────────────

const QUICK_CHIPS    = ['1.00', '5.00', '10.00', '50.00']
const QUICK_CASHOUTS = ['1.5', '2.0', '3.0', '5.0', '10.0']
const BET_STEP       = 0.50
const BET_MIN        = 0.50

// ─── Button state union ───────────────────────────────────────────────────────

type ButtonState =
  | { kind: 'join' }
  | { kind: 'placed'; amount: string }
  | { kind: 'cashout'; payout: number }
  | { kind: 'escaped'; mult: number }
  | { kind: 'caught'; mult: number }
  | { kind: 'waiting' }
  | { kind: 'riding'; amount: string }

// ─── Per-panel hook — fully local, zero shared state between panels ───────────
//
// RULE: Each panel tracks its own activeBet/cashedOut/cashoutMult in local
// React state. No panel reads the other panel's state from Zustand.
// The only store values read are round-level globals:
//   roundState, roundId, crashPoint, setBalance

function useBetPanel(token: string) {
  // ── Round globals (read-only) ───────────────────────────────────────────────
  const roundState    = useGame((s) => s.roundState)
  const roundId       = useGame((s) => s.roundId)
  const crashPoint    = useGame((s) => s.crashPoint)
  const setBalance    = useGame((s) => s.setBalance)
  const addActiveBet  = useGame((s) => s.addActiveBet)
  const addCashedOut  = useGame((s) => s.addCashedOutBet)
  const mult          = useRealtimeMult()

  // ── Per-panel local state ───────────────────────────────────────────────────
  const [activeBet,     setActiveBet]     = useState<ActiveBet | null>(null)
  const [cashedOut,     setCashedOut]     = useState(false)
  const [cashoutMult,   setCashoutMult]   = useState<number | null>(null)
  const [cashoutPayout, setCashoutPayout] = useState<number | null>(null)

  const [betAmount,   setBetAmount]   = useState('1.00')
  const [autoCashout, setAutoCashout] = useState('')
  const [acEnabled,   setAcEnabled]   = useState(false)
  const [autoBet,     setAutoBet]     = useState(false)
  const [loading,     setLoading]     = useState(false)
  const [error,       setError]       = useState<string | null>(null)

  // ── Derived ─────────────────────────────────────────────────────────────────
  const betAmt = parseFloat(betAmount) || 0
  const payout = betAmt * mult
  const locked = activeBet !== null

  const canBet     = roundState === 'STARTING' && !locked && !loading
  const canCashout = roundState === 'RUNNING'  &&  locked && !cashedOut && !loading
  const isBusted   = roundState === 'CRASHED'  &&  locked && !cashedOut

  // ── Reset on new round ──────────────────────────────────────────────────────
  const prevRoundId = useRef<string | null>(null)
  useEffect(() => {
    if (roundId && roundId !== prevRoundId.current) {
      prevRoundId.current = roundId
      setActiveBet(null)
      setCashedOut(false)
      setCashoutMult(null)
      setCashoutPayout(null)
      setError(null)
    }
  }, [roundId])

  // ── Bet handler ─────────────────────────────────────────────────────────────
  const handleBet = useCallback(async () => {
    if (roundState !== 'STARTING' || locked || loading) return
    setError(null)
    setLoading(true)
    haptic('tap')
    try {
      const ac = acEnabled && autoCashout ? parseFloat(autoCashout) : undefined
      const resp = await api.placeBet(token, {
        amount: betAmount, currency: 'USD', auto_cashout: ac,
      })
      setActiveBet({ betId: resp.bet_id, amount: betAmount, currency: 'USD', autoCashout: ac ?? 0 })
      addActiveBet()
      haptic('success')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to join')
      haptic('error')
    } finally {
      setLoading(false)
    }
  }, [roundState, locked, loading, token, betAmount, acEnabled, autoCashout])

  // ── Cashout handler — updates only THIS panel's local state ─────────────────
  const handleCashout = useCallback(async () => {
    if (!canCashout || !activeBet) return
    setError(null)
    setLoading(true)
    haptic('tap')
    try {
      const resp = await api.cashout(token, { bet_id: activeBet.betId })
      setCashedOut(true)
      setCashoutMult(resp.multiplier ?? null)
      if (resp.payout) {
        const p = parseFloat(resp.payout)
        setCashoutPayout(p)
        setBalance(p)
      }
      addCashedOut()
      playWinSound()
      haptic('success')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to escape')
      haptic('error')
    } finally {
      setLoading(false)
    }
  }, [canCashout, activeBet, token, setBalance])

  // ── Auto-cashout sync — backend fires cashout, WS event arrives, update local state ──
  const lastCashoutEvent = useGame((s) => s.lastCashoutEvent)
  const activeBetRef = useRef(activeBet)
  activeBetRef.current = activeBet
  useEffect(() => {
    const ev = lastCashoutEvent
    const ab = activeBetRef.current
    if (!ev || !ab || ev.bet_id !== ab.betId || cashedOut) return
    setCashedOut(true)
    setCashoutMult(ev.multiplier)
    const p = parseFloat(ev.payout)
    setCashoutPayout(p)
    setBalance(p)
    addCashedOut()
    playWinSound()
    haptic('success')
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lastCashoutEvent])

  // ── Auto bet — fires once per round using a stable ref ─────────────────────
  const handleBetRef = useRef(handleBet)
  handleBetRef.current = handleBet

  const autoBetFiredRef = useRef(false)
  useEffect(() => {
    if (!autoBet) { autoBetFiredRef.current = false; return }
    if (roundState === 'STARTING' && !locked && !loading && !autoBetFiredRef.current) {
      autoBetFiredRef.current = true
      void handleBetRef.current()
    }
    if (roundState !== 'STARTING') autoBetFiredRef.current = false
  }, [roundState, autoBet, locked, loading])

  // ── Button state ────────────────────────────────────────────────────────────
  function getButtonState(): ButtonState {
    if (cashedOut && cashoutMult !== null) return { kind: 'escaped', mult: cashoutMult }
    if (isBusted   && crashPoint !== null) return { kind: 'caught',  mult: crashPoint }
    if (canCashout)                        return { kind: 'cashout', payout }
    if (locked && roundState === 'RUNNING')  return { kind: 'riding', amount: activeBet!.amount }
    if (locked && roundState === 'STARTING') return { kind: 'placed', amount: activeBet!.amount }
    if (canBet)                            return { kind: 'join' }
    return { kind: 'waiting' }
  }

  return {
    // Amount controls
    betAmount, setBetAmount,
    betAmt,
    // Options
    acEnabled,   setAcEnabled,
    autoCashout, setAutoCashout,
    autoBet,     setAutoBet,
    // Status
    loading, error, locked,
    // Win overlay data
    cashedOut,
    cashoutMult,
    cashoutPayout,
    // Action
    buttonState: getButtonState(),
    onAction: canCashout ? handleCashout : handleBet,
  }
}

// ─── Shared action button ─────────────────────────────────────────────────────

function ActionButton({ state, loading, onAction }: {
  state: ButtonState
  loading: boolean
  onAction: () => void
}) {
  switch (state.kind) {
    case 'join':
      return (
        <button className="dbp-btn dbp-btn--join" onClick={onAction} disabled={loading}>
          {loading ? 'Joining…' : 'BET NOW'}
        </button>
      )
    case 'placed':
      return (
        <button className="dbp-btn dbp-btn--placed" disabled>
          ✓ ${state.amount} PLACED
        </button>
      )
    case 'cashout':
      return (
        <button className="dbp-btn dbp-btn--cashout" onClick={onAction} disabled={loading}>
          {loading ? 'Cashing out…' : `CASHOUT $${state.payout.toFixed(2)}`}
        </button>
      )
    case 'escaped':
      return (
        <button className="dbp-btn dbp-btn--escaped" disabled>
          WIN {state.mult.toFixed(2)}×
        </button>
      )
    case 'caught':
      return (
        <button className="dbp-btn dbp-btn--caught" disabled>
          WASTED {state.mult.toFixed(2)}×
        </button>
      )
    case 'riding':
      return (
        <button className="dbp-btn dbp-btn--riding" disabled>
          RIDING… ${state.amount}
        </button>
      )
    case 'waiting':
    default:
      return (
        <button className="dbp-btn dbp-btn--waiting" disabled>
          NEXT HEIST…
        </button>
      )
  }
}

// ─── Shared presentational UI ─────────────────────────────────────────────────

interface BetBlockUIProps {
  label: string
  betAmount: string
  onBetAmountChange: (v: string) => void
  onIncrement: () => void
  onDecrement: () => void
  acEnabled: boolean
  onAcToggle: () => void
  autoCashout: string
  onAutoCashoutChange: (v: string) => void
  autoBet: boolean
  onAutoBetToggle: () => void
  loading: boolean
  error: string | null
  buttonState: ButtonState
  onAction: () => void
  locked: boolean
}

function BetBlockUI({
  label, betAmount, onBetAmountChange, onIncrement, onDecrement,
  acEnabled, onAcToggle, autoCashout, onAutoCashoutChange,
  autoBet, onAutoBetToggle,
  loading, error, buttonState, onAction, locked,
}: BetBlockUIProps) {
  return (
    <div className="dbp-block">
      <div className="dbp-block__header">
        <span className="dbp-block__label">{label}</span>
        {autoBet && <span className="dbp-block__auto-tag">AUTO</span>}
      </div>

      {/* Bet amount stepper */}
      <div className="dbp-amount-row">
        <button
          className="dbp-step-btn"
          onClick={onDecrement}
          disabled={locked || loading}
          aria-label="Decrease bet"
        >−</button>
        <div className="dbp-amount-display">
          <span className="dbp-amount-currency">$</span>
          <input
            className="dbp-amount-input"
            type="number"
            min={BET_MIN}
            step={BET_STEP}
            value={betAmount}
            onChange={(e) => onBetAmountChange(e.target.value)}
            disabled={locked || loading}
            inputMode="decimal"
          />
        </div>
        <button
          className="dbp-step-btn"
          onClick={onIncrement}
          disabled={locked || loading}
          aria-label="Increase bet"
        >+</button>
      </div>

      {/* Quick chips — hidden while bet is active */}
      {!locked && (
        <div className="dbp-chips">
          {QUICK_CHIPS.map((v) => (
            <button
              key={v}
              className={`dbp-chip ${betAmount === v ? 'dbp-chip--active' : ''}`}
              onClick={() => onBetAmountChange(v)}
              disabled={loading}
            >
              ${v.replace('.00', '')}
            </button>
          ))}
        </div>
      )}

      {/* Options row — auto cash hidden while locked, auto bet always visible */}
      <div className="dbp-options">
        {!locked && (
          <label className={`dbp-toggle ${acEnabled ? 'dbp-toggle--on' : ''}`}>
            <input type="checkbox" checked={acEnabled} onChange={onAcToggle} disabled={loading} />
            <span className="dbp-toggle__track" />
            <span className="dbp-toggle__label">
              Auto {acEnabled && autoCashout ? `${autoCashout}×` : 'cash'}
            </span>
          </label>
        )}

        <label className={`dbp-toggle ${autoBet ? 'dbp-toggle--on' : ''}`}>
          <input type="checkbox" checked={autoBet} onChange={onAutoBetToggle} />
          <span className="dbp-toggle__track" />
          <span className="dbp-toggle__label">Auto bet</span>
        </label>
      </div>

      {/* Auto cashout multiplier picker */}
      {acEnabled && !locked && (
        <div className="dbp-ac-row">
          <span className="dbp-ac-label">Cash at</span>
          <div className="dbp-ac-chips">
            {QUICK_CASHOUTS.map((v) => (
              <button
                key={v}
                className={`dbp-chip dbp-chip--sm ${autoCashout === v ? 'dbp-chip--active' : ''}`}
                onClick={() => onAutoCashoutChange(v)}
                disabled={locked || loading}
              >
                {v}×
              </button>
            ))}
          </div>
          <input
            className="dbp-ac-input"
            type="number"
            min="1.01"
            step="0.01"
            placeholder="2.00"
            value={autoCashout}
            onChange={(e) => onAutoCashoutChange(e.target.value)}
            disabled={locked || loading}
            inputMode="decimal"
          />
        </div>
      )}

      {error && <div className="dbp-error">{error}</div>}

      <div className="dbp-btn-area">
        <ActionButton state={buttonState} loading={loading} onAction={onAction} />
      </div>
    </div>
  )
}

// ─── Panel A & B — identical architecture, zero shared bet state ───────────────

function PanelA({ token }: { token: string; playerId: string }) {
  const p = useBetPanel(token)
  return (
    <>
      <YouWinOverlay
        panel="a"
        show={p.cashedOut && p.cashoutMult !== null}
        amount={p.cashoutPayout ?? 0}
        multiplier={p.cashoutMult ?? 0}
      />
      <BetBlockUI
        label="PANEL A"
      betAmount={p.betAmount}
      onBetAmountChange={p.setBetAmount}
      onIncrement={() => p.setBetAmount((Math.max(BET_MIN, p.betAmt + BET_STEP)).toFixed(2))}
      onDecrement={() => p.setBetAmount((Math.max(BET_MIN, p.betAmt - BET_STEP)).toFixed(2))}
      acEnabled={p.acEnabled}
      onAcToggle={() => p.setAcEnabled((v) => !v)}
      autoCashout={p.autoCashout}
      onAutoCashoutChange={p.setAutoCashout}
      autoBet={p.autoBet}
      onAutoBetToggle={() => p.setAutoBet((v) => !v)}
      loading={p.loading}
      error={p.error}
      buttonState={p.buttonState}
      onAction={p.onAction}
      locked={p.locked}
    />
    </>
  )
}

function PanelB({ token }: { token: string }) {
  const p = useBetPanel(token)
  return (
    <>
      <YouWinOverlay
        panel="b"
        show={p.cashedOut && p.cashoutMult !== null}
        amount={p.cashoutPayout ?? 0}
        multiplier={p.cashoutMult ?? 0}
      />
      <BetBlockUI
        label="PANEL B"
      betAmount={p.betAmount}
      onBetAmountChange={p.setBetAmount}
      onIncrement={() => p.setBetAmount((Math.max(BET_MIN, p.betAmt + BET_STEP)).toFixed(2))}
      onDecrement={() => p.setBetAmount((Math.max(BET_MIN, p.betAmt - BET_STEP)).toFixed(2))}
      acEnabled={p.acEnabled}
      onAcToggle={() => p.setAcEnabled((v) => !v)}
      autoCashout={p.autoCashout}
      onAutoCashoutChange={p.setAutoCashout}
      autoBet={p.autoBet}
      onAutoBetToggle={() => p.setAutoBet((v) => !v)}
      loading={p.loading}
      error={p.error}
      buttonState={p.buttonState}
      onAction={p.onAction}
      locked={p.locked}
    />
    </>
  )
}

// ─── Public export ────────────────────────────────────────────────────────────

interface DualBetPanelProps {
  token: string
  playerId: string
}

export function DualBetPanel({ token, playerId }: DualBetPanelProps) {
  return (
    <div className="dbp-root">
      <PanelA token={token} playerId={playerId} />
      <div className="dbp-divider" aria-hidden="true" />
      <PanelB token={token} />
    </div>
  )
}
