import { Container, Graphics, Sprite, Assets, Texture } from 'pixi.js'
import { ASSET_PATHS, HORSE_SCALE, HORSE_HOOF_OFFSET_Y } from '../../game/config'

export type RiderState = 'idle' | 'crouch' | 'airborne' | 'impact' | 'fall' | 'tumble'

// All normal states use idle texture — prevents direction flip when textures
// have different original facing. Only fall/tumble switch texture.
const HERO_TEX_MAP: Record<RiderState, string> = {
  idle:     ASSET_PATHS.hero.idle,
  crouch:   ASSET_PATHS.hero.idle,
  airborne: ASSET_PATHS.hero.jump,
  impact:   ASSET_PATHS.hero.idle,
  tumble:   ASSET_PATHS.hero.fall,   // hero_fall.png during mid-air crash tumble
  fall:     ASSET_PATHS.hero.fall,
}

/**
 * Horse + outlaw rider.
 *
 * Sprite mode (active when PNG assets are in the Assets cache):
 *   horseSprite  — horse PNG, anchor bottom-center
 *   heroSprite   — hero PNG, anchored to sit on horse's back
 *   Container squash/stretch/rotation drives animation — no per-frame redraws.
 *
 * Procedural fallback (when sprites unavailable):
 *   Identical to original Graphics-based implementation.
 *
 * State machine (managed externally by GameScene):
 *   idle / crouch / airborne / impact / fall
 */
export class HorseRider {
  readonly container: Container

  // ── Sprite mode ────────────────────────────────────────────────────────────
  private heroSprite?: Sprite
  private readonly useSprites: boolean

  // ── Procedural fallback ────────────────────────────────────────────────────
  private body:   Graphics
  private legs:   Graphics
  private shadow: Graphics
  private loot:   Graphics
  private lines:  Graphics

  // ── Shared state ───────────────────────────────────────────────────────────
  private state:     RiderState = 'idle'
  private time      = 0
  private phaseTime = 0

  private scaleX            = 1
  private scaleY            = 1
  private rotation          = 0
  private fallSpin          = 0
  private flashOn           = false
  private fallEntryRotation = 0   // rotation captured when entering fall from tumble

  constructor() {
    this.container = new Container()

    // Always create procedural Graphics (needed for fallback, no-op if sprite mode)
    this.shadow = new Graphics()
    this.legs   = new Graphics()
    this.body   = new Graphics()
    this.loot   = new Graphics()
    this.lines  = new Graphics()
    this.container.addChild(this.shadow, this.legs, this.loot, this.body, this.lines)

    const heroTex = Assets.get<Texture>(ASSET_PATHS.hero.idle)
    this.useSprites = !!heroTex
    if (heroTex) {
      // ── Sprite mode: hero only, mirrored to face right ───────────────────
      this.body.visible = this.legs.visible = this.shadow.visible =
        this.loot.visible = this.lines.visible = false

      this.heroSprite = new Sprite(heroTex)
      this.heroSprite.anchor.set(0.5, 1.0)          // bottom-center = foot point
      // Negative x-scale mirrors the sprite horizontally
      this.heroSprite.scale.set(-HORSE_SCALE, HORSE_SCALE)
      this.heroSprite.y = HORSE_HOOF_OFFSET_Y
      this.container.addChild(this.heroSprite)

      console.log('[HorseRider] hero-only sprite mode —', heroTex.width, 'x', heroTex.height)
    } else {
      // ── Procedural fallback ────────────────────────────────────────────────
      this.buildBody()
      console.log('[HorseRider] procedural fallback (sprites not in cache)')
    }
  }

  // ── Public API ─────────────────────────────────────────────────────────────

  setState(state: RiderState): void {
    if (this.state === state) return
    // Capture rotation so fall can decay smoothly from tumble angle
    if (state === 'fall') this.fallEntryRotation = this.container.rotation
    this.state     = state
    this.phaseTime = 0
    this.fallSpin  = 0
    this.flashOn   = false

    if (this.useSprites && this.heroSprite) {
      const etex = Assets.get<Texture>(HERO_TEX_MAP[state])
      if (etex) this.heroSprite.texture = etex
    }

    if (state === 'idle') {
      this.scaleX = 1; this.scaleY = 1; this.rotation = 0
      this.container.rotation = 0
    }
  }

  update(dt: number): void {
    this.time      += dt
    this.phaseTime += dt

    if (this.useSprites) {
      this.updateSpriteMode(dt)
    } else {
      switch (this.state) {
        case 'idle':     this.tickIdle();     break
        case 'crouch':   this.tickCrouch();   break
        case 'airborne': this.tickAirborne(); break
        case 'impact':   this.tickImpact();   break
        case 'fall':     this.tickFall(dt);   break
      }
      this.container.scale.x   = this.scaleX
      this.container.scale.y   = this.scaleY
      this.container.rotation  = this.rotation
      this.drawShadow()
    }
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }

  // ── Sprite-mode animation ─────────────────────────────────────────────────

  private updateSpriteMode(_dt: number): void {
    if (!this.heroSprite) return

    // hero_fall.png faces right; all others face left and need flip
    const scaleX = (this.state === 'fall' || this.state === 'tumble') ? HORSE_SCALE : -HORSE_SCALE
    this.heroSprite.scale.set(scaleX, HORSE_SCALE)

    // Container scale is NEVER touched in sprite mode (would fight hero scale.x).
    // Only rotation and vertical bob are animated.
    switch (this.state) {
      case 'idle': {
        this.heroSprite.y     = HORSE_HOOF_OFFSET_Y + Math.sin(this.time * 2.5) * 1.5
        this.container.rotation = 0
        break
      }
      case 'crouch':
      case 'impact': {
        this.heroSprite.y     = HORSE_HOOF_OFFSET_Y
        this.container.rotation = 0
        break
      }
      case 'airborne': {
        // hero_jump.png — exact reference pose, stable silhouette
        this.heroSprite.y       = HORSE_HOOF_OFFSET_Y
        this.container.rotation = -0.06   // very slight upward lean, cinematic only
        break
      }
      case 'tumble': {
        // hero_fall.png rotating 90° head-down during mid-air crash
        this.heroSprite.y       = HORSE_HOOF_OFFSET_Y
        this.container.rotation = Math.min(this.phaseTime * 6.0, Math.PI / 2)
        break
      }
      case 'fall': {
        this.heroSprite.y = HORSE_HOOF_OFFSET_Y
        if (this.fallEntryRotation > Math.PI / 4) {
          // coming from air tumble — lock at 90° (head pointing down)
          this.container.rotation = Math.PI / 2
        } else {
          // coming from platform crash — slight forward tilt
          const decay = Math.exp(-this.phaseTime * 7)
          this.container.rotation = 0.08 + (this.fallEntryRotation - 0.08) * decay
        }
        break
      }
    }
  }

  // ── Procedural: build static body ─────────────────────────────────────────

  private buildBody(flash = false): void {
    this.body.clear()

    const horseBody  = flash ? 0xff4422 : 0x8b4513
    const horseDark  = flash ? 0xcc2200 : 0x5c2e00
    const horseLight = flash ? 0xff8844 : 0xc4711c

    this.body.roundRect(-28, -22, 56, 18, 8).fill({ color: horseBody })
    this.body.roundRect(-24, -12, 48, 6, 4).fill({ color: horseLight, alpha: 0.25 })
    this.body.poly([14, -20, 22, -20, 26, -38, 18, -38]).fill({ color: horseBody })
    this.body.ellipse(26, -40, 12, 9).fill({ color: horseBody })
    this.body.ellipse(35, -39, 6, 5).fill({ color: horseLight })
    this.body.circle(37, -38, 1.5).fill({ color: horseDark })
    this.body.circle(27, -43, 2.5).fill({ color: horseDark })
    this.body.circle(26.5, -43.5, 0.9).fill({ color: 0xffffff, alpha: 0.8 })
    this.body.poly([22, -48, 18, -44, 25, -44]).fill({ color: horseBody })
    this.body.poly([22, -47, 19, -44, 24, -44]).fill({ color: 0xff9988, alpha: 0.6 })

    for (let i = 0; i < 4; i++) {
      const mx = 18 - i * 3
      this.body.poly([mx, -44, mx - 2, -30, mx + 2, -30]).fill({ color: horseDark, alpha: 0.8 })
    }
    for (let i = 0; i < 3; i++) {
      const ty = -14 - i * 3
      const tx = -28 - i * 2
      this.body.poly([tx, ty - 2, tx - 6, ty + 4, tx - 4, ty + 8, tx - 1, ty + 5])
        .fill({ color: horseDark, alpha: 0.85 - i * 0.15 })
    }

    this.body.roundRect(-4, -26, 16, 6, 3).fill({ color: 0x4a2406 })
    this.body.roundRect(-2, -27, 12, 3, 2).fill({ color: 0x7a3c10, alpha: 0.8 })

    const riderBody  = flash ? 0xff4422 : 0x3d1f06
    const riderShirt = flash ? 0xee3311 : 0x6b3510
    this.body.roundRect(-2, -48, 14, 22, 4).fill({ color: riderBody })
    this.body.roundRect(0, -46, 10, 8, 2).fill({ color: riderShirt, alpha: 0.7 })
    this.body.rect(-2, -30, 14, 3).fill({ color: 0x2a1003 })
    this.body.circle(5, -29, 2).fill({ color: 0xd4a017 })

    const skinColor = flash ? 0xff9955 : 0xe8a87c
    this.body.circle(6, -54, 8).fill({ color: skinColor })
    this.body.roundRect(0, -54, 12, 5, 2).fill({ color: 0xb22222, alpha: 0.85 })
    this.body.circle(9, -56, 1.8).fill({ color: 0x1a0a00 })
    this.body.circle(9, -56, 0.7).fill({ color: 0xffffff, alpha: 0.6 })
    this.body.roundRect(-6, -62, 26, 3, 1.5).fill({ color: 0x3d2200 })
    this.body.roundRect(-1, -74, 16, 13, 4).fill({ color: 0x5c3410 })
    this.body.rect(-1, -63, 16, 2).fill({ color: 0x7a4a20 })
    this.body.roundRect(1, -72, 10, 6, 3).fill({ color: 0x7a4a20, alpha: 0.4 })
    this.body.poly([12, -44, 16, -44, 26, -38, 22, -37]).fill({ color: riderBody })
    this.body.roundRect(22, -40, 8, 3, 1).fill({ color: 0x1a1a1a })
    this.body.circle(24, -38, 3).fill({ color: 0x2a2a2a })
    this.body.circle(24, -38, 2).fill({ color: 0x333333 })
  }

  // ── Procedural: loot ──────────────────────────────────────────────────────

  private drawLoot(bounceY: number): void {
    this.loot.clear()
    this.loot.roundRect(-22, -38 + bounceY, 12, 14, 5).fill({ color: 0xb8860b })
    this.loot.roundRect(-20, -36 + bounceY, 8, 10, 3).fill({ color: 0xdaa520, alpha: 0.6 })
    this.loot.roundRect(-18, -34 + bounceY, 4, 8, 1).fill({ color: 0xffd700, alpha: 0.9 })
    this.loot.roundRect(-19, -31 + bounceY, 6, 1.5, 0).fill({ color: 0xffd700, alpha: 0.9 })
    this.loot.roundRect(-19, -28 + bounceY, 6, 1.5, 0).fill({ color: 0xffd700, alpha: 0.9 })
    this.loot.circle(-16, -38 + bounceY, 2.5).fill({ color: 0x8b6914 })
  }

  // ── Procedural: shadow ────────────────────────────────────────────────────

  private drawShadow(): void {
    this.shadow.clear()
    const squish = this.state === 'impact' ? 2.0 : 1.0
    const alpha  = this.state === 'impact' ? 0.4 : 0.18
    this.shadow.ellipse(0, 2, 28 * squish, 5).fill({ color: 0x000000, alpha })
  }

  // ── Procedural: legs ──────────────────────────────────────────────────────

  private drawLegs(gallop: number): void {
    this.legs.clear()
    const hoofColor = 0x2a1500
    const legColor  = 0x6b3510
    const shinColor = 0x3d1f06
    const phases    = [gallop * 0.8, -gallop * 0.8 + 0.15, -gallop * 0.7 - 0.1, gallop * 0.7 - 0.15]
    const legAttachX = [-18, -10, 10, 18]
    for (let i = 0; i < 4; i++) {
      const ax = legAttachX[i]
      const phase = phases[i]
      const kx = ax + Math.sin(phase) * 10
      const ky = -4 + 14
      const fx = kx + Math.sin(phase * 0.6) * 8
      const fy = ky + 10
      this.legs.poly([ax - 2, -4, ax + 2, -4, kx + 2, ky, kx - 2, ky]).fill({ color: legColor })
      this.legs.poly([kx - 2, ky, kx + 2, ky, fx + 2, fy, fx - 2, fy]).fill({ color: shinColor })
      this.legs.roundRect(fx - 4, fy, 8, 4, 2).fill({ color: hoofColor })
    }
  }

  // ── Procedural: speed lines ────────────────────────────────────────────────

  private drawSpeedLines(intensity: number): void {
    this.lines.clear()
    if (intensity <= 0) return
    const alpha = intensity * 0.4
    for (let i = 0; i < 4; i++) {
      const yOff = -20 + i * 10
      const len  = 18 - i * 3
      const pulse = Math.sin(this.phaseTime * 14 + i * 0.7) * 0.15 + 0.85
      this.lines.rect(-40, yOff - 1, len, 2)
        .fill({ color: 0xd4a017, alpha: alpha * pulse * (1 - i * 0.2) })
    }
  }

  // ── Procedural: tick phases ────────────────────────────────────────────────

  private tickIdle(): void {
    const bob = Math.sin(this.time * 2.5) * 2
    this.drawLegs(Math.sin(this.time * 2.5) * 0.1)
    this.drawLoot(bob * 0.5)
    this.drawSpeedLines(0)
    this.scaleX = 1; this.scaleY = 1; this.rotation = 0
  }

  private tickCrouch(): void {
    const t = Math.min(this.phaseTime / 0.18, 1)
    this.scaleY = 1 - t * 0.22
    this.scaleX = 1 + t * 0.15
    this.rotation = 0
    this.drawLegs(0.3)
    this.drawLoot(4 * t)
    this.drawSpeedLines(0)
  }

  private tickAirborne(): void {
    const gallop = Math.sin(this.phaseTime * 12) * 0.9
    this.rotation = -0.18
    this.scaleX = 1; this.scaleY = 1
    this.drawLegs(gallop)
    this.drawLoot(Math.sin(this.phaseTime * 12) * 3)
    this.drawSpeedLines(1)
  }

  private tickImpact(): void {
    const t = Math.min(this.phaseTime / 0.25, 1)
    const squash = 1 - Math.sin(t * Math.PI) * 0.38
    this.scaleY = Math.max(0.68, squash)
    this.scaleX = Math.min(1.3, 1 + (1 - squash) * 0.7)
    this.rotation = 0
    this.drawLegs(0.25)
    this.drawLoot(6 * (1 - t))
    this.drawSpeedLines(0)
  }

  private tickFall(dt: number): void {
    this.fallSpin += dt * (4 + this.phaseTime * 3)
    this.rotation = this.fallSpin
    this.scaleX = 1; this.scaleY = 1
    this.drawLegs(Math.sin(this.phaseTime * 8) * 0.6)
    this.drawLoot(Math.sin(this.phaseTime * 10) * 5)
    this.drawSpeedLines(0)

    const period  = 0.07
    const nowFlash = Math.floor(this.phaseTime / period) % 2 === 1
    if (nowFlash !== this.flashOn) {
      this.flashOn = nowFlash
      this.buildBody(this.flashOn)
    }
  }
}
