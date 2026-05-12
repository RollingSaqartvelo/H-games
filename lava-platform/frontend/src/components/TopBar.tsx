import { useGame } from '../store/game'
import type { RoundState } from '../ws/types'

function StateBadge({ state }: { state: RoundState | null }) {
  if (!state || state === 'CREATED') {
    return <span className="state-badge state-badge--waiting">WAITING</span>
  }
  if (state === 'STARTING') {
    return <span className="state-badge state-badge--betting">BETTING</span>
  }
  if (state === 'RUNNING') {
    return <span className="state-badge state-badge--running">RUNNING</span>
  }
  return <span className="state-badge state-badge--crashed">BUSTED</span>
}

function WsIndicator({ status }: { status: string }) {
  const cls =
    status === 'connected'    ? 'ws-dot ws-dot--live'    :
    status === 'connecting'   ? 'ws-dot ws-dot--pulse'   :
    status === 'disconnected' ? 'ws-dot ws-dot--warn'    :
                                'ws-dot ws-dot--error'
  return <span className={cls} aria-hidden="true" />
}

export function TopBar() {
  const wsStatus       = useGame((s) => s.wsStatus)
  const roundState     = useGame((s) => s.roundState)
  const roundId        = useGame((s) => s.roundId)
  const serverSeedHash = useGame((s) => s.serverSeedHash)
  const serverSeed     = useGame((s) => s.serverSeed)
  const balance        = useGame((s) => s.balance)

  const shortId = roundId ? `#${roundId.slice(0, 6)}` : '#------'

  return (
    <header className="top-bar">
      <div className="top-bar__row">
        {/* Left: back + balance */}
        <div className="top-bar__left">
          <button className="top-bar__back" onClick={() => window.location.href = '/'} aria-label="Back">&#8592;</button>
          <WsIndicator status={wsStatus} />
          <div className="top-bar__balance">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
              <circle cx="7" cy="7" r="6.5" stroke="#d97706" strokeWidth="1"/>
              <text x="7" y="10.5" textAnchor="middle" fill="#d97706" fontSize="7" fontWeight="bold">$</text>
            </svg>
            <span className="top-bar__balance-val">
              {balance !== null ? balance.toFixed(2) : '—'}
            </span>
          </div>
        </div>

        {/* Center: logo */}
        <div className="top-bar__center">
          <span className="top-bar__logo">OUTLAW ESCAPE</span>
        </div>

        {/* Right: state badge + round ID */}
        <div className="top-bar__right">
          <StateBadge state={roundState} />
          <span className="top-bar__round-id">{shortId}</span>
        </div>
      </div>

      {/* Provably-fair hash strip */}
      {(serverSeedHash || serverSeed) && (
        <div className="top-bar__hash">
          <span className="top-bar__hash-icon">🔒</span>
          {serverSeed ? (
            <span className="top-bar__hash-text top-bar__hash-text--revealed" title={serverSeed}>
              SEED&nbsp;{serverSeed.slice(0, 22)}…
            </span>
          ) : (
            <span className="top-bar__hash-text" title={serverSeedHash!}>
              {serverSeedHash!.slice(0, 28)}…
            </span>
          )}
        </div>
      )}
    </header>
  )
}
