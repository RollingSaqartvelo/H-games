import { useEffect } from 'react'
import { useGame }   from '../store/game'
import { startMusic, stopMusic } from '../audio/MusicManager'

/**
 * Subscribes to roundState and drives background music:
 *   RUNNING              → startMusic() (fade in, loop)
 *   CRASHED / FINISHED   → stopMusic()  (fade out, pause)
 *
 * `ready` gates everything: while the loading/splash screen is up the music
 * must stay silent, so we don't subscribe (or start) until ready === true.
 */
export function useBgMusic(ready: boolean): void {
  useEffect(() => {
    if (!ready) return

    // Handle already-RUNNING state once the splash is gone (mid-game join)
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
  }, [ready])
}
