import { applyLook, lookDefaults, useLook } from 'mokei/clay'
import { Canvas } from '@react-three/fiber'
import { useEffect } from 'react'
import { quarryLook } from './palette'
import { quarryScene } from './scene'
import { bindLook } from './sim'
import { World } from './World'

export function QuarryCanvas({ pretty }: { pretty: boolean }) {
  useEffect(() => {
    applyLook({
      ...lookDefaults,
      ...quarryScene.look,
      cameraZoom: quarryScene.camera?.zoom,
      cameraAzimuth: quarryScene.camera?.azimuth,
      cameraElevation: quarryScene.camera?.elevation,
      panX: 0,
      panZ: 0,
    })
    bindLook({
      zoomBy: (delta) => useLook.getState().zoomBy(delta),
      rotateBy: (deg) => useLook.getState().rotateBy(deg),
      resetView: () =>
        applyLook({
          cameraZoom: quarryScene.camera?.zoom,
          cameraAzimuth: quarryScene.camera?.azimuth,
          cameraElevation: quarryScene.camera?.elevation,
          panX: 0,
          panZ: 0,
        }),
    })
    return () => bindLook(null)
  }, [])

  return (
    <Canvas
      shadows
      dpr={pretty ? [1, 1.6] : [1, 1.2]}
      gl={{ antialias: true, alpha: false, powerPreference: pretty ? 'high-performance' : 'low-power' }}
      onCreated={({ gl }) => {
        gl.setClearColor(quarryLook.skyColor)
      }}
    >
      <World pretty={pretty} />
    </Canvas>
  )
}

export default QuarryCanvas
