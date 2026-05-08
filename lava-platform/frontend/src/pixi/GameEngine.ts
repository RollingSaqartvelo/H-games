/**
 * GameEngine wraps the PixiJS v8 Application.
 *
 * Responsibilities:
 *   - Init + destroy the Application
 *   - Resize tracking (uses resizeTo)
 *   - Expose the ticker and stage
 *
 * Usage:
 *   const engine = await GameEngine.create(mountDiv)
 *   engine.onResize(w, h => scene.resize(w, h))
 *   engine.destroy()
 */

import { Application } from 'pixi.js'
import { GameScene } from './GameScene'
import { preloadGameAssets } from '../game/AssetLoader'

export class GameEngine {
  readonly app: Application
  readonly scene: GameScene

  private resizeObserver: ResizeObserver

  private constructor(app: Application, scene: GameScene, mount: HTMLElement) {
    this.app = app
    this.scene = scene

    // Observe the parent (game-area) — position:absolute mount has no intrinsic size,
    // so observing it directly can miss size changes driven by flex layout.
    const container = mount.parentElement ?? mount
    this.resizeObserver = new ResizeObserver(() => {
      requestAnimationFrame(() => {
        const w = container.clientWidth
        const h = container.clientHeight
        if (w > 0 && h > 0) {
          this.app.renderer.resize(w, h)
          this.scene.resize(w, h)
          // Re-apply after PixiJS autoDensity overrides width/height with px values
          const canvas = this.app.canvas as HTMLCanvasElement
          canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;display:block;'
        }
      })
    })
    this.resizeObserver.observe(container)
  }

  static async create(mount: HTMLElement): Promise<GameEngine> {
    const app = new Application()

    // Prefer parent (game-area) dimensions — mount is position:absolute
    // and may report 0 before the browser lays out the flex container.
    const container = mount.parentElement ?? mount
    const initW = container.clientWidth  || 390
    const initH = container.clientHeight || 700

    console.log('[PixiJS] game-area dimensions:', initW, 'x', initH)
    console.log('[PixiJS] mount dimensions:', mount.clientWidth, 'x', mount.clientHeight)

    await app.init({
      width:           initW,
      height:          initH,
      backgroundColor: 0x1a0902,
      antialias:       true,
      resolution:      Math.min(window.devicePixelRatio ?? 1, 2),
      autoDensity:     true,
    })

    // Style the canvas
    const canvas = app.canvas as HTMLCanvasElement
    canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;display:block;'

    mount.appendChild(canvas)

    console.log('[PixiJS] canvas size after init:', canvas.width, 'x', canvas.height)

    // Preload PNG sprites — textures land in Assets cache before GameScene reads them
    await preloadGameAssets()

    const scene = new GameScene(app)
    app.stage.addChild(scene.container)

    return new GameEngine(app, scene, mount)
  }

  destroy(): void {
    this.resizeObserver.disconnect()
    this.scene.destroy()
    this.app.destroy({ removeView: true })
  }
}
