import { RoundCyl, SoftBox, Wheel } from 'mokei/clay'
import { useFrame } from '@react-three/fiber'
import { useRef } from 'react'
import type { Group } from 'three'
import type { StationId } from './types'

type V3 = [number, number, number]

export function Bench({ position, size }: { position: V3; size: V3 }) {
  return <SoftBox size={size} r={0.16} material="ground" position={position} cast={false} />
}

export function RailPath({ points }: { points: { x: number; z: number }[] }) {
  const segs = []
  for (let i = 0; i < points.length - 1; i += 1) {
    const a = points[i]
    const b = points[i + 1]
    const dx = b.x - a.x
    const dz = b.z - a.z
    const len = Math.hypot(dx, dz)
    const mid: V3 = [(a.x + b.x) / 2, 0.07, (a.z + b.z) / 2]
    const yaw = Math.atan2(dx, dz)
    segs.push(
      <group key={i} position={mid} rotation={[0, yaw, 0]}>
        <SoftBox size={[0.12, 0.07, len + 0.08]} r={0.03} material="base" position={[-0.16, 0, 0]} />
        <SoftBox size={[0.12, 0.07, len + 0.08]} r={0.03} material="base" position={[0.16, 0, 0]} />
        <SoftBox size={[0.62, 0.05, len]} r={0.02} material="detail.dark" position={[0, -0.04, 0]} cast={false} />
      </group>,
    )
  }
  return <group>{segs}</group>
}

export function DrillRig({ active }: { active: number }) {
  const bit = useRef<Group>(null)
  useFrame((_, dt) => {
    if (bit.current) bit.current.rotation.y += dt * (0.4 + active * 8)
  })
  return (
    <group position={[-7.35, 0, 0.05]}>
      <SoftBox size={[2.4, 0.22, 2.1]} r={0.08} material="ground" position={[0, 0.11, 0]} />
      <SoftBox size={[1.15, 0.95, 1.2]} r={0.12} material="base" position={[-0.35, 0.68, 0.05]} />
      <SoftBox size={[0.42, 0.38, 0.06]} r={0.03} material="detail.dark" position={[0.12, 0.78, 0.64]} />
      <SoftBox size={[1.2, 0.16, 1.25]} r={0.06} material="base" position={[-0.35, 1.2, 0.05]} />
      <SoftBox size={[1.24, 0.04, 0.1]} r={0.02} material="accent1" position={[-0.35, 1.3, 0.64]} />
      <SoftBox size={[0.08, 0.04, 1.25]} r={0.02} material="accent1" position={[-0.91, 1.3, 0.05]} />
      <SoftBox size={[0.08, 0.04, 1.25]} r={0.02} material="accent1" position={[0.21, 1.3, 0.05]} />
      <RoundCyl radius={0.16} height={3.1} fillet={0.05} material="accent1" position={[0.55, 1.7, 0]} />
      <SoftBox size={[0.7, 0.22, 0.7]} r={0.08} material="accent1" position={[0.55, 3.28, 0]} />
      <group ref={bit} position={[0.55, 0.55, 0]}>
        <RoundCyl radius={0.2} height={0.55} fillet={0.06} material="accent1" />
        <SoftBox size={[0.08, 0.7, 0.08]} r={0.02} material="accent1" position={[0.28, 0.1, 0]} />
        <SoftBox size={[0.08, 0.7, 0.08]} r={0.02} material="accent1" position={[-0.28, 0.1, 0]} />
      </group>
      <SoftBox size={[0.42, 0.12, 0.55]} r={0.05} material="accent3" position={[-1.15, 0.22, 0.7]} />
      <SoftBox size={[0.42, 0.12, 0.55]} r={0.05} material="accent3" position={[-1.15, 0.22, -0.7]} />
    </group>
  )
}

export function Station({
  id,
  position,
  lit,
}: {
  id: StationId
  position: V3
  lit: boolean
}) {
  return (
    <group position={position}>
      <SoftBox size={[1.35, 0.95, 1.15]} r={0.12} material="base" position={[0, 0.62, -0.85]} />
      <SoftBox size={[1.45, 0.06, 0.1]} r={0.03} material="accent3" position={[0, 0.98, -0.32]} />
      <SoftBox size={[1.45, 0.16, 1.25]} r={0.06} material="base" position={[0, 1.14, -0.85]} />
      <SoftBox size={[1.48, 0.04, 0.1]} r={0.02} material="accent1" position={[0, 1.24, -0.26]} />
      <SoftBox size={[0.08, 0.04, 1.25]} r={0.02} material="accent1" position={[-0.7, 1.24, -0.85]} />
      <SoftBox size={[0.08, 0.04, 1.25]} r={0.02} material="accent1" position={[0.7, 1.24, -0.85]} />
      <SoftBox size={[0.55, 0.38, 0.08]} r={0.04} material={lit ? 'accent2' : 'detail.dark'} position={[0, 0.72, -0.26]} />
      <SoftBox size={[1.6, 0.1, 1.4]} r={0.04} material="ground" position={[0, 0.06, -0.2]} cast={false} />
      {id === 'bash' ? <RoundCyl radius={0.14} height={0.7} fillet={0.05} material="accent3" position={[0.45, 1.5, -0.85]} /> : null}
      {id === 'tool' ? (
        <group position={[0.55, 1.35, -0.55]}>
          <RoundCyl radius={0.08} height={0.9} fillet={0.03} material="accent3" />
          <SoftBox size={[1.1, 0.1, 0.16]} r={0.04} material="accent1" position={[0.4, 0.38, 0]} />
        </group>
      ) : null}
    </group>
  )
}

export function Gate({ closed }: { closed: number }) {
  const angle = -0.08 + closed * 1.25
  const allowed = closed < 0.35
  return (
    <group position={[1.15, 0, 0.55]}>
      <RoundCyl radius={0.1} height={1.15} fillet={0.04} material="base" position={[-0.7, 0.58, 0]} />
      <RoundCyl radius={0.1} height={1.15} fillet={0.04} material="base" position={[0.7, 0.58, 0]} />
      <group position={[-0.7, 0.95, 0]} rotation={[0, 0, angle]}>
        <SoftBox size={[1.55, 0.12, 0.16]} r={0.05} material="accent2" position={[0.75, 0, 0]} />
        <SoftBox size={[0.22, 0.22, 0.22]} r={0.06} material={allowed ? 'accent1' : 'detail.dark'} position={[1.45, 0, 0]} />
      </group>
    </group>
  )
}

export function Cart({ position, running }: { position: V3; running?: boolean }) {
  return (
    <group position={position}>
      <SoftBox size={[0.95, 0.32, 0.7]} r={0.08} material="base" position={[0, 0.38, 0]} />
      <SoftBox size={[0.78, 0.28, 0.56]} r={0.07} material="base" position={[0, 0.62, 0]} />
      <SoftBox size={[0.8, 0.05, 0.58]} r={0.03} material="accent1" position={[0, 0.76, 0]} />
      {running ? <SoftBox size={[0.95, 0.05, 0.7]} r={0.03} material="accent2" position={[0, 0.22, 0]} /> : null}
      <Wheel position={[0.32, 0.2, 0.32]} radius={0.16} width={0.14} tire="detail.dark" hub="detail.mid" />
      <Wheel position={[-0.32, 0.2, 0.32]} radius={0.16} width={0.14} tire="detail.dark" hub="detail.mid" />
      <Wheel position={[0.32, 0.2, -0.32]} radius={0.16} width={0.14} tire="detail.dark" hub="detail.mid" />
      <Wheel position={[-0.32, 0.2, -0.32]} radius={0.16} width={0.14} tire="detail.dark" hub="detail.mid" />
      <SoftBox size={[0.14, 0.14, 0.14]} r={0.04} material="detail.light" position={[0.42, 0.5, 0]} />
    </group>
  )
}

export function Dock({ crates }: { crates: number }) {
  const stack: V3[] = [
    [8.35, 0.42, 0.55],
    [8.95, 0.42, 0.35],
    [8.35, 0.86, 0.5],
    [8.95, 0.86, 0.4],
  ]
  return (
    <group>
      <SoftBox size={[3.2, 0.28, 2.6]} r={0.1} material="base" position={[8.4, 0.14, 0.55]} />
      <SoftBox size={[0.35, 1.4, 0.35]} r={0.08} material="accent3" position={[9.5, 0.9, 1.35]} />
      <SoftBox size={[1.4, 0.16, 0.22]} r={0.05} material="accent1" position={[8.85, 1.55, 1.35]} />
      {stack.slice(0, crates).map((p, i) => (
        <group key={i} position={p}>
          <SoftBox size={[0.62, 0.4, 0.5]} r={0.07} material="base" />
          <SoftBox size={[0.64, 0.08, 0.52]} r={0.03} material="accent1" position={[0, 0.02, 0]} />
        </group>
      ))}
    </group>
  )
}

export function Cliff() {
  return (
    <group>
      <Bench position={[-1.4, 0.36, -3.5]} size={[9.4, 0.62, 2.0]} />
      <Bench position={[0.8, 0.82, -4.55]} size={[6.0, 0.68, 1.45]} />
      <Bench position={[-5.4, 0.95, -4.05]} size={[2.6, 0.7, 1.25]} />
      <Bench position={[5.2, 0.46, -3.15]} size={[3.2, 0.46, 1.15]} />
      <SoftBox size={[1.05, 0.38, 0.8]} r={0.12} material="ground" position={[-8.15, 0.22, 1.55]} />
      <RoundCyl radius={0.24} height={0.52} fillet={0.08} material="ground" position={[2.35, 0.3, 2.05]} />
    </group>
  )
}
