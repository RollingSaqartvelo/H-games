import { useEffect, useRef, useState } from 'react'
import { useGame } from '../store/game'

const LOADER_MS = 4000

/**
 * Inter-round loader: after the crash sequence (WASTED) finishes, show the
 * H-GAMES logo + a loading bar inside the broadcast screen for 2s, then reveal
 * the betting stage. The bet panels (footer) stay visible underneath.
 */
export function RoundLoader() {
  const crashSequenceDone = useGame((s) => s.crashSequenceDone)
  const roundState        = useGame((s) => s.roundState)
  const [show, setShow]   = useState(false)
  const wasDone           = useRef(false)
  const timer             = useRef<number | undefined>(undefined)

  // Rising edge of crashSequenceDone (crash anim + WASTED done) → show 2s
  useEffect(() => {
    if (crashSequenceDone && !wasDone.current) {
      wasDone.current = true
      setShow(true)
      window.clearTimeout(timer.current)
      timer.current = window.setTimeout(() => setShow(false), LOADER_MS)
    } else if (!crashSequenceDone) {
      wasDone.current = false
    }
  }, [crashSequenceDone])

  // Safety: hide immediately if a new round actually starts running
  useEffect(() => {
    if (roundState === 'RUNNING') {
      window.clearTimeout(timer.current)
      setShow(false)
    }
  }, [roundState])

  // Clear timer on unmount
  useEffect(() => () => window.clearTimeout(timer.current), [])

  if (!show) return null
  return (
    <div className="round-loader" aria-hidden="true">
      <img className="round-loader__logo" src="/assets/hgames-logo.jpg" alt="H-GAMES" />
      <div className="round-loader__bar"><span className="round-loader__fill" /></div>
    </div>
  )
}
