import { useState } from 'react'
import { useChromeTheme } from './theme'

export type ShotPair = { dark: string; light: string; label: string; alt: string }

const STATES: { id: string; pair: ShotPair }[] = [
  {
    id: 'idle',
    pair: {
      dark: 'idle-dark.png',
      light: 'idle-light.png',
      label: 'Idle',
      alt: 'Rock TUI idle: framed Ask Rock composer with a ❯ prompt, model and mode chips, and jev:live',
    },
  },
  {
    id: 'slash',
    pair: {
      dark: 'slash-dark.png',
      light: 'slash-light.png',
      label: 'Slash',
      alt: 'Rock TUI slash-command pop-up above the framed composer after typing /',
    },
  },
  {
    id: 'model',
    pair: {
      dark: 'model-dark.png',
      light: 'model-light.png',
      label: 'Model',
      alt: 'Rock TUI model picker pop-up opened from the composer frame chip',
    },
  },
  {
    id: 'tools',
    pair: {
      dark: 'tools-dark.png',
      light: 'tools-light.png',
      label: 'Tools',
      alt: 'Rock TUI after an allowed edit_file call, with the diff in the transcript',
    },
  },
  {
    id: 'ask',
    pair: {
      dark: 'permission-dark.png',
      light: 'permission-light.png',
      label: 'Ask',
      alt: 'Rock TUI permission card asking to allow or deny edit_file',
    },
  },
  {
    id: 'askjev',
    pair: {
      dark: 'askjev-dark.png',
      light: 'askjev-light.png',
      label: 'ask_jev',
      alt: 'Rock TUI after an ask_jev batch, painted as diamond jev marks with a gutter',
    },
  },
  {
    id: 'plan',
    pair: {
      dark: 'plan-dark.png',
      light: 'plan-light.png',
      label: 'Plan',
      alt: 'Rock TUI in plan mode with the side plan pane',
    },
  },
]

export function TuiFrame({
  pair,
  states = true,
}: {
  pair?: ShotPair
  states?: boolean
}) {
  const { theme, toggle } = useChromeTheme()
  const [state, setState] = useState('tools')

  const active = pair ?? STATES.find((s) => s.id === state)?.pair ?? STATES[0].pair
  const src = `/rock/tui/${theme === 'dark' ? active.dark : active.light}`

  return (
    <figure className="tui-frame">
      <div className="tui-chrome">
        <span className="tui-dots" aria-hidden="true">
          <i />
          <i />
          <i />
        </span>
        <figcaption className="tui-title">rock · {active.label}</figcaption>
        <div className="tui-tools">
          {states
            ? STATES.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  className="tui-chip"
                  aria-pressed={state === item.id}
                  onClick={() => setState(item.id)}
                >
                  {item.pair.label}
                </button>
              ))
            : null}
          <button
            type="button"
            className="tui-chip"
            aria-pressed={theme === 'dark'}
            onClick={toggle}
          >
            {theme === 'dark' ? 'Dark' : 'Light'}
          </button>
        </div>
      </div>
      <div className={`tui-screen tui-screen-${theme}`}>
        <img src={src} alt={active.alt} width={1400} height={720} />
      </div>
    </figure>
  )
}
