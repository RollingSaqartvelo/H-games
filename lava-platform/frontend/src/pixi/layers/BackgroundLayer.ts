import { Container, Graphics, FillGradient } from 'pixi.js'

interface AshParticle {
  x: number; y: number; vx: number; vy: number
  life: number; maxLife: number; size: number
}

/**
 * Cave atmosphere background — replaces StarField.
 * Layers (bottom→top within this container):
 *   bg        – dark gradient
 *   hotGlow   – orange radiance from below, intensity tracks lavaRise
 *   farRocks  – slow parallax cave silhouette
 *   midRocks  – faster parallax rock outlines
 *   ashGfx    – floating ash particles
 */
export class BackgroundLayer {
  readonly container: Container

  private bg:       Graphics
  private hotGlow:  Graphics
  private farRocks: Graphics
  private midRocks: Graphics
  private ashGfx:   Graphics

  private W = 0
  private H = 0
  private time = 0

  // Parallax scroll offset (driven by charWorldX)
  private farOffX = 0
  private midOffX = 0

  // Ash pool
  private ash: AshParticle[] = []
  private ashAcc = 0

  // Pre-computed rock profiles
  private farProfile: number[] = []
  private midProfile: number[] = []

  constructor(w: number, h: number) {
    this.W = w
    this.H = h
    this.container = new Container()

    this.bg       = new Graphics()
    this.hotGlow  = new Graphics()
    this.farRocks = new Graphics()
    this.midRocks = new Graphics()
    this.ashGfx   = new Graphics()

    this.container.addChild(this.bg, this.hotGlow, this.farRocks, this.midRocks, this.ashGfx)

    // Pre-fill ash pool
    for (let i = 0; i < 60; i++) this.ash.push(this.deadAsh(true))

    this.buildStatic()
  }

  private deadAsh(initialise = false): AshParticle {
    return {
      x: initialise ? Math.random() * this.W : (Math.random() - 0.5) * this.W + this.W / 2,
      y: initialise ? Math.random() * this.H : this.H * 0.85,
      vx: (Math.random() - 0.5) * 12,
      vy: -(8 + Math.random() * 14),
      life: initialise ? Math.random() * 6 : 6 + Math.random() * 4,
      maxLife: 10,
      size: 0.8 + Math.random() * 1.4,
    }
  }

  /** Draw gradient bg + rock profiles (called once and on resize). */
  private buildStatic(): void {
    const { W, H } = this

    // Background gradient (dark volcanic cave)
    this.bg.clear()
    const grad = new FillGradient(0, 0, 0, H)
    grad.addColorStop(0,   0x05080f)
    grad.addColorStop(0.6, 0x090812)
    grad.addColorStop(1,   0x1a0800)
    this.bg.rect(0, 0, W, H).fill(grad)

    // Build rock silhouette profiles (randomised per width)
    this.farProfile = this.buildProfile(W * 3, H, 42, 0.35, 0.55)
    this.midProfile = this.buildProfile(W * 3, H, 73, 0.20, 0.45)
  }

  private buildProfile(totalW: number, H: number, seed: number, minH: number, maxH: number): number[] {
    const pts: number[] = [0, H]
    const steps = Math.ceil(totalW / 28)
    for (let i = 0; i <= steps; i++) {
      const x = (i / steps) * totalW
      const t = this.hash(seed + i * 7.13)
      const y = H * (minH + t * (maxH - minH))
      pts.push(x, y)
    }
    pts.push(totalW, H)
    return pts
  }

  private hash(n: number): number {
    const x = Math.sin(n) * 43758.5453
    return x - Math.floor(x)
  }

  update(dt: number, charWorldX: number, lavaRise: number): void {
    this.time += dt

    // Parallax offsets wrap every 3W
    const RAW_FAR = charWorldX * 0.04
    const RAW_MID = charWorldX * 0.10
    this.farOffX = -(RAW_FAR % (this.W * 3))
    this.midOffX = -(RAW_MID % (this.W * 3))

    // Draw far rocks
    this.farRocks.clear()
    this.farRocks.x = this.farOffX
    this.farRocks.poly(this.farProfile).fill({ color: 0x0d1220, alpha: 0.85 })

    // Draw mid rocks (slightly warmer)
    this.midRocks.clear()
    this.midRocks.x = this.midOffX
    this.midRocks.poly(this.midProfile).fill({ color: 0x0f0e18, alpha: 0.92 })

    // Hot glow from below (orange gradient fading up)
    this.hotGlow.clear()
    const glowH  = this.H * (0.18 + lavaRise * 0.22)
    const alpha  = 0.08 + lavaRise * 0.18
    for (let row = 0; row < 8; row++) {
      const frac = row / 8
      const a    = alpha * (1 - frac)
      const y    = this.H - glowH * (1 - frac)
      this.hotGlow.rect(0, y, this.W, glowH / 8 + 1).fill({ color: 0xff4400, alpha: a })
    }

    // Ash particles
    this.updateAsh(dt, lavaRise)
  }

  private updateAsh(dt: number, lavaRise: number): void {
    const spawnRate = 1.5 + lavaRise * 3
    this.ashAcc += spawnRate * dt

    for (const p of this.ash) {
      if (p.life <= 0) {
        if (this.ashAcc >= 1) { this.ashAcc--; Object.assign(p, this.deadAsh()) }
        continue
      }
      p.x    += p.vx * dt
      p.y    += p.vy * dt
      p.vx   += (Math.random() - 0.5) * 2 * dt
      p.life -= dt
    }

    this.ashGfx.clear()
    for (const p of this.ash) {
      if (p.life <= 0) continue
      const a = Math.min(1, p.life / p.maxLife) * 0.45
      this.ashGfx.circle(p.x, p.y, p.size).fill({ color: 0x888888, alpha: a })
    }
  }

  resize(w: number, h: number): void {
    this.W = w
    this.H = h
    this.buildStatic()
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }
}
