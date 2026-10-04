export type SceneEvent =
  | { type: 'task-started'; id?: string; label?: string }
  | { type: 'task-progress'; id?: string; progress: number }
  | { type: 'task-finished'; id?: string }
  | { type: 'select'; id: string | null }
  | { type: 'reset' }

export type Phase =
  | 'idle'
  | 'prompt'
  | 'agent'
  | 'read'
  | 'subagent'
  | 'gate'
  | 'edit'
  | 'bash'
  | 'tool'
  | 'crates'
  | 'done'

export type StationId = 'read' | 'edit' | 'bash' | 'tool' | 'gate' | 'dock' | 'drill'

export type Vec2 = { x: number; z: number }
