/**
 * GameScene — Outlaw Chase crash game.
 *
 * Architecture:
 *   scene root
 *     ├─ desert         (sunset sky + canyon parallax)
 *     ├─ worldContainer (scrolls horizontally)
 *     │     ├─ buildings  (procedural western town, 4 floor levels)
 *     │     ├─ particles  (dust, escape burst, crash burst)
 *     │     ├─ sheriff    (pursuer, always visible behind outlaw)
 *     │     └─ rider      (outlaw runner, fixed screen X)
 *     ├─ floor          (ground tiles on top)
 *     └─ flash          (gunfire flash on crash)
 *
 * MECHANIC:
 *   Outlaw runs horizontally LEFT→RIGHT through connected western building floors.
 *   Sheriff chases behind — gap shrinks as multiplier climbs.
 *   Floor level (0=street, 1=balcony, 2=rooftop, 3=high) shifts automatically.
 *   Crash = sheriff captures outlaw / shoots.
 *   No jumping. No platforms. Continuous forward escape.
 */

import { Application, Container } from 'pixi.js'
import { DesertSkyLayer }  from './layers/DesertSkyLayer'
import { FloorLayer }      from './layers/FloorLayer'
import { ObstacleLayer }   from './layers/ObstacleLayer'
import { ParticleLayer }   from './layers/ParticleLayer'
import { GunfireCrashFX } from './layers/GunfireCrashFX'
import { HorseRider }      from './entities/HorseRider'
import { SheriffRider }    from './entities/SheriffRider'
import { useGame }         from '../store/game'
import { lerp, expLerp } from './utils/easing'
import type { RoundState } from '../ws/types'
import { playKnutSound }  from '../audio/KnutSound'

// Outlaw is pinned at 38% from left — leaves room for sheriff behind
const CHAR_X_FRAC = 0.38

// Floor Y positions as fraction of canvas height (world Y = screen Y, no vertical scroll)
const FLOOR_Y_FRACS = [0.705, 0.545, 0.390, 0.248] as const

// Sheriff starts 260px behind, gap closes as time passes
function getSheriffTargetGap(elapsedMs: number): number {
  const t = elapsedMs / 1000
  return Math.max(58, 260 - t * 3.5)
}

// Run speed increases over time
function getRunSpeed(elapsedMs: number): number {
  return Math.min(420, 190 + (elapsedMs / 1000) * 3.4)
}

// Floor transition interval shortens over time for rising tension
function getFloorInterval(elapsedMs: number): number {
  return Math.max(2.2, 5.8 - (elapsedMs / 1000) * 0.045)
}

// Approximate integral of getRunSpeed for position sync on late join
function estimateWorldX(elapsedMs: number): number {
  const t = elapsedMs / 1000
  return 190 * t + 1.7 * t * t
}

type RunPhase = 'idle' | 'running' | 'captured'

// Floor picking: pick adjacent floor, bias upward with time
function pickNextFloor(current: number, elapsedMs: number, rng: number): number {
  const t = elapsedMs / 1000
  const maxFloor = Math.min(3, Math.floor(t / 9))
  const candidates: number[] = [current]
  if (current > 0)        candidates.push(current - 1)
  if (current < maxFloor) candidates.push(current + 1)
  // Bias toward higher floor when going higher is possible
  if (current < maxFloor) candidates.push(current + 1)
  return candidates[Math.floor(rng * candidates.length)]
}

export class GameScene {
  readonly container: Container

  private desert:    DesertSkyLayer
  private floor:     FloorLayer
  private world:     Container
  private buildings: ObstacleLayer
  private particles: ParticleLayer
  private sheriff:   SheriffRider
  private rider:     HorseRider
  private flash:     GunfireCrashFX

  private W = 0
  private H = 0
  private charScreenX = 0

  private charWorldX = 0
  private cameraX    = 0
  private shakeAmt   = 0

  // Floor system
  private targetFloorIdx      = 0
  private floorY              = 0          // current smooth Y
  private floorTransitionTimer = 3.5       // seconds until next floor change
  private floorRng            = 0.31       // deterministic rng seed for floor picks

  // Sheriff approach
  private sheriffGap       = 260          // current visual gap (world px)
  private sheriffTargetGap = 260

  // Running dust timer
  private dustTimer = 0

  // Run machine
  private phase:          RunPhase = 'idle'
  private captureElapsed  = 0
  private captureDone     = false

  // Cashout: only triggers visual burst, never stops the runner
  private cashedOut       = false
  private escapeBurstDone = false

  private alreadyCaptured = false
  private prevRoundState: RoundState | null = null
  private unsubs: Array<() => void> = []

  constructor(private readonly app: Application) {
    this.W = app.screen.width
    this.H = app.screen.height
    this.charScreenX = this.W * CHAR_X_FRAC

    this.container = new Container()

    this.desert    = new DesertSkyLayer(this.W, this.H)
    this.floor     = new FloorLayer(this.W, this.H)
    this.world     = new Container()
    this.buildings = new ObstacleLayer(this.W, this.H)
    this.particles = new ParticleLayer()
    this.sheriff   = new SheriffRider()
    this.rider     = new HorseRider()
    this.flash     = new GunfireCrashFX(this.W, this.H)

    // Draw order: buildings behind, then sheriff, then particles, then rider
    this.world.addChild(
      this.buildings.container,
      this.sheriff.container,
      this.particles.container,
      this.rider.container,
    )

    this.container.addChild(
      this.desert.container,
      this.world,
      this.floor.container,    // floor tiles on top for ground-in-front effect
      this.flash.container,
    )

    this.floorY = this.H * FLOOR_Y_FRACS[0]
    this.rider.container.x = this.charWorldX
    this.rider.container.y = this.floorY
    this.sheriff.container.x = this.charWorldX - this.sheriffGap
    this.sheriff.container.y = this.floorY
    this.updateCameraSnap()

    // Subscribe to game state changes
    const unsubState = useGame.subscribe(
      (s) => s.roundState,
      (state) => this.onRoundStateChange(state),
    )
    const unsubCashout = useGame.subscribe(
      (s) => s.cashedOut,
      (v) => {
        if (v && !this.cashedOut) {
          this.cashedOut = true
          this.escapeBurstDone = false
        }
      },
    )
    this.unsubs.push(unsubState, unsubCashout)
    this.app.ticker.add(this.tick)

    const currentState = useGame.getState().roundState
    if (currentState !== null) this.onRoundStateChange(currentState)
  }

  // ── Round state ───────────────────────────────────────────────────────────

  private onRoundStateChange(state: RoundState | null): void {
    const prev = this.prevRoundState
    this.prevRoundState = state

    if (state === 'STARTING' || state === 'CREATED') {
      this.resetRound()
      return
    }
    if (state === 'RUNNING' && prev !== 'RUNNING') {
      this.beginRound()
      return
    }
    if (state === 'CRASHED' && !this.alreadyCaptured) {
      this.startCapture()
    }
  }

  private resetRound(): void {
    this.alreadyCaptured    = false
    this.cashedOut          = false
    this.escapeBurstDone    = false
    this.captureElapsed     = 0
    this.captureDone        = false
    this.shakeAmt           = 0
    this.charWorldX         = 0
    this.targetFloorIdx     = 0
    this.floorTransitionTimer = 3.5
    this.floorRng           = 0.31
    this.sheriffGap         = 260
    this.sheriffTargetGap   = 260
    this.dustTimer          = 0

    this.buildings.reset()
    this.buildings.ensureGenerated(8, 0)
    this.floor.reset()

    this.floorY = this.H * FLOOR_Y_FRACS[0]
    this.rider.container.x = this.charWorldX
    this.rider.container.y = this.floorY
    this.updateCameraSnap()

    this.rider.setState('idle')
    this.sheriff.reset()
    this.phase = 'idle'
  }

  private beginRound(): void {
    const { elapsedMs, lastTickAt } = useGame.getState()
    const interp = elapsedMs + (Date.now() - lastTickAt)

    this.charWorldX = estimateWorldX(interp)
    this.floorY     = this.H * FLOOR_Y_FRACS[0]

    this.buildings.ensureGenerated(Math.ceil(this.charWorldX / 300) + 10, interp)

    this.rider.container.x = this.charWorldX
    this.rider.container.y = this.floorY
    this.updateCameraSnap()

    this.phase = 'running'
    this.rider.setState('running')
    this.sheriff.startRunning()
    this.triggerShake(6)
  }

  private startCapture(): void {
    playKnutSound()
    this.alreadyCaptured = true
    this.captureElapsed  = 0
    this.captureDone     = false
    this.phase           = 'captured'
    this.rider.setState('fall')
    this.sheriff.startAttack(this.sheriffGap, 0)
    this.particles.spawnCrashBurst(this.charWorldX, this.floorY)
    this.triggerShake(15)
  }

  private updateCameraSnap(): void {
    this.cameraX = this.charScreenX - this.charWorldX
    this.world.x = this.cameraX
    this.world.y = 0
  }

  // ── Main tick ─────────────────────────────────────────────────────────────

  private tick = ({ deltaMS }: { deltaMS: number }): void => {
    const dt  = deltaMS / 1000
    const now = Date.now()

    const { roundState, elapsedMs, lastTickAt } = useGame.getState()
    const interpMs = roundState === 'RUNNING'
      ? elapsedMs + (now - lastTickAt)
      : 0

    // Cashout: spawn escape burst exactly once
    if (this.cashedOut && !this.escapeBurstDone && roundState === 'RUNNING') {
      this.escapeBurstDone = true
      this.particles.spawnEscapeBurst(this.charWorldX, this.floorY)
    }

    if (this.phase === 'running')  this.tickRunning(dt, interpMs)
    else if (this.phase === 'captured') this.tickCapture(dt)

    this.updateCamera(dt)

    const camMinWorldX = -this.cameraX - 120
    const camMaxWorldX = -this.cameraX + this.W + 120
    this.buildings.update(dt, camMinWorldX, camMaxWorldX)

    this.particles.update(dt)
    this.rider.update(dt)
    this.desert.update(dt, this.charWorldX, 0)
    this.floor.update(this.charWorldX, this.cameraX)
    this.flash.update(dt)
    this.sheriff.update(dt)
  }

  private tickRunning(dt: number, interpMs: number): void {
    const speed = getRunSpeed(interpMs)
    this.charWorldX += speed * dt

    // Floor transition countdown
    this.floorTransitionTimer -= dt
    if (this.floorTransitionTimer <= 0) {
      this.floorRng = (this.floorRng * 9301 + 49297) % 233280 / 233280  // LCG
      const next = pickNextFloor(this.targetFloorIdx, interpMs, this.floorRng)
      if (next !== this.targetFloorIdx) {
        this.targetFloorIdx = next
      }
      this.floorTransitionTimer = getFloorInterval(interpMs)
    }

    // Smooth Y lerp toward target floor
    const targetY = this.H * FLOOR_Y_FRACS[this.targetFloorIdx]
    this.floorY = expLerp(this.floorY, targetY, 3.5, dt)

    // Position outlaw
    this.rider.container.x = this.charWorldX
    this.rider.container.y = this.floorY

    // Sheriff gap closes over time
    this.sheriffTargetGap = getSheriffTargetGap(interpMs)
    this.sheriffGap = expLerp(this.sheriffGap, this.sheriffTargetGap, 0.9, dt)

    // Sheriff follows on same floor, slightly lower for depth
    this.sheriff.container.x = this.charWorldX - this.sheriffGap
    this.sheriff.container.y = this.floorY + 8

    // Running dust trail
    this.dustTimer -= dt
    if (this.dustTimer <= 0) {
      this.dustTimer = 0.18
      this.particles.spawnLandingDust(
        this.charWorldX - 20,
        this.floorY + 12,
      )
    }

    // Ensure buildings visible ahead
    this.buildings.ensureGenerated(Math.ceil(this.charWorldX / 300) + 10, interpMs)
  }

  private tickCapture(dt: number): void {
    this.captureElapsed += dt

    // Sheriff rushes to close the gap
    if (this.captureElapsed < 0.65) {
      const t = this.captureElapsed / 0.65
      const eased = 1 - Math.pow(1 - t, 2.5)
      this.sheriffGap = lerp(this.sheriffGap, 0, eased)
    } else {
      this.sheriffGap = 0
      if (!this.captureDone) {
        this.captureDone = true
        this.flash.trigger()
        this.triggerShake(20)
        this.particles.spawnLavaSplash(this.charWorldX, this.floorY)
        // Coin spill
        for (let i = 0; i < 3; i++) {
          this.particles.spawnFallCoins(
            this.charWorldX + (Math.random() - 0.5) * 40,
            this.floorY,
          )
        }
      }
    }

    this.sheriff.container.x = this.charWorldX - this.sheriffGap
    this.sheriff.container.y = this.floorY + 8
    this.rider.container.x   = this.charWorldX
    this.rider.container.y   = this.floorY
  }

  // ── Camera ────────────────────────────────────────────────────────────────

  private triggerShake(amount: number): void {
    this.shakeAmt = Math.max(this.shakeAmt, amount)
  }

  private updateCamera(dt: number): void {
    const targetCamX = this.charScreenX - this.charWorldX
    this.cameraX = expLerp(this.cameraX, targetCamX, 5.5, dt)
    this.shakeAmt = Math.max(0, this.shakeAmt - dt * 9 * this.shakeAmt)
    const sx = (Math.random() - 0.5) * this.shakeAmt
    const sy = (Math.random() - 0.5) * this.shakeAmt * 0.3
    this.world.x = this.cameraX + sx
    this.world.y = sy
  }

  // ── Resize ────────────────────────────────────────────────────────────────

  resize(w: number, h: number): void {
    this.W = w; this.H = h
    this.charScreenX = w * CHAR_X_FRAC
    this.floorY = h * FLOOR_Y_FRACS[this.targetFloorIdx]
    this.desert.resize(w, h)
    this.floor.resize(w, h)
    this.buildings.resize(w, h)
    this.flash.resize(w, h)
  }

  // ── Cleanup ───────────────────────────────────────────────────────────────

  destroy(): void {
    this.app.ticker.remove(this.tick)
    this.unsubs.forEach((u) => u())
    this.desert.destroy()
    this.floor.destroy()
    this.buildings.destroy()
    this.particles.destroy()
    this.sheriff.destroy()
    this.rider.destroy()
    this.flash.destroy()
    this.container.destroy({ children: true })
  }
}
