import { ClayCameraRig, ClayGround, Lights, PostFX, SoftBox } from 'mokei/clay'
import { useFrame } from '@react-three/fiber'
import { Cart, Cliff, Dock, DrillRig, Gate, RailPath, Station } from './models'
import { RAILS, agentPos, subPos, useQuarry } from './sim'

const NO_AO = typeof location !== 'undefined' && /noao|reduced/.test(location.search)
const HOME = { x: 0.6, z: 0.15 }

export function World({ pretty = true }: { pretty?: boolean }) {
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
  const live = phase !== 'idle' && phase !== 'done'

  useFrame((_, dt) => {
    useQuarry.getState().tick(dt)
  })

  return (
    <>
      <ClayCameraRig home={HOME} />
      <Lights />
      <ClayGround onMiss={() => dispatch({ type: 'select', id: null })} />
      <SoftBox size={[22, 0.08, 14]} r={0.05} material="ground" position={[0.4, 0.03, -0.4]} cast={false} />
      <Cliff />
      <RailPath points={RAILS.main} />
      <RailPath points={RAILS.spur} />
      <DrillRig active={drill} />
      <Station id="read" position={[-4.05, 0, 0.35]} lit={station === 'read'} />
      <Station id="edit" position={[-1.35, 0, 0.12]} lit={station === 'edit'} />
      <Station id="bash" position={[3.55, 0, 0.22]} lit={station === 'bash'} />
      <Station id="tool" position={[5.85, 0, -0.12]} lit={station === 'tool'} />
      <Gate closed={gate} />
      <Dock crates={crates} />
      <Cart position={[agent.x, 0, agent.z]} running={live} />
      <Cart position={[sub.x, 0, sub.z]} running={subT > 0.08 && phase === 'subagent'} />
      {pretty && !NO_AO ? <PostFX /> : null}
    </>
  )
}
