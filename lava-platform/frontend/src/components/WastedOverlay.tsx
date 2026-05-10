import { useEffect, useState } from 'react'
import { useGame } from '../store/game'
import { playWastedSound } from '../audio/WastedSound'

const WASTED_IMG_SRC = '/assets/ui/Wasted/wasted.png'

// Preload at module load — 8MB PNG must be in cache before the 1.3s crash delay.
if (typeof window !== 'undefined') {
  const _preload = new window.Image()
  _preload.src = WASTED_IMG_SRC
}

type Phase = 'hidden' | 'active' | 'fading'

export function WastedOverlay() {
  const roundState = useGame((s) => s.roundState)
  const [phase, setPhase] = useState<Phase>('hidden')

  useEffect(() => {
    if (roundState !== 'CRASHED') {
      setPhase('hidden')
      return
    }
    // 1300ms after crash — newspaper launches + wasted sound fires in sync
    const t1 = setTimeout(() => { setPhase('active'); playWastedSound() }, 1300)
    // hold after fly-in (750ms anim + 1400ms hold)
    const t2 = setTimeout(() => setPhase('fading'),  3450)
    // gone
    const t3 = setTimeout(() => setPhase('hidden'),  3750)
    return () => { clearTimeout(t1); clearTimeout(t2); clearTimeout(t3) }
  }, [roundState])

  if (phase === 'hidden') return null

  return (
    <div className={`wo${phase === 'fading' ? ' wo--fading' : ''}`}>
      <img
        className="wo__img"
        src={WASTED_IMG_SRC}
        alt="WASTED"
        draggable={false}
      />
    </div>
  )
}
