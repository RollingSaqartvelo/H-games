import { Container, Graphics } from 'pixi.js'

/**
 * Full-screen gunfire flash on crash — white/orange burst then fast fade.
 */
export class GunfireCrashFX {
  readonly container: Container

  private gfx:   Graphics
  private alpha  = 0
  private W      = 0
  private H      = 0
  private phase  = 0   // 0=off, 1=white burst, 2=orange fade

  constructor(w: number, h: number) {
    this.W = w
    this.H = h
    this.container = new Container()
    this.gfx = new Graphics()
    this.container.addChild(this.gfx)
    this.container.alpha = 0
  }

  trigger(): void {
    this.alpha = 1.0
    this.phase = 1
  }

  update(dt: number): void {
    if (this.alpha <= 0) {
      this.container.alpha = 0
      return
    }

    if (this.phase === 1) {
      // Fast white burst (0 → 0.3s: goes orange)
      this.alpha -= dt * 2.5
      if (this.alpha <= 0.4) this.phase = 2
    } else {
      // Orange fade out
      this.alpha = Math.max(0, this.alpha - dt * 1.8)
    }

    this.container.alpha = Math.min(0.9, this.alpha)

    this.gfx.clear()
    const color = this.phase === 1 ? 0xffffff : 0xff6600
    this.gfx.rect(0, 0, this.W, this.H).fill({ color })

    // Radial lines emanating from center (gunshot effect)
    if (this.phase === 1 && this.alpha > 0.5) {
      const cx = this.W / 2
      const cy = this.H / 2
      const numLines = 8
      const lineLen  = Math.min(this.W, this.H) * 0.4
      for (let i = 0; i < numLines; i++) {
        const angle = (i / numLines) * Math.PI * 2
        const x2 = cx + Math.cos(angle) * lineLen
        const y2 = cy + Math.sin(angle) * lineLen
        this.gfx
          .poly([cx - 1, cy - 1, cx + 1, cy + 1, x2 + 1, y2 + 1, x2 - 1, y2 - 1])
          .fill({ color: 0xffcc00, alpha: 0.5 })
      }
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
