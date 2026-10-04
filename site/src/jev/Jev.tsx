const VISION = [
  {
    title: 'Dynamic delegation',
    body: 'ask_jev is the planned first-class tool: the agent calls it on its own to classify, filter, and verify. That path is in progress. It is not shipped.',
  },
  {
    title: 'Self-validation mid-task',
    body: 'A cheap check partway through a turn, before the main model spends another completion. The tool is not live yet. The loop already asks Jev whether the work is stuck or the transcript should compact.',
  },
  {
    title: 'Cheap triage first',
    body: 'Jev picks fast or strong, and which skill bodies enter the prompt, before the LLM writes. That turn check ships today.',
  },
] as const

const SHIPPED = [
  {
    title: 'Turn check',
    body: 'Before each completion: fast or strong, which skills to load, whether the loop is repeating, whether to compact. Live Jev when a key is set; otherwise a local policy.',
  },
  {
    title: 'Risk gate',
    body: 'Before a tool: is this destructive? A block still applies under yolo. inspect says jev:live or jev:offline.',
  },
  {
    title: '/ready',
    body: 'Plan exit asks whether the plan is ready. The verdict prints on the pane. It does not approve the work.',
  },
] as const

export function Jev() {
  return (
    <section id="jev" className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
      <p className="mono text-[0.72rem] tracking-[0.16em] uppercase text-[var(--muted)]">
        <span className="text-[var(--primary)]">jev</span> system one
      </p>
      <h2 className="mt-2 max-w-[18ch] text-4xl tracking-tight sm:text-5xl">The primitive in the loop.</h2>
      <p className="mt-4 max-w-2xl text-lg text-[var(--ink-2)]">
        Jev answers typed questions — choice, score, yes or no — in one round trip. It is not an LLM
        and it does not write code. The main model reasons. Jev decides the bounded questions so the
        turn is faster, safer, and cheaper.
      </p>
      <p className="mt-3 max-w-2xl text-[var(--ink-2)]">
        <code>ask_jev</code> as a first-class classify / filter / verify tool is in progress and not
        shipped. What follows is the destination, then what the binary already runs.
      </p>
      <div className="mt-8 grid gap-4 md:grid-cols-3">
        {VISION.map((item) => (
          <article key={item.title} className="glass-hud rounded-2xl p-5">
            <p className="mono text-[0.68rem] uppercase tracking-[0.08em] text-[var(--muted)]">ahead</p>
            <h3 className="mt-2 text-xl tracking-tight">{item.title}</h3>
            <p className="mt-2 text-[var(--ink-2)]">{item.body}</p>
          </article>
        ))}
      </div>
      <h3 className="mt-10 text-2xl tracking-tight">Shipped today</h3>
      <ul className="mt-4 grid list-none gap-4 p-0 md:grid-cols-3">
        {SHIPPED.map((item) => (
          <li key={item.title} className="rounded-2xl border border-[var(--border)] p-5">
            <strong className="block tracking-tight">{item.title}</strong>
            <p className="mt-2 text-[var(--ink-2)]">{item.body}</p>
          </li>
        ))}
      </ul>
      <p className="mt-6 max-w-2xl text-[var(--muted)]">
        The model can already call <code>jev_decide</code> for one extra bounded question. That is not{' '}
        <code>ask_jev</code>. The hard gates do not wait on the model to remember them. No key: the
        same gates run offline, and <code>rock inspect</code> says so.
      </p>
    </section>
  )
}
