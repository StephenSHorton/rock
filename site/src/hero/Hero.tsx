import { InstallLine } from './InstallLine'
import { TuiFrame } from './TuiFrame'

const POINTS = [
  {
    title: 'One harness, many clients',
    body: 'The Charm TUI, headless print, ACP on stdio, and loopback HTTP share one session store. The face is a client, not a second product.',
  },
  {
    title: 'Safer defaults than run-as-the-user',
    body: 'Allow / ask / deny, plus bash globs. Plan mode blocks edits and every shell command. Yolo skips asks; it does not skip the destructive gate.',
  },
  {
    title: 'Honest when offline',
    body: 'No model key, no Jev key: the binary still runs, the same gates still run, and inspect says they are offline.',
  },
] as const

export function Hero() {
  return (
    <section id="top" className="mx-auto max-w-6xl px-4 pb-6 pt-10 sm:px-6 lg:pt-14">
      <div className="grid items-start gap-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)]">
        <div>
          <p className="mono text-[0.72rem] tracking-[0.16em] uppercase text-[var(--muted)]">
            <span className="text-[var(--primary)]">rock</span> 1.0 · MIT
          </p>
          <h1 className="mt-3 text-4xl leading-[1.05] tracking-tight sm:text-5xl">
            A Go AI coding CLI.
            <span className="mt-2 block italic text-[var(--primary)]">Charm TUI. One session. Real gates.</span>
          </h1>
          <p className="mt-5 max-w-xl text-lg text-[var(--ink-2)]">
            Ask it of the repo. A fullscreen Charm TUI, <code>rock -p</code>, <code>rock acp</code>, and{' '}
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
