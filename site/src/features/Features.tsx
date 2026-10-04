import { TuiFrame, type ShotPair } from '../hero/TuiFrame'

const ITEMS: {
  id: string
  title: string
  body: string
  pair?: ShotPair
}[] = [
  {
    id: 'fork',
    title: '/fork',
    body: 'Prints Suzuri OSC 7880 with brand=rock so the host can split a pane. That is a new Rock process, not an in-process peer agent.',
  },
  {
    id: 'plan',
    title: 'Plan pane',
    body: 'Plan mode writes plan.md and blocks edits and every shell command. The pane shows the plan and a /ready verdict. It never auto-approves.',
    pair: {
      dark: 'plan-dark.png',
      light: 'plan-light.png',
      label: 'Plan',
      alt: 'Rock TUI plan pane beside the transcript',
    },
  },
  {
    id: 'ask',
    title: 'Permission cards',
    body: 'Ask is Allow or Deny. An allowed call still hits the destructive-action gate. /permissions lists the loaded rules and says permissions are not a sandbox.',
    pair: {
      dark: 'permission-dark.png',
      light: 'permission-light.png',
      label: 'Ask',
      alt: 'Rock TUI permission card for edit_file',
    },
  },
  {
    id: 'subagents',
    title: 'Subagents',
    body: 'spawn_subagent starts explore, plan, or general work, optionally on a git worktree. /agents is the 1.0 table. Recorded demos land in slice 2.',
  },
  {
    id: 'inspect',
    title: 'inspect',
    body: 'rock inspect prints config, skills, MCP, Jev mode, and permissions. Offline provider and offline gates are labeled. Discovery stays a command other clients can run.',
  },
]

export function Features() {
  return (
    <section id="features" className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
      <p className="mono text-[0.72rem] tracking-[0.16em] uppercase text-[var(--muted)]">
        <span className="text-[var(--primary)]">next</span> slice 2
      </p>
      <h2 className="mt-2 max-w-[16ch] text-4xl tracking-tight sm:text-5xl">Features, still.</h2>
      <p className="mt-4 max-w-2xl text-lg text-[var(--ink-2)]">
        Recorded terminal demos come later. Until then: headings, and a still frame where we already
        have one.
      </p>
      <div className="mt-8 grid gap-6 lg:grid-cols-2">
        {ITEMS.map((item) => (
          <article key={item.id} className="glass-hud rounded-2xl p-5">
            <h3 className="mono text-[0.8rem] uppercase tracking-[0.08em] text-[var(--primary)]">{item.title}</h3>
            <p className="mt-2 text-[var(--ink-2)]">{item.body}</p>
            {item.pair ? (
              <div className="mt-4">
                <TuiFrame pair={item.pair} states={false} />
              </div>
            ) : (
              <p className="mt-3 mono text-[0.72rem] text-[var(--muted)]">Demo clip incoming.</p>
            )}
          </article>
        ))}
      </div>
    </section>
  )
}
