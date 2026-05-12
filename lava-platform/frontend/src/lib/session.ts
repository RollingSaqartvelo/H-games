/**
 * Session token resolution order:
 *   1. ?token=<value> URL search param  (set by operator's TMA deep-link)
 *   2. Telegram Mini App start_param    (passed via bot link)
 *   3. localStorage                   (persisted from a previous page load)
 *   4. VITE_DEV_TOKEN env var           (local development convenience)
 */
export function getSessionToken(): string {
  // 1. URL param — operator redirects player here with ?token=xxx
  const params = new URLSearchParams(window.location.search)
  const urlToken = params.get('token')
  if (urlToken) {
    localStorage.setItem('crash_token', urlToken)
    return urlToken
  }

  // 2. Telegram Mini App start_param
  try {
    const tma = (window as Window & typeof globalThis & { Telegram?: { WebApp?: { initDataUnsafe?: { start_param?: string } } } }).Telegram?.WebApp
    const startParam = tma?.initDataUnsafe?.start_param
    if (startParam) {
      localStorage.setItem('crash_token', startParam)
      return startParam
    }
  } catch {
    // Not in Telegram context
  }

  // 3. Persisted token from earlier in the same session
  const stored = localStorage.getItem('crash_token')
  if (stored) return stored

  // 4. Development fallback
  return import.meta.env.VITE_DEV_TOKEN ?? ''
}
