import type { SceneEvent } from 'mokei/scene'
import { create } from 'zustand'
import type { Phase, StationId, Vec2 } from './types'

const MAIN_RAIL: Vec2[] = [
  { x: -7.3, z: 0.05 },
  { x: -4.05, z: 0.35 },
  { x: -1.35, z: 0.12 },
  { x: 1.15, z: 0.0 },
  { x: 3.55, z: 0.22 },
  { x: 5.85, z: -0.12 },
  { x: 8.15, z: 0.38 },
]

const SPUR_RAIL: Vec2[] = [
  { x: -4.05, z: 0.35 },
  { x: -3.15, z: 2.15 },
  { x: 5.35, z: 1.85 },
  { x: 5.85, z: -0.12 },
  { x: 8.15, z: 0.38 },
]

export const STATIONS: { id: StationId; label: string; at: Vec2 }[] = [
  { id: 'drill', label: 'prompt', at: { x: -7.3, z: 0.05 } },
  { id: 'read', label: 'read_file', at: { x: -4.05, z: 0.35 } },
  { id: 'edit', label: 'edit_file', at: { x: -1.35, z: 0.12 } },
  { id: 'gate', label: 'ask', at: { x: 1.15, z: 0.0 } },
  { id: 'bash', label: 'shell', at: { x: 3.55, z: 0.22 } },
  { id: 'tool', label: 'mcp', at: { x: 5.85, z: -0.12 } },
  { id: 'dock', label: 'stdout', at: { x: 8.15, z: 0.38 } },
]

const STATION_T: Record<StationId, number> = {
  drill: 0,
  read: 0.22,
  edit: 0.38,
  gate: 0.5,
  bash: 0.68,
  tool: 0.84,
  dock: 1,
}

const STATION_IDS = new Set<string>(Object.keys(STATION_T))

const PHASE_FOR_STATION: Record<StationId, Phase> = {
  drill: 'prompt',
  read: 'read',
  edit: 'edit',
  bash: 'bash',
  tool: 'tool',
  gate: 'gate',
  dock: 'crates',
}

type Cue = { t: number; event: SceneEvent }

const CUES: Cue[] = [
  { t: 0.9, event: { type: 'task-started', id: 'prompt', label: 'Does this comment match the test?' } },
  { t: 2.4, event: { type: 'tool-station', station: 'read', label: 'agent cart on the main rail' } },
  { t: 4.1, event: { type: 'tool-station', id: 'read', station: 'read', label: 'read_file  internal/cli/app.go' } },
  { t: 5.8, event: { type: 'subagent-spawn', id: 'explore', label: 'spawn_subagent  explore' } },
  { t: 7.8, event: { type: 'permission-gate', allowed: false, label: 'ask edit_file — waiting' } },
  { t: 10.2, event: { type: 'permission-gate', allowed: true, label: 'allow edit_file' } },
  { t: 10.2, event: { type: 'tool-station', station: 'edit', label: 'allow edit_file' } },
  { t: 10.2, event: { type: 'crate', count: 1 } },
  { t: 12.2, event: { type: 'tool-station', station: 'bash', label: 'shell  go test ./internal/cli' } },
  { t: 12.2, event: { type: 'crate', count: 2 } },
  { t: 14.2, event: { type: 'tool-station', station: 'tool', label: 'mcp tools joined the loop' } },
  { t: 14.2, event: { type: 'crate', count: 3 } },
  { t: 16.2, event: { type: 'output', label: 'stdout on the dock' } },
  { t: 16.2, event: { type: 'crate', count: 4 } },
  { t: 18.6, event: { type: 'subagent-finish', id: 'explore' } },
  { t: 18.6, event: { type: 'task-finished', id: 'dock' } },
]

const LOOP_AT = 21.2

type LookApi = {
  zoomBy: (delta: number) => void
  rotateBy: (deg: number) => void
  resetView: () => void
}

let lookApi: LookApi | null = null

export function bindLook(api: LookApi | null) {
  lookApi = api
}

export type QuarryState = {
  time: number
  playing: boolean
  phase: Phase
  label: string
  station: StationId
  agentT: number
  subT: number
  crates: number
  drill: number
  gate: number
  agentTarget: number
  subTarget: number
  drillTarget: number
  gateTarget: number
  cue: number
  selected: string | null
  tick: (dt: number) => void
  toggle: () => void
  replay: () => void
  zoomBy: (delta: number) => void
  rotateBy: (deg: number) => void
  resetView: () => void
  dispatch: (event: SceneEvent) => void
}

const IDLE = {
  phase: 'idle' as const,
  label: 'ready',
  station: 'drill' as const,
  agentT: 0,
  subT: 0,
  crates: 0,
  drill: 0,
  gate: 1,
  agentTarget: 0,
  subTarget: 0,
  drillTarget: 0,
  gateTarget: 1,
  cue: -1,
  selected: null as string | null,
}

function asStation(value?: string): StationId | undefined {
  if (value && STATION_IDS.has(value)) return value as StationId
  return undefined
}

function along(path: Vec2[], t: number): Vec2 {
  const clamped = Math.max(0, Math.min(1, t))
  const segs = path.length - 1
  const scaled = clamped * segs
  const i = Math.min(segs - 1, Math.floor(scaled))
  const u = scaled - i
  return {
    x: path[i].x + (path[i + 1].x - path[i].x) * u,
    z: path[i].z + (path[i + 1].z - path[i].z) * u,
  }
}

export function agentPos(t: number): Vec2 {
  return along(MAIN_RAIL, t)
}

export function subPos(t: number): Vec2 {
  return along(SPUR_RAIL, t)
}

export const RAILS = { main: MAIN_RAIL, spur: SPUR_RAIL }

function lerp(from: number, to: number, dt: number, rate: number) {
  return from + (to - from) * Math.min(1, dt * rate)
}

function applyCues(time: number, from: number, dispatch: (event: SceneEvent) => void) {
  let cue = from
  for (let i = from + 1; i < CUES.length; i += 1) {
    if (CUES[i].t > time) break
    dispatch(CUES[i].event)
    cue = i
  }
  return cue
}

export const useQuarry = create<QuarryState>((set, get) => ({
  time: 0,
  playing: true,
  ...IDLE,
  tick: (dt) => {
    if (!get().playing) return
    let time = get().time + dt
    if (time >= LOOP_AT) {
      get().replay()
      return
    }
    const cue = applyCues(time, get().cue, (event) => get().dispatch(event))
    const cur = get()
    set({
      time,
      cue,
      agentT: lerp(cur.agentT, cur.agentTarget, dt, 2.4),
      subT: lerp(cur.subT, cur.subTarget, dt, 1.6),
      drill: lerp(cur.drill, cur.drillTarget, dt, 3.2),
      gate: lerp(cur.gate, cur.gateTarget, dt, 3.2),
    })
  },
  toggle: () => set({ playing: !get().playing }),
  replay: () => set({ time: 0, playing: true, ...IDLE }),
  zoomBy: (delta) => lookApi?.zoomBy(delta),
  rotateBy: (deg) => lookApi?.rotateBy(deg),
  resetView: () => lookApi?.resetView(),
  dispatch: (event) => {
    if (event.type === 'reset') {
      get().replay()
      return
    }
    if (event.type === 'select') {
      set({ selected: event.id })
      return
    }
    if (event.type === 'task-started') {
      set({
        playing: true,
        phase: 'prompt',
        label: event.label ?? 'Does this comment match the test?',
        station: 'drill',
        drillTarget: 1,
        agentTarget: 0,
        selected: event.id ?? 'agent',
      })
      return
    }
    if (event.type === 'task-progress') {
      const time = 0.9 + event.progress * 17.7
      set({ time, playing: get().playing, ...IDLE, selected: event.id ?? get().selected })
      const cue = applyCues(time, -1, (next) => get().dispatch(next))
      const cur = get()
      set({
        cue,
        agentT: cur.agentTarget,
        subT: cur.subTarget,
        drill: cur.drillTarget,
        gate: cur.gateTarget,
      })
      return
    }
    if (event.type === 'task-finished') {
      set({
        phase: 'done',
        label: 'done end_turn',
        station: 'dock',
        agentTarget: 1,
        drillTarget: 0,
        gateTarget: 0,
        selected: event.id ?? 'dock',
      })
      return
    }
    if (event.type === 'tool-station') {
      const station = asStation(event.station) ?? get().station
      const firstRead = station === 'read' && !event.id && (get().phase === 'prompt' || get().phase === 'idle' || get().phase === 'agent')
      set({
        phase: firstRead ? 'agent' : PHASE_FOR_STATION[station],
        label: event.label ?? get().label,
        station,
        agentTarget: STATION_T[station],
        selected: event.id ?? station,
      })
      return
    }
    if (event.type === 'permission-gate') {
      const allowed = Boolean(event.allowed)
      set({
        phase: allowed ? 'edit' : 'gate',
        label: event.label ?? (allowed ? 'allow' : 'ask'),
        station: allowed ? 'edit' : 'gate',
        gateTarget: allowed ? 0 : 1,
        agentTarget: allowed ? STATION_T.edit : STATION_T.gate,
        selected: event.id ?? 'gate',
      })
      return
    }
    if (event.type === 'crate') {
      set({
        crates: event.count ?? get().crates + 1,
        phase: get().phase === 'done' ? 'done' : 'crates',
        label: event.label ?? get().label,
      })
      return
    }
    if (event.type === 'output') {
      set({
        phase: 'crates',
        label: event.label ?? 'stdout on the dock',
        station: 'dock',
        agentTarget: 1,
        selected: event.id ?? 'dock',
      })
      return
    }
    if (event.type === 'subagent-spawn') {
      set({
        phase: 'subagent',
        label: event.label ?? 'spawn_subagent',
        subTarget: 1,
        selected: event.id ?? 'explore',
      })
      return
    }
    if (event.type === 'subagent-finish') {
      set({
        subTarget: 1,
        selected: event.id ?? get().selected,
      })
    }
  },
}))
