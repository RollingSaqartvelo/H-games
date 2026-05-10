import { useEffect, useState } from 'react'

const WIN_IMG_SRC = '/assets/ui/wins/you_win_combo_full.png'

// Preload at module load time so the image is cached before the user cashes out.
// Without this, the 2MB PNG races against the 1.45s overlay lifetime.
if (typeof window !== 'undefined') {
  const _preload = new window.Image()
  _preload.src = WIN_IMG_SRC
}

interface YouWinOverlayProps {
  panel: 'a' | 'b'
  show: boolean
  amount: number    // actual payout from server
  multiplier: number
}

export function YouWinOverlay({ panel, show, amount, multiplier }: YouWinOverlayProps) {
  const [visible, setVisible] = useState(false)
  const [fading,  setFading]  = useState(false)

  useEffect(() => {
    if (!show) return
    setVisible(true)
    setFading(false)
    const t1 = setTimeout(() => setFading(true), 1200)
    const t2 = setTimeout(() => { setVisible(false); setFading(false) }, 1450)
    return () => { clearTimeout(t1); clearTimeout(t2) }
  }, [show])

  if (!visible) return null

  const amountStr = amount.toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })

  const fontSize = amount >= 1000 ? '10px' : amount >= 100 ? '12px' : '14px'

  return (
    <div className={`ywi ywi--${panel}${fading ? ' ywi--fading' : ''}`}>
      <div className="ywi__inner">
        <img
          className="ywi__img"
          src={WIN_IMG_SRC}
          alt="YOU WIN"
          draggable={false}
        />
        {/* Amount — overlaid over "$_____" field in the wooden sign */}
        <span className="ywi__amount" style={{ fontSize }}>
          ${amountStr}
        </span>
        {/* Multiplier — overlaid over "X__" badge bottom-left */}
        <span className="ywi__mult">
          x{multiplier.toFixed(2)}
        </span>
      </div>
    </div>
  )
}
