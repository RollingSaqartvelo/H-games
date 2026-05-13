import { Container, Sprite, Assets, Texture } from 'pixi.js'
import { ASSET_PATHS } from '../../game/config'

/**
 * Deterministic scrolling floor built from three PNG tiles.
 *
 * Tile sequence (repeating):
 *   floor → floor → floor2 → floor → floor → floor3
 *
 * Each tile is a Sprite anchored bottom-left.  All three PNGs must share
 * the same canvas dimensions — width is derived from the base texture's
 * aspect ratio so scale is identical across all tiles.
 *
 * update(charWorldX, cameraX) must be called every frame.
 * cameraX = charScreenX - charWorldX  (already computed by GameScene)
 *
 * floor2 / floor3 fall back to floor1 if the PNG is not yet in cache.
 */

const DISPLAY_HEIGHT = 80

const FLOOR_SEQUENCE = [
  'floor', 'floor', 'floor2', 'floor', 'floor', 'floor3', 'floor', 'floor', 'floor4',
] as const

type FloorKey = typeof FLOOR_SEQUENCE[number]

interface TileEntry {
  sprite: Sprite
  worldX: number
}

export class FloorLayer {
  readonly container: Container
  private tiles: TileEntry[] = []
  private textures: Partial<Record<FloorKey, Texture>> = {}
  private tileWidth = 0
  private seqPos    = 0
  private nextWorldX = 0
  private W = 0
  private H = 0
  private ready = false

  constructor(w: number, h: number) {
    this.W = w
    this.H = h
    this.container = new Container()

    const tex = Assets.get<Texture>(ASSET_PATHS.environment.floor)
    if (!tex) {
      console.warn(
        '[FloorLayer] seamless_desert_floor.png not in cache — floor hidden.\n' +
        'Place it at /public/assets/environment/floor/seamless_desert_floor.png',
      )
      return
    }

    this.textures.floor  = tex
    this.textures.floor2 = Assets.get<Texture>(ASSET_PATHS.environment.floor2) ?? tex
    this.textures.floor3 = Assets.get<Texture>(ASSET_PATHS.environment.floor3) ?? tex
    this.textures.floor4 = Assets.get<Texture>(ASSET_PATHS.environment.floor4) ?? tex

    this.tileWidth  = Math.round(tex.width * DISPLAY_HEIGHT / tex.height)
    this.nextWorldX = -this.tileWidth * 2   // pre-seed 2 tiles left of origin
    this.ready      = true
  }

  /**
   * @param charWorldX  world X of the rider (for future use / reset alignment)
   * @param cameraX     charScreenX - charWorldX  (already computed in GameScene)
   */
  update(_charWorldX: number, cameraX: number): void {
    if (!this.ready) return

    // Spawn tiles to fill right edge of screen + one-tile buffer
    while (this.nextWorldX + cameraX < this.W + this.tileWidth) {
      this.spawnTile()
    }

    // Reposition all tiles; cull tiles that slid off the left
    for (let i = this.tiles.length - 1; i >= 0; i--) {
      const entry  = this.tiles[i]
      const screenX = entry.worldX + cameraX
      entry.sprite.x = screenX
      entry.sprite.y = this.H    // bottom of canvas (anchor is bottom-left)

      if (screenX + this.tileWidth < -this.tileWidth) {
        entry.sprite.destroy()
        this.tiles.splice(i, 1)
      }
    }
  }

  private spawnTile(): void {
    const key = FLOOR_SEQUENCE[this.seqPos % FLOOR_SEQUENCE.length]
    this.seqPos++

    const tex = this.textures[key] ?? this.textures.floor!
    const s   = new Sprite(tex)
    s.anchor.set(0, 1)          // bottom-left origin
    s.width  = this.tileWidth   // uniform scale across all tile types
    s.height = DISPLAY_HEIGHT
    this.container.addChild(s)

    this.tiles.push({ sprite: s, worldX: this.nextWorldX })
    this.nextWorldX += this.tileWidth
  }

  /** Call on round reset so the sequence restarts from the beginning. */
  reset(): void {
    for (const entry of this.tiles) entry.sprite.destroy()
    this.tiles      = []
    this.seqPos     = 0
    this.nextWorldX = -this.tileWidth * 2
  }

  resize(w: number, h: number): void {
    this.W = w
    this.H = h
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }
}
