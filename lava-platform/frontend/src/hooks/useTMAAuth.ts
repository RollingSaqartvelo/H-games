/**
 * useTMAAuth — Telegram Mini App authentication hook.
 *
 * Resolution order:
 *   1. Existing valid token in localStorage / URL param  → use immediately
 *   2. window.Telegram.WebApp.initData available           → exchange via /tma/auth
 *   3. VITE_DEV_TOKEN env var                              → use for local dev
 *   4. No token                                            → show "no session" UI
 *
 * The hook is idempotent: it stores the resolved token in localStorage so
 * subsequent renders (and StrictMode double-invocations) are instant.
 */

import { useState, useEffect } from 'react'
import { getSessionToken } from '../lib/session'

interface TMAAuthState {
  token: string
  playerId: string
  firstName: string
  loading: boolean
  error: string | null
}

const TMA_AUTH_URL = '/tma/auth'

async function exchangeInitData(initData: string): Promise<{ token: string; player_id: string; first_name: string }> {
  const res = await fetch(TMA_AUTH_URL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ init_data: initData, currency: 'USD' }),
  })
  const json = await res.json() as { token?: string; player_id?: string; first_name?: string; error?: string }
  if (!res.ok) throw new Error(json.error ?? `Auth failed: ${res.status}`)
  if (!json.token) throw new Error('No token in response')
  return { token: json.token, player_id: json.player_id ?? 'anon', first_name: json.first_name ?? '' }
}

/** Extract player ID from a JWT payload (sub or player_id claim). */
function playerIdFromJWT(token: string): string {
  try {
    const parts = token.split('.')
    if (parts.length === 3) {
      const payload = JSON.parse(atob(parts[1])) as { sub?: string; player_id?: string }
      return payload.player_id ?? payload.sub ?? 'anon'
    }
  } catch { /* not a JWT */ }
  return 'anon'
}

export function useTMAAuth(): TMAAuthState {
  const [state, setState] = useState<TMAAuthState>({
    token:     '',
    playerId:  'anon',
    firstName: '',
    loading:   true,
    error:     null,
  })

  useEffect(() => {
    let cancelled = false

    async function resolve() {
      // 1. Existing token (URL param → localStorage → VITE_DEV_TOKEN)
      const existing = getSessionToken()
      if (existing) {
        if (!cancelled) {
          setState({
            token:     existing,
            playerId:  playerIdFromJWT(existing),
            firstName: '',
            loading:   false,
            error:     null,
          })
        }
        return
      }

      // 2. Telegram initData
      try {
        const tma = (window as Window & { Telegram?: { WebApp?: { initData?: string } } }).Telegram?.WebApp
        const initData = tma?.initData

        if (initData) {
          const data = await exchangeInitData(initData)
          localStorage.setItem('crash_token', data.token)
          if (!cancelled) {
            setState({
              token:     data.token,
              playerId:  data.player_id,
              firstName: data.first_name,
              loading:   false,
              error:     null,
            })
          }
          return
        }
      } catch (e) {
        const msg = e instanceof Error ? e.message : 'TMA auth failed'
        if (!cancelled) {
          setState((prev) => ({ ...prev, loading: false, error: msg }))
        }
        return
      }

      // 3. No token available
      if (!cancelled) {
        setState((prev) => ({ ...prev, loading: false }))
      }
    }

    void resolve()
    return () => { cancelled = true }
  }, [])

  return state
}
