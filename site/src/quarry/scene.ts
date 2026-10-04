import type { SceneDefinition } from 'mokei/scene'
import { lazy } from 'react'
import { quarryLook } from './palette'
import { useQuarry } from './sim'

const QuarryWorld = lazy(async () => {
  const { World } = await import('./World')
  return { default: World }
})

export const quarryScene = {
  id: 'quarry',
  name: 'Clay quarry',
  description: 'Prompt is a drill. Agents are ore carts. Tools are stations. Output is crates.',
  themeId: 'quarry',
  World: QuarryWorld,
  camera: {
    zoom: 52,
    azimuth: 32,
    elevation: 36,
    target: { x: 0.6, z: 0.15 },
    zoomMin: 28,
    zoomMax: 72,
    zoomReferenceWidth: 1728,
  },
  look: quarryLook,
  dispatch: (event) => useQuarry.getState().dispatch(event),
} satisfies SceneDefinition
