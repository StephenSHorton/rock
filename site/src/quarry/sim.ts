import { create } from 'zustand'
import type { Phase, SceneEvent, StationId, Vec2 } from './types'

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

type Beat = {
  t: number
  phase: Phase
  label: string
  station: StationId
  agent: number
  sub: number
  crates: number
  drill: number
  gate: number
}

const SCRIPT: Beat[] = [
  { t: 0, phase: 'idle', label: 'ready', station: 'drill', agent: 0, sub: 0, crates: 0, drill: 0, gate: 1 },
  { t: 0.9, phase: 'prompt', label: 'Does this comment match the test?', station: 'drill', agent: 0, sub: 0, crates: 0, drill: 1, gate: 1 },
  { t: 2.4, phase: 'agent', label: 'agent cart on the main rail', station: 'read', agent: 0.18, sub: 0, crates: 0, drill: 0.7, gate: 1 },
  { t: 4.1, phase: 'read', label: 'read_file  internal/cli/app.go', station: 'read', agent: 0.22, sub: 0, crates: 0, drill: 0.2, gate: 1 },
  { t: 5.8, phase: 'subagent', label: 'spawn_subagent  explore', station: 'tool', agent: 0.28, sub: 0.45, crates: 0, drill: 0.15, gate: 1 },
  { t: 7.8, phase: 'gate', label: 'ask edit_file — waiting', station: 'gate', agent: 0.5, sub: 0.72, crates: 0, drill: 0.1, gate: 1 },
  { t: 10.2, phase: 'edit', label: 'allow edit_file', station: 'edit', agent: 0.38, sub: 0.85, crates: 1, drill: 0.1, gate: 0 },
  { t: 12.2, phase: 'bash', label: 'shell  go test ./internal/cli', station: 'bash', agent: 0.68, sub: 0.92, crates: 2, drill: 0.05, gate: 0 },
  { t: 14.2, phase: 'tool', label: 'mcp tools joined the loop', station: 'tool', agent: 0.84, sub: 1, crates: 3, drill: 0, gate: 0 },
  { t: 16.2, phase: 'crates', label: 'stdout on the dock', station: 'dock', agent: 1, sub: 1, crates: 4, drill: 0, gate: 0 },
  { t: 18.6, phase: 'done', label: 'done end_turn', station: 'dock', agent: 1, sub: 1, crates: 4, drill: 0, gate: 0 },
]

const LOOP_AT = 21.2

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
  selected: string | null
  tick: (dt: number) => void
  toggle: () => void
  replay: () => void
  dispatch: (event: SceneEvent) => void
}

function at(time: number): Beat {
  let beat = SCRIPT[0]
  for (const next of SCRIPT) {
    if (time >= next.t) beat = next
  }
  const i = SCRIPT.indexOf(beat)
  const after = SCRIPT[i + 1]
  if (!after) return beat
  const u = (time - beat.t) / Math.max(0.001, after.t - beat.t)
  return {
    ...beat,
    agent: beat.agent + (after.agent - beat.agent) * u,
    sub: beat.sub + (after.sub - beat.sub) * u,
    drill: beat.drill + (after.drill - beat.drill) * u,
    gate: beat.gate + (after.gate - beat.gate) * u,
    crates: u > 0.55 ? after.crates : beat.crates,
  }
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

export const useQuarry = create<QuarryState>((set, get) => ({
  time: 0,
  playing: true,
  phase: 'idle',
  label: 'ready',
  station: 'drill',
  agentT: 0,
  subT: 0,
  crates: 0,
  drill: 0,
  gate: 1,
  selected: null,
  tick: (dt) => {
    if (!get().playing) return
    let time = get().time + dt
    if (time >= LOOP_AT) time = 0
    const beat = at(time)
    set({
      time,
      phase: beat.phase,
      label: beat.label,
      station: beat.station,
      agentT: beat.agent,
      subT: beat.sub,
      crates: beat.crates,
      drill: beat.drill,
      gate: beat.gate,
    })
  },
  toggle: () => set({ playing: !get().playing }),
  replay: () => set({ time: 0, playing: true, ...at(0), selected: null }),
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
      set({ time: 0.9, playing: true, ...at(0.9), label: event.label ?? at(0.9).label, selected: event.id ?? 'agent' })
      return
    }
    if (event.type === 'task-progress') {
      const time = 0.9 + event.progress * 17.7
      set({ time, ...at(time) })
      return
    }
    if (event.type === 'task-finished') {
      set({ time: 18.6, ...at(18.6), selected: event.id ?? 'dock' })
    }
  },
}))
