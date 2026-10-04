import { ClayCameraRig, ClayGround, Lights, PostFX, SoftBox } from 'mokei/clay'
import { useFrame } from '@react-three/fiber'
import { clay } from './palette'
import { Cart, Cliff, Dock, DrillRig, Gate, RailPath, Station } from './models'
import { RAILS, agentPos, subPos, useQuarry } from './sim'

const NO_AO = typeof location !== 'undefined' && /noao|reduced/.test(location.search)

export function World({ pretty }: { pretty: boolean }) {
  const phase = useQuarry((s) => s.phase)
  const station = useQuarry((s) => s.station)
  const agentT = useQuarry((s) => s.agentT)
  const subT = useQuarry((s) => s.subT)
  const crates = useQuarry((s) => s.crates)
  const drill = useQuarry((s) => s.drill)
  const gate = useQuarry((s) => s.gate)
  const dispatch = useQuarry((s) => s.dispatch)
  const agent = agentPos(agentT)
  const sub = subPos(subT)

  useFrame((_, dt) => {
    useQuarry.getState().tick(dt)
  })

  return (
    <>
      <ClayCameraRig home={{ x: 0.6, z: 0.15 }} />
      <Lights />
      <ClayGround onMiss={() => dispatch({ type: 'select', id: null })} />
      <SoftBox size={[22, 0.08, 14]} r={0.05} color={clay.sand} position={[0.4, 0.03, -0.4]} cast={false} />
      <Cliff />
      <RailPath points={RAILS.main} />
      <RailPath points={RAILS.spur} color={clay.slate} />
      <DrillRig active={drill} />
      <Station id="read" position={[-4.05, 0, 0.35]} accent={clay.teal} lit={station === 'read'} />
      <Station id="edit" position={[-1.35, 0, 0.12]} accent={clay.orange} lit={station === 'edit'} />
      <Station id="bash" position={[3.55, 0, 0.22]} accent={clay.tealDeep} lit={station === 'bash'} />
      <Station id="tool" position={[5.85, 0, -0.12]} accent={clay.orangeHot} lit={station === 'tool'} />
      <Gate closed={gate} />
      <Dock crates={crates} />
      <Cart position={[agent.x, 0, agent.z]} color={clay.orange} lamp={phase === 'gate' ? clay.orangeHot : clay.cream} />
      <Cart position={[sub.x, 0, sub.z]} color={clay.teal} lamp={clay.cream} />
      {pretty && !NO_AO ? <PostFX /> : null}
    </>
  )
}
