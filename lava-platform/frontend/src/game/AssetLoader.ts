import { Assets } from 'pixi.js'
import { ASSET_PATHS } from './config'

const ALL_ASSETS: string[] = [
  ...Object.values(ASSET_PATHS.environment),
  ...Object.values(ASSET_PATHS.horse),
  ...Object.values(ASSET_PATHS.hero),
  ...Object.values(ASSET_PATHS.platforms),
  ...Object.values(ASSET_PATHS.bg),
]

// Loads all game sprites into the Assets cache before the scene is created.
// Any individual failure is caught and logged — the game falls back to
// procedural graphics for assets that couldn't be fetched.
export async function preloadGameAssets(): Promise<void> {
  const results = await Promise.allSettled(
    ALL_ASSETS.map((url) => Assets.load(url)),
  )
  const failed = results
    .map((r, i) => (r.status === 'rejected' ? ALL_ASSETS[i] : null))
    .filter(Boolean)

  if (failed.length === 0) {
    console.log('[Assets] all game assets loaded ✓')
  } else {
    console.warn('[Assets] failed to load — procedural fallback for:', failed)
  }
}
