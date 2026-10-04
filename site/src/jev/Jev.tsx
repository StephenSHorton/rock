const ITEMS = [
  {
    title: 'ask_jev',
    body: 'The agent calls it on its own. One call can ask boolean, choice, and score questions against the same state. That is classify, filter, and verify. A failed call does not invent an answer.',
  },
  {
    title: 'Self-validation',
    body: 'After an edit or an allowed shell, the loop can nudge the agent to ask Jev before it declares done. The nudge is text. The agent still decides whether to call. Risk still blocks destructive work.',
  },
  {
    title: 'Triage and clips',
    body: 'A failed shell can be classified. ask_jev can take paths: Rock reads a clip into state. Jev does not open files.',
  },
  {
    title: 'One path. Risk stays mandatory.',
    body: 'Turn, subagent kind, plan-ready, and snippet filter share the same Ask encoder as ask_jev. The destructive Risk check is still a hard Go call. Yolo cannot skip it.',
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
        Jev answers typed questions — boolean, choice, score — in one round trip. It is not an LLM
        and it does not write code. The main model reasons. Jev decides the bounded questions so the
        turn is faster, safer, and cheaper.
      </p>
      <p className="mt-3 max-w-2xl text-[var(--ink-2)]">
        <code>ask_jev</code> is the first-class tool. Built-in checks use that same path. A working Jev
        key is required; Rock will not start an agent without one.
      </p>
      <div className="mt-8 grid gap-4 md:grid-cols-2">
        {ITEMS.map((item) => (
          <article key={item.title} className="glass-hud rounded-2xl p-5">
            <h3 className="text-xl tracking-tight">{item.title}</h3>
            <p className="mt-2 text-[var(--ink-2)]">{item.body}</p>
          </article>
        ))}
      </div>
      <p className="mt-6 max-w-2xl text-[var(--muted)]">
        Showing <code>ask_jev</code> calls in the TUI is still coming. <code>/ready</code> still prints a
        plan-readiness verdict. It does not approve the work.
      </p>
    </section>
  )
}
