import { Container, Graphics } from 'pixi.js'

interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  life: number
  maxLife: number
  size: number
  color: number
  alpha: number
}

const POOL_SIZE = 400

export class ParticleLayer {
  readonly container: Container

  private gfx: Graphics
  private pool: Particle[] = []

  constructor() {
    this.container = new Container()
    this.gfx = new Graphics()
    this.container.addChild(this.gfx)

    for (let i = 0; i < POOL_SIZE; i++) this.pool.push(this.dead())
  }

  private dead(): Particle {
    return { x: 0, y: 0, vx: 0, vy: 0, life: 0, maxLife: 1, size: 2, color: 0, alpha: 0 }
  }

  private acquire(): Particle | undefined {
    return this.pool.find((p) => p.life <= 0)
  }

  // ── Public spawn API ────────────────────────────────────────────────────────

  /** Hoof dust puff when horse lands. */
  spawnLandingDust(worldX: number, screenY: number): void {
    for (let i = 0; i < 18; i++) {
      const p = this.acquire()
      if (!p) break
      const angle = Math.PI + (Math.random() - 0.5) * Math.PI * 1.3
      const speed = 25 + Math.random() * 70
      const life  = 0.35 + Math.random() * 0.4
      const colors = [0xc2954e, 0xa07040, 0xd4a87a, 0x8b6340, 0xe8c090]
      Object.assign(p, {
        x: worldX + (Math.random() - 0.5) * 24,
        y: screenY,
        vx: Math.cos(angle) * speed,
        vy: Math.sin(angle) * speed - 15,
        life, maxLife: life,
        size: 2.5 + Math.random() * 3.5,
        color: colors[Math.floor(Math.random() * colors.length)],
        alpha: 0.85,
      })
    }
  }

  /** Dust explosion when sheriff catches the rider. */
  spawnLavaSplash(worldX: number, screenY: number): void {
    for (let i = 0; i < 55; i++) {
      const p = this.acquire()
      if (!p) break
      const angle = -Math.PI * 0.5 + (Math.random() - 0.5) * Math.PI * 1.6
      const speed = 70 + Math.random() * 200
      const life  = 0.5 + Math.random() * 0.9
      const colors = [0xc2954e, 0xa07040, 0xd4aa5a, 0x8b5e30, 0xe8c060, 0xffd700]
      Object.assign(p, {
        x: worldX + (Math.random() - 0.5) * 30,
        y: screenY,
        vx: Math.cos(angle) * speed,
        vy: Math.sin(angle) * speed,
        life, maxLife: life,
        size: 3 + Math.random() * 5,
        color: colors[Math.floor(Math.random() * colors.length)],
        alpha: 1,
      })
    }
  }

  /** Arrest burst — rider tackled by sheriffs. */
  spawnCrashBurst(worldX: number, screenY: number): void {
    for (let i = 0; i < 65; i++) {
      const p = this.acquire()
      if (!p) break
      const angle = Math.random() * Math.PI * 2
      const speed = 50 + Math.random() * 200
      const life  = 0.5 + Math.random() * 1.1
      const colors = [0xc2954e, 0xffd700, 0xff8800, 0xffffff, 0xd4aa5a, 0xa07040]
      Object.assign(p, {
        x: worldX + (Math.random() - 0.5) * 22,
        y: screenY + (Math.random() - 0.5) * 22,
        vx: Math.cos(angle) * speed,
        vy: Math.sin(angle) * speed,
        life, maxLife: life,
        size: 2 + Math.random() * 4.5,
        color: colors[Math.floor(Math.random() * colors.length)],
        alpha: 1,
      })
    }
  }

  /** Gold coin/dust burst when player successfully escapes (cashes out). */
  spawnEscapeBurst(worldX: number, screenY: number): void {
    for (let i = 0; i < 40; i++) {
      const p = this.acquire()
      if (!p) break
      const angle = Math.random() * Math.PI * 2
      const speed = 40 + Math.random() * 150
      const life  = 0.7 + Math.random() * 1.0
      const colors = [0xffd700, 0xffcc00, 0xf59e0b, 0xfbbf24, 0xfde68a, 0xffffff]
      Object.assign(p, {
        x: worldX + (Math.random() - 0.5) * 15,
        y: screenY + (Math.random() - 0.5) * 15,
        vx: Math.cos(angle) * speed,
        vy: Math.sin(angle) * speed - 30,  // slight upward bias
        life, maxLife: life,
        size: 1.5 + Math.random() * 3.5,
        color: colors[Math.floor(Math.random() * colors.length)],
        alpha: 1,
      })
    }
  }

  /** Sparse coin trail behind hero during jump arc — called every ~100 ms while airborne. */
  spawnJumpCoinTrail(worldX: number, worldY: number): void {
    for (let i = 0; i < 4; i++) {
      const p = this.acquire()
      if (!p) break
      const angle = Math.PI + (Math.random() - 0.5) * 0.9   // spray backward
      const speed = 20 + Math.random() * 55
      const life  = 0.4 + Math.random() * 0.5
      const colors = [0xffd700, 0xffcc00, 0xf59e0b, 0xfbbf24]
      Object.assign(p, {
        x:       worldX + (Math.random() - 0.5) * 18,
        y:       worldY + (Math.random() - 0.5) * 14,
        vx:      Math.cos(angle) * speed,
        vy:      Math.sin(angle) * speed - 10,
        life, maxLife: life,
        size:    1.5 + Math.random() * 2.2,
        color:   colors[Math.floor(Math.random() * colors.length)],
        alpha:   1,
      })
    }
  }

  /** Gold coins spilling from hero during fall — called repeatedly while falling. */
  spawnFallCoins(worldX: number, worldY: number): void {
    for (let i = 0; i < 7; i++) {
      const p = this.acquire()
      if (!p) break
      const angle = -Math.PI * 0.5 + (Math.random() - 0.5) * Math.PI * 1.4
      const speed = 35 + Math.random() * 90
      const life  = 0.55 + Math.random() * 0.75
      const colors = [0xffd700, 0xffcc00, 0xf59e0b, 0xfbbf24, 0xfde68a]
      Object.assign(p, {
        x:       worldX + (Math.random() - 0.5) * 34,
        y:       worldY + (Math.random() - 0.5) * 22,
        vx:      Math.cos(angle) * speed - 18,
        vy:      Math.sin(angle) * speed - 25,
        life, maxLife: life,
        size:    1.8 + Math.random() * 2.8,
        color:   colors[Math.floor(Math.random() * colors.length)],
        alpha:   1,
      })
    }
  }

  // ── Tick ────────────────────────────────────────────────────────────────────

  update(dt: number): void {
    const gravity = 220

    for (const p of this.pool) {
      if (p.life <= 0) continue
      p.x    += p.vx * dt
      p.y    += p.vy * dt
      p.vy   += gravity * dt
      p.life -= dt
      p.alpha = Math.max(0, p.life / p.maxLife)
    }

    this.gfx.clear()
    for (const p of this.pool) {
      if (p.life <= 0 || p.alpha < 0.01) continue
      this.gfx.circle(p.x, p.y, p.size).fill({ color: p.color, alpha: p.alpha })
    }
  }

  destroy(): void {
    this.container.destroy({ children: true })
  }
}
