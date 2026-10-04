import { useLook } from 'mokei/clay'
import { activateScene } from 'mokei/scene'
import { Canvas } from '@react-three/fiber'
import { useEffect } from 'react'
import { bindLook } from './sim'
import { World } from './World'

export function QuarryCanvas({ pretty }: { pretty: boolean }) {
  useEffect(() => {
    activateScene('quarry')
    bindLook({
      zoomBy: (delta) => useLook.getState().zoomBy(delta),
      rotateBy: (deg) => useLook.getState().rotateBy(deg),
      resetView: () => useLook.getState().resetView(),
    })
    return () => bindLook(null)
  }, [])

  return (
    <Canvas
      shadows
      dpr={pretty ? [1, 1.6] : [1, 1.2]}
      gl={{ antialias: true, alpha: false, powerPreference: pretty ? 'high-performance' : 'low-power' }}
      onCreated={({ gl }) => {
        gl.setClearColor(useLook.getState().skyColor)
      }}
    >
      <World pretty={pretty} />
    </Canvas>
  )
}

export default QuarryCanvas
