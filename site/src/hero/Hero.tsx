import { InstallLine } from './InstallLine'
import { TuiFrame } from './TuiFrame'

const POINTS = [
  {
    title: 'One harness, four faces',
    body: 'A fullscreen Charm TUI, plus rock -p, rock acp, and rock serve. They share a session.',
  },
  {
    title: 'Jev for cheap checks',
    body: 'A fast decision API the agent uses for classify, triage, and filtering. The Risk gate is mandatory. Yolo cannot skip it.',
  },
  {
    title: 'A Jev key is required',
    body: 'Rock will not start an agent without one. rock setup jev validates and stores it. The TUI asks on first run.',
  },
] as const

export function Hero() {
  return (
    <section id="top" className="mx-auto max-w-6xl px-4 pb-6 pt-10 sm:px-6 lg:pt-14">
      <div className="grid items-start gap-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)]">
        <div>
          <p className="mono text-[0.72rem] tracking-[0.16em] uppercase text-[var(--muted)]">
            <span className="text-[var(--primary)]">rock</span> 1.0 · MIT · Go
          </p>
          <h1 className="mt-3 text-4xl leading-[1.05] tracking-tight sm:text-5xl">
            An AI coding CLI in Go.
          </h1>
          <p className="mt-5 max-w-xl text-lg text-[var(--ink-2)]">
            A fullscreen Charm TUI, <code>rock -p</code>, <code>rock acp</code>, and{' '}
            <code>rock serve</code> share the same harness.
          </p>
          <ol className="mt-6 space-y-4 p-0">
            {POINTS.map((point, i) => (
              <li key={point.title} className="flex gap-3">
                <span className="mono mt-0.5 text-[0.72rem] text-[var(--primary)]">{String(i + 1).padStart(2, '0')}</span>
                <div>
                  <strong className="block tracking-tight">{point.title}</strong>
                  <p className="mt-1 text-[var(--ink-2)]">{point.body}</p>
                </div>
              </li>
            ))}
          </ol>
          <InstallLine />
          <div className="mt-5 flex flex-wrap gap-3">
            <a className="rounded-full bg-[var(--primary)] px-4 py-2.5 mono text-[0.78rem] text-[var(--primary-foreground)] no-underline" href="#install">
              Install
            </a>
            <a className="rounded-full border border-[var(--border)] px-4 py-2.5 mono text-[0.78rem] no-underline hover:border-[var(--primary)]" href="https://github.com/StephenSHorton/rock">
              GitHub
            </a>
            <a className="rounded-full border border-[var(--border)] px-4 py-2.5 mono text-[0.78rem] no-underline hover:border-[var(--primary)]" href="https://github.com/StephenSHorton/rock/blob/main/docs/synthesis/v1-plan.md">
              Docs
            </a>
          </div>
        </div>
        <TuiFrame />
      </div>
    </section>
  )
}
