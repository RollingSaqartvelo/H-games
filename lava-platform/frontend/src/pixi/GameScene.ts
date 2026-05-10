/**
 * GameScene — Western Outlaw Escape crash game.
 *
 * Architecture:
 *   scene root
 *     ├─ desert         (sunset sky + canyon parallax, screen space)
 *     ├─ worldContainer (scrolls horizontally as rider jumps)
 *     │     ├─ obstacles  (PNG platform sprites)
 *     │     ├─ particles  (hoof dust, gold bursts, arrest explosions)
 *     │     └─ rider      (horse + outlaw, the global runner)
 *     └─ flash           (gunfire crash flash overlay)
 *
 * ── CASHOUT BUG FIX ──────────────────────────────────────────────────────────
 * The rider is a GLOBAL RUNNER, not the player's personal character.
 * Personal cashout (cashedOut=true) NEVER changes the jump machine phase.
 * The horse keeps galloping until the round actually crashes.
 *
 * On cashout: spawn gold escape burst at rider position (visual feedback),
 *             continue normal gallop.
 * On crash:   rider falls into the dust cloud, sheriff catches outlaw.
 *
 * This ensures all spectators see a continuous chase regardless of
 * individual cashout events.
 * ─────────────────────────────────────────────────────────────────────────────
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

const CHAR_X_FRAC      = 0.55   // rider fixed at 55% from left
// this.sheriffBehind is computed dynamically so sheriff stays at the same screen X
// when the hero moves: base 105px + compensation for the 10% hero shift
const SHERIFF_BEHIND_BASE = 105
const FLOOR_DISPLAY_H  = 220    // must match FloorLayer DISPLAY_HEIGHT
// Visual ground surface: tile is 220px, cacti bases appear ~130px into the tile from top
const FLOOR_GROUND_INSET = 185

function jumpCycleMs(elapsedMs: number): number {
  if (elapsedMs < 5_000)  return 1_550
  if (elapsedMs < 15_000) return 1_250
  if (elapsedMs < 30_000) return  980
  if (elapsedMs < 60_000) return  720
  return 560
}

function resolvePosition(elapsedMs: number): { platIndex: number; msUntilNextJump: number } {
  let acc = 0; let index = 0
  while (true) {
    const dur = jumpCycleMs(acc)
    if (acc + dur > elapsedMs) return { platIndex: index, msUntilNextJump: acc + dur - elapsedMs }
    acc += dur; index++
  }
}

type JumpPhase = 'idle' | 'crouch' | 'airborne' | 'impact' | 'fall' | 'airTumble'
// NOTE: 'cashout' phase removed — horse always continues until crash

export class GameScene {
  readonly container: Container

  private desert:    DesertSkyLayer
  private floor:     FloorLayer
  private world:     Container
  private obstacles: ObstacleLayer
  private particles: ParticleLayer
  private sheriff:   SheriffRider
  private rider:     HorseRider
  private flash:     GunfireCrashFX

  private W = 0
  private H = 0
  private charScreenX = 0
  private sheriffBehind = 0

  private charWorldX = 0
  private cameraX    = 0
  private shakeAmt   = 0

  private phase:     JumpPhase = 'idle'
  private phaseStart = 0
  private platIndex  = 0
  private nextJumpAt = Infinity

  private jumpFromX = 0; private jumpFromY = 0
  private jumpToX   = 0; private jumpToY   = 0
  private jumpArcH  = 0; private jumpDurMs = 0

  private fallStartY      = 0
  private fallSplashed    = false
  private riderFallStarted  = false
  private lastCoinSpawn     = 0
  private lastJumpCoinSpawn = 0

  private pendingCrash  = false
  private alreadyFallen = false

  // cashedOut: only triggers a visual burst, never stops the jump machine
  private cashedOut       = false
  private escapeBurstDone = false

  private prevRoundState: RoundState | null = null
  private unsubs: Array<() => void> = []

  constructor(private readonly app: Application) {
    this.W = app.screen.width
    this.H = app.screen.height
    this.charScreenX = this.W * CHAR_X_FRAC
    this.sheriffBehind = SHERIFF_BEHIND_BASE + Math.round(this.W * 0.10)

    this.container = new Container()

    this.desert    = new DesertSkyLayer(this.W, this.H)
    this.floor     = new FloorLayer(this.W, this.H)
    this.world     = new Container()
    this.obstacles = new ObstacleLayer(this.W, this.H)
    this.particles = new ParticleLayer()
    this.sheriff   = new SheriffRider()
    this.rider     = new HorseRider()
    this.flash     = new GunfireCrashFX(this.W, this.H)

    this.world.addChild(
      this.sheriff.container,  // behind obstacles/hero — runs on floor below platforms
      this.obstacles.container,
      this.particles.container,
      this.rider.container,
    )

    this.container.addChild(
      this.desert.container,
      this.world,
      this.floor.container,    // floor on top of world so cacti appear in front
      this.flash.container,
    )

    this.obstacles.ensureGenerated(5, 0)
    this.placeRiderOnObstacle(0)
    this.updateCameraSnap()

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
          // Jump machine is NOT modified — horse keeps galloping
        }
      },
    )
    this.unsubs.push(unsubState, unsubCashout)
    this.app.ticker.add(this.tick)

    // If the game is already running when the scene mounts, Zustand's subscribe
    // won't fire (it only fires on changes). Handle the current state immediately.
    const currentState = useGame.getState().roundState
    if (currentState !== null) {
      this.onRoundStateChange(currentState)
    }
  }

  // ── Round state ──────────────────────────────────────────────────────────────

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
    if (state === 'CRASHED' && !this.alreadyFallen) {
      if (this.phase === 'idle' || this.phase === 'crouch') {
        this.startFall()
      } else if (this.phase === 'airborne') {
        this.startAirTumble(Date.now())
      } else {
        // impact / other — break platform immediately, fall on next idle
        this.obstacles.triggerCrash(this.platIndex)
        this.pendingCrash = true
      }
    }
  }

  private resetRound(): void {
    this.pendingCrash     = false
    this.alreadyFallen    = false
    this.cashedOut        = false
    this.escapeBurstDone  = false
    this.fallSplashed     = false
    this.riderFallStarted  = false
    this.lastCoinSpawn     = 0
    this.lastJumpCoinSpawn = 0
    this.platIndex       = 0
    this.nextJumpAt      = Infinity
    this.shakeAmt        = 0

    this.obstacles.reset()
    this.obstacles.ensureGenerated(5, 0)
    this.floor.reset()
    this.placeRiderOnObstacle(0)
    this.updateCameraSnap()
    this.rider.setState('idle')
    this.sheriff.reset()
    this.phase = 'idle'
  }

  private beginRound(): void {
    const { elapsedMs, lastTickAt } = useGame.getState()
    const interp = elapsedMs + (Date.now() - lastTickAt)

    const { platIndex } = resolvePosition(interp)
    this.obstacles.ensureGenerated(platIndex + 6, interp)

    this.platIndex = platIndex
    this.placeRiderOnObstacle(platIndex)
    this.updateCameraSnap()

    this.nextJumpAt = Date.now()   // start jumping immediately on load
    this.phase      = 'idle'
    this.rider.setState('idle')
    this.sheriff.startRunning()
    this.triggerShake(7)
  }

  private placeRiderOnObstacle(index: number): void {
    const def = this.obstacles.getPlatform(index)
    if (!def) return
    this.charWorldX        = def.worldX
    this.rider.container.x = def.worldX
    this.rider.container.y = def.worldY   // surface lock: container origin = platform flat top
  }

  private updateCameraSnap(): void {
    this.cameraX = this.charScreenX - this.charWorldX
    this.world.x = this.cameraX
    this.world.y = 0
  }

  // ── Main tick ────────────────────────────────────────────────────────────────

  private tick = ({ deltaMS }: { deltaMS: number }): void => {
    const dt  = deltaMS / 1000
    const now = Date.now()

    const { roundState, elapsedMs, lastTickAt } = useGame.getState()
    const interpMs = roundState === 'RUNNING'
      ? elapsedMs + (now - lastTickAt)
      : 0

    // Spawn escape burst exactly once on cashout (gold coins visual)
    if (this.cashedOut && !this.escapeBurstDone && roundState === 'RUNNING') {
      this.escapeBurstDone = true
      this.particles.spawnEscapeBurst(
        this.charWorldX,
        this.rider.container.y,
      )
    }

    this.updateJumpMachine(now, interpMs)
    this.updateCamera(dt)

    // Zoom intentionally removed — multiplier must NEVER move hero/floor vertically.
    // Camera is Y-locked; only X scrolls. Floor stays pinned to screen bottom.

    const camMinWorldX = -this.cameraX - 80
    const camMaxWorldX = -this.cameraX + this.W + 80
    this.obstacles.update(dt, camMinWorldX, camMaxWorldX)

    // Sheriff: locked to visual floor surface, always this.sheriffBehind world-px behind hero
    this.sheriff.container.x = this.charWorldX - this.sheriffBehind
    this.sheriff.container.y = this.H - FLOOR_DISPLAY_H + FLOOR_GROUND_INSET
    this.sheriff.update(dt)

    this.particles.update(dt)
    this.rider.update(dt)
    this.desert.update(dt, this.charWorldX, 0)
    this.floor.update(this.charWorldX, this.cameraX)
    this.flash.update(dt)
  }

  // ── Jump machine ─────────────────────────────────────────────────────────────
  // cashedOut flag is IGNORED here — it only triggers particles, not phase changes.
  // The horse gallops continuously until the round CRASHES.

  private updateJumpMachine(now: number, elapsedMs: number): void {
    if (this.phase === 'fall')      { this.tickFall(now);     return }
    if (this.phase === 'airTumble') { this.tickAirTumble(now); return }

    if (useGame.getState().roundState !== 'RUNNING') return

    switch (this.phase) {
      case 'idle':
        if (now >= this.nextJumpAt) {
          if (this.pendingCrash) {
            this.startFall()
          } else {
            this.startCrouch(now)
          }
        }
        break

      case 'crouch':
        if (now - this.phaseStart >= 195) {
          this.startAirborne(now, elapsedMs)
        }
        break

      case 'airborne': {
        // Crash flagged mid-air — start tumble immediately, don't wait for landing
        if (this.pendingCrash) { this.startAirTumble(now); break }

        const elapsed = now - this.phaseStart
        const t = Math.min(elapsed / this.jumpDurMs, 1)
        const wx = lerp(this.jumpFromX, this.jumpToX, t)
        const wy = lerp(this.jumpFromY, this.jumpToY, t) + this.jumpArcH * Math.sin(Math.PI * t)
        this.rider.container.x = wx
        this.rider.container.y = wy
        this.charWorldX = wx

        // Coin trail behind hero during jump
        if (now - this.lastJumpCoinSpawn > 100) {
          this.lastJumpCoinSpawn = now
          this.particles.spawnJumpCoinTrail(wx, wy)
        }

        if (t >= 1) {
          this.startImpact(now, this.platIndex + 1)
        }
        break
      }

      case 'impact':
        if (now - this.phaseStart >= 175) {
          this.rider.setState('idle')
          this.phase = 'idle'
          this.nextJumpAt = now + jumpCycleMs(elapsedMs) * 0.28
        }
        break
    }
  }

  private startCrouch(now: number): void {
    const { elapsedMs, lastTickAt } = useGame.getState()
    const interp = elapsedMs + (Date.now() - lastTickAt)
    this.obstacles.ensureGenerated(this.platIndex + 6, interp)
    this.phase      = 'crouch'
    this.phaseStart = now
    this.rider.setState('crouch')
  }

  private startAirborne(now: number, elapsedMs: number): void {
    const fromDef = this.obstacles.getPlatform(this.platIndex)
    const toDef   = this.obstacles.getPlatform(this.platIndex + 1)
    if (!fromDef || !toDef) return

    const foot = 14
    this.jumpFromX = fromDef.worldX
    this.jumpFromY = fromDef.worldY - foot
    this.jumpToX   = toDef.worldX
    this.jumpToY   = toDef.worldY - foot

    const dy = Math.abs(this.jumpToY - this.jumpFromY)
    this.jumpArcH  = -(70 + dy * 0.25 + Math.random() * 18)
    this.jumpDurMs = jumpCycleMs(elapsedMs) * 0.60

    this.phase      = 'airborne'
    this.phaseStart = now
    this.rider.setState('airborne')
  }

  private startImpact(now: number, newIndex: number): void {
    this.platIndex = newIndex
    this.placeRiderOnObstacle(newIndex)
    this.charWorldX = this.rider.container.x

    this.phase      = 'impact'
    this.phaseStart = now
    this.rider.setState('impact')
    this.triggerShake(5)

    this.particles.spawnLandingDust(
      this.charWorldX,
      this.rider.container.y + 14 + 4,
    )
  }

  private startAirTumble(now: number): void {
    playKnutSound()
    this.pendingCrash     = false
    this.alreadyFallen    = true
    this.fallStartY       = this.rider.container.y   // physics baseline set here
    this.fallSplashed     = false
    this.riderFallStarted = false
    this.lastCoinSpawn    = now

    this.phase      = 'airTumble'
    this.phaseStart = now
    this.rider.setState('tumble')
    this.obstacles.triggerCrash(this.platIndex)
    this.particles.spawnCrashBurst(this.charWorldX, this.fallStartY)
    this.triggerShake(12)

    // Sheriff lasso throw: hero local coords relative to sheriff container
    this.sheriff.startAttack(
      this.sheriffBehind,                                       // heroLocalX
      this.rider.container.y - (this.H - FLOOR_DISPLAY_H + FLOOR_GROUND_INSET),
    )
  }

  private tickAirTumble(now: number): void {
    const elapsed = (now - this.phaseStart) / 1000
    const gravity = 380

    this.rider.container.y = this.fallStartY + 0.5 * gravity * elapsed * elapsed

    // Chaotic coin spill during tumble
    if (now - this.lastCoinSpawn > 85) {
      this.lastCoinSpawn = now
      this.particles.spawnFallCoins(this.charWorldX, this.rider.container.y)
    }

    // After ~0.30 s, switch to hero_fall.png — physics timeline is UNCHANGED
    // (same phaseStart + fallStartY) so tickFall continues seamlessly
    if (!this.riderFallStarted && elapsed >= 0.30) {
      this.riderFallStarted = true
      this.rider.setState('fall')
      this.phase = 'fall'
      return
    }

    const splashY = this.H * 0.82
    if (!this.fallSplashed && this.rider.container.y >= splashY) {
      this.fallSplashed = true
      this.particles.spawnLavaSplash(this.charWorldX, splashY)
      this.flash.trigger()
      this.triggerShake(16)
    }
  }

  private startFall(): void {
    playKnutSound()
    this.pendingCrash     = false
    this.alreadyFallen    = true
    this.fallStartY       = this.rider.container.y
    this.fallSplashed     = false
    this.riderFallStarted = false
    this.lastCoinSpawn    = 0

    this.phase      = 'fall'
    this.phaseStart = Date.now()
    // Platform breaks and particles fire immediately
    this.obstacles.triggerCrash(this.platIndex)
    this.particles.spawnCrashBurst(this.charWorldX, this.fallStartY)
    this.triggerShake(20)
    // rider.setState('fall') is intentionally delayed — see tickFall

    // Sheriff lasso throw: hero local coords relative to sheriff container
    this.sheriff.startAttack(
      this.sheriffBehind,
      this.rider.container.y - (this.H - FLOOR_DISPLAY_H + FLOOR_GROUND_INSET),
    )
  }

  private tickFall(now: number): void {
    const elapsed = (now - this.phaseStart) / 1000
    const gravity = 380

    // Switch hero to fall pose after short delay (platform breaks first visually)
    if (!this.riderFallStarted && elapsed >= 0.18) {
      this.riderFallStarted = true
      this.rider.setState('fall')
    }

    this.rider.container.y = this.fallStartY + 0.5 * gravity * elapsed * elapsed

    // Spill gold coins continuously while hero is falling and still on screen
    if (this.riderFallStarted && elapsed < 2.2 && now - this.lastCoinSpawn > 115) {
      this.lastCoinSpawn = now
      this.particles.spawnFallCoins(this.charWorldX, this.rider.container.y)
    }

    const splashY = this.H * 0.82
    if (!this.fallSplashed && this.rider.container.y >= splashY) {
      this.fallSplashed = true
      this.particles.spawnLavaSplash(this.charWorldX, splashY)
      this.flash.trigger()
      this.triggerShake(16)
    }
  }

  // ── Camera ───────────────────────────────────────────────────────────────────

  private triggerShake(amount: number): void {
    this.shakeAmt = Math.max(this.shakeAmt, amount)
  }

  private updateCamera(dt: number): void {
    const targetCamX = this.charScreenX - this.charWorldX
    this.cameraX = expLerp(this.cameraX, targetCamX, 5.5, dt)
    this.shakeAmt = Math.max(0, this.shakeAmt - dt * 9 * this.shakeAmt)
    const sx = (Math.random() - 0.5) * this.shakeAmt
    const sy = (Math.random() - 0.5) * this.shakeAmt * 0.4
    this.world.x = this.cameraX + sx
    this.world.y = sy
  }

  // ── Resize ───────────────────────────────────────────────────────────────────

  resize(w: number, h: number): void {
    this.W = w; this.H = h
    this.charScreenX = w * CHAR_X_FRAC
    this.sheriffBehind = SHERIFF_BEHIND_BASE + Math.round(w * 0.10)
    this.desert.resize(w, h)
    this.floor.resize(w, h)
    this.obstacles.resize(w, h)
    this.flash.resize(w, h)
  }

  // ── Cleanup ──────────────────────────────────────────────────────────────────

  destroy(): void {
    this.app.ticker.remove(this.tick)
    this.unsubs.forEach((u) => u())
    this.desert.destroy()
    this.floor.destroy()
    this.obstacles.destroy()
    this.particles.destroy()
    this.sheriff.destroy()
    this.rider.destroy()
    this.flash.destroy()
    this.container.destroy({ children: true })
  }
}
