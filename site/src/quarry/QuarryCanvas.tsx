import { Canvas } from '@react-three/fiber'
import { World } from './World'
import { quarryLook } from './palette'

export function QuarryCanvas({ pretty }: { pretty: boolean }) {
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
