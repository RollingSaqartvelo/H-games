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
  | { kind: 'next-round' }
  | { kind: 'queued'; amount: string }

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
  const [autoBet,     setAutoBet]     = useState(false)
  const [loading,     setLoading]     = useState(false)
  const [error,       setError]       = useState<string | null>(null)

  // ── Queued next-round bet ────────────────────────────────────────────────────
  const [queuedBet, setQueuedBet] = useState<{
    amount: string
  } | null>(null)

  // ── Derived ─────────────────────────────────────────────────────────────────
  const betAmt = parseFloat(betAmount) || 0
  const payout = betAmt * mult
  const locked = activeBet !== null

  const canBet     = roundState === 'STARTING' && !locked && !loading
  const canCashout = roundState === 'RUNNING'  &&  locked && !cashedOut && !loading
  const isBusted   = roundState === 'CRASHED'  &&  locked && !cashedOut
  const canQueue   = roundState === 'RUNNING'  && !locked && !cashedOut && !queuedBet

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
      // queuedBet is handled by the auto-fire effect below
    }
  }, [roundId])

  // ── Shared bet placement ─────────────────────────────────────────────────────
  const placeBet = useCallback(async (amount: string) => {
    setError(null)
    setLoading(true)
    haptic('tap')
    try {
      const resp = await api.placeBet(token, { amount, currency: 'USD' })
      setActiveBet({ betId: resp.bet_id, amount, currency: 'USD', autoCashout: 0 })
      addActiveBet()
      haptic('success')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to join')
      haptic('error')
    } finally {
      setLoading(false)
    }
  }, [token, addActiveBet])

  // ── Bet handler ─────────────────────────────────────────────────────────────
  const handleBet = useCallback(async () => {
    if (!token) { alert('Открой игру через Telegram бот чтобы делать ставки'); return }
    if (roundState !== 'STARTING' || locked || loading) return
    await placeBet(betAmount)
  }, [token, roundState, locked, loading, betAmount, placeBet])

  // ── Queue next-round bet ─────────────────────────────────────────────────────
  const handleQueueNextRound = useCallback(() => {
    if (!canQueue) return
    setQueuedBet({ amount: betAmount })
    haptic('tap')
  }, [canQueue, betAmount])

  // ── Cancel queued bet ────────────────────────────────────────────────────────
  const handleCancelQueue = useCallback(() => {
    setQueuedBet(null)
    haptic('tap')
  }, [])

  // ── Auto-fire queued bet when next STARTING round begins ─────────────────────
  const placeBetRef = useRef(placeBet)
  placeBetRef.current = placeBet
  useEffect(() => {
    if (!queuedBet || roundState !== 'STARTING' || locked || loading) return
    const q = queuedBet
    setQueuedBet(null)
    void placeBetRef.current(q.amount)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [roundState, locked, loading, queuedBet])

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

  // ── Auto bet — skips if a queued bet will handle this round ─────────────────
  const handleBetRef = useRef(handleBet)
  handleBetRef.current = handleBet

  const autoBetFiredRef = useRef(false)
  useEffect(() => {
    if (!autoBet) { autoBetFiredRef.current = false; return }
    if (roundState === 'STARTING' && !locked && !loading && !autoBetFiredRef.current && !queuedBet) {
      autoBetFiredRef.current = true
      void handleBetRef.current()
    }
    if (roundState !== 'STARTING') autoBetFiredRef.current = false
  }, [roundState, autoBet, locked, loading, queuedBet])

  // ── Button state ────────────────────────────────────────────────────────────
  function getButtonState(): ButtonState {
    if (cashedOut && cashoutMult !== null) return { kind: 'escaped', mult: cashoutMult }
    if (isBusted   && crashPoint !== null) return { kind: 'caught',  mult: crashPoint }
    if (canCashout)                        return { kind: 'cashout', payout }
    if (locked && roundState === 'RUNNING')  return { kind: 'riding',     amount: activeBet!.amount }
    if (locked && roundState === 'STARTING') return { kind: 'placed',     amount: activeBet!.amount }
    if (queuedBet)                           return { kind: 'queued',     amount: queuedBet.amount }
    if (canBet)                              return { kind: 'join' }
    if (canQueue)                            return { kind: 'next-round' }
    return { kind: 'waiting' }
  }

  const bs = getButtonState()
  const onAction =
    bs.kind === 'queued'     ? handleCancelQueue    :
    bs.kind === 'cashout'    ? handleCashout        :
    bs.kind === 'next-round' ? handleQueueNextRound :
    handleBet

  return {
    // Amount controls
    betAmount, setBetAmount,
    betAmt,
    // Options
    autoBet, setAutoBet,
    // Status
    loading, error, locked,
    hasQueuedBet: queuedBet !== null,
    // Win overlay data
    cashedOut,
    cashoutMult,
    cashoutPayout,
    // Action
    buttonState: bs,
    onAction,
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
    case 'next-round':
      return (
        <button className="dbp-btn dbp-btn--next-round" onClick={onAction} disabled={loading}>
          NEXT ROUND BET
        </button>
      )
    case 'queued':
      return (
        <button className="dbp-btn dbp-btn--cancel" onClick={onAction} disabled={loading}>
          <span className="dbp-btn__sub">QUEUED ${state.amount}</span>
          CANCEL
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
  autoBet: boolean
  onAutoBetToggle: () => void
  loading: boolean
  error: string | null
  buttonState: ButtonState
  onAction: () => void
  locked: boolean
  hasQueuedBet: boolean
}

function BetBlockUI({
  label, betAmount, onBetAmountChange, onIncrement, onDecrement,
  autoBet, onAutoBetToggle,
  loading, error, buttonState, onAction, locked, hasQueuedBet,
}: BetBlockUIProps) {
  const inputLocked = locked || hasQueuedBet
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
          disabled={inputLocked || loading}
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
            disabled={inputLocked || loading}
            inputMode="decimal"
          />
        </div>
        <button
          className="dbp-step-btn"
          onClick={onIncrement}
          disabled={inputLocked || loading}
          aria-label="Increase bet"
        >+</button>
      </div>

      {/* Quick chips — hidden while bet is active or queued */}
      {!inputLocked && (
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

      {/* Auto Bet toggle — overlaid on pre-drawn gear circle at X=81-92% of panel */}
      <div className="dbp-options">
        <label className={`dbp-toggle ${autoBet ? 'dbp-toggle--on' : ''}`}>
          <input type="checkbox" checked={autoBet} onChange={onAutoBetToggle} />
          <span className="dbp-toggle__track" />
          <span className="dbp-toggle__label">Auto bet</span>
        </label>
      </div>

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
        autoBet={p.autoBet}
        onAutoBetToggle={() => p.setAutoBet((v) => !v)}
        loading={p.loading}
        error={p.error}
        buttonState={p.buttonState}
        onAction={p.onAction}
        locked={p.locked}
        hasQueuedBet={p.hasQueuedBet}
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
        autoBet={p.autoBet}
        onAutoBetToggle={() => p.setAutoBet((v) => !v)}
        loading={p.loading}
        error={p.error}
        buttonState={p.buttonState}
        onAction={p.onAction}
        locked={p.locked}
        hasQueuedBet={p.hasQueuedBet}
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
