import { useEffect, useState } from 'react'

const UNIX = 'curl -fsSL https://stephenshorton.github.io/rock/install.sh | bash'
const WINDOWS = 'irm https://stephenshorton.github.io/rock/install.ps1 | iex'
const SETUP = 'rock setup jev'

function isWindows() {
  if (typeof navigator === 'undefined') return false
  return /Win/.test(navigator.platform || '') || /Windows/.test(navigator.userAgent || '')
}

export function InstallLine() {
  const [install, setInstall] = useState(UNIX)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (isWindows()) setInstall(WINDOWS)
  }, [])

  const text = `${install}\n${SETUP}`

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
    <div className="mt-6">
      <div className="flex flex-col gap-3 rounded-xl border border-[var(--border)] bg-[var(--mokei-base)] p-3 sm:flex-row sm:items-center">
        <pre className="m-0 flex-1 overflow-x-auto whitespace-pre-wrap break-all text-[0.8rem] leading-6">
          <code>{text}</code>
        </pre>
        <button type="button" className="hud-btn bg-[var(--primary)] text-[var(--primary-foreground)]" onClick={copy}>
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>
      <p className="mt-2 text-sm text-[var(--muted)]">
        A Jev key is required. The TUI asks for it on first run if none is stored.
      </p>
    </div>
  )
}
