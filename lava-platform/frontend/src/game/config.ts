// Asset paths — all filenames use double extensions as they exist on disk
export const ASSET_PATHS = {
  environment: {
    floor:  '/assets/environment/floor/seamless_desert_floor.png',
    floor2: '/assets/environment/floor/seamless_desert_floor2.png',
    floor3: '/assets/environment/floor/seamless_desert_floor3.png',
    floor4: '/assets/environment/floor/seamless_desert_floor4.png',
  },
  horse: {
    idle:   '/assets/horse/horse_idle.png.png',
    gallop: '/assets/horse/horse_gallop.png.png',
    run:    '/assets/horse/horse_run.png.png',
    fall:   '/assets/horse/horse_fall.png.png',
  },
  hero: {
    idle: '/assets/hero/hero_idle.png.png',
    run:  '/assets/hero/hero_run.png.png',
    jump: '/assets/hero/hero_jump.png.png',
    fall: '/assets/hero/hero_fall.png.png',
  },
  platforms: {
    main:        '/assets/platforms/platform_main_round.png.png',
    desert:      '/assets/platforms/platform_desert.png.png',
    bridge:      '/assets/platforms/platform_bridge_rope.png.png',
    ground:      '/assets/platforms/ground_desert_layer.png.png',
    spires:      '/assets/platforms/decor_canyon_spires.png.png',
    spikes:      '/assets/platforms/hazard_spike_cluster.png.png',
    brokenLeft:         '/assets/platforms/platform_main_broken_left.png',
    brokenRight:        '/assets/platforms/platform_main_broken_right.png',
    desertBrokenLeft:   '/assets/platforms/platform_desert_broken_left.png',
    desertBrokenRight:  '/assets/platforms/platform_desert_broken_right.png',
  },
  bg: {
    dawn:   '/assets/bg/bg_betting_dawn.png.jpeg',
    sunset: '/assets/bg/bg_running_sunset.png.jpeg',
  },
} as const

// Scale for horse+hero sprite (tune after first render — see in-game)
export const HORSE_SCALE = 0.12

// Transparent pixels below the hooves in the hero PNG at HORSE_SCALE rendering.
// Hero PNG 2000×1090 at scale 0.12 → 130.8px tall.  ~37% empty below hooves ≈ 48px.
//
// Usage: heroSprite.anchor.y = 1.0 (bottom of PNG), heroSprite.y = HORSE_HOOF_OFFSET_Y
//        rider.container.y   = def.worldY  (platform surface — no extra offset needed)
//
// To recalibrate: if hooves sink, decrease this value; if hooves float, increase it.
export const HORSE_HOOF_OFFSET_Y = 48
