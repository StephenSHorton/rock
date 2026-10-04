import { useLook } from 'mokei/clay'
import { activateScene, registerScene } from 'mokei/scene'
import { Canvas } from '@react-three/fiber'
import { useEffect } from 'react'
import { yardlineScene } from './scene'

export function YardCanvas() {
  useEffect(() => {
    registerScene(yardlineScene)
    activateScene('yardline')
  }, [])

  return (
    <Canvas
      shadows
      dpr={[1, 1.4]}
      gl={{ antialias: true, alpha: false, powerPreference: 'low-power' }}
      onCreated={({ gl }) => {
        gl.setClearColor(useLook.getState().skyColor)
      }}
    >
      <yardlineScene.World />
    </Canvas>
  )
}

export default YardCanvas
