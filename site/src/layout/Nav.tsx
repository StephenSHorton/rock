import { useState } from 'react'

const links = [
  { href: '#install', label: 'Install' },
  { href: '#jev', label: 'Jev' },
  { href: '#features', label: 'Features' },
  { href: '#cli', label: 'CLI' },
  { href: '#docs', label: 'Docs' },
]

export function Nav() {
  const [open, setOpen] = useState(false)

  return (
    <header className="fixed inset-x-0 top-0 z-20 border-b border-[var(--topbar-border)] bg-[var(--topbar-bg)] shadow-[var(--topbar-shadow)] backdrop-blur">
      <div className="mx-auto flex max-w-5xl items-center gap-3 px-4 py-3 sm:px-6">
        <a href="#top" className="flex items-center gap-2 no-underline" aria-label="Rock, top">
          <svg viewBox="0 0 32 32" width="28" height="28" aria-hidden="true">
            <rect width="32" height="32" rx="7" fill="var(--mokei-base)" />
            <rect x="7" y="14" width="18" height="4" rx="2" fill="var(--mokei-accent-1)" />
          </svg>
          <span className="mono tracking-[0.14em] lowercase">rock</span>
        </a>
        <button
          type="button"
          className="ml-auto rounded-full border border-[var(--border)] px-3 py-1 mono text-[0.72rem] uppercase tracking-[0.08em] md:hidden"
          aria-expanded={open}
          aria-controls="nav-links"
          onClick={() => setOpen((v) => !v)}
        >
          Menu
        </button>
        <nav
          id="nav-links"
          className={`${open ? 'flex' : 'hidden'} absolute left-0 right-0 top-full flex-col border-b border-[var(--border)] bg-[var(--topbar-bg)] md:static md:ml-auto md:flex md:flex-row md:border-0 md:bg-transparent`}
        >
          {links.map((link) => (
            <a
              key={link.href}
              href={link.href}
              className="px-4 py-3 mono text-[0.75rem] uppercase tracking-[0.08em] text-[var(--muted)] no-underline hover:text-[var(--primary)] md:px-3 md:py-1"
              onClick={() => setOpen(false)}
            >
              {link.label}
            </a>
          ))}
        </nav>
        <a
          className="hidden rounded-full border border-[var(--border)] px-3 py-1.5 mono text-[0.75rem] uppercase tracking-[0.08em] no-underline hover:border-[var(--primary)] md:inline"
          href="https://github.com/StephenSHorton/rock"
        >
          GitHub
        </a>
      </div>
    </header>
  )
}
