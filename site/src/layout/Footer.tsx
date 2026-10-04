const docs = [
  { href: 'https://github.com/StephenSHorton/rock', label: 'Repository' },
  { href: 'https://github.com/StephenSHorton/rock/blob/main/docs/synthesis/v1-plan.md', label: '1.0 plan' },
  { href: 'https://github.com/StephenSHorton/rock/blob/main/VISION.md', label: 'Vision' },
  { href: 'https://github.com/StephenSHorton/rock/blob/main/AGENTS.md', label: 'Agents' },
]

export function Footer() {
  return (
    <footer id="docs" className="relative bg-[var(--background)]">
      <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-4 border-t border-[var(--border)] px-4 py-6 sm:px-6">
        <p className="mono text-[0.75rem] text-[var(--muted)]">MIT · Copyright 2026 Stephen Horton</p>
        <p className="mono text-[0.75rem]">
          {docs.map((d) => (
            <a key={d.href} href={d.href} className="mr-4 no-underline hover:text-[var(--primary)]">
              {d.label}
            </a>
          ))}
        </p>
      </div>
    </footer>
  )
}
