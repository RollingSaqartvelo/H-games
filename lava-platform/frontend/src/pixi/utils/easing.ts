/** Smooth deceleration — fast start, gradual stop. */
export function easeOut(t: number): number {
  return 1 - (1 - t) ** 3
}

/** Smooth acceleration. */
export function easeIn(t: number): number {
  return t * t * t
}

/** Symmetric ease in-out. */
export function easeInOut(t: number): number {
  return t < 0.5 ? 4 * t ** 3 : 1 - (-2 * t + 2) ** 3 / 2
}

/** Linear interpolation. */
export function lerp(a: number, b: number, t: number): number {
  return a + (b - a) * t
}

/** Exponential lerp — smooth approach with no overshoot. */
export function expLerp(current: number, target: number, speed: number, dt: number): number {
  return lerp(current, target, 1 - Math.exp(-speed * dt))
}

/** Clamp value between min and max. */
export function clamp(v: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, v))
}

/**
 * Map a multiplier (1.0 → ∞) to a 0..1 progress value using log scale.
 * maxMult is the multiplier at which the visual reaches 100%.
 */
export function multToRatio(mult: number, maxMult = 10): number {
  return clamp(Math.log(mult) / Math.log(maxMult), 0, 1)
}
