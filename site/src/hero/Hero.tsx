import { InstallLine, MobileInstallChip } from './InstallLine'
import { TuiFrame } from './TuiFrame'
import { YardStage } from '../yard/YardStage'

export function Hero() {
  return (
    <section id="top" className="relative isolate min-h-[100svh] w-full overflow-hidden">
      <YardStage />

      <div className="relative z-10 flex min-h-[100svh] flex-col justify-between px-4 pb-5 pt-20 sm:hidden">
        <h1 className="glass-panel w-fit rounded-full px-3.5 py-1.5 text-[1.15rem] leading-tight tracking-tight">
          An AI coding CLI in Go.
        </h1>
        <div className="flex flex-col items-stretch gap-2">
          <MobileInstallChip />
          <div className="flex flex-wrap gap-2">
            <a className="rounded-full bg-[var(--primary)] px-3 py-1.5 mono text-[0.72rem] text-[var(--primary-foreground)] no-underline" href="#install">
              Install
            </a>
            <a className="rounded-full border border-[var(--border)] bg-[var(--card)] px-3 py-1.5 mono text-[0.72rem] no-underline" href="https://github.com/StephenSHorton/rock">
              GitHub
            </a>
          </div>
        </div>
      </div>

      <div className="relative z-10 mx-auto hidden max-w-6xl px-4 sm:block sm:px-6 lg:grid lg:min-h-[100svh] lg:grid-cols-[minmax(0,24rem)_minmax(0,1fr)] lg:items-center lg:gap-5 xl:grid-cols-[minmax(0,26rem)_minmax(0,1fr)]">
        <div className="flex min-h-[100svh] items-end pb-4 pt-[42svh] lg:min-h-0 lg:items-center lg:py-0">
          <div className="glass-panel w-full rounded-2xl p-3 sm:p-4 lg:p-6">
            <p className="mono text-[0.72rem] tracking-[0.16em] uppercase text-[var(--muted)]">
              <span className="text-[var(--primary)]">rock</span> 1.0 · MIT · Go
            </p>
            <h1 className="mt-2 text-[1.65rem] leading-[1.05] tracking-tight lg:mt-3 lg:text-5xl">
              An AI coding CLI in Go.
            </h1>
            <p className="mt-2 text-sm text-[var(--ink-2)] lg:mt-4 lg:text-lg">
              A fullscreen Charm TUI, <code>rock -p</code>, <code>rock acp</code>, and{' '}
              <code>rock serve</code> share the same harness.
            </p>
            <InstallLine />
            <div className="mt-3 flex flex-wrap gap-2 lg:mt-5 lg:gap-3">
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
        </div>
        <div className="min-w-0 pb-10 lg:pb-0">
          <TuiFrame />
        </div>
      </div>

      <div className="relative z-10 mx-auto max-w-6xl px-4 pb-10 sm:hidden">
        <TuiFrame />
      </div>
    </section>
  )
}
