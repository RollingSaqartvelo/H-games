import { Container, Sprite, Assets, Texture, Graphics } from 'pixi.js'
import { ASSET_PATHS } from '../../game/config'

export interface ObstacleDef {
  index:   number
  worldX:  number
  worldY:  number   // landing surface Y in world space
  width:   number   // rendered width in px
  height:  number   // approx rendered height in px
  variant: 0 | 1   // 0=round platform (75%), 1=desert cliff (25%)
}

function hash(i: number): number {
  const x = Math.sin(i * 127.1 + 311.7) * 43758.5453
  return x - Math.floor(x)
}

function getVariant(index: number): 0 | 1 {
  return hash(index * 13) < 0.75 ? 0 : 1
}

function getObstacleY(index: number, H: number): number {
  // Tight gameplay band: 40–46% from top. Hero never drifts above/below this lane.
  return H * 0.40 + hash(index * 3 + 1) * H * 0.06
}

function getObstacleWidth(index: number, variant: 0 | 1, elapsedMs: number): number {
  if (variant === 0) {
    const base = Math.max(145, 175 - elapsedMs / 1200)
    return base * (0.90 + 0.25 * hash(index * 7 + 2))
  }
  const base = Math.max(210, 260 - elapsedMs / 1000)
  return base * (0.90 + 0.25 * hash(index * 7 + 2))
}

function getObstacleSpacing(elapsedMs: number): number {
  return Math.max(280, 370 - elapsedMs / 360)
}

const SURFACE_TOP_FRAC: Record<0 | 1, number> = {
  0: 0.22,
  1: 0.07,
}

const APPROX_HEIGHTS: Record<0 | 1, number> = { 0: 65, 1: 55 }

const GRAVITY = 320   // px/s²

interface BrokenPiece {
  sprite: Sprite
  x: number
  y: number
  velX: number
  velY: number
  rotVel: number
}

interface CrashAnim {
  left:  BrokenPiece
  right: BrokenPiece
}

export class ObstacleLayer {
  readonly container: Container

  private defs:           ObstacleDef[] = []
  private sprites       = new Map<number, { c: Container; float: number }>()
  private crashAnims:     CrashAnim[]   = []
  private crashedIndices = new Set<number>()

  private time = 0
  private H    = 0

  constructor(_w: number, h: number) {
    this.H = h
    this.container = new Container()
    this.reset()
  }

  reset(): void {
    for (const s of this.sprites.values()) s.c.destroy({ children: true })
    this.sprites.clear()

    for (const anim of this.crashAnims) {
      anim.left.sprite.destroy()
      anim.right.sprite.destroy()
    }
    this.crashAnims     = []
    this.crashedIndices = new Set()

    this.defs = []
    this.defs.push({
      index: 0, worldX: 0,
      worldY: this.H * 0.43,
      width: 180, height: APPROX_HEIGHTS[0], variant: 0,
    })
  }

  getDefs(): ObstacleDef[] { return this.defs }

  ensureGenerated(upToIndex: number, elapsedMs: number): void {
    while (this.defs.length <= upToIndex) {
      const prev    = this.defs[this.defs.length - 1]
      const i       = this.defs.length
      const spacing = getObstacleSpacing(elapsedMs)
      const worldX  = prev.worldX + spacing
      const variant = getVariant(i)
      const worldY  = getObstacleY(i, this.H)
      const width   = getObstacleWidth(i, variant, elapsedMs)
      this.defs.push({ index: i, worldX, worldY, width, height: APPROX_HEIGHTS[variant], variant })
    }
  }

  getPlatform(index: number): ObstacleDef | undefined {
    return this.defs[index]
  }

  /**
   * Trigger platform-break animation for the given platform index.
   * Works for both variant 0 (round) and variant 1 (desert cliff).
   */
  triggerCrash(platIndex: number): void {
    const def = this.defs[platIndex]
    if (!def) return
    if (this.crashedIndices.has(def.index)) return

    this.destroySprite(def.index)
    this.crashedIndices.add(def.index)

    const isDesert = def.variant === 1

    const texLeftPath  = isDesert ? ASSET_PATHS.platforms.desertBrokenLeft  : ASSET_PATHS.platforms.brokenLeft
    const texRightPath = isDesert ? ASSET_PATHS.platforms.desertBrokenRight : ASSET_PATHS.platforms.brokenRight
    const texBasePath  = isDesert ? ASSET_PATHS.platforms.desert             : ASSET_PATHS.platforms.main

    const texLeft  = Assets.get<Texture>(texLeftPath)
    const texRight = Assets.get<Texture>(texRightPath)
    const texBase  = Assets.get<Texture>(texBasePath)
    if (!texLeft || !texRight || !texBase) return

    const scale      = def.width / texBase.width
    const surfaceAnchorY = SURFACE_TOP_FRAC[def.variant]

    const makePiece = (tex: Texture, anchorX: number, velX: number, rotVel: number): BrokenPiece => {
      const s = new Sprite(tex)
      s.anchor.set(anchorX, surfaceAnchorY)
      s.scale.set(scale)
      s.x = def.worldX
      s.y = def.worldY
      this.container.addChild(s)
      return { sprite: s, x: def.worldX, y: def.worldY, velX, velY: -45, rotVel }
    }

    this.crashAnims.push({
      left:  makePiece(texLeft,  1.0, -60, -2.4),
      right: makePiece(texRight, 0.0, +60, +2.4),
    })
  }

  update(dt: number, camMinWorldX: number, camMaxWorldX: number): void {
    this.time += dt
    const min = camMinWorldX - 120
    const max = camMaxWorldX + 120

    for (const def of this.defs) {
      const visible   = def.worldX >= min && def.worldX <= max
      const hasSprite = this.sprites.has(def.index)
      const crashed   = this.crashedIndices.has(def.index)
      if (visible && !hasSprite && !crashed) this.createSprite(def)
      else if (!visible && hasSprite) this.destroySprite(def.index)
    }

    for (const [idx, s] of this.sprites) {
      const def = this.defs[idx]
      if (!def) continue
      s.c.y = def.worldY + Math.sin(this.time * 0.7 + s.float) * 2.5
    }

    this.tickCrashAnims(dt)
  }

  private tickCrashAnims(dt: number): void {
    for (let i = this.crashAnims.length - 1; i >= 0; i--) {
      const { left, right } = this.crashAnims[i]

      for (const p of [left, right]) {
        p.velY          += GRAVITY * dt
        p.x             += p.velX  * dt
        p.y             += p.velY  * dt
        p.sprite.x       = p.x
        p.sprite.y       = p.y
        p.sprite.rotation += p.rotVel * dt
      }

      if (left.y > this.H + 300) {
        left.sprite.destroy()
        right.sprite.destroy()
        this.crashAnims.splice(i, 1)
      }
    }
  }

  private createSprite(def: ObstacleDef): void {
    const c = new Container()
    c.x = def.worldX
    c.y = def.worldY

    const texPath = def.variant === 0
      ? ASSET_PATHS.platforms.main
      : ASSET_PATHS.platforms.desert
    const tex = Assets.get<Texture>(texPath)

    if (tex) {
      const sprite = new Sprite(tex)
      sprite.anchor.set(0.5, SURFACE_TOP_FRAC[def.variant])
      sprite.scale.set(def.width / tex.width)
      c.addChild(sprite)
    } else {
      const gfx = new Graphics()
      gfx.roundRect(-def.width / 2, 0, def.width, 35, 6)
        .fill({ color: def.variant === 0 ? 0xb8601e : 0xc07030 })
      c.addChild(gfx)
    }

    this.container.addChild(c)
    this.sprites.set(def.index, { c, float: Math.random() * Math.PI * 2 })
  }

  private destroySprite(index: number): void {
    const s = this.sprites.get(index)
    if (!s) return
    s.c.destroy({ children: true })
    this.sprites.delete(index)
  }

  resize(_w: number, h: number): void {
    this.H = h
    const count = this.defs.length
    this.reset()
    this.ensureGenerated(count - 1, 0)
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }
}
