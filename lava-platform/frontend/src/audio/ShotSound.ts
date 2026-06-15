const SRC = '/audio/%D0%BD%D0%BE%D0%B2%D1%8B%D0%B9%20%D0%B7%D0%B2%D1%83%D0%BA%20%D0%B2%D1%8B%D1%81%D1%82%D1%80%D0%B5%D0%BB%D0%B0.mp3'

const audio = new Audio(SRC)
audio.preload = 'auto'
audio.loop    = false
audio.volume  = 0.7
// Must be in DOM so App.tsx unlockAllMedia() finds it on first gesture
// (no 'music' class → unlockAllMedia pauses it after unlock, plays on demand)
audio.style.cssText = 'position:fixed;width:1px;height:1px;opacity:0;pointer-events:none;top:-9999px;left:-9999px;'
if (typeof document !== 'undefined') document.body.appendChild(audio)

/** Fire the gunshot sound — retriggers from the start on every shot. */
// File has ~256ms of leading silence; start 50ms in so the shot lands 50ms earlier.
const LEAD_SKIP = 0.05

export function playShotSound(): void {
  audio.pause()
  audio.currentTime = LEAD_SKIP
  audio.volume = 0.7
  const p = audio.play()
  if (p) p.catch(() => {})
}
