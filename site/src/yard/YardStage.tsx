import { lazy, Suspense, useEffect, useRef, useState } from 'react'

const YardCanvas = lazy(() => import('./YardCanvas'))

function prefersReducedMotion() {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

function can3D() {
  if (typeof document === 'undefined') return false
  const canvas = document.createElement('canvas')
  return Boolean(canvas.getContext('webgl2') || canvas.getContext('webgl'))
}

export function YardStage() {
  const root = useRef<HTMLDivElement>(null)
  const [ready, setReady] = useState(false)
  const [onScreen, setOnScreen] = useState(true)

  useEffect(() => {
    if (prefersReducedMotion() || !can3D()) return
    const start = () => setReady(true)
    const ric = window.requestIdleCallback
    if (typeof ric === 'function') {
      const idle = ric(start, { timeout: 900 })
      return () => window.cancelIdleCallback(idle)
    }
    const t = window.setTimeout(start, 80)
    return () => window.clearTimeout(t)
  }, [])

  useEffect(() => {
    const el = root.current
    if (!el || typeof IntersectionObserver === 'undefined') return
    const io = new IntersectionObserver(
      ([entry]) => {
        setOnScreen(entry.isIntersecting && entry.intersectionRatio > 0.04)
      },
      { threshold: [0, 0.04, 0.2] },
    )
    io.observe(el)
    return () => io.disconnect()
  }, [])

  return (
    <div
      ref={root}
      className="pointer-events-none absolute inset-0 -z-10 min-h-[100svh] w-full bg-[var(--clay-sky)]"
      aria-hidden="true"
    >
      {ready ? (
        <Suspense fallback={null}>
          <div className="absolute inset-0">
            <YardCanvas play={onScreen} />
          </div>
        </Suspense>
      ) : null}
    </div>
  )
}
