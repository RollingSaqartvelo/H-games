export type RoundState = 'CREATED' | 'STARTING' | 'RUNNING' | 'CRASHED' | 'FINISHED'

export type MsgType =
  | 'state'
  | 'tick'
  | 'crashed'
  | 'pre_crash'
  | 'bet_placed'
  | 'cashout'
  | 'error'

export interface WsMsg<T = unknown> {
  type: MsgType
  data: T
}

/** Received on connect and on every state transition */
export interface StateData {
  id: string
  state: RoundState
  server_seed_hash: string
  client_seed: string
  rtp_profile: number
  started_at?: number // unix seconds (backend sends this when running)
}

/** Emitted every 100 ms while round is RUNNING */
export interface TickData {
  round_id: string
  multiplier: number
  elapsed_ms: number
}

/** Emitted when the round crashes — reveals the server seed for fairness proof */
export interface CrashedData {
  round_id: string
  crash_point: number
  server_seed: string
  client_seed: string
  nonce: number
}

export interface BetPlacedData {
  round_id: string
  player_id: string
  amount: string
  currency: string
}

export interface CashoutData {
  round_id: string
  bet_id: string
  player_id: string
  multiplier: number
  payout: string
  currency: string
}

export interface ErrorData {
  message: string
}
