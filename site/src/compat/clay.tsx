import { OrthographicCamera, RoundedBox } from '@react-three/drei'
import { useFrame, useThree } from '@react-three/fiber'
import { useEffect, useMemo, useRef } from 'react'
import { LatheGeometry, Vector2, Vector3 } from 'three'
import type { OrthographicCamera as OrthographicCameraImpl } from 'three'
import { useLook } from './look'

export type { LookPatch, LookState } from './look'
export { applyLook, lookDefaults, useLook } from './look'

type Vec3 = [number, number, number]

export const SOFT_EDGE_SCALE = 0.8

export function scaleSoft(value: number) {
  return value * SOFT_EDGE_SCALE
}

export function Matte({ color, emissive }: { color: string; emissive?: string }) {
  return <meshLambertMaterial color={color} emissive={emissive ?? '#000000'} />
}

export function SoftBox({
  size,
  color,
  position,
  rotation,
  r,
  smooth = 4,
  cast = true,
  receive = true,
  emissive,
}: {
  size: Vec3
  color: string
  position?: Vec3
  rotation?: Vec3
  r?: number
  smooth?: number
  cast?: boolean
  receive?: boolean
  emissive?: string
}) {
  const min = Math.min(size[0], size[1], size[2])
  const radius = Math.max(0.004, Math.min(scaleSoft(r ?? min * 0.24), min / 2 - 0.002))
  return (
    <RoundedBox
      args={size}
      radius={radius}
      smoothness={smooth}
      position={position}
      rotation={rotation}
      castShadow={cast}
      receiveShadow={receive}
    >
      <Matte color={color} emissive={emissive} />
    </RoundedBox>
  )
}

export function useRoundCylinder(radius: number, height: number, fillet: number, segments = 28) {
  return useMemo(() => {
    const f = Math.min(fillet, radius * 0.95, height / 2 - 0.001)
    const pts: Vector2[] = [new Vector2(0, -height / 2)]
    const steps = 6
    for (let i = 0; i <= steps; i += 1) {
      const a = -Math.PI / 2 + (i / steps) * (Math.PI / 2)
      pts.push(new Vector2(radius - f + Math.cos(a) * f, -height / 2 + f + Math.sin(a) * f))
    }
    for (let i = 0; i <= steps; i += 1) {
      const a = (i / steps) * (Math.PI / 2)
      pts.push(new Vector2(radius - f + Math.cos(a) * f, height / 2 - f + Math.sin(a) * f))
    }
    pts.push(new Vector2(0, height / 2))
    return new LatheGeometry(pts, segments)
  }, [radius, height, fillet, segments])
}

export function RoundCyl({
  radius,
  height,
  fillet,
  color,
  position,
  rotation,
  cast = true,
  segments,
}: {
  radius: number
  height: number
  fillet?: number
  color: string
  position?: Vec3
  rotation?: Vec3
  cast?: boolean
  segments?: number
}) {
  const geo = useRoundCylinder(radius, height, scaleSoft(fillet ?? Math.min(radius, height) * 0.35), segments)
  return (
    <mesh geometry={geo} position={position} rotation={rotation} castShadow={cast} receiveShadow>
      <Matte color={color} />
    </mesh>
  )
}

export function Wheel({
  position,
  radius = 0.36,
  width = 0.3,
  tire = '#1f2533',
  hub = '#cbd5e1',
}: {
  position: Vec3
  radius?: number
  width?: number
  tire?: string
  hub?: string
}) {
  const side = position[0] >= 0 ? 1 : -1
  return (
    <group position={position} rotation={[0, 0, Math.PI / 2]}>
      <RoundCyl radius={radius} height={width} fillet={width * 0.42} color={tire} />
      <RoundCyl
        radius={radius * 0.46}
        height={0.05}
        fillet={0.02}
        color={hub}
        position={[0, (-side * width) / 2, 0]}
        cast={false}
      />
    </group>
  )
}

function damp(current: number, target: number, lambda: number, dt: number) {
  return current + (target - current) * (1 - Math.exp(-lambda * dt))
}

function dampAngle(current: number, target: number, lambda: number, dt: number) {
  let delta = target - current
  while (delta > Math.PI) delta -= Math.PI * 2
  while (delta < -Math.PI) delta += Math.PI * 2
  return current + delta * (1 - Math.exp(-lambda * dt))
}

export function Lights() {
  const look = useLook()
  const az = (look.sunAzimuth * Math.PI) / 180
  const el = (look.sunElevation * Math.PI) / 180
  const dist = 60
  return (
    <>
      <hemisphereLight args={[look.skyColor, look.groundBounce, look.skyIntensity * Math.PI]} />
      <directionalLight
        color={look.sunColor}
        intensity={look.sunIntensity * Math.PI}
        position={[dist * Math.cos(el) * Math.sin(az), dist * Math.sin(el), dist * Math.cos(el) * Math.cos(az)]}
        castShadow
        shadow-mapSize={[2048, 2048]}
        shadow-radius={look.shadowSoftness}
        shadow-bias={-0.0004}
        shadow-normalBias={0.03}
        shadow-camera-near={1}
        shadow-camera-far={140}
        shadow-camera-left={-42}
        shadow-camera-right={42}
        shadow-camera-top={42}
        shadow-camera-bottom={-42}
      />
      <directionalLight color="#eef2ff" intensity={0.12 * Math.PI} position={[30, 20, 40]} />
    </>
  )
}

export function ClayGround({ onMiss }: { onMiss?: () => void }) {
  const ground = useLook((s) => s.ground)
  const setLook = useLook((s) => s.setLook)
  const drag = useRef<{ active: boolean; x: number; y: number; panX: number; panZ: number } | null>(null)
  return (
    <mesh
      rotation={[-Math.PI / 2, 0, 0]}
      receiveShadow
      onPointerDown={(event) => {
        if (event.button !== 0) return
        drag.current = { active: false, x: event.clientX, y: event.clientY, panX: useLook.getState().panX, panZ: useLook.getState().panZ }
        event.stopPropagation()
      }}
      onPointerMove={(event) => {
        const state = drag.current
        if (!state) return
        const dx = event.clientX - state.x
        const dy = event.clientY - state.y
        if (!state.active && Math.hypot(dx, dy) < 5) return
        state.active = true
        const az = (useLook.getState().cameraAzimuth * Math.PI) / 180
        const scale = 1.25 / useLook.getState().cameraZoom
        setLook({
          panX: state.panX - Math.cos(az) * dx * scale - Math.sin(az) * dy * scale * 1.6,
          panZ: state.panZ + Math.sin(az) * dx * scale - Math.cos(az) * dy * scale * 1.6,
        })
      }}
      onPointerUp={() => {
        if (drag.current && !drag.current.active) onMiss?.()
        drag.current = null
      }}
      onPointerLeave={() => {
        drag.current = null
      }}
    >
      <planeGeometry args={[400, 400]} />
      <meshLambertMaterial color={ground} />
    </mesh>
  )
}

export function ClayCameraRig({ home = { x: 0, z: 0 } }: { home?: { x: number; z: number } }) {
  const camera = useThree((s) => s.camera)
  const gl = useThree((s) => s.gl)
  const size = useThree((s) => s.size)
  const look = useLook()
  const target = useRef(new Vector3(home.x, 0, home.z))
  const az = useRef((look.cameraAzimuth * Math.PI) / 180)
  const zoomBy = useLook((s) => s.zoomBy)

  useEffect(() => {
    const element = gl.domElement
    const onWheel = (event: WheelEvent) => {
      event.preventDefault()
      zoomBy(event.deltaY > 0 ? -1.6 : 1.6)
    }
    element.addEventListener('wheel', onWheel, { passive: false })
    return () => element.removeEventListener('wheel', onWheel)
  }, [gl, zoomBy])

  useFrame((_, dt) => {
    target.current.x = damp(target.current.x, home.x + look.panX, 2.4, dt)
    target.current.z = damp(target.current.z, home.z + look.panZ, 2.4, dt)
    az.current = dampAngle(az.current, (look.cameraAzimuth * Math.PI) / 180, 5, dt)
    const el = (look.cameraElevation * Math.PI) / 180
    const dist = 80
    const cam = camera as OrthographicCameraImpl
    cam.position.set(
      target.current.x + dist * Math.cos(el) * Math.sin(az.current),
      dist * Math.sin(el),
      target.current.z + dist * Math.cos(el) * Math.cos(az.current),
    )
    cam.lookAt(target.current)
    const goalZoom = look.cameraZoom * (size.width / 1728)
    cam.zoom = damp(cam.zoom || goalZoom, goalZoom, 7, dt)
    cam.near = -200
    cam.far = 400
    cam.updateProjectionMatrix()
  })

  return <OrthographicCamera makeDefault position={[40, 36, 40]} zoom={look.cameraZoom} near={-200} far={400} />
}

/** Real kit uses N8AO. The fallback skips it so the site builds without postprocessing. */
export function PostFX() {
  return null
}
