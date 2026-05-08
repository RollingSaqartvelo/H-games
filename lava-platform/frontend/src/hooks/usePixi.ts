import { useEffect, useRef } from 'react'
import { GameEngine } from '../pixi/GameEngine'

/**
 * Mounts a PixiJS GameEngine into a DOM element and tears it down on unmount.
 *
 * Usage:
 *   const mountRef = useRef<HTMLDivElement>(null)
 *   usePixi(mountRef)
 *   return <div ref={mountRef} />
 */
export function usePixi(mountRef: React.RefObject<HTMLDivElement | null>): void {
  // Track the engine instance so we can destroy it on cleanup
  const engineRef = useRef<GameEngine | null>(null)
  // Guard against StrictMode double-invocation
  const destroyedRef = useRef(false)

  useEffect(() => {
    const el = mountRef.current
    if (!el) return

    destroyedRef.current = false

    GameEngine.create(el)
      .then((engine) => {
        if (destroyedRef.current) {
          // Component unmounted before async init completed
          engine.destroy()
          return
        }
        engineRef.current = engine
      })
      .catch((err) => {
        console.error('[PixiJS] GameEngine init failed:', err)
      })

    return () => {
      destroyedRef.current = true
      engineRef.current?.destroy()
      engineRef.current = null
    }
  // mountRef is stable — only run once on mount
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
}
