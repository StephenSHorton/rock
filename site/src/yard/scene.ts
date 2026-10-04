import { lookDefaults } from 'mokei/clay'
import type { SceneDefinition } from 'mokei/scene'
import { World } from '../../node_modules/mokei/src/scenes/yardline/World.tsx'

export const yardlineScene = {
  id: 'yardline',
  name: 'Yardline',
  description: 'Clay-diorama warehouse yard with docks, forklifts, and trucks.',
  themeId: 'yardline',
  World,
  camera: {
    zoom: lookDefaults.cameraZoom,
    azimuth: lookDefaults.cameraAzimuth,
    elevation: lookDefaults.cameraElevation,
    target: { x: -1.5, z: -5.5 },
    zoomMin: lookDefaults.zoomMin,
    zoomMax: lookDefaults.zoomMax,
    zoomReferenceWidth: lookDefaults.zoomReferenceWidth,
  },
  look: lookDefaults,
} satisfies SceneDefinition
