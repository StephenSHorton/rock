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
      <div className="glass-panel mt-3 flex flex-col gap-3 rounded-2xl p-4 sm:flex-row sm:items-center">
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
      <h3 className="mt-10 text-2xl tracking-tight">Bring your subscription</h3>
      <ul className="mt-4 space-y-3 text-[var(--ink-2)]">
        <li>
          <strong>ChatGPT Plus / Pro.</strong> <code>rock login chatgpt</code> is OpenAI’s sign-in for
          open-source tools. It stores a model session. It is not Jev and does not start the agent.
        </li>
        <li>
          <strong>SuperGrok.</strong> Install the official <code>grok</code> CLI and run{' '}
          <code>grok login</code>. Rock talks to that binary. Rock still runs the tools and the Risk
          check.
        </li>
        <li>
          <strong>Claude and Gemini.</strong> API key only. Their terms do not allow a third-party CLI
          to reuse the chat subscription.
        </li>
        <li>
          <strong>API keys</strong> work for every model provider. The Jev key is separate and always
          required.
        </li>
      </ul>
      <p className="mt-2 text-[var(--muted)]">
        Downloads the latest GitHub release and verifies the SHA-256 in{' '}
        <code>checksums.txt</code>. If no asset matches your OS/arch, it falls back to{' '}
        <code>go install github.com/StephenSHorton/rock/cmd/rock@latest</code> (needs{' '}
        <a className="text-[var(--primary)] underline-offset-2 hover:underline" href="https://go.dev/dl/">Go 1.27</a>
        ). After that, <code>rock update</code> keeps a release binary current.
      </p>
    </section>
  )
}
