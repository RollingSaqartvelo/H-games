/**
 * Thin fetch wrapper for the crash game API.
 *
 * In production, VITE_API_BASE points to the operator's backend which:
 *   1. Validates the player session
 *   2. Adds HMAC operator auth headers (X-API-KEY, X-TIMESTAMP, X-SIGNATURE)
 *   3. Proxies the request to the game provider
 *
 * In development, Vite's proxy handles the /api path (see vite.config.ts).
 */

import type { BetRequest, BetResponse, CashoutRequest, CashoutResponse } from './types'

const API_BASE = import.meta.env.VITE_API_BASE ?? ''

class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<TRes>(
  method: string,
  path: string,
  token: string,
  body?: unknown,
): Promise<TRes> {
  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  const json = (await res.json()) as { error?: string } & TRes
  if (!res.ok) {
    throw new ApiError(res.status, json.error ?? `HTTP ${res.status}`)
  }
  return json
}

// TMA Mini App uses /tma/v1/crash/* — no operator HMAC required.
// The session token issued by /tma/auth encodes the operator identity.
export const api = {
  placeBet: (token: string, req: BetRequest) =>
    request<BetResponse>('POST', '/tma/v1/crash/bet', token, req),

  cashout: (token: string, req: CashoutRequest) =>
    request<CashoutResponse>('POST', '/tma/v1/crash/cashout', token, req),
}

export { ApiError }
