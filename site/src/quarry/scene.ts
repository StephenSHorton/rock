import { applyLook, lookDefaults } from '../compat/look'
import { applyTheme } from 'mokei/theme'
import { quarryLook } from './palette'
import { useQuarry } from './sim'
import type { SceneEvent } from './types'

/**
 * Host-side scene activation.
 *
 * The kit's `activateScene` / `getScene` live in `mokei/scene`, which
 * statically imports the Yardline warehouse. Rock only needs clay + theme,
 * so we apply the same contract here without pulling that world.
 */
export const quarryScene = {
  id: 'quarry',
  name: 'Clay quarry',
  description: 'Prompt is a drill. Agents are ore carts. Tools are stations. Output is crates.',
  themeId: 'quarry',
  camera: {
    zoom: quarryLook.cameraZoom ?? 30,
    azimuth: quarryLook.cameraAzimuth ?? 34,
    elevation: quarryLook.cameraElevation ?? 39,
    target: { x: 0.6, z: 0.15 },
  },
  look: quarryLook,
  dispatch: (event: SceneEvent) => useQuarry.getState().dispatch(event),
}

export function activateQuarry() {
  applyTheme(quarryScene.themeId)
  applyLook({
    ...lookDefaults,
    ...quarryScene.look,
    cameraZoom: quarryScene.camera.zoom,
    cameraAzimuth: quarryScene.camera.azimuth,
    cameraElevation: quarryScene.camera.elevation,
    panX: 0,
    panZ: 0,
  })
  return quarryScene
}
