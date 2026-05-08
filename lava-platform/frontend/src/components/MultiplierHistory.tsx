import { useGame } from '../store/game'
import type { HistoryEntry } from '../store/game'

function pillVariant(cp: number): string {
  if (cp >= 50) return 'epic'
  if (cp >= 10) return 'huge'
  if (cp >= 5)  return 'high'
  if (cp >= 2)  return 'med'
  return 'low'
}

function Pill({ entry }: { entry: HistoryEntry }) {
  const v = pillVariant(entry.crashPoint)
  return (
    <span
      className={`mh-pill mh-pill--${v}`}
      title={`Round ${entry.roundId.slice(0, 8)}`}
    >
      {entry.crashPoint >= 100
        ? `${entry.crashPoint.toFixed(0)}×`
        : `${entry.crashPoint.toFixed(2)}×`}
    </span>
  )
}

export function MultiplierHistory() {
  const history = useGame((s) => s.history)

  return (
    <div className="mh-row" aria-label="Recent crash points">
      {history.length === 0 ? (
        <span className="mh-empty">No rounds yet</span>
      ) : (
        history.slice(0, 10).map((e) => (
          <Pill key={e.roundId} entry={e} />
        ))
      )}
    </div>
  )
}
