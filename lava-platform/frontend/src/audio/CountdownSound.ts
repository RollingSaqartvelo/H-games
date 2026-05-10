// 321.mp3 — pre-heist countdown SFX, plays once at 3s mark during STARTING phase
const SRC = '/audio/321.mp3'

const audio = new Audio(SRC)
audio.preload = 'auto'
audio.loop    = false
audio.volume  = 1.0
// Must be in DOM so App.tsx unlockAllMedia() finds it on first gesture
audio.style.cssText = 'position:fixed;width:1px;height:1px;opacity:0;pointer-events:none;top:-9999px;left:-9999px;'
if (typeof document !== 'undefined') document.body.appendChild(audio)

export function playCountdownSound(): void {
  audio.pause()
  audio.currentTime = 0
  audio.volume = 1.0
  void audio.play().catch(() => {})
}

export function stopCountdownSound(): void {
  audio.pause()
  audio.currentTime = 0
}
