import { useGame } from '../store/game'

export function CashoutFeed() {
  const entries = useGame((s) => s.recentCashouts)

  if (entries.length === 0) return null

  return (
    <ul className="cashout-feed" aria-label="Escape feed">
      {entries.map((e) => (
        <li key={e.id} className="cashout-feed__entry">
          <span className="cashout-feed__player">
            {e.playerId.length > 6 ? `…${e.playerId.slice(-5)}` : e.playerId}
          </span>
          <span className="cashout-feed__mult">🤠 {e.multiplier.toFixed(2)}×</span>
        </li>
      ))}
    </ul>
  )
}
