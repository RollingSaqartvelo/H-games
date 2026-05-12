/**
 * BuildingLayer — scrolling western town buildings with 4 floor levels.
 *
 * Generates procedural connected western buildings:
 *   Floor 0 (street):  saloon doors, ground shop fronts
 *   Floor 1 (balcony): wooden balcony planks + rails
 *   Floor 2 (rooftop): flat roof surface, chimneys
 *   Floor 3 (high):    water towers, bridge planks, high ledges
 *
 * Buildings are generated ahead of the runner and culled as they scroll offscreen.
 * Each building tile is ~300px wide; types cycle through 5 western variants.
 *
 * API kept compatible with original ObstacleLayer (ensureGenerated, update, etc.)
 */

import { Container, Graphics } from 'pixi.js'

// Floor Y positions as fraction of canvas H (same as GameScene FLOOR_Y_FRACS)
const FLOOR_Y_FRACS = [0.705, 0.545, 0.390, 0.248] as const

const BUILDING_SPACING = 295   // average px between building worldX anchors
const GROUND_FRAC      = 0.86  // bottom of buildings (ground surface)

// 5 western building types
type BldgType = 0 | 1 | 2 | 3 | 4
// 0 = saloon        (3 floors, false front)
// 1 = bank          (2 floors, brick)
// 2 = general store (2 floors, wood)
// 3 = hotel         (4 floors, tall)
// 4 = stable + shed (1 floor, wide)

function hash(n: number): number {
  const x = Math.sin(n * 127.1 + 311.7) * 43758.5453
  return x - Math.floor(x)
}

interface BuildingDef {
  index:   number
  worldX:  number
  width:   number
  type:    BldgType
  floors:  number
  seed:    number
}

// Dummy ObstacleDef interface kept for API compatibility with GameScene
export interface ObstacleDef {
  index:   number
  worldX:  number
  worldY:  number
  width:   number
  height:  number
  variant: 0 | 1
}

export class ObstacleLayer {
  readonly container: Container

  private defs:    BuildingDef[] = []
  private gfxMap   = new Map<number, Container>()

  private H = 0

  constructor(_w: number, h: number) {
    this.H = h
    this.container = new Container()
    this.reset()
  }

  reset(): void {
    for (const c of this.gfxMap.values()) c.destroy({ children: true })
    this.gfxMap.clear()
    this.defs = []

    // Seed first building at x = -300 so it's already visible on start
    this.addBuilding(-300, 0)
    this.addBuilding(0, 0)
  }

  // Keep API compatibility: GameScene no longer calls getPlatform but keep it
  getPlatform(index: number): ObstacleDef | undefined {
    const def = this.defs[index]
    if (!def) return undefined
    return {
      index:   def.index,
      worldX:  def.worldX,
      worldY:  this.H * FLOOR_Y_FRACS[0],
      width:   def.width,
      height:  80,
      variant: 0,
    }
  }

  // upToIndex: generate buildings up to worldX = upToIndex * BUILDING_SPACING
  ensureGenerated(upToIndex: number, _elapsedMs: number): void {
    const targetWorldX = upToIndex * BUILDING_SPACING
    let maxWorldX = this.defs.length > 0
      ? this.defs[this.defs.length - 1].worldX
      : -BUILDING_SPACING

    while (maxWorldX < targetWorldX) {
      const nextX = maxWorldX + BUILDING_SPACING + (hash(this.defs.length * 7) - 0.5) * 60
      this.addBuilding(nextX, this.defs.length)
      maxWorldX = nextX
    }
  }

  private addBuilding(worldX: number, seed: number): void {
    const h = hash(seed * 13)
    const type = Math.floor(h * 5) as BldgType
    const floors = type === 3 ? 4 : type === 4 ? 1 : type === 1 ? 2 : 3
    const baseWidth = type === 4 ? 360 : 240 + hash(seed * 3) * 120
    this.defs.push({
      index:  this.defs.length,
      worldX,
      width:  baseWidth,
      type,
      floors,
      seed,
    })
  }

  // No-op for API compatibility (GameScene calls this on crash)
  triggerCrash(_platIndex: number): void { /* crash handled by GameScene directly */ }

  update(dt: number, camMinWorldX: number, camMaxWorldX: number): void {
    void dt
    const margin = 300
    const min = camMinWorldX - margin
    const max = camMaxWorldX + margin

    for (const def of this.defs) {
      const rightEdge  = def.worldX + def.width
      const inView     = rightEdge >= min && def.worldX <= max
      const hasGfx     = this.gfxMap.has(def.index)

      if (inView && !hasGfx) {
        this.createBuilding(def)
      } else if (!inView && hasGfx) {
        this.gfxMap.get(def.index)!.destroy({ children: true })
        this.gfxMap.delete(def.index)
      }
    }
  }

  resize(_w: number, h: number): void {
    this.H = h
    // Rebuild all visible buildings with new dimensions
    const existing = [...this.gfxMap.keys()]
    for (const idx of existing) {
      this.gfxMap.get(idx)!.destroy({ children: true })
      this.gfxMap.delete(idx)
    }
    for (const def of this.defs) {
      if (this.gfxMap.size < 12) this.createBuilding(def)
    }
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }

  // ── Procedural building rendering ─────────────────────────────────────────

  private createBuilding(def: BuildingDef): void {
    const c = new Container()
    c.x = def.worldX
    c.y = 0

    const g = new Graphics()
    c.addChild(g)
    this.drawBuilding(g, def)

    this.container.addChild(c)
    this.gfxMap.set(def.index, c)
  }

  private drawBuilding(g: Graphics, def: BuildingDef): void {
    const H  = this.H
    const W  = def.width
    const gY = H * GROUND_FRAC    // bottom of building at ground surface

    // Floor Y positions in screen / world space
    const floorY = FLOOR_Y_FRACS.map(f => H * f)

    // Building top: one floor above highest usable floor
    const topY   = floorY[3] - 48

    // ── Shared palettes ────────────────────────────────────────────────────
    const palettes: Record<BldgType, { wall: number; trim: number; accent: number; roof: number }> = {
      0: { wall: 0x8B6028, trim: 0xC8903C, accent: 0xD44444, roof: 0x5C3010 },  // saloon
      1: { wall: 0x8B4030, trim: 0xC06048, accent: 0xD4A017, roof: 0x4A2018 },  // bank
      2: { wall: 0x9A7040, trim: 0xC8A060, accent: 0x5080A0, roof: 0x6A4822 },  // general
      3: { wall: 0x7A5A30, trim: 0xB08040, accent: 0x8B3A3A, roof: 0x4A3010 },  // hotel
      4: { wall: 0x7A6030, trim: 0xA08040, accent: 0x604020, roof: 0x4A3820 },  // stable
    }
    const pal = palettes[def.type]

    // ── Ground-to-top building facade ──────────────────────────────────────
    // Main wall
    g.rect(0, topY, W, gY - topY).fill({ color: pal.wall })

    // Horizontal floor dividers (wood planks between floors)
    for (let f = 0; f < 4; f++) {
      const fy = floorY[f]
      if (fy < topY || fy > gY) continue
      // Floor sill / beam
      g.rect(0, fy - 4, W, 8).fill({ color: pal.trim })
      g.rect(0, fy,     W, 3).fill({ color: 0x000000, alpha: 0.25 })
    }

    // ── Vertical wood grain lines ─────────────────────────────────────────
    const grainCount = Math.floor(W / 18)
    for (let i = 0; i < grainCount; i++) {
      const gx = (i + 0.5) * (W / grainCount) + hash(def.seed + i * 7) * 4 - 2
      g.rect(gx, topY, 1.5, gY - topY)
        .fill({ color: 0x000000, alpha: 0.08 + hash(def.seed + i) * 0.06 })
    }

    // ── Windows per floor ─────────────────────────────────────────────────
    this.drawWindows(g, def, floorY, topY, gY, pal)

    // ── Floor platforms (balcony rails, rooftop edge) ──────────────────────
    this.drawFloorPlatforms(g, def, floorY, W, gY, pal)

    // ── Building-type specific features ───────────────────────────────────
    switch (def.type) {
      case 0: this.drawSaloon(g, def, floorY, W, topY, gY, pal);       break
      case 1: this.drawBank(g, def, floorY, W, topY, gY, pal);         break
      case 2: this.drawGeneralStore(g, def, floorY, W, topY, gY, pal); break
      case 3: this.drawHotel(g, def, floorY, W, topY, gY, pal);        break
      case 4: this.drawStable(g, def, floorY, W, topY, gY, pal);       break
    }

    // ── Roof ──────────────────────────────────────────────────────────────
    this.drawRoof(g, def, W, topY, pal)

    // ── Ground level decorations ──────────────────────────────────────────
    this.drawGroundLevel(g, def, W, floorY[0], gY, pal)
  }

  private drawWindows(
    g: Graphics, def: BuildingDef,
    floorY: readonly number[], topY: number, gY: number,
    pal: { wall: number; trim: number; accent: number; roof: number },
  ): void {
    const W = def.width
    const winsPerFloor = Math.max(2, Math.floor(W / 70))

    for (let floor = 1; floor < 4; floor++) {
      const fy = floorY[floor]
      if (fy - 36 < topY || fy > gY) continue

      for (let wi = 0; wi < winsPerFloor; wi++) {
        const wx = 20 + (wi + 0.5) * ((W - 40) / winsPerFloor) + hash(def.seed + floor * 7 + wi) * 8 - 4
        const wy = fy - 40
        const ww = 26 + hash(def.seed + wi) * 12
        const wh = 30

        // Window frame
        g.roundRect(wx - ww / 2 - 2, wy - 2, ww + 4, wh + 4, 2).fill({ color: pal.trim })
        // Glass (lit or dark)
        const lit = hash(def.seed + floor * 13 + wi * 7) > 0.35
        const glassColor = lit ? 0xFFD060 : 0x1A2A3A
        const glassAlpha = lit ? 0.7 : 0.8
        g.roundRect(wx - ww / 2, wy, ww, wh, 1).fill({ color: glassColor, alpha: glassAlpha })
        // Window bars
        g.rect(wx - 1, wy, 2, wh).fill({ color: pal.trim, alpha: 0.6 })
        g.rect(wx - ww / 2, wy + wh / 2 - 1, ww, 2).fill({ color: pal.trim, alpha: 0.6 })

        // Shutters on some windows
        if (hash(def.seed + wi * 11) > 0.6) {
          g.roundRect(wx - ww / 2 - 10, wy, 9, wh, 1).fill({ color: pal.accent, alpha: 0.7 })
          g.roundRect(wx + ww / 2 + 1, wy, 9, wh, 1).fill({ color: pal.accent, alpha: 0.7 })
        }
      }
    }
  }

  private drawFloorPlatforms(
    g: Graphics, _def: BuildingDef,
    floorY: readonly number[], W: number, gY: number,
    pal: { wall: number; trim: number; accent: number; roof: number },
  ): void {
    // Floor 1 — balcony railing along the full building width
    const f1Y = floorY[1]
    if (f1Y < gY) {
      // Wooden balcony floor planks
      g.rect(0, f1Y, W, 12).fill({ color: 0x8B6028, alpha: 0.9 })
      for (let i = 0; i < Math.floor(W / 14); i++) {
        g.rect(i * 14, f1Y, 2, 12).fill({ color: 0x000000, alpha: 0.12 })
      }
      g.rect(0, f1Y - 1, W, 3).fill({ color: pal.trim })

      // Balcony railing posts
      const postCount = Math.max(3, Math.floor(W / 28))
      for (let i = 0; i <= postCount; i++) {
        const px = (i / postCount) * W
        g.rect(px - 2, f1Y - 36, 4, 40).fill({ color: pal.trim })
        // Post cap
        g.roundRect(px - 4, f1Y - 40, 8, 6, 2).fill({ color: pal.trim })
      }
      // Rail top bar
      g.rect(0, f1Y - 34, W, 6).fill({ color: pal.trim })
    }

    // Floor 2 — rooftop edge / parapet
    const f2Y = floorY[2]
    if (f2Y < gY - 20) {
      // Rooftop surface
      g.rect(0, f2Y, W, 10).fill({ color: pal.roof })
      g.rect(0, f2Y, W, 3).fill({ color: pal.trim, alpha: 0.8 })
      // Parapet crenellations
      const merlonCount = Math.floor(W / 22)
      for (let i = 0; i < merlonCount; i++) {
        if (i % 2 === 0) {
          g.rect(i * 22, f2Y - 18, 16, 18).fill({ color: pal.wall })
          g.rect(i * 22, f2Y - 18, 16, 3).fill({ color: pal.trim, alpha: 0.7 })
        }
      }
    }

    // Floor 3 — high platform (water tower base, chimney stack platform)
    const f3Y = floorY[3]
    if (f3Y < gY - 60) {
      // High platform planks
      g.rect(W * 0.1, f3Y, W * 0.8, 10).fill({ color: 0x6A4822 })
      g.rect(W * 0.1, f3Y, W * 0.8, 3).fill({ color: pal.trim, alpha: 0.6 })
      // Bridge supports hanging down
      g.rect(W * 0.15, f3Y, 5, 30).fill({ color: 0x5A3818, alpha: 0.8 })
      g.rect(W * 0.85, f3Y, 5, 30).fill({ color: 0x5A3818, alpha: 0.8 })
    }
  }

  // ── Building type specifics ───────────────────────────────────────────────

  private drawSaloon(
    g: Graphics, def: BuildingDef,
    floorY: readonly number[], W: number, topY: number, _gY: number,
    pal: { wall: number; trim: number; accent: number; roof: number },
  ): void {
    // Classic false front: extends higher than actual roof
    const falseFrontH = 38
    g.rect(0, topY - falseFrontH, W, falseFrontH).fill({ color: pal.wall })
    // Decorative false front top trim
    g.rect(0, topY - falseFrontH - 6, W, 8).fill({ color: pal.trim })
    g.rect(6, topY - falseFrontH - 3, W - 12, 3).fill({ color: pal.accent })

    // SALOON sign board
    const signY = topY - falseFrontH + 8
    const signW = Math.min(W - 40, 140)
    const signX = (W - signW) / 2
    g.roundRect(signX, signY, signW, 22, 3).fill({ color: 0x2A1400 })
    g.roundRect(signX + 2, signY + 2, signW - 4, 18, 2).fill({ color: 0xC89040 })

    // Sign text (procedural letter boxes — no actual text rendering)
    const letters = 6  // "SALOON"
    for (let i = 0; i < letters; i++) {
      const lx = signX + 8 + i * (signW - 16) / letters
      g.roundRect(lx, signY + 5, (signW - 16) / letters - 3, 12, 1)
        .fill({ color: 0x8B2020, alpha: 0.8 })
    }

    // Swing doors at street level
    const f0Y = floorY[0]
    const doorW = 28; const doorH = 38
    const doorX = W / 2 - doorW - 2
    // Left door (slightly ajar)
    g.rect(doorX, f0Y - doorH, doorW, doorH).fill({ color: 0x5C3010 })
    g.rect(doorX, f0Y - doorH + 4, doorW, 2).fill({ color: 0x3A1A00 })
    g.rect(doorX, f0Y - doorH / 2, doorW, 2).fill({ color: 0x3A1A00 })
    // Right door
    g.rect(doorX + doorW + 4, f0Y - doorH, doorW, doorH).fill({ color: 0x5C3010 })
    g.rect(doorX + doorW + 4, f0Y - doorH + 4, doorW, 2).fill({ color: 0x3A1A00 })

    // Hanging oil lamp on balcony
    const lampX = hash(def.seed * 3) * (W - 40) + 20
    this.drawLamp(g, lampX, floorY[1] - 50, pal)
  }

  private drawBank(
    g: Graphics, _def: BuildingDef,
    floorY: readonly number[], W: number, topY: number, _gY: number,
    pal: { wall: number; trim: number; accent: number; roof: number },
  ): void {
    // Brick pattern overlay
    const rows = Math.floor((floorY[0] - topY) / 14)
    for (let r = 0; r < rows; r++) {
      const rowY = topY + r * 14
      const offset = (r % 2) * 30
      const cols = Math.ceil(W / 60)
      for (let c = 0; c < cols; c++) {
        const bx = c * 60 + offset - 30
        g.rect(bx, rowY, 56, 11).fill({ color: 0x000000, alpha: 0.07 })
        g.rect(bx, rowY, 56, 1).fill({ color: 0x000000, alpha: 0.12 })
        g.rect(bx, rowY + 11, 1, 3).fill({ color: 0x000000, alpha: 0.12 })
      }
    }

    // Pillar columns (classical bank)
    const pillarCount = Math.max(3, Math.floor(W / 55))
    for (let i = 0; i <= pillarCount; i++) {
      const px = (i / pillarCount) * W
      g.rect(px - 8, topY, 16, floorY[0] - topY).fill({ color: pal.trim, alpha: 0.4 })
      // Capital
      g.rect(px - 12, topY, 24, 8).fill({ color: pal.trim })
      g.rect(px - 10, floorY[0] - 10, 20, 10).fill({ color: pal.trim })
    }

    // "FIRST NATIONAL BANK" pediment triangle
    const pedW = Math.min(W - 20, 160)
    const pedX = (W - pedW) / 2
    g.poly([pedX, topY, pedX + pedW, topY, pedX + pedW / 2, topY - 30])
      .fill({ color: pal.accent, alpha: 0.7 })

    // Vault door at street level (circle)
    const vaultX = W / 2; const vaultY = floorY[0] - 30
    g.circle(vaultX, vaultY, 22).fill({ color: 0x3A3A3A })
    g.circle(vaultX, vaultY, 18).fill({ color: 0x4A4A4A })
    g.circle(vaultX, vaultY, 14).fill({ color: 0x222222 })
    // Spokes
    for (let i = 0; i < 8; i++) {
      const a = (i / 8) * Math.PI * 2
      g.rect(vaultX - 1, vaultY - 14, 2, 14).fill({ color: 0x666666 })
      void a
    }
  }

  private drawGeneralStore(
    g: Graphics, def: BuildingDef,
    floorY: readonly number[], W: number, topY: number, _gY: number,
    pal: { wall: number; trim: number; accent: number; roof: number },
  ): void {
    // Large storefront awning
    const awningY = floorY[0] - 45
    g.poly([
      -10, awningY, W + 10, awningY,
      W - 5, awningY + 22, 5, awningY + 22,
    ]).fill({ color: pal.accent, alpha: 0.85 })
    // Awning stripes
    const stripes = Math.floor(W / 18)
    for (let i = 0; i < stripes; i++) {
      if (i % 2 === 0) {
        g.rect(i * 18, awningY, 9, 22).fill({ color: 0xffffff, alpha: 0.15 })
      }
    }
    // Awning fringe
    const fringeCount = Math.floor(W / 12)
    for (let i = 0; i < fringeCount; i++) {
      g.rect(i * 12 + 3, awningY + 20, 3, 10).fill({ color: pal.accent })
    }

    // Storefront window (large display)
    const dispW = W * 0.55; const dispH = 30
    const dispX = (W - dispW) / 2
    g.roundRect(dispX, floorY[0] - dispH - 2, dispW, dispH, 2)
      .fill({ color: 0x223355, alpha: 0.7 })
    g.roundRect(dispX, floorY[0] - dispH - 2, dispW, dispH, 2)
      .stroke({ color: pal.trim, width: 2 })

    // Store sign above awning
    const sW = Math.min(W - 30, 110); const sX = (W - sW) / 2
    g.roundRect(sX, topY + 12, sW, 20, 3).fill({ color: 0x2A1800 })
    g.roundRect(sX + 2, topY + 14, sW - 4, 16, 2).fill({ color: pal.accent, alpha: 0.5 })

    // Barrel on porch
    const bX = hash(def.seed * 5) * (W - 60) + 20
    this.drawBarrel(g, bX, floorY[0])
  }

  private drawHotel(
    g: Graphics, _def: BuildingDef,
    floorY: readonly number[], W: number, topY: number, _gY: number,
    pal: { wall: number; trim: number; accent: number; roof: number },
  ): void {
    // Grand hotel: ornate cornices between every floor
    for (let f = 1; f < 4; f++) {
      const fy = floorY[f]
      if (fy < topY + 10) continue
      // Fancy cornice
      g.rect(0, fy - 10, W, 10).fill({ color: pal.trim })
      g.rect(4, fy - 14, W - 8, 6).fill({ color: pal.accent, alpha: 0.5 })
    }

    // HOTEL name plate
    const npW = Math.min(W - 20, 100); const npX = (W - npW) / 2
    g.roundRect(npX, topY + 6, npW, 28, 4).fill({ color: 0x1A0A00 })
    g.roundRect(npX + 3, topY + 9, npW - 6, 22, 3).fill({ color: pal.accent, alpha: 0.7 })

    // Decorative balcony ironwork (floor 1 & 2)
    for (let f = 1; f <= 2; f++) {
      const fy = floorY[f]
      if (fy < topY + 20) continue
      const railCount = Math.floor(W / 14)
      for (let i = 0; i <= railCount; i++) {
        const rx = (i / railCount) * W
        // Decorative spindle
        g.roundRect(rx - 2, fy - 28, 4, 28, 2).fill({ color: pal.trim, alpha: 0.7 })
        g.circle(rx, fy - 28, 4).fill({ color: pal.trim })
      }
      g.rect(0, fy - 30, W, 4).fill({ color: pal.trim })
    }

    // Entrance arch at street level
    const archW = 40; const archX = W / 2 - archW / 2
    g.rect(archX, floorY[0] - 50, archW, 50).fill({ color: 0x1A0A00 })
    g.circle(archX + archW / 2, floorY[0] - 50, archW / 2).fill({ color: 0x1A0A00 })
    // Arch frame
    g.circle(archX + archW / 2, floorY[0] - 50, archW / 2 + 3)
      .stroke({ color: pal.trim, width: 3 })
  }

  private drawStable(
    g: Graphics, _def: BuildingDef,
    floorY: readonly number[], W: number, topY: number, gY: number,
    pal: { wall: number; trim: number; accent: number; roof: number },
  ): void {
    // Wide low stable with pitched roof
    const stableH = gY - topY
    void stableH

    // Large double doors
    const dW = W * 0.35; const dX = W / 2 - dW / 2
    const dH = Math.min(55, floorY[0] - topY - 10)
    g.rect(dX, floorY[0] - dH, dW, dH).fill({ color: 0x2A1400 })
    // Door panels X pattern
    g.poly([dX, floorY[0] - dH, dX + dW / 2, floorY[0]])
      .stroke({ color: 0x4A2800, width: 2 })
    g.poly([dX + dW, floorY[0] - dH, dX + dW / 2, floorY[0]])
      .stroke({ color: 0x4A2800, width: 2 })
    // Left half door
    g.rect(dX, floorY[0] - dH, dW / 2 - 1, dH).fill({ color: 0x3A1A00, alpha: 0.4 })

    // Hay bale
    const haX = W - 50; const haY = floorY[0]
    g.roundRect(haX, haY - 20, 35, 20, 4).fill({ color: 0xC8A840 })
    for (let i = 0; i < 3; i++) {
      g.rect(haX + 6 + i * 10, haY - 20, 2, 20).fill({ color: 0xA08830, alpha: 0.7 })
    }
    g.roundRect(haX - 2, haY - 8, 12, 10, 3).fill({ color: pal.wall, alpha: 0.5 })

    // Stable name board
    const nbW = 90; const nbX = (W - nbW) / 2
    g.roundRect(nbX, topY + 10, nbW, 18, 2).fill({ color: 0x2A1800 })
    g.roundRect(nbX + 2, topY + 12, nbW - 4, 14, 2).fill({ color: pal.trim, alpha: 0.4 })
  }

  // ── Shared decoration helpers ─────────────────────────────────────────────

  private drawRoof(
    g: Graphics, _def: BuildingDef,
    W: number, topY: number,
    pal: { wall: number; trim: number; accent: number; roof: number },
  ): void {
    // Flat roof with parapet cap
    g.rect(0, topY - 8, W, 10).fill({ color: pal.roof })
    g.rect(-4, topY - 10, W + 8, 5).fill({ color: pal.trim })

    // Chimney stacks
    const chimneyCount = 1 + Math.floor(W / 160)
    for (let i = 0; i < chimneyCount; i++) {
      const cx = 40 + (i / chimneyCount) * (W - 80) + 20
      const ch = 24 + Math.floor(i * 7) % 16
      g.rect(cx, topY - 8 - ch, 18, ch).fill({ color: pal.wall })
      g.rect(cx - 3, topY - 8 - ch, 24, 5).fill({ color: pal.trim })
      // Smoke suggestion (faint circles)
      g.circle(cx + 9, topY - 14 - ch, 6).fill({ color: 0xffffff, alpha: 0.06 })
      g.circle(cx + 12, topY - 22 - ch, 8).fill({ color: 0xffffff, alpha: 0.04 })
    }

    // Water tower on top (floor 3 feature)
    if (W > 280) {
      const twX = W * 0.72
      const twY = topY - 8
      this.drawWaterTower(g, twX, twY)
    }
  }

  private drawWaterTower(g: Graphics, x: number, baseY: number): void {
    const h = 55; const r = 22
    // Legs
    g.rect(x - r + 4, baseY - h, 5, h).fill({ color: 0x5C3A1A })
    g.rect(x + r - 9, baseY - h, 5, h).fill({ color: 0x5C3A1A })
    g.poly([x - r + 4, baseY - h, x + r - 4, baseY - h, x, baseY - h - 10])
      .fill({ color: 0x4A2A0A, alpha: 0.6 })
    // Tank body
    g.rect(x - r, baseY - h - 30, r * 2, 30).fill({ color: 0x7A5228 })
    for (let i = 0; i < 5; i++) {
      g.rect(x - r, baseY - h - 30 + i * 6, r * 2, 1)
        .fill({ color: 0x000000, alpha: 0.15 })
    }
    // Conical roof
    g.poly([x - r - 3, baseY - h - 30, x + r + 3, baseY - h - 30, x, baseY - h - 45])
      .fill({ color: 0x5C3A1A })
    // Hoop rings
    for (let i = 0; i < 3; i++) {
      g.rect(x - r - 2, baseY - h - 28 + i * 9, r * 2 + 4, 3)
        .fill({ color: 0x3A2008, alpha: 0.7 })
    }
  }

  private drawLamp(g: Graphics, x: number, y: number,
    pal: { wall: number; trim: number; accent: number; roof: number },
  ): void {
    // Hanging chain
    g.rect(x - 1, y - 12, 2, 12).fill({ color: pal.trim, alpha: 0.7 })
    // Lamp body
    g.rect(x - 6, y, 12, 14).fill({ color: 0x2A1800 })
    // Glass panel (glowing)
    g.rect(x - 4, y + 2, 8, 10).fill({ color: 0xFFD060, alpha: 0.85 })
    // Top cap
    g.poly([x - 8, y, x + 8, y, x + 5, y - 5, x - 5, y - 5])
      .fill({ color: 0x1A0A00 })
  }

  private drawBarrel(g: Graphics, x: number, baseY: number): void {
    const bw = 22; const bh = 30
    g.roundRect(x - bw / 2, baseY - bh, bw, bh, 4).fill({ color: 0x7A5228 })
    for (let i = 0; i < 3; i++) {
      g.rect(x - bw / 2 - 2, baseY - bh + 6 + i * 9, bw + 4, 3)
        .fill({ color: 0x3A2008, alpha: 0.8 })
    }
    g.ellipse(x, baseY - bh, bw / 2, 5).fill({ color: 0x9A7040, alpha: 0.6 })
  }

  private drawGroundLevel(
    g: Graphics, _def: BuildingDef,
    W: number, f0Y: number, gY: number,
    pal: { wall: number; trim: number; accent: number; roof: number },
  ): void {
    // Porch / raised boardwalk at street level
    const porchH = gY - f0Y + 4
    g.rect(0, f0Y, W, porchH).fill({ color: 0x7A5228, alpha: 0.6 })
    // Boardwalk planks
    const plankW = 18
    for (let i = 0; i * plankW < W; i++) {
      g.rect(i * plankW + 1, f0Y + 1, plankW - 2, porchH - 2)
        .fill({ color: 0x000000, alpha: 0.06 })
    }
    // Front edge raised lip
    g.rect(0, f0Y - 2, W, 4).fill({ color: pal.trim, alpha: 0.5 })

    // Support posts
    const postCount = Math.max(2, Math.floor(W / 90))
    for (let i = 0; i <= postCount; i++) {
      const px = (i / postCount) * W
      g.rect(px - 3, f0Y - 4, 6, porchH + 4).fill({ color: 0x5C3A1A })
      g.roundRect(px - 5, f0Y - 8, 10, 6, 2).fill({ color: pal.trim, alpha: 0.7 })
    }
  }
}
