import { lazy, Suspense, useEffect, useState } from 'react'

const YardCanvas = lazy(() => import('./YardCanvas'))

function prefersReducedMotion() {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

function can3D() {
  if (typeof document === 'undefined') return false
  const canvas = document.createElement('canvas')
  return Boolean(canvas.getContext('webgl2') || canvas.getContext('webgl'))
}

export function YardBand({ tall = false }: { tall?: boolean }) {
  const [motion, setMotion] = useState(false)

  useEffect(() => {
    if (prefersReducedMotion() || !can3D()) return
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
    <div
      className={`pointer-events-none relative isolate overflow-hidden bg-[var(--clay-sky)] ${tall ? 'h-40 md:h-52' : 'h-32 md:h-40'}`}
      aria-hidden="true"
    >
      {motion ? (
        <Suspense fallback={null}>
          <div className="absolute inset-0">
            <YardCanvas />
          </div>
        </Suspense>
      ) : null}
    </div>
  )
}
