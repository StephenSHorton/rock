import { useQuarry } from './sim'
import type { Phase } from './types'

const STEPS: { id: Phase; label: string }[] = [
  { id: 'prompt', label: 'Prompt' },
  { id: 'agent', label: 'Agent' },
  { id: 'read', label: 'Read' },
  { id: 'subagent', label: 'Subagent' },
  { id: 'gate', label: 'Ask' },
  { id: 'edit', label: 'Edit' },
  { id: 'bash', label: 'Shell' },
  { id: 'crates', label: 'Crates' },
]

const ORDER: Phase[] = STEPS.map((s) => s.id)

function stepIndex(phase: Phase) {
  if (phase === 'idle') return -1
  if (phase === 'tool') return 6
  if (phase === 'done') return 7
  return ORDER.indexOf(phase)
}

export function Hud() {
  const phase = useQuarry((s) => s.phase)
  const label = useQuarry((s) => s.label)
  const crates = useQuarry((s) => s.crates)
  const playing = useQuarry((s) => s.playing)
  const toggle = useQuarry((s) => s.toggle)
  const replay = useQuarry((s) => s.replay)
  const resetView = useQuarry((s) => s.resetView)
  const zoomBy = useQuarry((s) => s.zoomBy)
  const rotateBy = useQuarry((s) => s.rotateBy)
  const current = stepIndex(phase)

  return (
    <div className="pointer-events-none absolute inset-0 z-10 p-3 sm:p-4 lg:p-5">
      <div className="mx-auto flex h-full max-w-[1728px] flex-col justify-between">
        <div className="pointer-events-auto flex flex-wrap items-center justify-between gap-2">
          <div className="glass-hud flex items-center gap-3 rounded-2xl px-3 py-2">
            <span className="mono text-[0.72rem] tracking-[0.14em] uppercase text-[var(--muted)]">WH-RQ</span>
            <strong className="text-[0.95rem]">Clay quarry</strong>
            <span className="hidden text-sm text-[var(--muted)] sm:inline">prompt · carts · stations · crates</span>
            <span className="rounded-full bg-[var(--accent)] px-2 py-0.5 text-[0.7rem] font-medium text-[var(--accent-foreground)]">
              {phase === 'done' || phase === 'idle' ? 'Idle' : 'Live'}
            </span>
          </div>
          <div className="glass-hud hidden items-center gap-3 rounded-2xl px-3 py-2 text-sm md:flex">
            <span>
              <span className="text-[var(--muted)]">mode</span> default
            </span>
            <span>
              <span className="text-[var(--muted)]">jev</span> offline
            </span>
            <span>
              <span className="text-[var(--muted)]">provider</span> offline
            </span>
          </div>
        </div>

        <div className="pointer-events-auto mt-3 grid max-w-xl grid-cols-3 gap-2">
          <Kpi label="Prompt" value="drill" note="user turn" />
          <Kpi label="Carts" value="2" note="agent + explore" />
          <Kpi label="Crates" value={String(crates)} note="stdout on the dock" />
        </div>

        <div className="pointer-events-auto mt-auto flex flex-col gap-2 lg:flex-row lg:items-end lg:justify-between">
          <div className="glass-hud max-w-xl rounded-2xl p-3">
            <p className="mono text-[0.68rem] tracking-[0.12em] uppercase text-[var(--muted)]">Session</p>
            <ol className="mt-2 flex flex-wrap gap-1.5">
              {STEPS.map((step, i) => {
                const on = i <= current
                return (
                  <li
                    key={step.id}
                    className={`rounded-full px-2 py-0.5 text-[0.72rem] ${
                      on ? 'bg-[var(--primary)] text-[var(--primary-foreground)]' : 'bg-[var(--secondary)] text-[var(--muted)]'
                    }`}
                  >
                    {i + 1} {step.label}
                  </li>
                )
              })}
            </ol>
            <p className="mt-2 text-sm" aria-live="polite">
              {label}
            </p>
          </div>
          <div className="glass-hud flex flex-wrap items-center gap-2 rounded-2xl p-2">
            <button type="button" className="hud-btn" onClick={toggle}>
              {playing ? 'Pause' : 'Play'}
            </button>
            <button type="button" className="hud-btn" onClick={replay}>
              Replay
            </button>
            <button type="button" className="hud-btn" onClick={() => zoomBy(2)}>
              +
            </button>
            <button type="button" className="hud-btn" onClick={() => zoomBy(-2)}>
              −
            </button>
            <button type="button" className="hud-btn" onClick={() => rotateBy(-8)}>
              ↶
            </button>
            <button type="button" className="hud-btn" onClick={() => rotateBy(8)}>
              ↷
            </button>
            <button type="button" className="hud-btn" onClick={resetView}>
              Home
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function Kpi({ label, value, note }: { label: string; value: string; note: string }) {
  return (
    <div className="glass-hud rounded-2xl px-3 py-2">
      <p className="mono text-[0.68rem] tracking-[0.1em] uppercase text-[var(--muted)]">{label}</p>
      <p className="text-xl font-medium tracking-tight">{value}</p>
      <p className="text-[0.75rem] text-[var(--muted)]">{note}</p>
    </div>
  )
}
