import { useEffect, useState } from 'react'

const UNIX = 'curl -fsSL https://stephenshorton.github.io/rock/install.sh | bash'
const WINDOWS = 'irm https://stephenshorton.github.io/rock/install.ps1 | iex'

function isWindows() {
  if (typeof navigator === 'undefined') return false
  return /Win/.test(navigator.platform || '') || /Windows/.test(navigator.userAgent || '')
}

export function InstallLine() {
  const [text, setText] = useState(UNIX)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (isWindows()) setText(WINDOWS)
  }, [])

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
    <div className="glass-hud mt-6 flex flex-col gap-3 rounded-2xl p-3 sm:flex-row sm:items-center">
      <pre className="m-0 flex-1 overflow-x-auto text-[0.88rem] leading-6">
        <code>{text}</code>
      </pre>
      <button type="button" className="hud-btn bg-[var(--primary)] text-[var(--primary-foreground)]" onClick={copy}>
        {copied ? 'Copied' : 'Copy'}
      </button>
    </div>
  )
}
