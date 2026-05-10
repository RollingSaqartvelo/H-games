const MUSIC_SRC  = '/audio/Blues%20Saraceno%20%E2%80%93%20Dogs%20of%20War.mp3'
const TARGET_VOL = 0.35
const FADE_MS    = 800
const TICK_MS    = 50
const FADE_STEPS = FADE_MS / TICK_MS

// Create audio element eagerly so unlock listeners fire as early as possible
const audio = new Audio(MUSIC_SRC)
audio.loop   = true
audio.volume = 0
// 'music' class: App.tsx unlockAllMedia() plays but does NOT pause music elements
audio.className = 'music'
// Must be in DOM so App.tsx unlockAllMedia() finds it on first gesture
audio.style.cssText = 'position:fixed;width:1px;height:1px;opacity:0;pointer-events:none;top:-9999px;left:-9999px;'
if (typeof document !== 'undefined') document.body.appendChild(audio)

let fadeTimer: ReturnType<typeof setInterval> | null = null
let playing = false

// ── Helpers ───────────────────────────────────────────────────────────────────

function clearFade(): void {
  if (fadeTimer !== null) { clearInterval(fadeTimer); fadeTimer = null }
}

function fadeIn(): void {
  clearFade()
  const step = TARGET_VOL / FADE_STEPS
  fadeTimer = setInterval(() => {
    const next = Math.min(TARGET_VOL, audio.volume + step)
    audio.volume = next
    if (next >= TARGET_VOL) clearFade()
  }, TICK_MS)
}

function fadeOut(onDone: () => void): void {
  clearFade()
  if (audio.volume <= 0) { onDone(); return }
  const step = audio.volume / FADE_STEPS
  fadeTimer = setInterval(() => {
    const next = Math.max(0, audio.volume - step)
    audio.volume = next
    if (next <= 0) { clearFade(); onDone() }
  }, TICK_MS)
}

// ── Public API ────────────────────────────────────────────────────────────────

export function startMusic(): void {
  if (playing) return
  playing = true
  clearFade()

  if (!audio.paused) {
    // Already playing silently (stopped without pause) — just fade in
    fadeIn()
    return
  }

  // Paused — try play(); retry on next gesture if blocked
  audio.volume = 0
  audio.play()
    .then(() => { fadeIn() })
    .catch(() => {
      const retry = () => {
        audio.play()
          .then(() => { if (playing) fadeIn() })
          .catch(() => {})
      }
      document.addEventListener('touchstart', retry, { once: true, passive: true })
      document.addEventListener('click',      retry, { once: true })
    })
}

export function stopMusic(): void {
  if (!playing) return
  playing = false
  // Fade to silence but never pause — keeps iOS audio context unlocked
  // so next startMusic() can fadeIn() without needing a new gesture
  fadeOut(() => {})
}
