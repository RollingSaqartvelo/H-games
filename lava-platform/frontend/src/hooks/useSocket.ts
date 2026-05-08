import { useEffect, useRef } from 'react'
import { ReconnectingSocket } from '../ws/socket'
import { useGame } from '../store/game'
import type {
  WsMsg,
  StateData,
  TickData,
  CrashedData,
  BetPlacedData,
  CashoutData,
} from '../ws/types'

const WS_URL =
  import.meta.env.VITE_WS_URL ||
  `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/ws/crash`

/**
 * Opens (and auto-reconnects) the WebSocket connection, dispatching every
 * incoming message into the Zustand store.
 *
 * Pass `playerId` so that `cashout` messages can be matched against the
 * current player (all cashout events are broadcast to everyone).
 */
export function useSocket(playerId: string): void {
  const { setWsStatus, applyState, applyTick, applyCrashed, applyCashoutMsg } =
    useGame.getState()

  // Keep the latest playerId in a ref so the message handler is always current
  // without needing to re-subscribe on every render.
  const playerIdRef = useRef(playerId)
  playerIdRef.current = playerId

  useEffect(() => {
    const socket = new ReconnectingSocket({
      url: WS_URL,
      onStatusChange: setWsStatus,
      onMessage: (msg: WsMsg) => {
        switch (msg.type) {
          case 'state':
            applyState(msg.data as StateData)
            break
          case 'tick':
            applyTick(msg.data as TickData)
            break
          case 'crashed':
            applyCrashed(msg.data as CrashedData)
            break
          case 'bet_placed':
            // No store update needed — just a social signal, rendered via history
            void (msg.data as BetPlacedData)
            break
          case 'cashout':
            applyCashoutMsg(msg.data as CashoutData, playerIdRef.current)
            break
          default:
            break
        }
      },
    })

    return () => socket.close()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []) // Single mount — socket lives for the component lifetime
}
