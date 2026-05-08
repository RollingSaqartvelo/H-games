import { Container, Graphics } from 'pixi.js'
import type { ObstacleDef } from './ObstacleLayer'

/**
 * World-space hazard pit modules, positioned in the gaps between platforms.
 *
 * Each module covers from left-platform-right-edge to right-platform-left-edge.
 * Module y=0 is the floor surface (FLOOR_Y in world/screen coords).
 * Pits extend downward by PIT_DEPTH.
 *
 * 5 hazard types (deterministic per gap index):
 *   bridge — wooden rope bridge over water pond
 *   spikes — spike pit with stalagmites
 *   cactus — sandy trench with cactus formations
 *   lava   — glowing lava crack
 *   ravine — broken sandstone ledge over deep void
 */

const PIT_DEPTH = 95
const WALL_W    = 10   // thickness of each pit wall

function hash(i: number): number {
  const x = Math.sin(i * 127.1 + 311.7) * 43758.5453
  return x - Math.floor(x)
}

type HazardType = 'bridge' | 'spikes' | 'cactus' | 'lava' | 'ravine'

function getType(index: number): HazardType {
  const h = hash(index * 31 + 7)
  if (h < 0.20) return 'bridge'
  if (h < 0.44) return 'spikes'
  if (h < 0.62) return 'cactus'
  if (h < 0.80) return 'lava'
  return 'ravine'
}

// ── Strata wall colours (top → bottom of pit) ─────────────────────────────
const WALL_STRATA = [
  0x8a5028, 0x7a4020, 0x6a3018, 0x5a2010,
  0x4a1808, 0x3a1206, 0x2a0e04,
]

export class HazardLayer {
  readonly container: Container
  private modules = new Map<number, Container>()
  private time = 0

  constructor(private H: number) {
    this.container = new Container()
  }

  floorY(): number { return this.H * 0.70 }

  update(dt: number, defs: ObstacleDef[], camMinX: number, camMaxX: number): void {
    this.time += dt

    for (let i = 0; i < defs.length - 1; i++) {
      const left  = defs[i]
      const right = defs[i + 1]

      const pitX = left.worldX  + left.width  / 2
      const pitW = right.worldX - right.width / 2 - pitX

      if (pitW < 18) continue

      const pad     = 160
      const visible = pitX + pitW >= camMinX - pad && pitX <= camMaxX + pad

      if (visible && !this.modules.has(i)) {
        this.createModule(i, pitX, pitW)
      } else if (!visible && this.modules.has(i)) {
        this.destroyModule(i)
      }
    }
  }

  // ── Module creation ────────────────────────────────────────────────────────

  private createModule(index: number, x: number, w: number): void {
    const c   = new Container()
    c.x = x
    c.y = this.floorY()

    const gfx = new Graphics()
    const type = getType(index)

    this.drawBase(gfx, w)           // dark pit + strata walls (all types share this)

    switch (type) {
      case 'bridge': this.drawBridge(gfx, w, index); break
      case 'spikes': this.drawSpikes(gfx, w, index); break
      case 'cactus': this.drawCactus(gfx, w, index); break
      case 'lava':   this.drawLava(gfx, w, index);   break
      case 'ravine': this.drawRavine(gfx, w, index); break
    }

    c.addChild(gfx)
    this.container.addChild(c)
    this.modules.set(index, c)
  }

  // ── Shared pit base ────────────────────────────────────────────────────────

  private drawBase(gfx: Graphics, w: number): void {
    // Dark void interior
    gfx.rect(0, 0, w, PIT_DEPTH).fill({ color: 0x0a0503 })

    // Left strata wall
    const lh = PIT_DEPTH / WALL_STRATA.length
    for (let i = 0; i < WALL_STRATA.length; i++) {
      gfx.rect(0, i * lh, WALL_W, lh + 1).fill({ color: WALL_STRATA[i] })
    }
    // Right strata wall
    for (let i = 0; i < WALL_STRATA.length; i++) {
      gfx.rect(w - WALL_W, i * lh, WALL_W, lh + 1).fill({ color: WALL_STRATA[i] })
    }

    // Wall edge highlights (bright lip at pit opening)
    gfx.rect(0,         0, WALL_W + 2,     2).fill({ color: 0xca8848, alpha: 0.70 })
    gfx.rect(w - WALL_W - 2, 0, WALL_W + 2, 2).fill({ color: 0xca8848, alpha: 0.70 })
  }

  // ── Bridge over water ──────────────────────────────────────────────────────

  private drawBridge(gfx: Graphics, w: number, index: number): void {
    const inner   = w - WALL_W * 2
    const innerX  = WALL_W
    const waterY  = PIT_DEPTH * 0.42
    const waterH  = PIT_DEPTH * 0.45

    // Water body
    gfx.roundRect(innerX, waterY, inner, waterH, 3).fill({ color: 0x1a5070 })
    // Water surface glint
    gfx.roundRect(innerX, waterY, inner, 5, 2).fill({ color: 0x3a90c0, alpha: 0.60 })
    // Underwater plants (simple)
    const numPlants = Math.max(2, Math.floor(inner / 22))
    for (let pi = 0; pi < numPlants; pi++) {
      const px = innerX + 8 + (pi / numPlants) * (inner - 16) + (hash(index * 7 + pi) - 0.5) * 8
      const ph = 12 + hash(index * 5 + pi) * 10
      const pc = hash(index * 3 + pi) > 0.5 ? 0x2a6040 : 0x3a7830
      gfx.poly([px - 2, waterY + waterH, px + 2, waterY + waterH, px, waterY + waterH - ph])
        .fill({ color: pc, alpha: 0.75 })
    }
    // Lily pad
    const lpx = innerX + inner * 0.55
    gfx.ellipse(lpx, waterY + 3, 7, 3).fill({ color: 0x3d7a30, alpha: 0.80 })

    // Wooden bridge planks
    const plankY    = -5           // slightly above floor surface (y=0)
    const plankThk  = 8
    const numPlanks = Math.max(3, Math.floor(inner / 9))
    const plankW    = (inner - 4) / numPlanks

    for (let pi = 0; pi < numPlanks; pi++) {
      const px = innerX + 2 + pi * plankW
      const tone = pi % 2 === 0 ? 0x7a4820 : 0x6a3c18
      gfx.roundRect(px, plankY, plankW - 1.5, plankThk, 1).fill({ color: tone })
      // Wood grain line
      gfx.rect(px + 2, plankY + 3, plankW - 5, 1).fill({ color: 0x2a1008, alpha: 0.4 })
    }
    // Rope handrails
    gfx.poly([
      innerX,     plankY - 6,
      innerX + inner * 0.5, plankY - 12,
      innerX + inner, plankY - 6,
    ]).stroke({ color: 0x9a7040, width: 1.5, alpha: 0.85 })
    // Rope posts
    for (let ri = 0; ri <= 2; ri++) {
      const rx = innerX + (ri / 2) * inner
      gfx.rect(rx - 1.5, plankY - 14, 3, 14).fill({ color: 0x7a5030 })
    }
  }

  // ── Spike pit ─────────────────────────────────────────────────────────────

  private drawSpikes(gfx: Graphics, w: number, index: number): void {
    const inner  = w - WALL_W * 2
    const innerX = WALL_W
    const floorY = PIT_DEPTH * 0.78

    // Sandy pit floor
    gfx.rect(innerX, floorY, inner, PIT_DEPTH - floorY).fill({ color: 0x3a1808 })

    // Spike/stalagmite formations
    const numSpikes = Math.max(3, Math.floor(inner / 12))
    for (let si = 0; si < numSpikes; si++) {
      const sx   = innerX + 6 + (si / numSpikes) * (inner - 12) + (hash(index * 9 + si) - 0.5) * 5
      const sh   = 22 + hash(index * 11 + si) * 28
      const sw   = 4 + hash(index * 13 + si) * 4
      const col  = hash(index * 7 + si) > 0.5 ? 0x6a3020 : 0x5a2818
      gfx.poly([
        sx - sw / 2, floorY,
        sx + sw / 2, floorY,
        sx,          floorY - sh,
      ]).fill({ color: col })
      // Spike highlight
      gfx.poly([
        sx - sw * 0.15, floorY,
        sx + sw * 0.15, floorY,
        sx,             floorY - sh * 0.8,
      ]).fill({ color: 0x8a4030, alpha: 0.45 })
    }
    // Blood-red ambient glow at bottom
    gfx.rect(innerX, floorY, inner, 6).fill({ color: 0x6b1a0a, alpha: 0.55 })
  }

  // ── Cactus trench ─────────────────────────────────────────────────────────

  private drawCactus(gfx: Graphics, w: number, index: number): void {
    const inner  = w - WALL_W * 2
    const innerX = WALL_W
    const groundY = PIT_DEPTH * 0.72

    // Sandy ground inside trench
    gfx.roundRect(innerX, groundY, inner, PIT_DEPTH - groundY, 2).fill({ color: 0xb87040 })
    gfx.rect(innerX, groundY, inner, 4).fill({ color: 0xc88050, alpha: 0.60 })

    // Cactus plants
    const numCactus = Math.max(2, Math.floor(inner / 28))
    for (let ci = 0; ci < numCactus; ci++) {
      const cx   = innerX + 14 + (ci / numCactus) * (inner - 28) + (hash(index * 5 + ci) - 0.5) * 8
      const ch   = 28 + hash(index * 7 + ci) * 18
      const cCol = 0x3a6020
      // Main trunk
      gfx.roundRect(cx - 3, groundY - ch, 6, ch, 2).fill({ color: cCol })
      // Arms
      if (hash(index * 11 + ci) > 0.4) {
        const armY  = groundY - ch * 0.55
        const armL  = 10 + hash(index * 13 + ci) * 8
        gfx.roundRect(cx + 3, armY, armL, 5, 2).fill({ color: cCol })
        gfx.roundRect(cx + 3 + armL - 5, armY - 8, 5, 12, 2).fill({ color: cCol })
      }
      if (hash(index * 17 + ci) > 0.55) {
        const armY  = groundY - ch * 0.40
        const armL  = 8 + hash(index * 19 + ci) * 7
        gfx.roundRect(cx - 3 - armL, armY, armL, 5, 2).fill({ color: cCol })
        gfx.roundRect(cx - 3 - armL, armY - 8, 5, 12, 2).fill({ color: cCol })
      }
      // Spines (tiny lines)
      gfx.circle(cx - 3, groundY - ch * 0.80, 1).fill({ color: 0xd4c070, alpha: 0.7 })
      gfx.circle(cx + 3, groundY - ch * 0.60, 1).fill({ color: 0xd4c070, alpha: 0.7 })
    }

    // Small rocks on trench floor
    for (let ri = 0; ri < 3; ri++) {
      const rx = innerX + 8 + (ri / 3) * (inner - 16) + hash(index * 23 + ri) * 8
      gfx.ellipse(rx, groundY + 4, 5 + hash(index + ri) * 4, 3).fill({ color: 0x9a6030, alpha: 0.6 })
    }
  }

  // ── Lava crack ────────────────────────────────────────────────────────────

  private drawLava(gfx: Graphics, w: number, index: number): void {
    const inner  = w - WALL_W * 2
    const innerX = WALL_W
    const lavaY  = PIT_DEPTH * 0.60

    // Lava body (orange-red glow)
    gfx.roundRect(innerX, lavaY, inner, PIT_DEPTH - lavaY, 3).fill({ color: 0xcc3300 })
    // Lava surface (brighter)
    gfx.roundRect(innerX, lavaY, inner, 8, 2).fill({ color: 0xff6600 })
    // Hot glow below pit opening
    gfx.roundRect(innerX, 0, inner, PIT_DEPTH * 0.60).fill({ color: 0x3d0800, alpha: 0.75 })
    // Glow cast on walls
    gfx.rect(0,         lavaY - 10, WALL_W, 15).fill({ color: 0xff4400, alpha: 0.25 })
    gfx.rect(w - WALL_W, lavaY - 10, WALL_W, 15).fill({ color: 0xff4400, alpha: 0.25 })

    // Lava bubbles (static — we don't animate in creation)
    for (let bi = 0; bi < 3; bi++) {
      const bx = innerX + 12 + (bi / 3) * (inner - 24) + hash(index * 7 + bi) * 8
      const br = 3 + hash(index * 9 + bi) * 3
      gfx.circle(bx, lavaY + 4, br).fill({ color: 0xff9900, alpha: 0.80 })
    }

    // Crack opening at pit lip (dark splits in the wall strata)
    const numCracks = 2 + Math.floor(hash(index * 3) * 3)
    for (let ci = 0; ci < numCracks; ci++) {
      const cx = innerX + (ci / numCracks) * inner + hash(index * 5 + ci) * 10
      gfx.poly([
        cx, 0,
        cx + 3, PIT_DEPTH * 0.20,
        cx - 1, PIT_DEPTH * 0.15,
      ]).fill({ color: 0xff4400, alpha: 0.45 })
    }
  }

  // ── Broken ravine ─────────────────────────────────────────────────────────

  private drawRavine(gfx: Graphics, w: number, index: number): void {
    const inner  = w - WALL_W * 2
    const innerX = WALL_W

    // Deep void
    gfx.rect(innerX, 0, inner, PIT_DEPTH).fill({ color: 0x080302 })

    // Broken sandstone ledge partially crossing the gap
    const ledgeW = inner * (0.35 + hash(index * 7) * 0.25)
    const ledgeSide = hash(index * 11) > 0.5  // left or right side
    const ledgeX  = ledgeSide ? innerX : innerX + inner - ledgeW
    const ledgeY  = PIT_DEPTH * (0.30 + hash(index * 13) * 0.25)
    const ledgeH  = 12

    // Ledge body
    gfx.roundRect(ledgeX, ledgeY, ledgeW, ledgeH, 3).fill({ color: 0x7a4820 })
    gfx.roundRect(ledgeX + 2, ledgeY, ledgeW - 4, 4, 2).fill({ color: 0xaa6830, alpha: 0.7 })
    // Broken edge (jagged)
    const jagX = ledgeSide ? ledgeX + ledgeW : ledgeX
    gfx.poly([
      jagX - 4, ledgeY,
      jagX,     ledgeY,
      jagX - 2, ledgeY + ledgeH,
      jagX - 6, ledgeY + ledgeH,
    ]).fill({ color: 0x5a3010 })

    // Rock debris at the bottom
    for (let ri = 0; ri < 4; ri++) {
      const rx = innerX + 6 + (ri / 4) * (inner - 12) + (hash(index * 5 + ri) - 0.5) * 6
      const ry = PIT_DEPTH * (0.75 + hash(index * 3 + ri) * 0.18)
      const rr = 3 + hash(index * 9 + ri) * 5
      gfx.ellipse(rx, ry, rr, rr * 0.6).fill({ color: 0x6a3818, alpha: 0.65 })
    }

    // Mist / depth haze at the very bottom
    gfx.rect(innerX, PIT_DEPTH * 0.82, inner, PIT_DEPTH * 0.18)
      .fill({ color: 0x150805, alpha: 0.50 })
  }

  // ── Lifecycle ──────────────────────────────────────────────────────────────

  private destroyModule(index: number): void {
    const m = this.modules.get(index)
    if (!m) return
    m.destroy({ children: true })
    this.modules.delete(index)
  }

  resize(h: number): void {
    this.H = h
    for (const m of this.modules.values()) m.destroy({ children: true })
    this.modules.clear()
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }
}
