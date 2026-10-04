import { lazy, Suspense, useEffect, useState } from 'react'
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

export function Stage() {
  const [motion, setMotion] = useState(false)
  const [pretty, setPretty] = useState(false)

  useEffect(() => {
    const reduce = prefersReducedMotion()
    const wide = window.matchMedia('(min-width: 720px)').matches
    setPretty(wide && !reduce)
    if (reduce) {
      useQuarry.getState().dispatch({ type: 'task-progress', progress: 0.38 })
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

  return (
    <div className="relative isolate h-[20rem] overflow-hidden bg-[var(--clay-sky)] md:h-[24rem]">
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
    </div>
  )
}
