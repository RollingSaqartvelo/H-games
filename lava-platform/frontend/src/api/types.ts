/** POST /api/v1/crash/bet */
export interface BetRequest {
  amount: string       // decimal string, e.g. "10.00"
  currency: string     // e.g. "USD"
  auto_cashout?: number // optional; 0 = manual cashout only
}

export interface BetResponse {
  bet_id: string
  round_id: string
  transaction_id: string
}

/** POST /api/v1/crash/cashout */
export interface CashoutRequest {
  bet_id: string
}

export interface CashoutResponse {
  multiplier: number
  payout: string // decimal string
}

/** GET /api/v1/crash/round/current */
export interface CurrentRoundResponse {
  round: {
    id: string
    state: string
    server_seed_hash: string
    client_seed: string
    rtp_profile: number
  }
  multiplier: number
}

export interface ApiError {
  error: string
}
