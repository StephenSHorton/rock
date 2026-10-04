const POINTS = [
  {
    title: 'Permissions are the default',
    body: 'Safer than “run as the user with no permissions.” Allow / ask / deny, bash globs, plan mode, review-only. inspect says permissions are not a sandbox.',
  },
  {
    title: 'One engine, many clients',
    body: 'TUI, headless print, ACP, and loopback HTTP share the harness. If a feature cannot be reached through a protocol, it is a TUI toy.',
  },
  {
    title: 'MIT and BYOK',
    body: 'Rock is MIT. Bring your own OpenAI-compatible key. No account as identity. No key: offline provider, offline gates, and inspect says so.',
  },
] as const

export function Compare() {
  return (
    <section id="compare" className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
      <p className="mono text-[0.72rem] tracking-[0.16em] uppercase text-[var(--muted)]">
        <span className="text-[var(--primary)]">why</span> this shape
      </p>
      <h2 className="mt-2 max-w-[18ch] text-4xl tracking-tight sm:text-5xl">What we refuse to copy.</h2>
      <p className="mt-4 max-w-2xl text-lg text-[var(--ink-2)]">
        Rock is a synthesis. Charm for the face. Grok Build as quarry, not a fork. OpenCode-class
        permissions. The claims below are in the repo, not a benchmark.
      </p>
      <ul className="mt-8 grid list-none gap-4 p-0 md:grid-cols-3">
        {POINTS.map((point) => (
          <li key={point.title} className="glass-hud rounded-2xl p-5">
            <h3 className="text-xl tracking-tight">{point.title}</h3>
            <p className="mt-2 text-[var(--ink-2)]">{point.body}</p>
          </li>
        ))}
      </ul>
    </section>
  )
}
