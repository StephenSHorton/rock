import { InstallLine } from './InstallLine'
import { TuiFrame } from './TuiFrame'
import { YardStage } from '../yard/YardStage'

export function Hero() {
  return (
    <section id="top" className="relative isolate min-h-[100svh] w-full overflow-hidden">
      <YardStage />
      <div className="relative z-10 mx-auto flex min-h-[100svh] max-w-6xl flex-col justify-center px-4 py-24 sm:px-6">
        <div className="grid items-center gap-5 lg:grid-cols-[minmax(0,24rem)_minmax(0,1fr)] xl:grid-cols-[minmax(0,26rem)_minmax(0,1fr)]">
          <div className="glass-panel rounded-2xl p-5 sm:p-6">
            <p className="mono text-[0.72rem] tracking-[0.16em] uppercase text-[var(--muted)]">
              <span className="text-[var(--primary)]">rock</span> 1.0 · MIT · Go
            </p>
            <h1 className="mt-3 text-4xl leading-[1.05] tracking-tight sm:text-5xl">
              An AI coding CLI in Go.
            </h1>
            <p className="mt-4 text-lg text-[var(--ink-2)]">
              A fullscreen Charm TUI, <code>rock -p</code>, <code>rock acp</code>, and{' '}
              <code>rock serve</code> share the same harness.
            </p>
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
          <div className="min-w-0">
            <TuiFrame />
          </div>
        </div>
      </div>
    </section>
  )
}
