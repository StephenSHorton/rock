import { useState } from 'react'
import {
  commands,
  helpText,
  inspectText,
  jsonStdout,
  permissionsText,
  printStderr,
  printStdout,
  slash,
  tuiTranscript,
} from './transcripts'

const samples = [
  { id: 'tui', label: 'rock', hint: 'TUI' },
  { id: 'help', label: 'rock --help', hint: 'help' },
  { id: 'print', label: 'rock -p', hint: 'print' },
  { id: 'json', label: 'streaming-json', hint: 'json' },
  { id: 'inspect', label: 'rock inspect', hint: 'inspect' },
  { id: 'perms', label: 'permissions', hint: 'perms' },
] as const

type Sample = (typeof samples)[number]['id']

export function CliSection() {
  const [sample, setSample] = useState<Sample>('tui')

  return (
    <section id="cli" className="mx-auto max-w-5xl px-4 py-8 sm:px-6">
      <p className="mono text-[0.72rem] tracking-[0.16em] uppercase text-[var(--muted)]">
        <span className="text-[var(--primary)]">02</span> the binary
      </p>
      <h2 className="mt-2 max-w-[18ch] text-4xl tracking-tight sm:text-5xl">The real CLI, not a sketch.</h2>
      <p className="mt-4 max-w-2xl text-lg text-[var(--ink-2)]">
        One Go binary. The TUI, headless print, ACP, and loopback HTTP share a session. The lines below
        were captured from this checkout with no model key set.
      </p>

      <div className="mt-8 overflow-hidden rounded-2xl border border-[var(--border)] bg-[var(--card)]">
        <table className="w-full text-left">
          <caption className="sr-only">Commands and what they do</caption>
          <tbody>
            {commands.map((row) => (
              <tr key={row.cmd} className="border-t border-[var(--border)] first:border-t-0">
                <th scope="row" className="w-[42%] px-4 py-3 align-top mono text-[0.92rem] font-normal">
                  <code>{row.cmd}</code>
                </th>
                <td className="px-4 py-3 text-[var(--ink-2)]">{row.detail}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="mt-10">
        <div className="flex flex-wrap gap-2" role="tablist" aria-label="Captured transcripts">
          {samples.map((item) => (
            <button
              key={item.id}
              type="button"
              role="tab"
              aria-selected={sample === item.id}
              className="os-tab rounded-full border border-[var(--border)] px-3 py-1.5 mono text-[0.75rem] tracking-[0.06em] text-[var(--muted)]"
              onClick={() => setSample(item.id)}
            >
              {item.label}
            </button>
          ))}
        </div>
        <div className="term mt-3">
          <div className="term-bar">
            <span>{samples.find((s) => s.id === sample)?.label}</span>
            <span className="live">captured</span>
          </div>
          <pre>
            <code>{transcript(sample)}</code>
          </pre>
        </div>
        <p className="mt-2 text-sm text-[var(--muted)]">{caption(sample)}</p>
      </div>

      <div className="mt-10 grid gap-6 md:grid-cols-2">
        <article className="glass-hud rounded-2xl p-5">
          <h3 className="text-2xl tracking-tight">Inside the TUI</h3>
          <p className="mt-2 text-[var(--ink-2)]">
            Composer placeholder: <code>Ask Rock. /help /plan /yolo /default /sessions /permissions /agents /ready /fork /quit</code>
          </p>
          <ul className="mt-4 space-y-2">
            {slash.map((row) => (
              <li key={row.cmd} className="flex gap-3">
                <code className="min-w-28 text-[var(--primary)]">{row.cmd}</code>
                <span className="text-[var(--ink-2)]">{row.detail}</span>
              </li>
            ))}
          </ul>
        </article>
        <article className="glass-hud rounded-2xl p-5">
          <h3 className="text-2xl tracking-tight">Gates, skills, MCP</h3>
          <ul className="mt-3 space-y-3 text-[var(--ink-2)]">
            <li>
              <strong>Permissions.</strong> allow / ask / deny, plus bash globs. Plan mode blocks edits
              and every shell command because redirections are not inspected. Yolo skips asks. It does
              not skip the destructive block.
            </li>
            <li>
              <strong>Skills.</strong> <code>SKILL.md</code> on the usual compat paths. <code>inspect</code> lists what
              loaded.
            </li>
            <li>
              <strong>MCP.</strong> stdio client, Content-Length frames. Enabled servers show up as{' '}
              <code>mcp_&lt;server&gt;_&lt;tool&gt;</code>.
            </li>
            <li>
              <strong>Jev.</strong> A working key is required. <code>rock setup jev</code> validates
              and stores it; the TUI asks on first run. <code>ask_jev</code> is the agent tool.
              Built-in checks share that path. Risk stays mandatory.
            </li>
          </ul>
        </article>
      </div>
    </section>
  )
}

function transcript(id: Sample) {
  switch (id) {
    case 'help':
      return `$ rock --help\n\n${helpText}`
    case 'print':
      return `$ rock -p "Say hello in one sentence"\n${printStdout}\n\n# stderr\n${printStderr}`
    case 'json':
      return `$ rock -p "Say hello in one sentence" --output streaming-json\n${jsonStdout}`
    case 'inspect':
      return `$ rock inspect\n\n${inspectText}`
    case 'perms':
      return `$ rock permissions\n\n${permissionsText}`
    default:
      return tuiTranscript
  }
}

function caption(id: Sample) {
  switch (id) {
    case 'help':
      return 'Exact stdout from rock --help on this tree. Exit 0.'
    case 'print':
      return 'Offline provider. Assistant text on stdout; Jev and done on stderr.'
    case 'json':
      return 'One JSON object per line. Same turn as the text print.'
    case 'inspect':
      return 'Paths shortened to ~/.config and ~/.rock. The binary printed the rest.'
    case 'perms':
      return 'Default policy from config.Default().'
    default:
      return 'Reconstructed from internal/tui: header, status badges, speaker column width 6, ready copy, and the offline print of the same prompt.'
  }
}
