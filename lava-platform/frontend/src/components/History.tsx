import { useGame } from '../store/game'
import type { HistoryEntry } from '../store/game'

/** Color-code crash points to give players a quick visual pattern. */
function pointClass(cp: number): string {
  if (cp < 1.5)  return 'history__entry--danger'
  if (cp < 2.0)  return 'history__entry--warning'
  if (cp < 5.0)  return 'history__entry--ok'
  if (cp < 10.0) return 'history__entry--good'
  return 'history__entry--epic'
}

function HistoryBadge({ entry }: { entry: HistoryEntry }) {
  return (
    <div
      className={`history__entry ${pointClass(entry.crashPoint)}`}
      title={`Round ${entry.roundId}`}
    >
      {entry.crashPoint.toFixed(2)}×
    </div>
  )
}

export function History() {
  const history = useGame((s) => s.history)

  if (history.length === 0) {
    return (
      <div className="history history--empty">
        <span className="history__empty-text">No rounds yet</span>
      </div>
    )
  }

  return (
    <div className="history" aria-label="Recent crash points">
      {history.map((entry) => (
        <HistoryBadge key={entry.roundId} entry={entry} />
      ))}
    </div>
  )
}
