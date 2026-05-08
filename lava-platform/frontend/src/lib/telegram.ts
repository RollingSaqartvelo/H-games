/**
 * Safe accessor for the Telegram Mini App SDK.
 * Returns null when the page is opened outside Telegram.
 */

interface TelegramWebApp {
  ready(): void
  expand(): void
  close(): void
  colorScheme: 'light' | 'dark'
  themeParams: {
    bg_color?: string
    text_color?: string
    hint_color?: string
    button_color?: string
    button_text_color?: string
    secondary_bg_color?: string
  }
  initDataUnsafe: {
    user?: {
      id: number
      first_name: string
      username?: string
    }
    start_param?: string
  }
  HapticFeedback: {
    impactOccurred(style: 'light' | 'medium' | 'heavy' | 'rigid' | 'soft'): void
    notificationOccurred(type: 'error' | 'success' | 'warning'): void
  }
}

function getTMA(): TelegramWebApp | null {
  try {
    const tma = (window as unknown as { Telegram?: { WebApp?: TelegramWebApp } }).Telegram?.WebApp
    return tma ?? null
  } catch {
    return null
  }
}

export const tma = getTMA()

/** Call once at app start to signal TMA that the app is ready. */
export function initTMA(): void {
  if (!tma) return
  tma.ready()
  tma.expand()
}

export function haptic(type: 'tap' | 'success' | 'error'): void {
  if (!tma) return
  if (type === 'tap') tma.HapticFeedback.impactOccurred('light')
  else tma.HapticFeedback.notificationOccurred(type === 'success' ? 'success' : 'error')
}
