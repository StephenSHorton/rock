import { useEffect, useState } from 'react'

const UNIX = 'curl -fsSL https://stephenshorton.github.io/rock/install.sh | bash'
const WINDOWS = 'irm https://stephenshorton.github.io/rock/install.ps1 | iex'
const SETUP = 'rock setup jev'

function isWindows() {
  if (typeof navigator === 'undefined') return false
  return /Win/.test(navigator.platform || '') || /Windows/.test(navigator.userAgent || '')
}

export function MobileInstallChip() {
  const [install, setInstall] = useState(UNIX)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (isWindows()) setInstall(WINDOWS)
  }, [])

  async function copy() {
    try {
      await navigator.clipboard.writeText(install)
    } catch {
      const area = document.createElement('textarea')
      area.value = install
      document.body.appendChild(area)
      area.select()
      document.execCommand('copy')
      document.body.removeChild(area)
    }
    setCopied(true)
    window.setTimeout(() => setCopied(false), 1400)
  }

  return (
    <div className="glass-panel flex items-center gap-2 rounded-full px-3 py-1.5">
      <code className="min-w-0 flex-1 truncate font-mono text-[0.68rem] leading-5">{install}</code>
      <button
        type="button"
        className="shrink-0 rounded-full p-1 text-[var(--primary)]"
        onClick={copy}
        aria-label={copied ? 'Copied install command' : 'Copy install command'}
      >
        {copied ? (
          <span className="mono text-[0.62rem]">ok</span>
        ) : (
          <svg width="14" height="14" viewBox="0 0 16 16" aria-hidden="true">
            <rect x="5" y="5" width="9" height="9" rx="1.5" fill="none" stroke="currentColor" strokeWidth="1.4" />
            <rect x="2" y="2" width="9" height="9" rx="1.5" fill="none" stroke="currentColor" strokeWidth="1.4" />
          </svg>
        )}
      </button>
    </div>
  )
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
    <div className="mt-3 lg:mt-6">
      <div className="flex flex-col gap-2 rounded-xl border border-[var(--border)] bg-[var(--mokei-base)] p-2 sm:flex-row sm:items-center lg:gap-3 lg:p-3">
        <pre className="m-0 flex-1 overflow-x-auto whitespace-pre-wrap break-all text-[0.68rem] leading-5 lg:text-[0.8rem] lg:leading-6">
          <code>{text}</code>
        </pre>
        <button type="button" className="hud-btn bg-[var(--primary)] text-[var(--primary-foreground)]" onClick={copy}>
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>
      <p className="mt-2 hidden text-sm text-[var(--muted)] lg:block">
        A Jev key is required. The TUI asks for it on first run if none is stored.
      </p>
    </div>
  )
}
