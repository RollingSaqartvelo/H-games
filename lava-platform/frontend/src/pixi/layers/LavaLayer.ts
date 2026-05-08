import { Container, Graphics } from 'pixi.js'
import { lerp } from '../utils/easing'

interface Bubble {
  x: number; y: number; r: number; speed: number
  life: number; maxLife: number; popping: boolean
}

interface Spark {
  x: number; y: number; vx: number; vy: number
  life: number; maxLife: number; color: number
}

/**
 * Animated lava at the bottom of the scene.
 *
 * Three sub-layers for depth:
 *   glow      – upward radiance (soft orange fog)
 *   back      – darker second wave, offset phase
 *   front     – bright surface wave
 *   shimmerGfx – subtle heat shimmer lines above surface
 *   bubbleGfx  – rising and popping bubbles on lava surface
 *   sparkGfx   – ejected sparks above lava
 *
 * lavaRise (0..1): 0 = resting level, 1 = fully risen (CRASHED)
 */
export class LavaLayer {
  readonly container: Container

  private glow:       Graphics
  private back:       Graphics
  private front:      Graphics
  private shimmerGfx: Graphics
  private bubbleGfx:  Graphics
  private sparkGfx:   Graphics

  private W = 0
  private H = 0
  private time = 0

  // Smoothly interpolated rise level
  private currentRise = 0
  private targetRise  = 0

  // Bubble pool
  private bubbles: Bubble[] = []
  private bubbleAcc = 0

  // Spark pool
  private sparks: Spark[] = []
  private sparkAcc = 0

  private static readonly SPARK_COLORS = [0xff6600, 0xff9900, 0xffcc00, 0xffffff]

  constructor(w: number, h: number) {
    this.W = w
    this.H = h

    this.container = new Container()

    this.glow       = new Graphics()
    this.back       = new Graphics()
    this.front      = new Graphics()
    this.shimmerGfx = new Graphics()
    this.bubbleGfx  = new Graphics()
    this.sparkGfx   = new Graphics()

    this.container.addChild(
      this.glow,
      this.back,
      this.front,
      this.shimmerGfx,
      this.bubbleGfx,
      this.sparkGfx,
    )

    // Pre-fill pools with dead particles
    for (let i = 0; i < 25; i++) this.bubbles.push(this.deadBubble())
    for (let i = 0; i < 40; i++) this.sparks.push(this.deadSpark())
  }

  private deadBubble(): Bubble {
    return {
      x: 0, y: 0, r: 0, speed: 0,
      life: 0, maxLife: 1, popping: false,
    }
  }

  private deadSpark(): Spark {
    return {
      x: 0, y: 0, vx: 0, vy: 0,
      life: 0, maxLife: 1,
      color: 0xff6600,
    }
  }

  /**
   * dt  – seconds since last frame
   * lavaRise – 0..1 target rise level (driven by multiplier / crash)
   */
  update(dt: number, lavaRise: number): void {
    this.time        += dt
    this.targetRise   = lavaRise
    this.currentRise += (this.targetRise - this.currentRise) * Math.min(1, dt * 2)

    this.draw()

    const surfaceY = this.surfaceY
    this.updateBubbles(dt, lavaRise, surfaceY)
    this.updateSparks(dt, lavaRise, surfaceY)
    this.drawShimmer(lavaRise, surfaceY)
  }

  private draw(): void {
    const { W, H, currentRise: rise, time: t } = this

    // Base Y of the lava surface (rises from 82% to 55% of canvas height)
    const baseY = lerp(H * 0.82, H * 0.55, rise)
    const amp1  = 10 + rise * 14   // wave amplitude grows with rise
    const amp2  = 14 + rise * 18

    // ── Glow (semi-transparent fog rising from lava) ──────────────────────────
    this.glow.clear()
    const glowH = lerp(60, 120, rise)
    for (let row = 0; row < 6; row++) {
      const frac  = row / 6
      const alpha = lerp(0.18, 0, frac)
      const yOff  = frac * glowH
      this.glow
        .rect(0, baseY - glowH + yOff, W, glowH / 6 + 2)
        .fill({ color: 0xff6600, alpha })
    }

    // ── Back wave (darker, slightly lower) ────────────────────────────────────
    this.back.clear()
    const backPts: number[] = [0, H]
    for (let x = 0; x <= W; x += 8) {
      const y =
        baseY + 18 +
        Math.sin((x * 0.018 + t * 0.55) * Math.PI * 2) * amp2
      backPts.push(x, y)
    }
    backPts.push(W, H)
    this.back.poly(backPts).fill({ color: 0xaa1a00 })

    // ── Front wave (bright surface) ───────────────────────────────────────────
    this.front.clear()
    const frontPts: number[] = [0, H]
    for (let x = 0; x <= W; x += 6) {
      const y =
        baseY +
        Math.sin((x * 0.022 + t * 0.8) * Math.PI * 2) * amp1
      frontPts.push(x, y)
    }
    frontPts.push(W, H)
    this.front.poly(frontPts).fill({ color: 0xff4400 })

    // Surface highlight line
    const hlPts: number[] = []
    for (let x = 0; x <= W; x += 6) {
      const y =
        baseY - 3 +
        Math.sin((x * 0.022 + t * 0.8) * Math.PI * 2) * amp1
      hlPts.push(x, y)
    }
    if (hlPts.length >= 4) {
      this.front.poly(hlPts).stroke({ color: 0xff9900, width: 2, alpha: 0.8 })
    }
  }

  private updateBubbles(dt: number, lavaRise: number, surfaceY: number): void {
    const spawnRate = 2 + lavaRise * 5
    this.bubbleAcc += spawnRate * dt

    // Spawn into dead slots
    for (const b of this.bubbles) {
      if (b.life > 0 || this.bubbleAcc < 1) continue
      this.bubbleAcc--
      b.x       = Math.random() * this.W
      b.y       = surfaceY + 2
      b.r       = 1.5 + Math.random() * 2.5
      b.speed   = 15 + Math.random() * 20
      b.maxLife = 1.5 + Math.random() * 1.5
      b.life    = b.maxLife
      b.popping = false
    }

    this.bubbleGfx.clear()
    for (const b of this.bubbles) {
      if (b.life <= 0) continue

      b.life -= dt

      if (!b.popping && b.y < surfaceY - 15 - b.r) {
        b.popping = true
        b.maxLife = 0.12  // short pop duration
        b.life    = b.maxLife
      }

      if (b.popping) {
        const popT = 1 - b.life / b.maxLife
        const dispR = b.r * (1 + popT * 0.5)
        const alpha = b.life / b.maxLife * 0.55
        this.bubbleGfx
          .circle(b.x, b.y, dispR)
          .stroke({ color: 0xff8800, width: 1, alpha })
      } else {
        b.y -= b.speed * dt
        const alpha = Math.min(1, b.life / b.maxLife) * 0.5
        this.bubbleGfx
          .circle(b.x, b.y, b.r)
          .stroke({ color: 0xff6600, width: 1, alpha })
      }
    }
  }

  private updateSparks(dt: number, lavaRise: number, surfaceY: number): void {
    const spawnRate = 4 + lavaRise * 12
    this.sparkAcc += spawnRate * dt

    // Spawn into dead slots
    for (const s of this.sparks) {
      if (s.life > 0 || this.sparkAcc < 1) continue
      this.sparkAcc--
      s.x       = Math.random() * this.W
      s.y       = surfaceY - 2
      s.vx      = (Math.random() - 0.5) * 60
      s.vy      = -(80 + Math.random() * 120)
      s.maxLife = 0.25 + Math.random() * 0.35
      s.life    = s.maxLife
      s.color   = LavaLayer.SPARK_COLORS[Math.floor(Math.random() * LavaLayer.SPARK_COLORS.length)]
    }

    const gravity = 150  // px/s²
    this.sparkGfx.clear()
    for (const s of this.sparks) {
      if (s.life <= 0) continue
      s.x    += s.vx * dt
      s.vy   += gravity * dt
      s.y    += s.vy * dt
      s.life -= dt

      const alpha = Math.min(1, s.life / s.maxLife) * 0.9
      const size  = 1.2 + (1 - s.life / s.maxLife) * 0.5
      this.sparkGfx.circle(s.x, s.y, size).fill({ color: s.color, alpha })
    }
  }

  private drawShimmer(lavaRise: number, surfaceY: number): void {
    this.shimmerGfx.clear()
    const alpha = 0.04 + lavaRise * 0.03
    for (let i = 0; i < 6; i++) {
      const yOff = -(10 + i * 8)
      const phase = this.time * 2.5 + i * 1.1
      const amp   = 4 + i * 1.5
      const pts: number[] = []
      for (let x = 0; x <= this.W; x += 14) {
        pts.push(x, surfaceY + yOff + Math.sin(x * 0.03 + phase) * amp)
      }
      if (pts.length >= 4) {
        this.shimmerGfx.poly(pts).stroke({ color: 0xff8800, width: 1.5, alpha })
      }
    }
  }

  /** The Y coordinate of the lava surface (for collision / character death). */
  get surfaceY(): number {
    return lerp(this.H * 0.82, this.H * 0.55, this.currentRise)
  }

  resize(w: number, h: number): void {
    this.W = w
    this.H = h
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }
}
