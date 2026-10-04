import { Stage } from '../quarry/Stage'
import { SessionTranscript } from './SessionTranscript'

export function HowItWorks() {
  return (
    <section id="how" className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
      <p className="mono text-[0.72rem] tracking-[0.16em] uppercase text-[var(--muted)]">
        <span className="text-[var(--primary)]">how</span> a session maps
      </p>
      <h2 className="mt-2 max-w-[18ch] text-4xl tracking-tight sm:text-5xl">How Rock works.</h2>
      <p className="mt-4 max-w-2xl text-lg text-[var(--ink-2)]">
        The quarry is a diagram, not the product. Prompt is the drill. Agents are carts. Tool calls
        are stations. A permission gate can stop a cart. Output is crates on the dock. The transcript
        on the right is the same turn.
      </p>
      <div className="mt-8 grid gap-4 lg:grid-cols-[minmax(0,1.15fr)_minmax(18rem,0.85fr)]">
        <div className="overflow-hidden rounded-2xl border border-[var(--border)]">
          <Stage />
        </div>
        <SessionTranscript />
      </div>
    </section>
  )
}
