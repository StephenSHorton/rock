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
