import { Container, Sprite, Texture, Assets, VideoSource, Filter, GlProgram } from 'pixi.js'

const VIDEO_SRC = '/video/sheriff_run3.mp4'
const LASSO_SRC = '/assets/lasso/lasso.png'

// Video is 1080×1920 (portrait). Rendered height so width ≈ hero's 242px:
//   scale = 340/1920 = 0.177  →  width = 1080 × 0.177 ≈ 191px
const SHERIFF_DISPLAY_H = 340

// Fraction from the TOP of the video frame where the horse hooves are.
const SHERIFF_HOOF_ANCHOR_Y = 0.59

// Sheriff hand position in local container coordinates (lasso launch point)
const HAND_X = -12
const HAND_Y = -88

// ── Luma-key filter ───────────────────────────────────────────────────────────

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

type SheriffState = 'hidden' | 'running' | 'attacking'

export class SheriffRider {
  readonly container: Container
  private sprite:      Sprite
  private lassoSprite: Sprite
  private videoEl: HTMLVideoElement | null = null
  private source:  VideoSource | null = null

  private state:   SheriffState = 'hidden'
  private time     = 0
  private attackT  = 0
  private lassoTX  = 0
  private lassoTY  = 0

  constructor() {
    this.container = new Container()
    this.container.visible = false

    // Lasso PNG: loop is at ~60% from left, 25% from top; tail at bottom-left.
    // Anchor at the loop so "position" = loop position flying toward hero.
    this.lassoSprite = new Sprite()
    this.lassoSprite.anchor.set(0.60, 0.25)
    this.lassoSprite.visible = false

    this.sprite = new Sprite()
    this.sprite.anchor.set(0.5, SHERIFF_HOOF_ANCHOR_Y)
    this.sprite.y = 0
    this.sprite.filters = [makeLumaKeyFilter()]

    // z-order: lasso behind sheriff sprite during travel, will flip at attach
    this.container.addChild(this.lassoSprite, this.sprite)

    this.loadVideo()
    this.loadLasso()
  }

  // ── Video init ────────────────────────────────────────────────────────────────

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
    // Must be in DOM for iOS WKWebView (Telegram) to load video metadata
    v.style.cssText = 'position:fixed;width:1px;height:1px;opacity:0;pointer-events:none;top:-9999px;left:-9999px;'
    document.body.appendChild(v)

    const apply = () => {
      if (!v.videoWidth || !v.videoHeight) return
      this.source = new VideoSource({
        resource: v, autoPlay: false, loop: true,
        width: v.videoWidth, height: v.videoHeight,
      })
      const tex = new Texture({ source: this.source })
      this.sprite.texture = tex
      const scale = SHERIFF_DISPLAY_H / v.videoHeight
      this.sprite.scale.set(scale, scale)
      this.videoEl = v
      if (this.state === 'running') void v.play().catch(() => {})
    }

    v.addEventListener('loadedmetadata', apply, { once: true })
    v.addEventListener('canplay', () => { if (!this.videoEl) apply() }, { once: true })
    v.load()
  }

  // ── Lasso texture ─────────────────────────────────────────────────────────────

  private loadLasso(): void {
    Assets.load(LASSO_SRC).then((tex: Texture) => {
      this.lassoSprite.texture = tex
    }).catch(() => {})
  }

  // ── State transitions ─────────────────────────────────────────────────────────

  startRunning(): void {
    this.state = 'running'
    this.time  = 0
    this.sprite.rotation = 0
    this.lassoSprite.visible = false
    this.container.visible = true
    void this.videoEl?.play().catch(() => {})
  }

  startAttack(heroLocalX: number, heroLocalY: number): void {
    if (this.state !== 'running') return
    this.state   = 'attacking'
    this.attackT = 0
    this.lassoTX = heroLocalX
    this.lassoTY = heroLocalY
    this.videoEl?.pause()
  }

  reset(): void {
    this.state = 'hidden'
    this.container.visible = false
    this.sprite.rotation = 0
    this.lassoSprite.visible = false
    this.time    = 0
    this.attackT = 0
    if (this.videoEl) {
      this.videoEl.pause()
      this.videoEl.currentTime = 0
    }
  }

  // ── Per-frame update ──────────────────────────────────────────────────────────

  update(dt: number): void {
    if (this.state === 'hidden') return

    this.time += dt

    if (this.state === 'running') {
      this.sprite.rotation = -0.06
      this.lassoSprite.visible = false
    }

    if (this.state === 'attacking') {
      this.attackT = Math.min(this.attackT + dt / 0.38, 1)
      const targetRot = -0.62
      this.sprite.rotation += (targetRot - this.sprite.rotation) * Math.min(dt * 14, 1)
      this.sprite.y = -Math.sin(Math.PI * this.attackT) * 13
      this.updateLasso()
    }
  }

  // ── Lasso PNG animation ───────────────────────────────────────────────────────
  //
  // The lasso.png sprite flies from the sheriff's hand to the hero.
  // No procedural lines, curves, or canvas drawing — PNG only.

  // Image anatomy: loop at upper-right (~60%,25%), tail at bottom-left.
  // Natural tail-to-loop angle ≈ −70° (atan2(-950,350) ≈ −1.22 rad).
  // Rotation formula: atan2(dy,dx) + 1.22 makes tail point back toward sheriff.
  private updateLasso(): void {
    const t = this.attackT

    if (t < 0.04 || !this.lassoSprite.texture) {
      this.lassoSprite.visible = false
      return
    }

    this.lassoSprite.visible = true

    const dx = this.lassoTX - HAND_X
    const dy = this.lassoTY - HAND_Y
    const easedT = 1 - Math.pow(1 - t, 1.8)

    // Loop (anchor) flies from sheriff hand toward hero
    this.lassoSprite.x = HAND_X + dx * easedT
    this.lassoSprite.y = HAND_Y + dy * easedT

    // Rotate: compensate image natural angle so tail trails back to sheriff
    this.lassoSprite.rotation = Math.atan2(dy, dx) + 1.22

    // Small uniform scale — image is very tall so keep it small
    const snap = t > 0.88 ? 1 + Math.sin(((t - 0.88) / 0.12) * Math.PI) * 0.12 : 1
    this.lassoSprite.scale.set(0.08 * snap)

    if (t > 0.92) {
      this.container.setChildIndex(this.lassoSprite, this.container.children.length - 1)
    }
  }

  // ── Cleanup ───────────────────────────────────────────────────────────────────

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
