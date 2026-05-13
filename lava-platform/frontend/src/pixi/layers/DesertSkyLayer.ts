import { Container, Graphics, Sprite, Assets, Texture } from 'pixi.js'
import { ASSET_PATHS } from '../../game/config'

interface DustMote {
  x: number; y: number; vx: number; vy: number
  life: number; maxLife: number; size: number
}

// Panels scroll at this fraction of charWorldX (parallax — slower than ground)
const PARALLAX = 0.18

/**
 * Scrolling panoramic background built from 8 panel images in order.
 * Panels loop: 1→2→3→4→5→6→7→8→1→2→…
 * Each panel is exactly screen-width wide, scaled to cover full height.
 *
 * Fallback: procedural gradient sky when panels are not loaded.
 */
export class DesertSkyLayer {
  readonly container: Container

  private panels:    Sprite[] = []
  private hasFallback = false

  private sky:       Graphics
  private dustMotes: Graphics

  private W = 0
  private H = 0

  private motes: DustMote[] = []
  private moteAcc = 0

  constructor(w: number, h: number) {
    this.W = w
    this.H = h
    this.container = new Container()

    this.sky       = new Graphics()
    this.dustMotes = new Graphics()
    this.container.addChild(this.sky)

    // Load 8 panel textures in order
    const paths = ASSET_PATHS.bg.panels
    for (let i = 0; i < paths.length; i++) {
      const tex = Assets.get<Texture>(paths[i])
      if (tex) {
        const s = new Sprite(tex)
        s.width  = w
        s.height = h
        this.panels.push(s)
        this.container.addChild(s)
      }
    }

    // Fallback: procedural sky when panels failed to load
    if (this.panels.length === 0) {
      this.hasFallback = true
      this._buildFallback()
    }

    this.container.addChild(this.dustMotes)

    for (let i = 0; i < 50; i++) this.motes.push(this._deadMote(true))
  }

  private _buildFallback(): void {
    const { W, H } = this
    this.sky.clear()
    // Simple warm desert gradient
    this.sky.rect(0, 0, W, H * 0.55).fill({ color: 0x1a0a3a })
    this.sky.rect(0, H * 0.45, W, H * 0.55).fill({ color: 0xb84020 })
  }

  update(dt: number, charWorldX: number, _pursuitIntensity: number): void {
    if (this.panels.length === 0) return

    const N     = this.panels.length
    const strip = this.W * N                            // total panorama width
    const raw   = charWorldX * PARALLAX                 // raw scroll offset
    const off   = ((raw % strip) + strip) % strip       // always positive, loops at strip

    for (let i = 0; i < N; i++) {
      let x = i * this.W - off
      // Wrap: if panel is off to the left, move to the right end of the strip
      if (x < -this.W) x += strip
      this.panels[i].x = x
    }

    this._updateMotes(dt, _pursuitIntensity)
  }

  // ── Dust motes ─────────────────────────────────────────────────────────────

  private _deadMote(init = false): DustMote {
    return {
      x:       init ? Math.random() * this.W : this.W * 0.5,
      y:       init ? Math.random() * this.H * 0.75 : this.H * 0.70,
      vx:      (Math.random() - 0.5) * 14,
      vy:      -(4 + Math.random() * 8),
      life:    init ? Math.random() * 8 : 4 + Math.random() * 5,
      maxLife: 9,
      size:    0.5 + Math.random() * 1.5,
    }
  }

  private _updateMotes(dt: number, intensity: number): void {
    const spawnRate = 0.8 + intensity * 2.5
    this.moteAcc += spawnRate * dt

    for (const m of this.motes) {
      if (m.life <= 0) {
        if (this.moteAcc >= 1) { this.moteAcc--; Object.assign(m, this._deadMote()) }
        continue
      }
      m.x    += m.vx * dt
      m.y    += m.vy * dt
      m.vx   += (Math.random() - 0.5) * 2.5 * dt
      m.life -= dt
    }

    this.dustMotes.clear()
    for (const m of this.motes) {
      if (m.life <= 0) continue
      const a = Math.min(1, m.life / m.maxLife) * 0.18
      const c = intensity > 0.5 ? 0xd4722a : 0xe8aa64
      this.dustMotes.circle(m.x, m.y, m.size).fill({ color: c, alpha: a })
    }
  }

  resize(w: number, h: number): void {
    this.W = w
    this.H = h
    for (const s of this.panels) { s.width = w; s.height = h }
    if (this.hasFallback) this._buildFallback()
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }
}
