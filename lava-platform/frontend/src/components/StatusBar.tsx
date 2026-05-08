import { useGame } from '../store/game'
import type { SocketStatus } from '../ws/socket'

const STATUS_LABEL: Record<SocketStatus, string> = {
  connecting:   'Connecting...',
  connected:    'Live',
  disconnected: 'Reconnecting...',
  error:        'Connection error',
}

const STATUS_CLASS: Record<SocketStatus, string> = {
  connecting:   'status--pulse',
  connected:    'status--live',
  disconnected: 'status--warn',
  error:        'status--error',
}

export function StatusBar() {
  const wsStatus       = useGame((s) => s.wsStatus)
  const roundId        = useGame((s) => s.roundId)
  const serverSeedHash = useGame((s) => s.serverSeedHash)
  const serverSeed     = useGame((s) => s.serverSeed)
  const roundState     = useGame((s) => s.roundState)

  const shortId = roundId ? roundId.slice(0, 8) : '—'

  return (
    <header className="status-bar">
      <div className="status-bar__left">
        <span className={`status-dot ${STATUS_CLASS[wsStatus]}`} />
        <span className="status-bar__label">{STATUS_LABEL[wsStatus]}</span>
      </div>

      <div className="status-bar__center">
        <span className="status-bar__game">OUTLAW ESCAPE</span>
      </div>

      <div className="status-bar__right">
        {roundState && (
          <span className={`round-badge round-badge--${roundState.toLowerCase()}`}>
            {roundState}
          </span>
        )}
        <span className="status-bar__round-id" title={roundId ?? ''}>
          #{shortId}
        </span>
      </div>

      {/* Provably-fair hash — visible when round is running/starting */}
      {serverSeedHash && !serverSeed && (
        <div className="status-bar__seed" title={serverSeedHash}>
          Hash: {serverSeedHash.slice(0, 16)}…
        </div>
      )}
      {/* Reveal server seed after crash so players can verify */}
      {serverSeed && (
        <div className="status-bar__seed status-bar__seed--revealed" title={serverSeed}>
          Seed: {serverSeed.slice(0, 16)}…
        </div>
      )}
    </header>
  )
}
