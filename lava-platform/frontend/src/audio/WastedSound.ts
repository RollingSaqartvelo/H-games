const SRC = '/audio/wasted.mp3'

const audio = new Audio(SRC)
audio.preload = 'auto'
audio.loop    = false
audio.volume  = 1.0
// Must be in DOM so App.tsx unlockAllMedia() finds it on first gesture
audio.style.cssText = 'position:fixed;width:1px;height:1px;opacity:0;pointer-events:none;top:-9999px;left:-9999px;'
if (typeof document !== 'undefined') document.body.appendChild(audio)

export function playWastedSound(): void {
  audio.pause()
  audio.currentTime = 0
  audio.volume = 1.0

  const p = audio.play()
  if (p) {
    p.catch(() => {
      // Not yet unlocked — try again on next gesture
      const retry = () => {
        audio.currentTime = 0
        void audio.play().catch(() => {})
      }
      document.addEventListener('touchstart', retry, { once: true, passive: true })
      document.addEventListener('click',      retry, { once: true })
    })
  }
}
