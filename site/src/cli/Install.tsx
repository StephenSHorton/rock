import { useEffect, useState } from 'react'

const commands = {
  mac: 'curl -fsSL https://stephenshorton.github.io/rock/install.sh | bash',
  linux: 'curl -fsSL https://stephenshorton.github.io/rock/install.sh | bash',
  windows: 'irm https://stephenshorton.github.io/rock/install.ps1 | iex',
} as const

const SETUP = 'rock setup jev'

type OS = keyof typeof commands

function detectOS(): OS {
  if (typeof navigator === 'undefined') return 'linux'
  const platform = navigator.platform || ''
  const ua = navigator.userAgent || ''
  if (/Win/.test(platform) || /Windows/.test(ua)) return 'windows'
  if (/Mac/.test(platform) || /Mac OS/.test(ua)) return 'mac'
  return 'linux'
}

export function Install() {
  const [os, setOS] = useState<OS>('linux')
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    setOS(detectOS())
  }, [])

  const text = `${commands[os]}\n${SETUP}`

  async function copy() {
    try {
      await navigator.clipboard.writeText(text)
    } catch {
      const area = document.createElement('textarea')
      area.value = text
      document.body.appendChild(area)
      area.select()
      document.execCommand('copy')
      document.body.removeChild(area)
    }
    setCopied(true)
    window.setTimeout(() => setCopied(false), 1400)
  }

  return (
    <section id="install" className="mx-auto max-w-5xl px-4 py-16 sm:px-6">
      <p className="mono text-[0.72rem] tracking-[0.16em] uppercase text-[var(--muted)]">
        <span className="text-[var(--primary)]">install</span> then jev
      </p>
      <h2 className="mt-2 max-w-[16ch] text-4xl tracking-tight sm:text-5xl">Try it in your terminal.</h2>
      <div className="mt-5 flex flex-wrap gap-2" role="tablist" aria-label="Operating system">
        {(
          [
            ['mac', 'macOS'],
            ['linux', 'Linux'],
            ['windows', 'Windows'],
          ] as const
        ).map(([id, label]) => (
          <button
            key={id}
            type="button"
            role="tab"
            className="os-tab rounded-full border border-[var(--border)] px-3 py-1.5 mono text-[0.75rem] tracking-[0.08em] uppercase text-[var(--muted)]"
            aria-selected={os === id}
            onClick={() => setOS(id)}
          >
            {label}
          </button>
        ))}
      </div>
      <div className="glass-hud mt-3 flex flex-col gap-3 rounded-2xl p-4 sm:flex-row sm:items-center">
        <pre className="m-0 flex-1 overflow-x-auto text-[0.92rem]">
          <code>{text}</code>
        </pre>
        <button type="button" className="hud-btn bg-[var(--primary)] text-[var(--primary-foreground)]" onClick={copy}>
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>
      <p className="mt-3 text-[var(--ink-2)]">
        A Jev key is required. <code>rock setup jev</code> validates and stores it. In the TUI, Rock
        asks for the key on first run.
      </p>
      <p className="mt-2 text-[var(--muted)]">
        Needs <a className="text-[var(--primary)] underline-offset-2 hover:underline" href="https://go.dev/dl/">Go 1.27</a> or
        newer. The command installs the current 1.0 build of <code>rock</code> with{' '}
        <code>go install github.com/StephenSHorton/rock/cmd/rock@main</code>.
      </p>
    </section>
  )
}
