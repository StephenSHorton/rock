import { RoundCyl, SoftBox, Wheel } from 'mokei/clay'
import { useFrame } from '@react-three/fiber'
import { useRef } from 'react'
import type { Group } from 'three'
import { clay } from './palette'
import type { StationId } from './types'

type V3 = [number, number, number]

export function Bench({ position, size, color = clay.sandDeep }: { position: V3; size: V3; color?: string }) {
  return <SoftBox size={size} r={0.16} color={color} position={position} cast={false} />
}

export function RailPath({ points, color = clay.slateDeep }: { points: { x: number; z: number }[]; color?: string }) {
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
        <SoftBox size={[0.12, 0.07, len + 0.08]} r={0.03} color={color} position={[-0.16, 0, 0]} />
        <SoftBox size={[0.12, 0.07, len + 0.08]} r={0.03} color={color} position={[0.16, 0, 0]} />
        <SoftBox size={[0.62, 0.05, len]} r={0.02} color={clay.sandHot} position={[0, -0.04, 0]} cast={false} />
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
      <SoftBox size={[2.4, 0.22, 2.1]} r={0.08} color={clay.sandDeep} position={[0, 0.11, 0]} />
      <SoftBox size={[1.15, 0.95, 1.2]} r={0.12} color={clay.slate} position={[-0.35, 0.68, 0.05]} />
      <SoftBox size={[1.2, 0.16, 1.25]} r={0.06} color={clay.orange} position={[-0.35, 1.2, 0.05]} />
      <RoundCyl radius={0.16} height={3.1} fillet={0.05} color={clay.slateDeep} position={[0.55, 1.7, 0]} />
      <SoftBox size={[0.7, 0.22, 0.7]} r={0.08} color={clay.orangeHot} position={[0.55, 3.28, 0]} />
      <group ref={bit} position={[0.55, 0.55, 0]}>
        <RoundCyl radius={0.2} height={0.55} fillet={0.06} color={active > 0.3 ? clay.teal : clay.slatePale} />
        <SoftBox size={[0.08, 0.7, 0.08]} r={0.02} color={clay.orange} position={[0.28, 0.1, 0]} />
        <SoftBox size={[0.08, 0.7, 0.08]} r={0.02} color={clay.orange} position={[-0.28, 0.1, 0]} />
      </group>
      <SoftBox size={[0.42, 0.12, 0.55]} r={0.05} color={clay.slateDeep} position={[-1.15, 0.22, 0.7]} />
      <SoftBox size={[0.42, 0.12, 0.55]} r={0.05} color={clay.slateDeep} position={[-1.15, 0.22, -0.7]} />
    </group>
  )
}

export function Station({
  id,
  position,
  accent,
  lit,
}: {
  id: StationId
  position: V3
  accent: string
  lit: boolean
}) {
  return (
    <group position={position}>
      <SoftBox size={[1.35, 0.95, 1.15]} r={0.12} color={clay.slate} position={[0, 0.62, -0.85]} />
      <SoftBox size={[1.45, 0.16, 1.25]} r={0.06} color={accent} position={[0, 1.14, -0.85]} />
      <SoftBox size={[0.55, 0.38, 0.08]} r={0.04} color={lit ? clay.cream : clay.slatePale} position={[0, 0.72, -0.26]} />
      <SoftBox size={[1.6, 0.1, 1.4]} r={0.04} color={clay.sandHot} position={[0, 0.06, -0.2]} cast={false} />
      {id === 'bash' ? <RoundCyl radius={0.14} height={0.7} fillet={0.05} color={clay.teal} position={[0.45, 1.5, -0.85]} /> : null}
      {id === 'tool' ? (
        <group position={[0.55, 1.35, -0.55]}>
          <RoundCyl radius={0.08} height={0.9} fillet={0.03} color={clay.slateDeep} />
          <SoftBox size={[1.1, 0.1, 0.16]} r={0.04} color={clay.orange} position={[0.4, 0.38, 0]} />
        </group>
      ) : null}
    </group>
  )
}

export function Gate({ closed }: { closed: number }) {
  const angle = -0.08 + closed * 1.25
  return (
    <group position={[1.15, 0, 0.55]}>
      <RoundCyl radius={0.1} height={1.15} fillet={0.04} color={clay.slateDeep} position={[-0.7, 0.58, 0]} />
      <RoundCyl radius={0.1} height={1.15} fillet={0.04} color={clay.slateDeep} position={[0.7, 0.58, 0]} />
      <group position={[-0.7, 0.95, 0]} rotation={[0, 0, angle]}>
        <SoftBox size={[1.55, 0.12, 0.16]} r={0.05} color={clay.orangeHot} position={[0.75, 0, 0]} />
        <SoftBox size={[0.22, 0.22, 0.22]} r={0.06} color={clay.teal} position={[1.45, 0, 0]} />
      </group>
    </group>
  )
}

export function Cart({ position, color, lamp }: { position: V3; color: string; lamp?: string }) {
  return (
    <group position={position}>
      <SoftBox size={[0.95, 0.32, 0.7]} r={0.08} color={color} position={[0, 0.38, 0]} />
      <SoftBox size={[0.78, 0.28, 0.56]} r={0.07} color={clay.slateDeep} position={[0, 0.62, 0]} />
      <Wheel position={[0.32, 0.2, 0.32]} radius={0.16} width={0.14} tire={clay.tire} hub={clay.slatePale} />
      <Wheel position={[-0.32, 0.2, 0.32]} radius={0.16} width={0.14} tire={clay.tire} hub={clay.slatePale} />
      <Wheel position={[0.32, 0.2, -0.32]} radius={0.16} width={0.14} tire={clay.tire} hub={clay.slatePale} />
      <Wheel position={[-0.32, 0.2, -0.32]} radius={0.16} width={0.14} tire={clay.tire} hub={clay.slatePale} />
      {lamp ? <SoftBox size={[0.14, 0.14, 0.14]} r={0.04} color={lamp} position={[0.42, 0.5, 0]} /> : null}
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
      <SoftBox size={[3.2, 0.28, 2.6]} r={0.1} color={clay.slate} position={[8.4, 0.14, 0.55]} />
      <SoftBox size={[0.35, 1.4, 0.35]} r={0.08} color={clay.slateDeep} position={[9.5, 0.9, 1.35]} />
      <SoftBox size={[1.4, 0.16, 0.22]} r={0.05} color={clay.orange} position={[8.85, 1.55, 1.35]} />
      {stack.slice(0, crates).map((p, i) => (
        <SoftBox key={i} size={[0.62, 0.4, 0.5]} r={0.07} color={i % 2 ? clay.crateDeep : clay.crate} position={p} />
      ))}
    </group>
  )
}

export function Cliff() {
  return (
    <group>
      <Bench position={[-2.2, 0.55, -3.6]} size={[14, 1.1, 3.2]} color={clay.sand} />
      <Bench position={[1.4, 1.35, -5.1]} size={[10, 1.4, 2.4]} color={clay.sandDeep} />
      <Bench position={[-6.4, 1.9, -4.4]} size={[4.4, 1.6, 2.2]} color={clay.slate} />
      <RoundCyl radius={0.42} height={1.15} fillet={0.12} color={clay.teal} position={[-1.8, 0.72, 2.6]} />
      <RoundCyl radius={0.32} height={0.85} fillet={0.1} color={clay.tealDeep} position={[2.4, 0.52, 2.2]} />
      <RoundCyl radius={0.38} height={1.0} fillet={0.11} color={clay.teal} position={[6.6, 0.62, 2.5]} />
    </group>
  )
}
