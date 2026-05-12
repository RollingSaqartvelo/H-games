/**
 * OutlawRunner — the player's escaped outlaw running through the western town.
 *
 * Uses hero PNG sprites when available, falls back to procedural graphics.
 * States:
 *   idle    — standing, light sway (before round starts)
 *   running — forward sprint, arm swing, dust kick, coat flapping
 *   fall    — captured / shot, tumbling forward with coat spread
 */

import { Container, Graphics, Sprite, Assets, Texture } from 'pixi.js'
import { ASSET_PATHS, HORSE_SCALE, HORSE_HOOF_OFFSET_Y } from '../../game/config'

export type RiderState = 'idle' | 'running' | 'fall' |
  // Legacy states (kept for type-safety, mapped to nearest equivalent)
  'crouch' | 'airborne' | 'impact' | 'tumble'

const HERO_TEX_MAP: Record<RiderState, string> = {
  idle:     ASSET_PATHS.hero.idle,
  running:  ASSET_PATHS.hero.idle,
  crouch:   ASSET_PATHS.hero.idle,
  airborne: ASSET_PATHS.hero.idle,
  impact:   ASSET_PATHS.hero.idle,
  tumble:   ASSET_PATHS.hero.fall,
  fall:     ASSET_PATHS.hero.fall,
}

export class HorseRider {
  readonly container: Container

  private heroSprite?: Sprite
  private readonly useSprites: boolean

  // Procedural graphics layers
  private shadow: Graphics
  private body:   Graphics
  private coat:   Graphics
  private arms:   Graphics
  private legs:   Graphics
  private dust:   Graphics

  private state:     RiderState = 'idle'
  private time       = 0
  private phaseTime  = 0
  private fallSpin   = 0
  private flashOn    = false
  private fallEntryRot = 0

  constructor() {
    this.container = new Container()

    this.shadow = new Graphics()
    this.body   = new Graphics()
    this.coat   = new Graphics()
    this.arms   = new Graphics()
    this.legs   = new Graphics()
    this.dust   = new Graphics()
    this.container.addChild(this.shadow, this.dust, this.legs, this.coat, this.body, this.arms)

    const heroTex = Assets.get<Texture>(ASSET_PATHS.hero.idle)
    this.useSprites = !!heroTex

    if (heroTex) {
      this.body.visible  = false
      this.coat.visible  = false
      this.arms.visible  = false
      this.legs.visible  = false
      this.dust.visible  = false
      this.shadow.visible = false

      this.heroSprite = new Sprite(heroTex)
      this.heroSprite.anchor.set(0.5, 1.0)
      this.heroSprite.scale.set(-HORSE_SCALE, HORSE_SCALE)
      this.heroSprite.y = HORSE_HOOF_OFFSET_Y
      this.container.addChild(this.heroSprite)
    } else {
      this.buildProceduralOutlaw()
    }
  }

  setState(state: RiderState): void {
    if (this.state === state) return
    if (state === 'fall') this.fallEntryRot = this.container.rotation
    this.state     = state
    this.phaseTime = 0
    this.fallSpin  = 0
    this.flashOn   = false

    if (this.useSprites && this.heroSprite) {
      const tex = Assets.get<Texture>(HERO_TEX_MAP[state])
      if (tex) this.heroSprite.texture = tex
    }

    if (state === 'idle' || state === 'running') {
      this.container.rotation = 0
    }
  }

  update(dt: number): void {
    this.time      += dt
    this.phaseTime += dt

    if (this.useSprites) {
      this.updateSprite(dt)
    } else {
      this.updateProcedural(dt)
    }
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }

  // ── Sprite-mode animation ─────────────────────────────────────────────────

  private updateSprite(_dt: number): void {
    if (!this.heroSprite) return

    switch (this.state) {
      case 'idle': {
        // Gentle idle sway
        this.heroSprite.scale.set(-HORSE_SCALE, HORSE_SCALE)
        this.heroSprite.y = HORSE_HOOF_OFFSET_Y + Math.sin(this.time * 1.8) * 1.5
        this.container.rotation = 0
        break
      }
      case 'running':
      case 'crouch':
      case 'impact':
      case 'airborne': {
        // Running: fast vertical bob, slight forward lean
        this.heroSprite.scale.set(-HORSE_SCALE, HORSE_SCALE)
        const bob = Math.sin(this.time * 9.5) * 3.5
        this.heroSprite.y = HORSE_HOOF_OFFSET_Y + bob
        // Lean forward based on bob cycle (feels like sprinting)
        this.container.rotation = -0.10 + Math.sin(this.time * 9.5) * 0.04
        break
      }
      case 'tumble': {
        this.heroSprite.scale.set(HORSE_SCALE, HORSE_SCALE)
        this.heroSprite.y = HORSE_HOOF_OFFSET_Y
        this.container.rotation = Math.min(this.phaseTime * 5.5, Math.PI / 2)
        break
      }
      case 'fall': {
        this.heroSprite.scale.set(HORSE_SCALE, HORSE_SCALE)
        this.heroSprite.y = HORSE_HOOF_OFFSET_Y
        if (this.fallEntryRot > Math.PI / 4) {
          this.container.rotation = Math.PI / 2
        } else {
          // Forward tumble on capture — rotate forward
          const target = Math.PI * 0.45
          this.container.rotation = lerp(this.fallEntryRot, target,
            Math.min(this.phaseTime * 2.5, 1))
        }
        break
      }
    }
  }

  // ── Procedural outlaw drawing ─────────────────────────────────────────────

  private buildProceduralOutlaw(flash = false): void {
    this.body.clear()

    const skin  = flash ? 0xff9966 : 0xe8a87c
    const coat  = flash ? 0xff4422 : 0x3d1f06
    const shirt = flash ? 0xff6633 : 0x6b3510
    const hat   = flash ? 0xff6644 : 0x5c3410
    const pants = flash ? 0xcc3322 : 0x2a1003
    const boot  = flash ? 0xaa2200 : 0x1a0a00
    const gold  = 0xd4a017

    // Shadow
    this.shadow.clear()
    this.shadow.ellipse(0, 4, 16, 4).fill({ color: 0x000000, alpha: 0.22 })

    // Boots
    this.body.roundRect(-7, -10, 8, 10, 2).fill({ color: boot })
    this.body.roundRect(1,  -10, 8, 10, 2).fill({ color: boot })
    // Boot tips angle forward (running direction)
    this.body.roundRect(-9, -2, 6, 4, 2).fill({ color: boot })
    this.body.roundRect(3, -2, 6, 4, 2).fill({ color: boot })

    // Pants
    this.body.roundRect(-6, -32, 7, 24, 3).fill({ color: pants })
    this.body.roundRect(1,  -32, 7, 24, 3).fill({ color: pants })
    // Belt with buckle
    this.body.rect(-7, -33, 16, 3).fill({ color: 0x2a1003 })
    this.body.rect(-2, -35, 6, 4).fill({ color: gold })

    // Torso / shirt
    this.body.roundRect(-7, -56, 16, 26, 4).fill({ color: shirt })
    // Vest overlay
    this.body.roundRect(-5, -54, 4, 20, 2).fill({ color: coat, alpha: 0.8 })
    this.body.roundRect(3,  -54, 4, 20, 2).fill({ color: coat, alpha: 0.8 })

    // Neck & head
    this.body.roundRect(-3, -64, 8, 10, 3).fill({ color: skin })
    this.body.circle(1, -70, 9).fill({ color: skin })
    // Stubble / face detail
    this.body.circle(-2, -68, 1.5).fill({ color: 0x8b6050, alpha: 0.6 })
    this.body.circle(4,  -68, 1.5).fill({ color: 0x8b6050, alpha: 0.6 })

    // Hat brim
    this.body.rect(-12, -78, 26, 3).fill({ color: hat })
    // Hat crown
    this.body.roundRect(-6, -96, 16, 20, 4).fill({ color: hat })
    this.body.roundRect(-4, -94, 12, 4, 2).fill({ color: 0x7a4a20, alpha: 0.4 })
    // Hat band
    this.body.rect(-6, -78, 16, 3).fill({ color: 0xd4a017, alpha: 0.85 })
  }

  private updateProcedural(dt: number): void {
    switch (this.state) {
      case 'idle':    this.tickProceduralIdle();          break
      case 'running':
      case 'crouch':
      case 'impact':
      case 'airborne': this.tickProceduralRunning();      break
      case 'tumble':
      case 'fall':    this.tickProceduralFall(dt);        break
    }
  }

  private tickProceduralIdle(): void {
    this.container.rotation = 0
    const bob = Math.sin(this.time * 2.0) * 1.5
    this.container.y = bob
    this.drawCoat(0, 0)
    this.drawArms(0, 0)
    this.drawLegs(0, false)
  }

  private tickProceduralRunning(): void {
    // Running bob
    const cycle = this.time * 9.5
    const bob   = Math.sin(cycle) * 3.5
    const lean  = -0.12 + Math.sin(cycle) * 0.035
    this.container.y = bob
    this.container.rotation = lean

    // Arms pump front-back
    const armSwing = Math.sin(cycle) * 0.55
    this.drawArms(armSwing, -armSwing)

    // Coat flapping behind
    const flapT = Math.sin(cycle * 0.5) * 0.4
    this.drawCoat(flapT, -bob * 0.4)

    // Legs stride
    this.drawLegs(cycle, true)

    // Foot dust at rear boot contact
    this.drawFootDust(cycle)
  }

  private tickProceduralFall(dt: number): void {
    this.fallSpin += dt * (3.5 + this.phaseTime * 2.8)
    this.container.rotation = this.fallSpin
    this.container.y = 0
    this.drawCoat(0.6, 0)
    this.drawArms(0.8, -0.8)
    this.drawLegs(this.time * 6, false)

    // Flash effect
    const period = 0.07
    const nowFlash = Math.floor(this.phaseTime / period) % 2 === 1
    if (nowFlash !== this.flashOn) {
      this.flashOn = nowFlash
      this.buildProceduralOutlaw(this.flashOn)
    }
  }

  // Coat: long western duster coat that flaps behind
  private drawCoat(flapAngle: number, offsetY: number): void {
    this.coat.clear()
    // Back panel of coat (flaps behind as outlaw runs)
    const coatColor = 0x3d1f06
    const flap = Math.sin(flapAngle) * 12
    // Main coat body
    this.coat.poly([
      -8, -55 + offsetY,
      8,  -55 + offsetY,
      12 + flap, -20 + offsetY,
      -12 - flap, -20 + offsetY,
    ]).fill({ color: coatColor, alpha: 0.9 })
    // Coat tail (longer flap)
    this.coat.poly([
      0, -30 + offsetY,
      10 + flap * 1.5, -10 + offsetY,
      -10 - flap * 1.5, -10 + offsetY,
    ]).fill({ color: 0x2a1003, alpha: 0.7 })
  }

  // Arms: pump front-back during running
  private drawArms(frontAngle: number, backAngle: number): void {
    this.arms.clear()
    const skin   = 0xe8a87c
    const sleeve = 0x6b3510

    // Back arm (left in screen space, swings forward when running)
    const bax = -8 + Math.sin(backAngle) * 14
    const bay = -46 + Math.abs(Math.cos(backAngle)) * 6
    this.arms.roundRect(-10, -56, 6, 20, 3).fill({ color: sleeve })
    this.arms.roundRect(bax - 3, bay, 6, 8, 2).fill({ color: skin })

    // Front arm (right, swings back)
    const fax = 6 + Math.sin(frontAngle) * 14
    const fay = -46 + Math.abs(Math.cos(frontAngle)) * 6
    this.arms.roundRect(5, -56, 6, 20, 3).fill({ color: sleeve })
    this.arms.roundRect(fax - 3, fay, 6, 8, 2).fill({ color: skin })

    // Revolver in front hand (always visible)
    this.arms.roundRect(fax + 2, fay + 2, 3, 9, 1).fill({ color: 0x2a2a2a })
    this.arms.roundRect(fax, fay + 4, 3, 4, 1).fill({ color: 0x1a1a1a })
  }

  // Legs: stride animation
  private drawLegs(cycle: number, running: boolean): void {
    this.legs.clear()
    if (!running) {
      // Static idle legs
      this.legs.roundRect(-7, -32, 7, 24, 3).fill({ color: 0x2a1003 })
      this.legs.roundRect(1,  -32, 7, 24, 3).fill({ color: 0x2a1003 })
      return
    }
    const pants = 0x2a1003
    // Front leg
    const frontKick = Math.sin(cycle) * 22
    this.legs.poly([
      -7, -32, 1, -32,
      1 + frontKick * 0.4, -18,
      -6 + frontKick * 0.3, -18,
    ]).fill({ color: pants })
    this.legs.roundRect(
      -8 + frontKick * 0.5, -18,
      8, 12, 2,
    ).fill({ color: pants })

    // Back leg
    const backKick = -Math.sin(cycle) * 22
    this.legs.poly([
      1, -32, 9, -32,
      9 + backKick * 0.3, -18,
      0 + backKick * 0.4, -18,
    ]).fill({ color: pants })
    this.legs.roundRect(
      0 + backKick * 0.5, -18,
      8, 12, 2,
    ).fill({ color: pants })
  }

  // Foot dust puff at ground contact
  private drawFootDust(cycle: number): void {
    this.dust.clear()
    const contact = Math.max(0, Math.sin(cycle)) * 0.7
    if (contact < 0.2) return
    const alpha = contact * 0.4
    this.dust.ellipse(-12, 4, 10 * contact, 4 * contact)
      .fill({ color: 0xc8a060, alpha })
    this.dust.ellipse(12, 4, 8 * contact, 3 * contact)
      .fill({ color: 0xc8a060, alpha: alpha * 0.6 })
  }
}

function lerp(a: number, b: number, t: number): number {
  return a + (b - a) * t
}
