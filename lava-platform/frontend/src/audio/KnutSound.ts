const SRC = '/audio/Knut.mp3'

const audio = new Audio(SRC)
audio.preload = 'auto'
audio.loop    = false
audio.volume  = 1.0

function onFirstGesture() {
  const p = audio.play()
  if (p) p.then(() => { audio.pause(); audio.currentTime = 0 }).catch(() => {})
}
document.addEventListener('touchstart', onFirstGesture, { once: true, passive: true })
document.addEventListener('click',      onFirstGesture, { once: true })

// One per crash — dedup across dual-bet panels
let lastPlayedAt = 0

export function playKnutSound(): void {
  const now = Date.now()
  if (now - lastPlayedAt < 300) return
  lastPlayedAt = now

  audio.pause()
  audio.currentTime = 0
  audio.volume = 1.0
  void audio.play().catch(() => {
    const retry = () => {
      audio.currentTime = 0
      void audio.play().catch(() => {})
    }
    document.addEventListener('touchstart', retry, { once: true, passive: true })
    document.addEventListener('click',      retry, { once: true })
  })
}
