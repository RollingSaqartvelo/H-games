import { create } from 'zustand'
import { subscribeWithSelector } from 'zustand/middleware'
import type { SocketStatus } from '../ws/socket'
import type {
  RoundState,
  StateData,
  TickData,
  CrashedData,
  CashoutData,
} from '../ws/types'

// ─── Shape ────────────────────────────────────────────────────────────────────

export interface HistoryEntry {
  roundId: string
  crashPoint: number
}

export interface ActiveBet {
  betId: string
  amount: string
  currency: string
  autoCashout: number // 0 = manual
}

export interface CashoutFeedEntry {
  id: number
  playerId: string
  multiplier: number
}

interface GameState {
  // ── Connection ──────────────────────────────────────────────────────────────
  wsStatus: SocketStatus

  // ── Round ───────────────────────────────────────────────────────────────────
  roundId: string | null
  roundState: RoundState | null
  serverSeedHash: string | null

  // ── STARTING phase wall-clock timestamp ─────────────────────────────────────
  startingAt: number | null

  // ── Tick interpolation (updated every 100 ms from WS) ───────────────────────
  elapsedMs: number
  lastTickAt: number

  // ── Crash ───────────────────────────────────────────────────────────────────
  crashPoint: number | null
  serverSeed: string | null

  // ── Player ──────────────────────────────────────────────────────────────────
  activeBet: ActiveBet | null
  cashedOut: boolean
  cashoutMultiplier: number | null
  payout: string | null

  // ── Per-round bet slot tracking (for DualBetPanel crash synchronisation) ────
  // activeBetCount  = number of panels that placed a bet this round
  // cashedOutCount  = number of those bets that cashed out before crash
  // anyBetLost      = at least one active bet was NOT cashed out → crash plays
  activeBetCount:  number
  cashedOutCount:  number
  anyBetLost:      boolean

  // ── History (last 20 crashed rounds) ────────────────────────────────────────
  history: HistoryEntry[]

  // ── Social cashout feed (last 6 cashouts by any player) ─────────────────────
  recentCashouts: CashoutFeedEntry[]

  // ── Form ────────────────────────────────────────────────────────────────────
  betAmount: string
  autoCashout: string

  // ── Wallet balance (updated from API responses) ──────────────────────────────
  balance: number | null
}

interface GameActions {
  setWsStatus(s: SocketStatus): void
  applyState(d: StateData): void
  applyTick(d: TickData): void
  applyCrashed(d: CrashedData): void
  applyCashoutMsg(d: CashoutData, myPlayerId: string): void
  setBetActive(bet: ActiveBet): void
  setBetAmount(v: string): void
  setAutoCashout(v: string): void
  setBalance(v: number): void
  addActiveBet(): void
  addCashedOutBet(): void
}

// ─── Store ────────────────────────────────────────────────────────────────────

export const useGame = create<GameState & GameActions>()(
  subscribeWithSelector((set) => ({
    // ── Initial state ──────────────────────────────────────────────────────────
    wsStatus: 'connecting',
    roundId: null,
    roundState: null,
    serverSeedHash: null,
    startingAt: null,
    elapsedMs: 0,
    lastTickAt: Date.now(),
    crashPoint: null,
    serverSeed: null,
    activeBet: null,
    cashedOut: false,
    cashoutMultiplier: null,
    payout: null,
    activeBetCount: 0,
    cashedOutCount: 0,
    anyBetLost: false,
    history: [],
    recentCashouts: [],
    betAmount: '1.00',
    autoCashout: '',
    balance: null,

    // ── Actions ────────────────────────────────────────────────────────────────

    setWsStatus: (wsStatus) => set({ wsStatus }),

    applyState: (d) =>
      set((prev) => {
        const newRound = prev.roundId !== d.id
        return {
          roundId: d.id,
          roundState: d.state,
          serverSeedHash: d.server_seed_hash,
          startingAt: d.state === 'STARTING' ? Date.now() : null,
          activeBet:         newRound ? null  : prev.activeBet,
          cashedOut:         newRound ? false : prev.cashedOut,
          cashoutMultiplier: newRound ? null  : prev.cashoutMultiplier,
          payout:            newRound ? null  : prev.payout,
          recentCashouts:    newRound ? []    : prev.recentCashouts,
          activeBetCount:    newRound ? 0     : prev.activeBetCount,
          cashedOutCount:    newRound ? 0     : prev.cashedOutCount,
          anyBetLost:        newRound ? false : prev.anyBetLost,
          crashPoint: null,
          serverSeed: null,
        }
      }),

    applyTick: (d) =>
      set({
        elapsedMs: d.elapsed_ms,
        lastTickAt: Date.now(),
      }),

    applyCrashed: (d) =>
      set((prev) => {
        const entry: HistoryEntry = { roundId: d.round_id, crashPoint: d.crash_point }
        // anyBetLost: true if any panel placed a bet AND at least one didn't cash out
        const anyBetLost = prev.activeBetCount > 0
          ? prev.activeBetCount > prev.cashedOutCount
          : true   // no tracked bets = spectator mode → crash animation plays
        return {
          roundState: 'CRASHED',
          crashPoint: d.crash_point,
          serverSeed: d.server_seed,
          activeBet:  prev.cashedOut ? prev.activeBet : null,
          anyBetLost,
          history: [entry, ...prev.history].slice(0, 20),
        }
      }),

    applyCashoutMsg: (d, myPlayerId) =>
      set((prev) => {
        const entry: CashoutFeedEntry = {
          id: Date.now() + Math.random(),
          playerId: d.player_id,
          multiplier: d.multiplier,
        }
        const recentCashouts = [entry, ...prev.recentCashouts].slice(0, 6)
        if (d.player_id === myPlayerId) {
          return { cashedOut: true, cashoutMultiplier: d.multiplier, payout: d.payout, recentCashouts }
        }
        return { recentCashouts }
      }),

    setBetActive: (bet) =>
      set({
        activeBet: bet,
        cashedOut: false,
        cashoutMultiplier: null,
        payout: null,
      }),

    setBetAmount: (betAmount) => set({ betAmount }),
    setAutoCashout: (autoCashout) => set({ autoCashout }),
    setBalance: (balance) => set({ balance }),
    addActiveBet:    () => set((s) => ({ activeBetCount: s.activeBetCount + 1 })),
    addCashedOutBet: () => set((s) => ({ cashedOutCount: s.cashedOutCount + 1 })),
  })),
)
