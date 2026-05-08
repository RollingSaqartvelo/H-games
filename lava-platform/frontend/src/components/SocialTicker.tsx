import { useGame } from '../store/game'

function shortPlayer(id: string): string {
  if (id.length <= 6) return id
  return `…${id.slice(-5)}`
}

export function SocialTicker() {
  const entries    = useGame((s) => s.recentCashouts)
  const roundState = useGame((s) => s.roundState)

  // Count active bets as a proxy for "players in round"
  // (we don't have an online count from WS, so we show recent cashouts count)
  const activeCount = entries.length

  return (
    <div className="social-ticker">
      {/* Online / active riders indicator */}
      <div className="social-ticker__count">
        <span className="social-ticker__dot" aria-hidden="true" />
        <span>{activeCount > 0 ? `${activeCount} escaped` : 'Waiting…'}</span>
      </div>

      {/* Cashout feed entries */}
      {entries.length > 0 && (
        <ul className="social-ticker__feed" aria-label="Recent cashouts">
          {entries.slice(0, 4).map((e) => (
            <li key={e.id} className="social-ticker__entry">
              <span className="social-ticker__cowboy">🤠</span>
              <span className="social-ticker__player">{shortPlayer(e.playerId)}</span>
              <span className="social-ticker__escaped">escaped</span>
              <span
                className={`social-ticker__mult ${e.multiplier >= 10 ? 'social-ticker__mult--epic' : ''}`}
              >
                {e.multiplier.toFixed(2)}×
              </span>
            </li>
          ))}
        </ul>
      )}

      {/* Waiting hint */}
      {entries.length === 0 && roundState !== 'RUNNING' && (
        <span className="social-ticker__hint">Join the heist below</span>
      )}
    </div>
  )
}
