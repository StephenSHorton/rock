import { lazy, Suspense, useEffect, useState } from 'react'
import { Hud } from './Hud'
import { StaticQuarry } from './StaticQuarry'
import { useQuarry } from './sim'

const QuarryCanvas = lazy(() => import('./QuarryCanvas'))

function prefersReducedMotion() {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

function can3D() {
  if (typeof document === 'undefined') return false
  const canvas = document.createElement('canvas')
  return Boolean(canvas.getContext('webgl2') || canvas.getContext('webgl'))
}

export function Hero() {
  const [motion, setMotion] = useState(false)
  const [pretty, setPretty] = useState(false)

  useEffect(() => {
    const reduce = prefersReducedMotion()
    const wide = window.matchMedia('(min-width: 720px)').matches
    setPretty(wide && !reduce)
    if (reduce) {
      useQuarry.getState().dispatch({ type: 'task-progress', progress: 0.88 })
      useQuarry.setState({ playing: false })
      return
    }
    if (!can3D()) return
    const start = () => setMotion(true)
    const ric = window.requestIdleCallback
    if (typeof ric === 'function') {
      const idle = ric(start, { timeout: 900 })
      return () => window.cancelIdleCallback(idle)
    }
    const t = window.setTimeout(start, 120)
    return () => window.clearTimeout(t)
  }, [])

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement) return
      if (event.code === 'Space') {
        event.preventDefault()
        useQuarry.getState().toggle()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  return (
    <section className="relative isolate min-h-[34rem] overflow-hidden bg-[var(--clay-sky)] md:min-h-[42rem] lg:min-h-[48rem]">
      <h1 className="sr-only">Rock, a Go AI coding CLI</h1>
      <div className="absolute inset-0">
        <StaticQuarry />
      </div>
      {motion ? (
        <Suspense fallback={null}>
          <div className="absolute inset-0">
            <QuarryCanvas pretty={pretty} />
          </div>
        </Suspense>
      ) : null}
      <Hud />
      <p className="sr-only">
        The diorama maps a Rock session: the prompt is a drill rig, agents are ore carts on rails,
        tool calls are stations, a permission gate can stop a cart, and output is crates on a dock.
        Pause and replay from the HUD. Space toggles playback. Reduced motion keeps this still picture.
      </p>
    </section>
  )
}
