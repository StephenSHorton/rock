import type { Phase } from '../quarry/types'
import { useQuarry } from '../quarry/sim'

export const SESSION_LINES: { phase: Phase; who: string; text: string; progress: number }[] = [
  { phase: 'prompt', who: 'you', text: 'Does this comment match the test?', progress: 0.04 },
  { phase: 'agent', who: 'rock', text: 'agent cart on the main rail', progress: 0.12 },
  { phase: 'read', who: '▸', text: 'read_file  internal/cli/app.go', progress: 0.2 },
  { phase: 'subagent', who: '▸', text: 'spawn_subagent  explore', progress: 0.28 },
  { phase: 'gate', who: '⚑', text: 'ask edit_file — waiting', progress: 0.38 },
  { phase: 'edit', who: '▸', text: 'allow edit_file', progress: 0.5 },
  { phase: 'bash', who: '▸', text: 'shell  go test ./internal/cli', progress: 0.58 },
  { phase: 'crates', who: '✓', text: 'stdout on the dock', progress: 0.76 },
]

function currentIndex(phase: Phase) {
  if (phase === 'tool') return SESSION_LINES.findIndex((l) => l.phase === 'bash')
  if (phase === 'done') return SESSION_LINES.length - 1
  if (phase === 'idle') return -1
  return SESSION_LINES.findIndex((l) => l.phase === phase)
}

export function SessionTranscript() {
  const phase = useQuarry((s) => s.phase)
  const playing = useQuarry((s) => s.playing)
  const toggle = useQuarry((s) => s.toggle)
  const replay = useQuarry((s) => s.replay)
  const current = currentIndex(phase)

  return (
    <div className="term h-full">
      <div className="term-bar">
        <span>session</span>
        <span className="live">{phase === 'idle' || phase === 'done' ? 'loop' : phase}</span>
      </div>
      <ol className="m-0 list-none space-y-0.5 px-3 py-3">
        {SESSION_LINES.map((line, i) => {
          const on = i === current
          const done = i < current
          return (
            <li key={line.phase}>
              <button
                type="button"
                className={`session-line ${on ? 'is-on' : ''} ${done ? 'is-done' : ''}`}
                aria-current={on ? 'step' : undefined}
                onClick={() => {
                  useQuarry.getState().dispatch({ type: 'task-progress', progress: line.progress })
                  useQuarry.setState({ playing: false })
                }}
              >
                <span className="who">{line.who}</span>
                <span>{line.text}</span>
              </button>
            </li>
          )
        })}
      </ol>
      <div className="flex flex-wrap gap-2 border-t border-[var(--term-trim)] px-3 py-2">
        <button type="button" className="hud-btn" onClick={toggle}>
          {playing ? 'Pause' : 'Play'}
        </button>
        <button type="button" className="hud-btn" onClick={replay}>
          Replay
        </button>
      </div>
    </div>
  )
}
