import { Container, Graphics, Sprite, Assets, Texture } from 'pixi.js'
import { ASSET_PATHS } from '../../game/config'

/**
 * Canyon floor background — Reference Image 1 style continuous sandstone terrain.
 *
 * Renders from FLOOR_Y_FRAC (70 % of H) downward:
 *   • Sandy / earthy top surface strip with surface pebble detail
 *   • 7 layered rock strata graduating from warm orange → deep dark maroon
 *   • Horizontal crack lines at strata boundaries
 *   • Scattered vertical fracture details
 *
 * The spire / spike SPRITE layers are kept as optional decorative elements
 * that sit just above the sandy surface.  If their textures are absent the
 * fallback draws a simple ridge silhouette.
 *
 * Container.y drives the pursuit-rise animation (floor rises toward the player).
 * Everything is drawn with extra bottom padding so the base stays pinned to the
 * screen bottom even at maximum rise.
 */

export const FLOOR_Y_FRAC = 0.70  // export so ObstacleLayer + HazardLayer share it

export class DustCloudLayer {
  readonly container: Container

  private base:       Graphics
  private backLayer:  Container
  private frontLayer: Container

  private W = 0
  private H = 0
  private floorH = 0

  private currentRise = 0
  private targetRise  = 0

  private backTileW  = 1
  private frontTileW = 1

  constructor(w: number, h: number) {
    this.W      = w
    this.H      = h
    this.floorH = h * (1 - FLOOR_Y_FRAC)

    this.container  = new Container()
    this.base       = new Graphics()
    this.backLayer  = new Container()
    this.frontLayer = new Container()
    this.container.addChild(this.base, this.backLayer, this.frontLayer)

    this.buildTiles()
  }

  private buildTiles(): void {
    this.base.clear()
    this.backLayer.removeChildren()
    this.frontLayer.removeChildren()

    const { W, H } = this
    const fH      = this.floorH
    const maxRise = H * 0.30 + 30

    const topY  = H - fH             // = H * FLOOR_Y_FRAC
    const botH  = fH + maxRise        // full coverage including rise headroom

    // ── Sandy top surface ──────────────────────────────────────────────────
    // Earthy sandy strip at the floor surface (where the horse "runs")
    this.base.rect(0, topY, W, 9).fill({ color: 0xd4956a })
    // Subtle surface texture bands
    this.base.rect(0, topY + 2, W, 2).fill({ color: 0xc47a4a, alpha: 0.40 })
    this.base.rect(0, topY + 6, W, 2).fill({ color: 0xb86840, alpha: 0.30 })
    // Sunlit top edge
    this.base.rect(0, topY, W, 1.5).fill({ color: 0xf0c090, alpha: 0.55 })

    // Surface pebble/rock details
    const numPebbles = Math.ceil(W / 28)
    for (let pi = 0; pi < numPebbles; pi++) {
      const px  = (pi + 0.5) * (W / numPebbles) + Math.sin(pi * 2.3) * 7
      const py  = topY + 2 + Math.abs(Math.sin(pi * 3.1)) * 5
      const pr  = 1.2 + Math.abs(Math.sin(pi * 5.7)) * 2.2
      const col = (pi % 3 === 0) ? 0x9a5828 : (pi % 3 === 1) ? 0xb87040 : 0x7a4020
      this.base.circle(px, py, pr).fill({ color: col, alpha: 0.60 })
    }

    // ── Layered rock strata ────────────────────────────────────────────────
    // Warm orange at top → deep dark maroon at bottom (Reference Image 1)
    const strata = [
      { frac: 0.00, hFrac: 0.08, color: 0x8a5030 },  // warm top (just below sandy strip)
      { frac: 0.08, hFrac: 0.11, color: 0x7a4028 },
      { frac: 0.19, hFrac: 0.12, color: 0x6a3020 },
      { frac: 0.31, hFrac: 0.13, color: 0x58200e },
      { frac: 0.44, hFrac: 0.14, color: 0x451608 },
      { frac: 0.58, hFrac: 0.15, color: 0x321006 },
      { frac: 0.73, hFrac: 0.27, color: 0x1e0a04 },   // deepest
    ]

    const strataTopY = topY + 9  // strata start just below sandy strip
    const strataTotalH = botH - 9

    for (const s of strata) {
      this.base.rect(0, strataTopY + s.frac * strataTotalH, W, s.hFrac * strataTotalH + 1)
        .fill({ color: s.color })
    }

    // Horizontal crack / strata boundary lines
    const crackFracs = [0.08, 0.19, 0.31, 0.44, 0.58]
    for (const fy of crackFracs) {
      const cy = strataTopY + fy * strataTotalH
      this.base.rect(0, cy,     W, 1.5).fill({ color: 0x080402, alpha: 0.60 })
      this.base.rect(0, cy + 2, W, 1.0).fill({ color: 0xa05030, alpha: 0.18 })
    }

    // Vertical fracture details
    const numFractures = Math.ceil(W / 60)
    for (let fi = 0; fi < numFractures; fi++) {
      const fx   = (fi + 0.5) * (W / numFractures) + Math.sin(fi * 4.1) * 20
      const fTop = strataTopY + strataTotalH * (0.06 + Math.abs(Math.sin(fi * 2.9)) * 0.18)
      const fLen = strataTotalH * (0.12 + Math.abs(Math.sin(fi * 2.1)) * 0.22)
      this.base.rect(fx, fTop, 1, fLen).fill({ color: 0x080402, alpha: 0.32 })
    }

    // ── Back layer: canyon spires (decorative, sits above sandy surface) ───
    const spiresTex = Assets.get<Texture>(ASSET_PATHS.platforms.spires)
    if (spiresTex) {
      const scale    = (fH * 0.90) / spiresTex.height
      const tileW    = spiresTex.width * scale
      const numTiles = Math.ceil((W + tileW) / tileW) + 1
      this.backTileW = tileW

      for (let i = 0; i < numTiles; i++) {
        const s = new Sprite(spiresTex)
        s.anchor.set(0, 1)
        s.scale.set(scale)
        s.x = i * tileW
        s.y = H
        this.backLayer.addChild(s)
      }
    }

    // ── Front layer: hazard spikes ─────────────────────────────────────────
    const spikesTex = Assets.get<Texture>(ASSET_PATHS.platforms.spikes)
    if (spikesTex) {
      const scale    = (fH * 0.65) / spikesTex.height
      const tileW    = spikesTex.width * scale
      const numTiles = Math.ceil((W + tileW) / tileW) + 1
      this.frontTileW = tileW

      for (let i = 0; i < numTiles; i++) {
        const s = new Sprite(spikesTex)
        s.anchor.set(0, 1)
        s.scale.set(scale)
        s.x = i * tileW - tileW * 0.30
        s.y = H
        this.frontLayer.addChild(s)
      }
    }

    // Procedural fallback ridge when both sprite textures are absent
    if (!spiresTex && !spikesTex) {
      const ridgeY  = H - fH * 0.32
      const numPts  = Math.ceil(W / 22)
      const pts: number[] = [0, H, 0, ridgeY]
      for (let i = 0; i <= numPts; i++) {
        const x  = (i / numPts) * W
        const dy = (i % 2 === 0) ? -(fH * 0.18) : 0
        pts.push(x, ridgeY + dy)
      }
      pts.push(W, ridgeY, W, H)
      this.base.poly(pts).fill({ color: 0x5a2a10 })
    }
  }

  update(dt: number, pursuitRise: number, charWorldX = 0): void {
    this.targetRise   = pursuitRise
    this.currentRise += (this.targetRise - this.currentRise) * Math.min(1, dt * 2.2)

    this.container.y = -(this.H * 0.24 * this.currentRise)

    this.backLayer.x  = -this.wrapOffset(charWorldX * 0.22, this.backTileW)
    this.frontLayer.x = -this.wrapOffset(charWorldX * 0.38, this.frontTileW)
  }

  private wrapOffset(raw: number, period: number): number {
    if (period <= 0) return 0
    return ((raw % period) + period) % period
  }

  get surfaceY(): number {
    return this.H * FLOOR_Y_FRAC - this.H * 0.24 * this.currentRise
  }

  resize(w: number, h: number): void {
    this.W      = w
    this.H      = h
    this.floorH = h * (1 - FLOOR_Y_FRAC)
    this.buildTiles()
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }
}
