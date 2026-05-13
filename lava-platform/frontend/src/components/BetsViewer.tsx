import { useEffect, useRef, useState, useCallback } from 'react'
import { useGame } from '../store/game'

interface LipEntry {
  pid: string
  amount: number
  cashoutAt: number | null
  payout: number | null
}

interface MyEntry {
  betId: string
  amount: number
  cashoutAt: number | null
  payout: number | null
}

type Tab = 'all' | 'my' | 'top'

const AVATAR_COLORS = [
  '#6b2e00','#4a1a00','#3d1500','#5c2800','#2a1200',
  '#3d2200','#1a0a00','#7a3500','#522000','#3a1800',
]

function avatarColor(pid: string): string {
  let h = 0
  for (let i = 0; i < pid.length; i++) h = (h * 31 + pid.charCodeAt(i)) & 0xffff
  return AVATAR_COLORS[h % AVATAR_COLORS.length]
}

function maskId(pid: string): string {
  return '…' + String(pid).slice(-5)
}

export function BetsViewer({ playerId }: { playerId: string }) {
  const [tab, setTab]   = useState<Tab>('all')
  const [, tick]        = useState(0)
  const rerender        = useCallback(() => tick(n => n + 1), [])

  const currentRound = useRef<Record<string, LipEntry>>({})
  const lastRound    = useRef<LipEntry[]>([])
  const topMap       = useRef<Record<string, { total: number; count: number }>>({})
  const myHistory    = useRef<MyEntry[]>([])
  const prevRoundId  = useRef<string | null>(null)

  // Round reset
  useEffect(() => useGame.subscribe(
    (s) => s.roundId,
    (id) => {
      if (id && id !== prevRoundId.current) {
        prevRoundId.current = id
        currentRound.current = {}
        rerender()
      }
    },
  ), [rerender])

  // New bet placed
  useEffect(() => useGame.subscribe(
    (s) => s.lastBetPlaced,
    (d) => {
      if (!d) return
      if (!currentRound.current[d.player_id]) {
        currentRound.current[d.player_id] = {
          pid: d.player_id, amount: parseFloat(d.amount),
          cashoutAt: null, payout: null,
        }
        rerender()
      }
    },
  ), [rerender])

  // Cashout
  useEffect(() => useGame.subscribe(
    (s) => s.lastCashoutEvent,
    (d) => {
      if (!d) return
      const e = currentRound.current[d.player_id] ??
        { pid: d.player_id, amount: 0, cashoutAt: null, payout: null }
      e.cashoutAt = d.multiplier
      e.payout    = parseFloat(d.payout)
      currentRound.current[d.player_id] = e
      if (!topMap.current[d.player_id]) topMap.current[d.player_id] = { total: 0, count: 0 }
      topMap.current[d.player_id].total += e.payout
      topMap.current[d.player_id].count++
      const my = myHistory.current.find(x => x.betId === d.bet_id)
      if (my) { my.cashoutAt = d.multiplier; my.payout = e.payout }
      rerender()
    },
  ), [rerender])

  // Crash — finalize round
  useEffect(() => useGame.subscribe(
    (s) => s.roundState,
    (state) => {
      if (state !== 'CRASHED') return
      for (const e of Object.values(currentRound.current)) {
        if (e.cashoutAt === null) e.cashoutAt = 0
      }
      lastRound.current = Object.values(currentRound.current).sort((a, b) => {
        const aw = (a.cashoutAt ?? 0) > 0, bw = (b.cashoutAt ?? 0) > 0
        if (aw && !bw) return -1
        if (!aw && bw)  return 1
        return (b.payout ?? 0) - (a.payout ?? 0)
      })
      myHistory.current.forEach(x => { if (x.cashoutAt === null) x.cashoutAt = 0 })
      currentRound.current = {}
      rerender()
    },
  ), [rerender])

  // My own bets
  useEffect(() => useGame.subscribe(
    (s) => s.activeBet,
    (bet) => {
      if (!bet) return
      if (myHistory.current.find(x => x.betId === bet.betId)) return
      myHistory.current.unshift({ betId: bet.betId, amount: parseFloat(bet.amount), cashoutAt: null, payout: null })
      if (myHistory.current.length > 30) myHistory.current.pop()
      rerender()
    },
  ), [rerender])

  // ── Compute display data ────────────────────────────────────────────────────
  const allRows = lastRound.current
  const myRows  = myHistory.current
  const topRows = Object.entries(topMap.current)
    .map(([pid, v]) => ({ pid, ...v }))
    .sort((a, b) => b.total - a.total)
    .slice(0, 15)

  const stats = (() => {
    if (tab === 'all') {
      const winners = allRows.filter(r => (r.cashoutAt ?? 0) > 0)
      const topWin  = winners.length ? Math.max(...winners.map(r => r.payout ?? 0)) : 0
      return {
        bets:    allRows.length   || '—',
        players: new Set(allRows.map(r => r.pid)).size || '—',
        top:     topWin > 0 ? '$' + topWin.toFixed(0) : '—',
      }
    }
    if (tab === 'my') {
      const wins = myRows.filter(r => (r.cashoutAt ?? 0) > 0)
      const topWin = wins.length ? Math.max(...wins.map(r => r.payout ?? 0)) : 0
      return {
        bets:    myRows.length || '—',
        players: wins.length + 'W',
        top:     topWin > 0 ? '$' + topWin.toFixed(2) : '—',
      }
    }
    return {
      bets:    topRows.reduce((s, x) => s + x.count, 0) || '—',
      players: topRows.length || '—',
      top:     topRows.length ? '$' + topRows[0].total.toFixed(0) : '—',
    }
  })()

  const pillLabel = tab === 'all' ? 'LAST ROUND' : tab === 'my' ? 'MY BETS' : 'TOP PLAYERS'

  return (
    <div className="bv-root">
      <div className="bv-header">
        <div className="bv-tabs">
          {(['all', 'my', 'top'] as Tab[]).map(t => (
            <button key={t} className={`bv-tab${tab === t ? ' bv-tab--active' : ''}`} onClick={() => setTab(t)}>
              {t.toUpperCase()}
            </button>
          ))}
        </div>
        <div className="bv-pill">{pillLabel}</div>
      </div>

      <div className="bv-stats">
        <div className="bv-stat">
          <span className="bv-stat-lbl">BETS</span>
          <span className="bv-stat-val">{stats.bets}</span>
        </div>
        <div className="bv-stat">
          <span className="bv-stat-lbl">PLAYERS</span>
          <span className="bv-stat-val">{stats.players}</span>
        </div>
        <div className="bv-stat">
          <span className="bv-stat-lbl">TOP WIN</span>
          <span className="bv-stat-val">{stats.top}</span>
        </div>
      </div>

      <div className="bv-list">
        {tab === 'all' && (
          allRows.length === 0
            ? <div className="bv-empty">WAITING FOR ROUND TO COMPLETE</div>
            : allRows.map((r, i) => {
                const won    = (r.cashoutAt ?? 0) > 0
                const isMe   = r.pid === playerId
                return (
                  <div key={i} className="bv-row">
                    <div className="bv-avatar" style={{ background: isMe ? '#3d1a00' : avatarColor(r.pid) }}>
                      {isMe ? 'ME' : String(r.pid).slice(-1).toUpperCase()}
                    </div>
                    <span className="bv-name">{isMe ? 'YOU' : maskId(r.pid)}</span>
                    <span className="bv-bet">${r.amount.toFixed(2)}</span>
                    <span className={`bv-mult${won ? '' : ' bv-mult--lost'}`}>
                      {won ? (r.cashoutAt ?? 0).toFixed(2) + '×' : '✗'}
                    </span>
                    <span className={`bv-profit${won ? ' bv-profit--win' : ' bv-profit--loss'}`}>
                      {won ? '+$' + (r.payout ?? 0).toFixed(2) : '-$' + r.amount.toFixed(2)}
                    </span>
                  </div>
                )
              })
        )}
        {tab === 'my' && (
          myRows.length === 0
            ? <div className="bv-empty">NO BETS YET</div>
            : myRows.map((r, i) => {
                const pending = r.cashoutAt === null
                const won     = (r.cashoutAt ?? 0) > 0
                return (
                  <div key={i} className="bv-row">
                    <div className="bv-avatar" style={{ background: '#3d1a00' }}>ME</div>
                    <span className="bv-name">MY BET</span>
                    <span className="bv-bet">${r.amount.toFixed(2)}</span>
                    <span className={`bv-mult${won ? '' : (!pending ? ' bv-mult--lost' : '')}`}>
                      {pending ? '…' : won ? (r.cashoutAt ?? 0).toFixed(2) + '×' : '✗'}
                    </span>
                    <span className={`bv-profit${won ? ' bv-profit--win' : (!pending ? ' bv-profit--loss' : ' bv-profit--live')}`}>
                      {pending ? 'LIVE' : won ? '+$' + (r.payout ?? 0).toFixed(2) : '-$' + r.amount.toFixed(2)}
                    </span>
                  </div>
                )
              })
        )}
        {tab === 'top' && (
          topRows.length === 0
            ? <div className="bv-empty">NO DATA YET</div>
            : topRows.map((r, i) => (
                <div key={i} className="bv-row">
                  <div className="bv-rank">#{i + 1}</div>
                  <span className="bv-name">{r.pid === playerId ? 'YOU' : maskId(r.pid)}</span>
                  <span className="bv-bet">{r.count} bet{r.count !== 1 ? 's' : ''}</span>
                  <span className="bv-profit bv-profit--win">+${r.total.toFixed(2)}</span>
                </div>
              ))
        )}
      </div>
    </div>
  )
}
