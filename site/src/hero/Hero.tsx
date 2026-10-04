import { InstallChip } from './InstallLine'
import { TuiFrame } from './TuiFrame'
import { YardStage } from '../yard/YardStage'

export function Hero() {
  return (
    <section id="top" className="relative isolate min-h-[100svh] w-full overflow-hidden">
      <YardStage />
      <div className="hero-scrim" aria-hidden="true" />

      <div className="relative z-10 mx-auto flex min-h-[100svh] max-w-6xl flex-col justify-between px-4 pb-5 pt-20 sm:px-6 lg:px-8">
        <div className="max-w-xl">
          <h1 className="hero-title text-[1.35rem] leading-[1.15] tracking-tight sm:text-4xl lg:text-5xl">
            An AI coding CLI in Go.
          </h1>
          <p className="hero-sub mt-2 hidden max-w-[36ch] text-sm text-[var(--ink-2)] sm:block lg:text-base">
            Charm TUI, <code>rock -p</code>, acp, and serve. One harness.
          </p>
        </div>

        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div className="flex max-w-md flex-col items-stretch gap-2">
            <InstallChip />
            <div className="flex flex-wrap gap-2">
              <a className="rounded-full bg-[var(--primary)] px-3 py-1.5 mono text-[0.72rem] text-[var(--primary-foreground)] no-underline" href="#install">
                Install
              </a>
              <a className="rounded-full border border-[var(--border)] bg-[var(--card)] px-3 py-1.5 mono text-[0.72rem] no-underline hover:border-[var(--primary)]" href="https://github.com/StephenSHorton/rock">
                GitHub
              </a>
              <a className="hidden rounded-full border border-[var(--border)] bg-[var(--card)] px-3 py-1.5 mono text-[0.72rem] no-underline hover:border-[var(--primary)] sm:inline" href="https://github.com/StephenSHorton/rock/blob/main/docs/synthesis/v1-plan.md">
                Docs
              </a>
            </div>
          </div>
          <div className="hidden min-w-0 w-[min(40vw,30rem)] lg:block">
            <TuiFrame />
          </div>
        </div>
      </div>

      <div className="relative z-10 mx-auto max-w-6xl px-4 pb-10 sm:px-6 lg:hidden">
        <TuiFrame />
      </div>
    </section>
  )
}
