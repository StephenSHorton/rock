import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './boot'
import './styles.css'
import { YardBand } from './yard/YardBand'

function NotFound() {
  return (
    <main className="min-h-screen">
      <YardBand tall />
      <section className="mx-auto max-w-xl px-6 py-14">
        <p className="mono text-[0.72rem] tracking-[0.16em] uppercase text-[var(--muted)]">
          <span className="text-[var(--primary)]">404</span> checked
        </p>
        <h1 className="mt-3 text-5xl tracking-tight">
          Page not found
          <span className="mt-2 block italic text-[var(--primary)]">The CLI still is.</span>
        </h1>
        <p className="mt-5 text-lg text-[var(--ink-2)]">That URL is not on the site.</p>
        <div className="mt-6 flex flex-wrap gap-3">
          <a className="rounded-full bg-[var(--primary)] px-4 py-2.5 mono text-[0.78rem] text-[var(--primary-foreground)] no-underline" href="/rock/">
            Back to Rock
          </a>
          <a className="rounded-full border border-[var(--border)] px-4 py-2.5 mono text-[0.78rem] no-underline" href="https://github.com/StephenSHorton/rock">
            Source on GitHub
          </a>
        </div>
      </section>
    </main>
  )
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <NotFound />
  </StrictMode>,
)
