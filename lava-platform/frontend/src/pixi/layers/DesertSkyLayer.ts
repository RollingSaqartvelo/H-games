import { Container, Graphics, FillGradient, Sprite, Assets, Texture } from 'pixi.js'
import { ASSET_PATHS } from '../../game/config'

interface DustMote {
  x: number; y: number; vx: number; vy: number
  life: number; maxLife: number; size: number
}

/**
 * Cinematic desert sunset sky.
 *
 * When bgSprite is loaded, the PNG handles all sky visuals — no procedural
 * sun disc, no bloom bands, no red overlay. Only subtle atmospheric haze
 * and dust motes are added on top at very low alpha.
 *
 * Fallback (no PNG): procedural gradient + single soft sun.
 *
 * Ground fill covers H*0.76 → H to eliminate black void below terrain.
 */
export class DesertSkyLayer {
  readonly container: Container

  private bgSprite?: Sprite
  private sky:       Graphics
  private sunGlow:   Graphics
  private farHaze:   Graphics
  private dustMotes: Graphics
  private ground:    Graphics

  private W    = 0
  private H    = 0
  private time = 0

  private hazeOffX   = 0
  private hazeProfile: number[] = []

  private motes: DustMote[] = []
  private moteAcc = 0

  constructor(w: number, h: number) {
    this.W = w
    this.H = h
    this.container = new Container()

    const bgTex = Assets.get<Texture>(ASSET_PATHS.bg.sunset)
    if (bgTex) {
      this.bgSprite = new Sprite(bgTex)
      this.bgSprite.width  = w
      this.bgSprite.height = h
      this.container.addChild(this.bgSprite)
    }

    this.sky       = new Graphics()
    this.sunGlow   = new Graphics()
    this.farHaze   = new Graphics()
    this.dustMotes = new Graphics()
    this.ground    = new Graphics()

    this.container.addChild(this.sky, this.sunGlow, this.farHaze, this.dustMotes, this.ground)

    for (let i = 0; i < 60; i++) this.motes.push(this.deadMote(true))
    this.buildStatic()
  }

  // ── Static build ───────────────────────────────────────────────────────────

  private buildStatic(): void {
    const { W, H } = this
    this.sky.clear()

    if (this.bgSprite) {
      // PNG background handles all sky visuals — no overlays needed.
      // Only build the haze profile for subtle atmospheric parallax.
      this.hazeProfile = this.buildProfile(W * 3, H, 17, 0.55, 0.72)
      return
    }

    // ── Fallback gradient sky ─────────────────────────────────────────────────
    const grad = new FillGradient(0, 0, 0, H)
    grad.addColorStop(0,    0x080d1e)
    grad.addColorStop(0.28, 0x1a1050)
    grad.addColorStop(0.52, 0x6b200a)
    grad.addColorStop(0.72, 0xcc4a08)
    grad.addColorStop(0.86, 0xe8700a)
    grad.addColorStop(1,    0x2d1005)
    this.sky.rect(0, 0, W, H).fill(grad)

    // ── Natural sun — soft disc, not concentric target rings ─────────────────
    // Sun is placed high in the sky at natural horizon position.
    const sx = W * 0.62
    const sy = H * 0.52   // upper third — not centered on gameplay area
    const sr = Math.min(W, H) * 0.085

    // Very soft outer diffusion (2 rings only, very low alpha)
    this.sky.circle(sx, sy, sr * 2.8).fill({ color: 0xff7700, alpha: 0.04 })
    this.sky.circle(sx, sy, sr * 1.6).fill({ color: 0xff9933, alpha: 0.08 })
    // Sun disc — clean, not layered into concentric targets
    this.sky.circle(sx, sy, sr).fill({ color: 0xff6600, alpha: 0.88 })
    this.sky.circle(sx, sy, sr * 0.62).fill({ color: 0xffaa22, alpha: 0.92 })
    this.sky.circle(sx, sy, sr * 0.28).fill({ color: 0xfff0bb, alpha: 1.0 })

    this.hazeProfile = this.buildProfile(W * 3, H, 17, 0.55, 0.72)
  }

  // ── Update ─────────────────────────────────────────────────────────────────

  update(dt: number, charWorldX: number, pursuitIntensity: number): void {
    this.time += dt

    // ── Atmospheric haze — fallback mode only ────────────────────────────────
    // When bgSprite is loaded the PNG provides its own atmosphere; any overlay
    // on top creates the visible horizontal banding the user sees.
    this.farHaze.clear()
    if (!this.bgSprite) {
      this.hazeOffX = -(charWorldX * 0.015 % (this.W * 3))
      this.farHaze.x = this.hazeOffX
      this.farHaze.poly(this.hazeProfile).fill({ color: 0x6b2808, alpha: 0.07 + pursuitIntensity * 0.09 })
    }

    // ── Horizon glow — fallback mode only ────────────────────────────────────
    this.sunGlow.clear()
    if (!this.bgSprite) {
      const glowH = this.H * (0.06 + pursuitIntensity * 0.10)
      const a     = 0.03 + pursuitIntensity * 0.08
      const col   = pursuitIntensity > 0.6 ? 0xff2200 : 0xff7700
      this.sunGlow.rect(0, this.H * 0.66 - glowH * 0.6, this.W, glowH)
        .fill({ color: col, alpha: a })
    }

    // ── Ground fill — fallback mode only ─────────────────────────────────────
    // With bgSprite: draw nothing — PNG extends cleanly to the panel border.
    this.ground.clear()
    if (!this.bgSprite) {
      const rimY   = this.H * 0.76
      const rimCol = pursuitIntensity > 0.5 ? 0x1e0a02 : 0x2e1205
      this.ground.rect(0, rimY, this.W, this.H - rimY).fill({ color: rimCol })
      const edgeA = 0.25 + pursuitIntensity * 0.15
      this.ground.rect(0, rimY, this.W, 3).fill({ color: 0x7a3010, alpha: edgeA })
    }

    this.updateMotes(dt, pursuitIntensity)
  }

  // ── Dust motes ─────────────────────────────────────────────────────────────

  private deadMote(init = false): DustMote {
    return {
      x:       init ? Math.random() * this.W : (Math.random() - 0.5) * this.W + this.W / 2,
      y:       init ? Math.random() * this.H * 0.80 : this.H * 0.70,
      vx:      (Math.random() - 0.5) * 16,
      vy:      -(5 + Math.random() * 9),
      life:    init ? Math.random() * 8 : 4 + Math.random() * 5,
      maxLife: 9,
      size:    0.5 + Math.random() * 1.6,
    }
  }

  private updateMotes(dt: number, intensity: number): void {
    const spawnRate = 1.0 + intensity * 3
    this.moteAcc += spawnRate * dt

    for (const m of this.motes) {
      if (m.life <= 0) {
        if (this.moteAcc >= 1) { this.moteAcc--; Object.assign(m, this.deadMote()) }
        continue
      }
      m.x    += m.vx * dt
      m.y    += m.vy * dt
      m.vx   += (Math.random() - 0.5) * 3 * dt
      m.life -= dt
    }

    this.dustMotes.clear()
    for (const m of this.motes) {
      if (m.life <= 0) continue
      const a = Math.min(1, m.life / m.maxLife) * (this.bgSprite ? 0.20 : 0.30)
      const c = intensity > 0.5 ? 0xd4722a : 0xe8aa64
      this.dustMotes.circle(m.x, m.y, m.size).fill({ color: c, alpha: a })
    }
  }

  // ── Helpers ────────────────────────────────────────────────────────────────

  private buildProfile(totalW: number, H: number, seed: number, minF: number, maxF: number): number[] {
    const pts: number[] = [0, H]
    const steps = Math.ceil(totalW / 28)
    for (let i = 0; i <= steps; i++) {
      const x    = (i / steps) * totalW
      const t    = this.hash(seed + i * 9.17)
      const flat = this.hash(seed + i * 3.14)
      const raw  = t
      const h    = flat > 0.55 ? Math.min(raw, this.hash(seed + (i - 1) * 9.17) + 0.03) : raw
      pts.push(x, H * (minF + h * (maxF - minF)))
    }
    pts.push(totalW, H)
    return pts
  }

  private hash(n: number): number {
    const x = Math.sin(n) * 43758.5453
    return x - Math.floor(x)
  }

  resize(w: number, h: number): void {
    this.W = w
    this.H = h
    if (this.bgSprite) {
      this.bgSprite.width  = w
      this.bgSprite.height = h
    }
    this.buildStatic()
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }
}
