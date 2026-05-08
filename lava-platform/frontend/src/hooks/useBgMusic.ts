import { useEffect } from 'react'
import { useGame }   from '../store/game'
import { startMusic, stopMusic } from '../audio/MusicManager'

/**
 * Subscribes to roundState and drives background music:
 *   RUNNING              → startMusic() (fade in, loop)
 *   CRASHED / FINISHED   → stopMusic()  (fade out, pause)
 *
 * Mounted once in App — no cleanup needed beyond unsubscribing.
 */
export function useBgMusic(): void {
  useEffect(() => {
    // Handle already-RUNNING state on mount (mid-game join)
    if (useGame.getState().roundState === 'RUNNING') {
      startMusic()
    }

    const unsub = useGame.subscribe(
      (s) => s.roundState,
      (state, prev) => {
        if (state === 'RUNNING' && prev !== 'RUNNING') {
          startMusic()
        } else if ((state === 'CRASHED' || state === 'FINISHED') && prev === 'RUNNING') {
          stopMusic()
        }
      },
    )
    return unsub
  }, [])
}
