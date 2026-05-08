import { Container, Graphics } from 'pixi.js'

export type CharState = 'idle' | 'crouch' | 'airborne' | 'impact' | 'cashout' | 'fall'

/**
 * Procedural humanoid character for the jump-crash game.
 *
 * Position is controlled externally (GameScene sets container.x / container.y
 * each frame based on the jump arc). This class only handles local
 * squash/stretch, limb animation, and rotation.
 */
export class Character {
  readonly container: Container

  private body:       Graphics
  private legs:       Graphics
  private shadow:     Graphics
  private speedLines: Graphics

  private state: CharState = 'idle'
  private time = 0
  private phaseTime = 0  // time in current state

  // Squash/stretch
  private scaleX = 1
  private scaleY = 1
  private rotation = 0

  // Fall spin
  private fallSpin = 0

  // Distress flash for fall state
  private flashBody = false

  // Cashout exit velocity (pixels/s, set by GameScene)
  exitVx = 0

  constructor() {
    this.container = new Container()
    this.shadow     = new Graphics()
    this.body       = new Graphics()
    this.legs       = new Graphics()
    this.speedLines = new Graphics()

    // shadow below character body, body above, speed lines on top
    this.container.addChild(this.shadow, this.legs, this.body, this.speedLines)
    this.build()
  }

  private build(): void {
    this.body.clear()

    // ── Shadow (drawn at y=14 relative to container origin, compressed during impact) ──
    // Shadow is redrawn each frame in update() — skip here

    // ── Head (circle r=12, warm yellow) ──────────────────────────────────────
    this.body.circle(0, -30, 12).fill({ color: 0xfcd34d })

    // Head highlight
    this.body.circle(-3, -34, 4).fill({ color: 0xffffff, alpha: 0.4 })

    // Eyes
    this.body.circle(-4, -31, 2.2).fill({ color: 0x1e293b })
    this.body.circle( 4, -31, 2.2).fill({ color: 0x1e293b })

    // Eye shine
    this.body.circle(-3.2, -32, 0.8).fill({ color: 0xffffff, alpha: 0.7 })
    this.body.circle( 4.8, -32, 0.8).fill({ color: 0xffffff, alpha: 0.7 })

    // Smile
    this.body.arc(0, -27, 3.8, 0.2, Math.PI - 0.2).stroke({ color: 0x1e293b, width: 1.5 })

    // ── Helmet visor (arc over head) ─────────────────────────────────────────
    this.body.arc(0, -30, 13, -Math.PI, 0).stroke({ color: 0x60a5fa, width: 3, alpha: 0.7 })

    // ── Torso (roundRect, blue) ───────────────────────────────────────────────
    this.body.roundRect(-8, -18, 16, 18, 5).fill({ color: 0x3b82f6 })

    // Center stripe (lighter blue)
    this.body.roundRect(-2, -18, 4, 18, 2).fill({ color: 0x93c5fd, alpha: 0.5 })

    // ── Arms (small circles at shoulders) ────────────────────────────────────
    this.body.circle(-11, -14, 4).fill({ color: 0x2563eb })
    this.body.circle( 11, -14, 4).fill({ color: 0x2563eb })
  }

  // ── State machine ───────────────────────────────────────────────────────────

  setState(state: CharState): void {
    if (this.state === state) return
    this.state     = state
    this.phaseTime = 0
    this.fallSpin  = 0
    this.flashBody = false

    if (state === 'idle') {
      this.scaleX = 1
      this.scaleY = 1
      this.rotation = 0
    }
  }

  // ── Tick ────────────────────────────────────────────────────────────────────

  update(dt: number): void {
    this.time      += dt
    this.phaseTime += dt

    switch (this.state) {
      case 'idle':    this.tickIdle();              break
      case 'crouch':  this.tickCrouch();            break
      case 'airborne':this.tickAirborne();          break
      case 'impact':  this.tickImpact();            break
      case 'cashout': this.tickCashout(dt);         break
      case 'fall':    this.tickFall(dt);            break
    }

    this.container.scale.x = this.scaleX
    this.container.scale.y = this.scaleY
    this.container.rotation = this.rotation

    // Redraw shadow each frame (it responds to scaleY/state)
    this.drawShadow()
  }

  private drawShadow(): void {
    this.shadow.clear()
    const squish = this.state === 'impact' ? 1.6 : 1.0
    const alpha  = this.state === 'impact' ? 0.35 : 0.18
    // Shadow is in container space — at foot level (y≈14 when standing)
    this.shadow.ellipse(0, 16, 14 * squish, 3.5).fill({ color: 0x000000, alpha })
  }

  private tickIdle(): void {
    // Gentle bob
    const bob = Math.sin(this.time * 3.5) * 2.5
    this.drawLegs(0, bob)
    this.scaleX = 1
    this.scaleY = 1
    this.rotation = 0
    this.speedLines.clear()
  }

  private tickCrouch(): void {
    // Anticipation squish
    const t = Math.min(this.phaseTime / 0.18, 1)
    this.scaleY = 1 - t * 0.28
    this.scaleX = 1 + t * 0.18
    this.rotation = 0
    this.drawLegs(0.4, 0)
    this.speedLines.clear()
  }

  private tickAirborne(): void {
    // Slight forward lean, legs streaming back
    this.scaleX = 1
    this.scaleY = 1 + 0.08 * Math.sin(this.phaseTime * 8)  // breathing oscillation
    this.rotation = -0.22  // lean right (direction of travel)
    this.drawLegs(-0.5, 0)  // legs trailing

    // Speed lines (2-3 horizontal dashes trailing behind)
    this.speedLines.clear()
    const alpha = 0.35 + 0.2 * Math.sin(this.phaseTime * 12)
    for (let i = 0; i < 3; i++) {
      const yOff = -20 + i * 10
      const len  = 14 - i * 3
      this.speedLines
        .rect(12, yOff - 1, len, 2)
        .fill({ color: 0x93c5fd, alpha: alpha * (1 - i * 0.25) })
    }
  }

  private tickImpact(): void {
    // Landing squash — snap then recover (more dramatic: 0.65 scaleY, 1.25 scaleX)
    const t = Math.min(this.phaseTime / 0.25, 1)
    const squash = 1 - Math.sin(t * Math.PI) * 0.45
    this.scaleY = Math.max(0.65, squash)
    this.scaleX = 1 + (1 - squash) * 0.8
    if (this.scaleX > 1.25) this.scaleX = 1.25
    this.rotation = 0
    this.drawLegs(0.35, 0)
    this.speedLines.clear()
  }

  private tickCashout(dt: number): void {
    // Running pose — legs alternate
    this.scaleX = 1
    this.scaleY = 1
    this.rotation = -0.15
    const legSwing = Math.sin(this.phaseTime * 14) * 0.5
    this.drawLegs(legSwing, Math.abs(Math.sin(this.phaseTime * 14)) * -3)
    this.speedLines.clear()
    // Horizontal movement is handled by GameScene via container.x
    void dt
  }

  private tickFall(dt: number): void {
    this.fallSpin += dt * (3 + this.phaseTime * 2.5)
    this.rotation = this.fallSpin
    this.scaleX = 1
    this.scaleY = 1
    this.drawLegs(0, 0)
    this.speedLines.clear()

    // Distress: flash body color red via alpha overlay drawn on body
    // We achieve this by toggling the visible color each 0.08s
    const flashPeriod = 0.08
    this.flashBody = Math.floor(this.phaseTime / flashPeriod) % 2 === 1

    // Rebuild body with flash tint when distressed
    this.body.clear()
    const headColor = this.flashBody ? 0xff4444 : 0xfcd34d
    const bodyColor = this.flashBody ? 0xff2222 : 0x3b82f6

    this.body.circle(0, -30, 12).fill({ color: headColor })
    this.body.circle(-3, -34, 4).fill({ color: 0xffffff, alpha: 0.4 })
    this.body.circle(-4, -31, 2.2).fill({ color: 0x1e293b })
    this.body.circle( 4, -31, 2.2).fill({ color: 0x1e293b })
    this.body.roundRect(-8, -18, 16, 18, 5).fill({ color: bodyColor })
    this.body.roundRect(-2, -18, 4, 18, 2).fill({ color: 0x93c5fd, alpha: 0.5 })
    this.body.circle(-11, -14, 4).fill({ color: 0x2563eb })
    this.body.circle( 11, -14, 4).fill({ color: 0x2563eb })
  }

  // ── Legs ────────────────────────────────────────────────────────────────────

  private drawLegs(bendAngle: number, yOff: number): void {
    this.legs.clear()

    const baseY = 1 + yOff

    // Left leg
    const lx = -4.5
    const rx =  2.5
    const legLen = 12

    const lAngle = bendAngle - 0.1
    const rAngle = -bendAngle + 0.1

    const lKneeX = lx + Math.sin(lAngle) * legLen * 0.55
    const lKneeY = baseY + Math.cos(Math.abs(lAngle)) * legLen * 0.55
    const lFootX = lKneeX + Math.sin(lAngle * 0.5) * legLen * 0.5
    const lFootY = lKneeY + legLen * 0.5

    const rKneeX = rx + Math.sin(rAngle) * legLen * 0.55
    const rKneeY = baseY + Math.cos(Math.abs(rAngle)) * legLen * 0.55
    const rFootX = rKneeX + Math.sin(rAngle * 0.5) * legLen * 0.5
    const rFootY = rKneeY + legLen * 0.5

    // Upper legs (dark navy pants)
    this.legs
      .poly([lx, baseY, lx + 1, baseY, lKneeX + 1, lKneeY, lKneeX, lKneeY])
      .fill({ color: 0x1d4ed8 })
    this.legs
      .poly([rx, baseY, rx + 1, baseY, rKneeX + 1, rKneeY, rKneeX, rKneeY])
      .fill({ color: 0x1d4ed8 })

    // Lower legs (deep blue)
    this.legs
      .poly([lKneeX, lKneeY, lKneeX + 1, lKneeY, lFootX + 1, lFootY, lFootX, lFootY])
      .fill({ color: 0x1e3a8a })
    this.legs
      .poly([rKneeX, rKneeY, rKneeX + 1, rKneeY, rFootX + 1, rFootY, rFootX, rFootY])
      .fill({ color: 0x1e3a8a })

    // Feet / boots (bright accent)
    this.legs.roundRect(lFootX - 3, lFootY, 7, 3.5, 1.5).fill({ color: 0x60a5fa })
    this.legs.roundRect(rFootX - 3, rFootY, 7, 3.5, 1.5).fill({ color: 0x60a5fa })

    // Boot sole (dark)
    this.legs.roundRect(lFootX - 3, lFootY + 2.5, 7, 1, 0.5).fill({ color: 0x0f172a })
    this.legs.roundRect(rFootX - 3, rFootY + 2.5, 7, 1, 0.5).fill({ color: 0x0f172a })
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }
}
