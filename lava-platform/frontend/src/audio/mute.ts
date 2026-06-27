// Global mute switch — one flag every sound/video element respects.
// Patches the Audio() constructor so each `new Audio()` SFX is tracked and
// muted, and keeps DOM <audio>/<video> elements in sync (incl. ones added later).

const KEY = 'outlaw-muted'
let muted = typeof localStorage !== 'undefined' && localStorage.getItem(KEY) === '1'

const tracked = new Set<HTMLMediaElement>()

function applyAll() {
  tracked.forEach((el) => { el.muted = muted })
  document.querySelectorAll('audio, video').forEach((el) => {
    ;(el as HTMLMediaElement).muted = muted
  })
}

// Patch the Audio constructor — must run before any audio module instantiates.
if (typeof window !== 'undefined' && (window as any).Audio) {
  const Orig = (window as any).Audio
  function PatchedAudio(this: unknown, ...args: unknown[]) {
    const a = new (Orig as any)(...args) as HTMLMediaElement
    tracked.add(a)
    a.muted = muted
    return a
  }
  PatchedAudio.prototype = Orig.prototype
  ;(window as any).Audio = PatchedAudio
}

// Keep media added after a mute toggle silent too (e.g. each round's video).
if (typeof MutationObserver !== 'undefined') {
  new MutationObserver((mutations) => {
    if (!muted) return
    mutations.forEach((m) => {
      m.addedNodes.forEach((n) => {
        if (n instanceof HTMLMediaElement) n.muted = true
        else if (n instanceof HTMLElement) {
          n.querySelectorAll?.('audio, video').forEach((e) => {
            ;(e as HTMLMediaElement).muted = true
          })
        }
      })
    })
  }).observe(document.documentElement, { childList: true, subtree: true })
}

export function isMuted() { return muted }

export function toggleMute() {
  muted = !muted
  try { localStorage.setItem(KEY, muted ? '1' : '0') } catch { /* ignore */ }
  applyAll()
  return muted
}
