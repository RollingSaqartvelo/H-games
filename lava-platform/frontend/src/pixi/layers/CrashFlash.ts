import { Container, Graphics } from 'pixi.js'

/**
 * Full-screen red flash overlay triggered on crash.
 * Flashes bright then fades over ~0.8 s.
 */
export class CrashFlash {
  readonly container: Container

  private gfx: Graphics
  private alpha = 0
  private W = 0
  private H = 0

  constructor(w: number, h: number) {
    this.W = w
    this.H = h
    this.container = new Container()
    this.gfx = new Graphics()
    this.container.addChild(this.gfx)
    this.container.alpha = 0
  }

  /** Trigger the flash animation. */
  trigger(): void {
    this.alpha = 0.75
  }

  update(dt: number): void {
    if (this.alpha <= 0) {
      this.container.alpha = 0
      return
    }

    this.alpha = Math.max(0, this.alpha - dt * 1.2)
    this.container.alpha = this.alpha

    // Redraw only if visible
    if (this.alpha > 0) {
      this.gfx.clear()
      this.gfx.rect(0, 0, this.W, this.H).fill({ color: 0xff0000 })
    }
  }

  resize(w: number, h: number): void {
    this.W = w
    this.H = h
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }
}
