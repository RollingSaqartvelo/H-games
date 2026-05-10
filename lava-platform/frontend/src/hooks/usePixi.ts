import { useEffect, useRef } from 'react'
import { GameEngine } from '../pixi/GameEngine'
import { useGame } from '../store/game'

export function usePixi(mountRef: React.RefObject<HTMLDivElement | null>): void {
  const engineRef   = useRef<GameEngine | null>(null)
  const destroyedRef = useRef(false)

  useEffect(() => {
    const el = mountRef.current
    if (!el) return

    destroyedRef.current = false

    let canvas: HTMLCanvasElement | null = null

    const initEngine = () => {
      GameEngine.create(el)
        .then((engine) => {
          if (destroyedRef.current) { engine.destroy(); return }
          engineRef.current = engine
          canvas = engine.app.canvas as HTMLCanvasElement

          // iOS: WebGL context can be lost when app backgrounds.
          // Reload the page so everything reinitialises cleanly.
          canvas.addEventListener('webglcontextlost', (e) => {
            e.preventDefault()
            console.warn('[PixiJS] WebGL context lost — reloading')
            window.location.reload()
          }, { once: true })
        })
        .catch((err) => {
          console.error('[PixiJS] GameEngine init failed:', err)
          // Retry once after a short delay (handles transient GPU init failures)
          setTimeout(() => {
            if (!destroyedRef.current) initEngine()
          }, 1500)
        })
    }

    initEngine()

    return () => {
      destroyedRef.current = true
      engineRef.current?.destroy()
      engineRef.current = null
      useGame.getState().setPixiReady(false)
    }
  // mountRef is stable — only run once on mount
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
}
