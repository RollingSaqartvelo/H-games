import { Container, Graphics } from 'pixi.js'

interface Star {
  x: number
  y: number
  r: number
  phase: number   // twinkling phase offset
  speed: number   // twinkling speed
}

/**
 * Static starfield with per-star alpha twinkling.
 * Redraws only when the canvas resizes.
 */
export class StarField {
  readonly container: Container
  private gfx: Graphics
  private stars: Star[] = []
  private time = 0

  constructor(w: number, h: number) {
    this.container = new Container()
    this.gfx = new Graphics()
    this.container.addChild(this.gfx)
    this.build(w, h)
  }

  private build(w: number, h: number): void {
    this.stars = []
    const count = Math.floor((w * h) / 4000)  // density scales with canvas area
    for (let i = 0; i < count; i++) {
      this.stars.push({
        x: Math.random() * w,
        y: Math.random() * h * 0.85,  // stars only in upper 85%
        r: Math.random() * 1.4 + 0.4,
        phase: Math.random() * Math.PI * 2,
        speed: 0.3 + Math.random() * 0.7,
      })
    }
    this.draw()
  }

  /** Call each frame for twinkling effect. dt is seconds since last frame. */
  update(dt: number): void {
    this.time += dt
    this.draw()
  }

  private draw(): void {
    this.gfx.clear()
    for (const s of this.stars) {
      const alpha = 0.35 + 0.55 * (0.5 + 0.5 * Math.sin(this.time * s.speed + s.phase))
      this.gfx.circle(s.x, s.y, s.r).fill({ color: 0xffffff, alpha })
    }
  }

  resize(w: number, h: number): void {
    this.build(w, h)
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }
}
