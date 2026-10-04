import { CliSection } from './cli/CliSection'
import { Install } from './cli/Install'
import { Footer } from './layout/Footer'
import { Nav } from './layout/Nav'
import { Hero } from './quarry/Hero'

export function App() {
  return (
    <>
      <a className="skip" href="#content">
        Skip to content
      </a>
      <Nav />
      <main id="content">
        <Hero />
        <section className="mx-auto max-w-5xl px-4 pb-4 pt-12 sm:px-6">
          <p className="mono text-[0.72rem] tracking-[0.16em] uppercase text-[var(--muted)]">
            <span className="text-[var(--primary)]">01</span> coding cli · 1.0
          </p>
          <p className="mt-3 max-w-3xl text-3xl tracking-tight sm:text-5xl">
            Go AI coding CLI.
            <span className="block italic text-[var(--primary)]">Charm TUI, ACP, print, permissions.</span>
          </p>
          <p className="mt-5 max-w-2xl text-lg text-[var(--ink-2)]">
            Ask it of the repo. A Charm TUI, headless print, ACP on stdio, and a loopback HTTP API share
            one session. Jev judges the bounded calls. With no key, those gates still run, and{' '}
            <code>inspect</code> says they are offline.
          </p>
          <div className="mt-6 flex flex-wrap gap-3">
            <a className="rounded-full bg-[var(--primary)] px-4 py-2.5 mono text-[0.78rem] text-[var(--primary-foreground)] no-underline" href="https://github.com/StephenSHorton/rock">
              Source on GitHub
            </a>
            <a className="rounded-full border border-[var(--border)] px-4 py-2.5 mono text-[0.78rem] no-underline hover:border-[var(--primary)]" href="https://github.com/StephenSHorton/rock/blob/main/docs/synthesis/v1-plan.md">
              Read the 1.0 plan
            </a>
          </div>
          <ul className="mt-5 flex list-none flex-wrap gap-2 p-0">
            {['Charm v2', 'Jev', 'ACP', 'Suzuri guest', 'MIT'].map((chip) => (
              <li key={chip} className="rounded-full border border-[var(--border)] px-2.5 py-1 mono text-[0.72rem] uppercase tracking-[0.06em] text-[var(--muted)]">
                {chip}
              </li>
            ))}
          </ul>
        </section>
        <Install />
        <CliSection />
      </main>
      <Footer />
    </>
  )
}
