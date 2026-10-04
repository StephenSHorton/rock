import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useChromeTheme, useReducedMotion } from '../hero/theme'

type DemoId = 'fork' | 'plan' | 'permission' | 'agents' | 'inspect'

const ITEMS: {
  id: DemoId
  title: string
  body: ReactNode
}[] = [
  {
    id: 'plan',
    title: 'Plan pane',
    body: (
      <>
        Plan mode blocks edits and every shell command until <code>/ready</code>. The pane fills from{' '}
        <code>update_plan</code>, then <code>/ready</code> prints a verdict. The verdict does not approve
        the work.
      </>
    ),
  },
  {
    id: 'permission',
    title: 'Permission cards',
    body: (
      <>
        Allow or Deny runs one call once. An allowed call still hits the destructive-action gate. This
        clip is a real <code>edit_file</code> card, then Allow.
      </>
    ),
  },
  {
    id: 'fork',
    title: '/fork',
    body: (
      <>
        <code>/fork</code> emits the fork signal; the status line confirms <code>sent OSC 7880 to the
        host</code>. The split into a second pane happens inside the Suzuri terminal host, not in a
        plain terminal. This clip does not show a split, and Rock does not create a git branch.
      </>
    ),
  },
  {
    id: 'agents',
    title: 'Subagents',
    body: (
      <>
        <code>spawn_subagent</code> really runs a child explore harness and returns.{' '}
        <code>/agents</code> lists it afterward.
      </>
    ),
  },
  {
    id: 'inspect',
    title: 'inspect',
    body: (
      <>
        <code>rock inspect</code> prints what is actually configured and running: config, models, Jev,
        and the session store. No guessing what the harness is doing.
      </>
    ),
  },
]

function DemoFrame({
  id,
  title,
  theme,
  reduced,
}: {
  id: DemoId
  title: string
  theme: 'dark' | 'light'
  reduced: boolean
}) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const [started, setStarted] = useState(false)
  const src = `/rock/demos/${id}-${theme}.mp4`
  const poster = `/rock/demos/${id}-${theme}.png`
  const showPlay = reduced && !started

  useEffect(() => {
    const video = videoRef.current
    if (!video || reduced) return
    const io = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) void video.play().catch(() => {})
        else video.pause()
      },
      { threshold: 0.25 },
    )
    io.observe(video)
    return () => io.disconnect()
  }, [reduced, src])

  function play() {
    setStarted(true)
    void videoRef.current?.play()
  }

  return (
    <figure className="tui-frame mt-4">
      <div className="tui-chrome">
        <span className="tui-dots" aria-hidden="true">
          <i />
          <i />
          <i />
        </span>
        <figcaption className="tui-title">rock · {title}</figcaption>
      </div>
      <div className={`tui-screen tui-screen-${theme} demo-screen`}>
        <video
          ref={videoRef}
          key={src}
          src={src}
          poster={poster}
          width={1100}
          height={620}
          muted
          loop
          playsInline
          autoPlay={!reduced}
          preload="metadata"
          aria-label={`${title} demo`}
        />
        {showPlay ? (
          <button type="button" className="demo-play" onClick={play}>
            Play
          </button>
        ) : null}
      </div>
    </figure>
  )
}

export function Features() {
  const { theme, toggle } = useChromeTheme()
  const reduced = useReducedMotion()

  return (
    <section id="features" className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="mono text-[0.72rem] tracking-[0.16em] uppercase text-[var(--muted)]">
          <span className="text-[var(--primary)]">rock</span> 1.0 · on tape
        </p>
        <button
          type="button"
          className="tui-chip"
          aria-pressed={theme === 'dark'}
          onClick={toggle}
        >
          {theme === 'dark' ? 'Dark' : 'Light'}
        </button>
      </div>
      <h2 className="mt-2 max-w-[16ch] text-4xl tracking-tight sm:text-5xl">Seen, not sketched.</h2>
      <p className="mt-4 max-w-2xl text-lg text-[var(--ink-2)]">
        Real recordings from main, through a stub LLM and xterm. Captions say only what the clip
        shows.
      </p>
      <div className="mt-8 grid gap-6 lg:grid-cols-2">
        {ITEMS.map((item) => (
          <article key={item.id} className="glass-hud rounded-2xl p-5">
            <h3 className="mono text-[0.8rem] uppercase tracking-[0.08em] text-[var(--primary)]">{item.title}</h3>
            <p className="mt-2 text-[var(--ink-2)]">{item.body}</p>
            <DemoFrame id={item.id} title={item.title} theme={theme} reduced={reduced} />
          </article>
        ))}
      </div>
    </section>
  )
}
