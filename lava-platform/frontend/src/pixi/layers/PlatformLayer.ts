import { Container, Graphics } from 'pixi.js'

export interface PlatformDef {
  index: number
  worldX: number  // center X in world space
  worldY: number  // Y in screen space (no vertical scrolling)
  width: number
  variant: 0 | 1 | 2  // 0=safe (gray), 1=cracked (dark), 2=hot (glowing)
}

// Pseudo-random from index — same on all clients for multiplayer sync
function hash(i: number): number {
  let x = Math.sin(i * 127.1 + 311.7) * 43758.5453
  return x - Math.floor(x)  // 0..1
}

function getPlatformY(index: number, H: number): number {
  return H * 0.30 + hash(index * 3 + 1) * H * 0.32
}

function getPlatformWidth(index: number, elapsedMs: number): number {
  const base = Math.max(120, 160 - elapsedMs / 800)
  return base * (0.85 + 0.15 * hash(index * 7 + 2))
}

function getPlatformSpacing(elapsedMs: number): number {
  return Math.max(115, 245 - elapsedMs / 380)
}

// Visual colors per variant
const COLORS = [
  { base: 0x1e2533, mid: 0x2d3748, top: 0x3d4f68, edge: 0x4b5a78, glow: 0x6b7280 },   // safe — dark rock
  { base: 0x111827, mid: 0x1f2937, top: 0x2d3748, edge: 0x374151, glow: 0xd97706 },    // cracked — warm
  { base: 0x1a0d03, mid: 0x2d1507, top: 0x3d1f09, edge: 0x92400e, glow: 0xf97316 },    // hot — orange glow
]

export class PlatformLayer {
  readonly container: Container

  private defs: PlatformDef[] = []
  // Active sprites: map from platform index to container
  private sprites = new Map<number, { c: Container; gfx: Graphics; float: number }>()

  private time = 0
  private H = 0

  constructor(_w: number, h: number) {
    this.H = h
    this.container = new Container()
    this.reset()
  }

  reset(): void {
    for (const s of this.sprites.values()) s.c.destroy({ children: true })
    this.sprites.clear()
    this.defs = []

    // Always seed the first platform at world origin
    this.defs.push({
      index: 0,
      worldX: 0,
      worldY: this.H * 0.62,
      width: 130,
      variant: 0,
    })
  }

  // Lazily generate platform definitions up to (and including) index
  ensureGenerated(upToIndex: number, elapsedMs: number): void {
    while (this.defs.length <= upToIndex) {
      const prev = this.defs[this.defs.length - 1]
      const i = this.defs.length
      const spacing = getPlatformSpacing(elapsedMs)
      const worldX = prev.worldX + spacing
      const worldY = getPlatformY(i, this.H)
      const width = getPlatformWidth(i, elapsedMs)
      const variant = (hash(i * 13) < 0.4 ? 1 : hash(i * 17) < 0.25 ? 2 : 0) as 0 | 1 | 2
      this.defs.push({ index: i, worldX, worldY, width, variant })
    }
  }

  getPlatform(index: number): PlatformDef | undefined {
    return this.defs[index]
  }

  // Called each frame — culls sprites outside view, creates sprites for visible platforms
  update(dt: number, camMinWorldX: number, camMaxWorldX: number): void {
    this.time += dt

    // Show platforms in [camMinWorldX-100, camMaxWorldX+100]
    const min = camMinWorldX - 100
    const max = camMaxWorldX + 100

    for (const def of this.defs) {
      const visible = def.worldX >= min && def.worldX <= max
      const hasSprite = this.sprites.has(def.index)

      if (visible && !hasSprite) {
        this.createSprite(def)
      } else if (!visible && hasSprite) {
        this.destroySprite(def.index)
      }
    }

    // Animate visible sprites
    for (const [idx, s] of this.sprites) {
      const def = this.defs[idx]
      if (!def) continue
      const floatY = Math.sin(this.time * 0.9 + s.float) * 3

      // For hot variant: pulse the glow effect by rebuilding at low frequency
      // We skip per-frame redraws for cost — only float is animated via y
      s.c.y = def.worldY + floatY

      // Hot variant pulse: rebuild each frame (cheap since gfx is simple)
      if (def.variant === 2) {
        const pulsePhase = Math.sin(this.time * 3.0 + s.float)
        s.gfx.clear()
        this.drawPlatformGfx(s.gfx, def, pulsePhase)
      }
    }
  }

  private buildJaggedTop(w: number, _h: number, defIndex: number, numPoints: number): number[] {
    const halfW = w / 2
    const pts: number[] = []
    for (let i = 0; i <= numPoints; i++) {
      const x = -halfW + (i / numPoints) * w
      const jag = (hash(defIndex * 31 + i * 7) - 0.5) * 5  // ±2.5px variation
      pts.push(x, jag)
    }
    return pts
  }

  private drawPlatformGfx(gfx: Graphics, def: PlatformDef, hotPulse = 0): void {
    const col = COLORS[def.variant]
    const w   = def.width
    const h   = 12
    const halfW = w / 2

    // ── Hot variant: pulsing orange glow underlay ─────────────────────────────
    if (def.variant === 2) {
      const glowAlpha = 0.18 + hotPulse * 0.10
      const glowW = w + 10
      gfx.roundRect(-glowW / 2, -4, glowW, h + 12, 5).fill({ color: 0xff4400, alpha: glowAlpha })
    }

    // ── Multi-layer body ──────────────────────────────────────────────────────
    // Darkest base
    gfx.roundRect(-halfW, 0, w, h, 3).fill({ color: col.base })

    // Mid highlight layer
    gfx.roundRect(-halfW + 2, 1, w - 4, h - 3, 2).fill({ color: col.mid, alpha: 0.7 })

    // ── Jagged top edge using polygon ─────────────────────────────────────────
    const numJagPts = Math.max(5, Math.ceil(w / 14))
    const jagPts    = this.buildJaggedTop(w, h, def.index, numJagPts)

    // Build closed polygon: jagged top + rectangular bottom
    const topPoly: number[] = []
    // Left side down
    topPoly.push(-halfW, h)
    // Jagged top points
    for (let i = 0; i < jagPts.length; i += 2) {
      topPoly.push(jagPts[i], jagPts[i + 1])
    }
    // Right side down
    topPoly.push(halfW, h)

    gfx.poly(topPoly).fill({ color: col.top })

    // ── Glowing top edge (bright line) ───────────────────────────────────────
    const edgePts: number[] = []
    for (let i = 0; i < jagPts.length; i += 2) {
      edgePts.push(jagPts[i], jagPts[i + 1] - 1)
    }
    if (edgePts.length >= 4) {
      gfx.poly(edgePts).stroke({ color: col.edge, width: 2, alpha: 0.9 })
    }

    // ── Hot variant: extra bright glow line ───────────────────────────────────
    if (def.variant === 2) {
      const glowA = 0.5 + hotPulse * 0.3
      gfx.poly(edgePts).stroke({ color: col.glow, width: 1.5, alpha: glowA })
    }

    // ── Cracks for cracked/hot variants ──────────────────────────────────────
    if (def.variant >= 1) {
      const cx = (hash(def.index * 5) - 0.5) * w * 0.4
      gfx.poly([cx - 1, 0, cx + 1, 0, cx + 2, h, cx, h]).fill({ color: 0x000000, alpha: 0.4 })
    }

    // ── Lava drip for hot variant ─────────────────────────────────────────────
    if (def.variant === 2) {
      const dx = (hash(def.index * 9) - 0.5) * w * 0.5
      gfx.poly([dx - 2, h, dx + 2, h, dx + 1, h + 6, dx - 1, h + 6]).fill({ color: 0xff6600, alpha: 0.8 })
    }
  }

  private createSprite(def: PlatformDef): void {
    const c = new Container()
    c.x = def.worldX
    c.y = def.worldY

    const gfx = new Graphics()
    this.drawPlatformGfx(gfx, def, 0)

    c.addChild(gfx)
    this.container.addChild(c)
    this.sprites.set(def.index, { c, gfx, float: Math.random() * Math.PI * 2 })
  }

  private destroySprite(index: number): void {
    const s = this.sprites.get(index)
    if (!s) return
    s.c.destroy({ children: true })
    this.sprites.delete(index)
  }

  resize(_w: number, h: number): void {
    this.H = h
    // Rebuild all defs with new height (Y values change)
    const count = this.defs.length
    this.reset()
    // Re-generate same count (spacing doesn't matter for resize, use 0 elapsed)
    this.ensureGenerated(count - 1, 0)
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }
}
