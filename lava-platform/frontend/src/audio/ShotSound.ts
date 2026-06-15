const SRC = '/audio/%D0%B2%D1%8B%D1%81%D1%82%D1%80%D0%B5%D0%BB%20%D0%B7%D0%B2%D1%83%D0%BA.mp3'

const audio = new Audio(SRC)
audio.preload = 'auto'
audio.loop    = false
audio.volume  = 0.8
// Must be in DOM so App.tsx unlockAllMedia() finds it on first gesture
// (no 'music' class → unlockAllMedia pauses it after unlock, plays on demand)
audio.style.cssText = 'position:fixed;width:1px;height:1px;opacity:0;pointer-events:none;top:-9999px;left:-9999px;'
if (typeof document !== 'undefined') document.body.appendChild(audio)

/** Fire the gunshot sound — retriggers from the start on every shot. */
export function playShotSound(): void {
  audio.pause()
  audio.currentTime = 0
  audio.volume = 0.8
  const p = audio.play()
  if (p) p.catch(() => {})
}
