// "wins звук.mp3" — one-shot win sound, preloaded, deduped across panels
const SRC = '/audio/wins%20%D0%B7%D0%B2%D1%83%D0%BA.mp3'

const audio = new Audio(SRC)
audio.preload = 'auto'
audio.loop    = false
audio.volume  = 1.0
// Must be in DOM so App.tsx unlockAllMedia() finds it on first gesture
audio.style.cssText = 'position:fixed;width:1px;height:1px;opacity:0;pointer-events:none;top:-9999px;left:-9999px;'
if (typeof document !== 'undefined') document.body.appendChild(audio)

// Dedup: if two panels escape within 100 ms, play only once
let lastPlayedAt = 0

export function playWinSound(): void {
  const now = Date.now()
  if (now - lastPlayedAt < 100) return
  lastPlayedAt = now

  audio.pause()
  audio.currentTime = 0
  audio.volume = 1.0
  void audio.play().catch(() => {
    // Blocked by browser — retry on next gesture
    const retry = () => {
      audio.currentTime = 0
      void audio.play().catch(() => {})
    }
    document.addEventListener('touchstart', retry, { once: true, passive: true })
    document.addEventListener('click',      retry, { once: true })
  })
}
