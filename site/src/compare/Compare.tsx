const POINTS = [
  {
    title: 'Triage before the completion',
    body: 'The agent can classify a failure or filter file clips with ask_jev before the main model reads the pile. Built-in turn checks share that path. The LLM still does the reasoning.',
  },
  {
    title: 'A risk gate yolo cannot skip',
    body: 'Destructive shell and write still hit the Jev Risk check. Allow / ask / deny, plan mode, and review-only stay in front. inspect says permissions are not a sandbox.',
  },
  {
    title: 'Jev is required',
    body: 'Rock will not start an agent without a working key. rock setup jev validates and stores it. The TUI asks on first run. Headless -p, serve, and acp fail without one.',
  },
] as const

export function Compare() {
  return (
    <section id="compare" className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
      <p className="mono text-[0.72rem] tracking-[0.16em] uppercase text-[var(--muted)]">
        <span className="text-[var(--primary)]">why</span> jev
      </p>
      <h2 className="mt-2 max-w-[18ch] text-4xl tracking-tight sm:text-5xl">Why the System One sits here.</h2>
      <p className="mt-4 max-w-2xl text-lg text-[var(--ink-2)]">
        Rock is a Grok Build-class CLI because the harness is serious. It is Rock because Jev is in
        the loop. The claims below are in the binary, not a benchmark.
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
