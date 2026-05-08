import type { WsMsg } from './types'

export type SocketStatus = 'connecting' | 'connected' | 'disconnected' | 'error'

interface SocketOptions {
  url: string
  onMessage: (msg: WsMsg) => void
  onStatusChange: (status: SocketStatus) => void
}

const BASE_DELAY_MS = 1_000
const MAX_DELAY_MS  = 30_000
const JITTER_RATIO  = 0.2

/**
 * WebSocket client with automatic exponential-backoff reconnection.
 *
 * Reconnect schedule (approximate, ±20% jitter):
 *   attempt 1 →  1s, attempt 2 →  2s, attempt 3 →  4s,
 *   attempt 4 →  8s, attempt 5 → 16s, attempt 6+ → 30s
 */
export class ReconnectingSocket {
  private ws: WebSocket | null = null
  private attempt = 0
  private timer: ReturnType<typeof setTimeout> | null = null
  private closed = false // set by close() to suppress reconnect

  constructor(private readonly opts: SocketOptions) {
    this.connect()
  }

  private connect(): void {
    this.opts.onStatusChange('connecting')

    const ws = new WebSocket(this.opts.url)
    this.ws = ws

    ws.onopen = () => {
      this.attempt = 0
      this.opts.onStatusChange('connected')
    }

    ws.onmessage = (ev: MessageEvent<string>) => {
      try {
        const msg = JSON.parse(ev.data) as WsMsg
        this.opts.onMessage(msg)
      } catch {
        // Malformed frame — ignore
      }
    }

    ws.onerror = () => {
      // onerror always precedes onclose; status is set there
      this.opts.onStatusChange('error')
    }

    ws.onclose = () => {
      if (this.closed) return
      this.opts.onStatusChange('disconnected')
      this.scheduleReconnect()
    }
  }

  private scheduleReconnect(): void {
    const base  = Math.min(BASE_DELAY_MS * 2 ** this.attempt, MAX_DELAY_MS)
    const jitter = base * JITTER_RATIO * (Math.random() * 2 - 1)
    const delay  = Math.round(base + jitter)

    this.attempt++
    this.timer = setTimeout(() => this.connect(), delay)
  }

  /** Permanently close — no further reconnect attempts. */
  close(): void {
    this.closed = true
    if (this.timer !== null) {
      clearTimeout(this.timer)
      this.timer = null
    }
    this.ws?.close()
  }
}
