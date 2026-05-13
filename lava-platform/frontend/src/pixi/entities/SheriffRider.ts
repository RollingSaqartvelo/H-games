/**
 * SheriffRunner — aggressive pursuer chasing the outlaw through western town.
 *
 * Uses video sprite (sheriff_run3.mp4) when available, falls back to
 * procedural running sheriff graphics.
 *
 * On crash: draws revolver, fires with muzzle flash + rotation.
 */

import { Container, Sprite, Texture, VideoSource, Filter, GlProgram, Graphics } from 'pixi.js'

const VIDEO_SRC = '/assets/outlaw/sheriff_run_shoot.mp4'

const SHERIFF_DISPLAY_H   = 300
const SHERIFF_HOOF_ANCHOR = 0.88

// ── Luma-key filter (removes black background from video) ─────────────────────

const LUMA_VERT = `
in vec2 aPosition;
out vec2 vTextureCoord;
uniform vec4 uInputSize;
uniform vec4 uOutputFrame;
uniform vec4 uOutputTexture;

vec4 filterVertexPosition(void) {
  vec2 position = aPosition * uOutputFrame.zw + uOutputFrame.xy;
  position.x = position.x * (2.0 / uOutputTexture.x) - 1.0;
  position.y = position.y * (2.0 * uOutputTexture.z / uOutputTexture.y) - uOutputTexture.z;
  return vec4(position, 0.0, 1.0);
}
vec2 filterTextureCoord(void) {
  return aPosition * (uOutputFrame.zw * uInputSize.zw);
}
void main(void) {
  gl_Position = filterVertexPosition();
  vTextureCoord = filterTextureCoord();
}
`

const LUMA_FRAG = `
in vec2 vTextureCoord;
out vec4 finalColor;
uniform sampler2D uTexture;
void main(void) {
  vec4 color = texture(uTexture, vTextureCoord);
  float luma = dot(color.rgb, vec3(0.299, 0.587, 0.114));
  float alpha = smoothstep(0.02, 0.10, luma);
  finalColor = vec4(color.rgb, alpha);
}
`

function makeLumaKeyFilter(): Filter {
  return new Filter({
    glProgram: new GlProgram({ vertex: LUMA_VERT, fragment: LUMA_FRAG }),
    resources: {},
  })
}

// ─────────────────────────────────────────────────────────────────────────────

type SheriffState = 'hidden' | 'running' | 'shooting'

export class SheriffRider {
  readonly container: Container

  private videoSprite: Sprite
  private muzzleFlash: Graphics
  private proceduralBody: Graphics
  private videoEl:  HTMLVideoElement | null = null
  private source:   VideoSource | null = null
  private hasVideo  = false

  private state:        SheriffState = 'hidden'
  private time          = 0
  private shootT        = 0
  constructor() {
    this.container = new Container()
    this.container.visible = false

    this.videoSprite = new Sprite()
    this.videoSprite.anchor.set(0.5, SHERIFF_HOOF_ANCHOR)
    this.videoSprite.filters = [makeLumaKeyFilter()]

    this.proceduralBody = new Graphics()
    this.muzzleFlash = new Graphics()

    this.container.addChild(this.videoSprite, this.proceduralBody, this.muzzleFlash)

    this.loadVideo()
  }

  // ── Video loading ─────────────────────────────────────────────────────────

  private loadVideo(): void {
    const v = document.createElement('video')
    v.src         = VIDEO_SRC
    v.loop        = true
    v.muted       = true
    v.volume      = 0
    v.playsInline = true
    v.preload     = 'auto'
    v.setAttribute('muted', '')
    v.setAttribute('webkit-playsinline', 'true')
    v.style.cssText = 'position:fixed;width:1px;height:1px;opacity:0;pointer-events:none;top:-9999px;left:-9999px;'
    document.body.appendChild(v)

    const apply = () => {
      if (!v.videoWidth || !v.videoHeight) return
      this.source = new VideoSource({
        resource: v, autoPlay: false, loop: true,
        width: v.videoWidth, height: v.videoHeight,
      })
      const tex = new Texture({ source: this.source })
      this.videoSprite.texture = tex
      const scale = SHERIFF_DISPLAY_H / v.videoHeight
      this.videoSprite.scale.set(scale)
      this.videoEl = v
      this.hasVideo = true
      this.proceduralBody.visible = false
      if (this.state === 'running') void v.play().catch(() => {})
    }

    v.addEventListener('loadedmetadata', apply, { once: true })
    v.addEventListener('canplay', () => { if (!this.videoEl) apply() }, { once: true })
    v.load()
  }

  // ── State transitions ─────────────────────────────────────────────────────

  startRunning(): void {
    this.state  = 'running'
    this.time   = 0
    this.shootT = 0
        this.container.visible = true
    this.videoSprite.rotation = 0
    this.muzzleFlash.clear()
    void this.videoEl?.play().catch(() => {})

    if (!this.hasVideo) {
      this.proceduralBody.visible = true
      this.buildSheriff()
    }
  }

  // Called by GameScene on crash: heroLocalX is the gap to close
  startAttack(heroLocalX: number, _heroLocalY: number): void {
    if (this.state !== 'running') return
    this.state     = 'shooting'
    this.shootT    = 0
        this.videoEl?.pause()
    // Store gap for visual effect
    void heroLocalX
  }

  reset(): void {
    this.state  = 'hidden'
    this.container.visible = false
    this.videoSprite.rotation = 0
    this.muzzleFlash.clear()
    this.time      = 0
    this.shootT    = 0
        if (this.videoEl) {
      this.videoEl.pause()
      this.videoEl.currentTime = 0
    }
  }

  // ── Per-frame update ──────────────────────────────────────────────────────

  update(dt: number): void {
    if (this.state === 'hidden') return
    this.time += dt

    if (this.state === 'running') {
      this.tickRunning()
    } else if (this.state === 'shooting') {
      this.tickShooting(dt)
    }
  }

  private tickRunning(): void {
    if (this.hasVideo) {
      // Slight forward lean, matching outlaw's intensity
      this.videoSprite.rotation = -0.07
    } else {
      this.buildSheriff(false)
    }
    this.muzzleFlash.clear()
  }

  private tickShooting(dt: number): void {
    this.shootT += dt

    if (this.hasVideo) {
      // Sheriff rears back as he fires
      const targetRot = -0.55
      this.videoSprite.rotation += (targetRot - this.videoSprite.rotation) * Math.min(dt * 12, 1)
    } else {
      this.buildSheriff(true)
    }

    // Muzzle flash pulse
    if (this.shootT < 0.35) {
      const pulse = Math.max(0, 1 - this.shootT / 0.35)
      this.drawMuzzleFlash(pulse)
    } else {
      this.muzzleFlash.clear()
    }
  }

  // ── Muzzle flash ──────────────────────────────────────────────────────────

  private drawMuzzleFlash(intensity: number): void {
    this.muzzleFlash.clear()
    if (intensity <= 0) return
    const alpha = intensity * 0.9
    // Star burst at revolver muzzle position
    const mx = 60    // in front of sheriff (positive X = forward)
    const my = -80   // at hand height
    this.muzzleFlash.circle(mx, my, 18 * intensity).fill({ color: 0xffdd44, alpha })
    this.muzzleFlash.circle(mx, my, 10 * intensity).fill({ color: 0xffffff, alpha: alpha * 0.8 })
    // Rays
    for (let i = 0; i < 6; i++) {
      const a  = (i / 6) * Math.PI * 2
      const r  = 24 * intensity
      this.muzzleFlash.poly([
        mx, my,
        mx + Math.cos(a - 0.15) * r, my + Math.sin(a - 0.15) * r,
        mx + Math.cos(a + 0.15) * r, my + Math.sin(a + 0.15) * r,
      ]).fill({ color: 0xffaa00, alpha: alpha * 0.6 })
    }
  }

  // ── Procedural sheriff ────────────────────────────────────────────────────

  private buildSheriff(shooting = false): void {
    this.proceduralBody.clear()

    const skin   = 0xe0b080
    const shirt  = shooting ? 0x556699 : 0x446688   // blue/grey uniform shirt
    const pants  = 0x2a3044   // dark navy pants
    const hat    = 0x2a2a1a   // dark hat
    const boot   = 0x1a1000
    const badge  = 0xffd700   // gold star badge

    const run    = Math.sin(this.time * 9.5)

    // Boots
    const kick = shooting ? 0 : run * 16
    this.proceduralBody.roundRect(-8, -8, 8, 10, 2).fill({ color: boot })
    this.proceduralBody.roundRect(2, -8, 8, 10, 2).fill({ color: boot })
    this.proceduralBody.roundRect(-10 + kick * 0.5, -1, 7, 4, 2).fill({ color: boot })
    this.proceduralBody.roundRect(4 - kick * 0.5, -1, 7, 4, 2).fill({ color: boot })

    // Pants
    const legSwing = shooting ? 0 : run * 18
    this.proceduralBody.poly([-8, -28, 0, -28, legSwing * 0.4, -8, -8 + legSwing * 0.3, -8]).fill({ color: pants })
    this.proceduralBody.poly([0, -28, 8, -28, 8 - legSwing * 0.3, -8, -legSwing * 0.4, -8]).fill({ color: pants })

    // Belt
    this.proceduralBody.rect(-8, -30, 18, 3).fill({ color: 0x2a1a00 })
    // Holster
    this.proceduralBody.roundRect(6, -30, 6, 10, 2).fill({ color: 0x3a2a00 })

    // Torso
    this.proceduralBody.roundRect(-8, -54, 18, 26, 4).fill({ color: shirt })
    // Star badge
    this.proceduralBody.circle(-2, -44, 5).fill({ color: badge })
    this.proceduralBody.circle(-2, -44, 3).fill({ color: 0xffffff, alpha: 0.5 })

    // Head / neck
    this.proceduralBody.roundRect(-3, -62, 8, 10, 3).fill({ color: skin })
    this.proceduralBody.circle(1, -68, 8).fill({ color: skin })

    // Moustache
    this.proceduralBody.ellipse(-2, -64, 4, 2).fill({ color: 0x4a3020, alpha: 0.9 })
    this.proceduralBody.ellipse(4, -64, 4, 2).fill({ color: 0x4a3020, alpha: 0.9 })

    // Hat
    this.proceduralBody.rect(-12, -76, 26, 3).fill({ color: hat })
    this.proceduralBody.roundRect(-6, -93, 14, 19, 3).fill({ color: hat })
    this.proceduralBody.rect(-6, -76, 14, 3).fill({ color: 0xd4a017, alpha: 0.7 })

    // Arm with revolver
    const armAngle = shooting ? 0.6 : run * 0.45
    const armX = 10 + Math.sin(armAngle) * 16
    const armY = -48 + Math.abs(Math.cos(armAngle)) * 5
    this.proceduralBody.roundRect(7, -54, 6, 20, 3).fill({ color: shirt })
    this.proceduralBody.roundRect(armX - 3, armY, 6, 8, 2).fill({ color: skin })
    // Revolver
    this.proceduralBody.roundRect(armX - 1, armY - 2, 3, 11, 1).fill({ color: 0x222222 })
    this.proceduralBody.roundRect(armX - 3, armY + 2, 3, 4, 1).fill({ color: 0x111111 })
  }

  // ── Cleanup ───────────────────────────────────────────────────────────────

  destroy(): void {
    if (this.videoEl) {
      this.videoEl.pause()
      this.videoEl.src = ''
      this.videoEl.remove()
    }
    this.source?.destroy()
    this.container.destroy({ children: true })
  }
}
