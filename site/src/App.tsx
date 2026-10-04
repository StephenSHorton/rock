import { CliSection } from './cli/CliSection'
import { Install } from './cli/Install'
import { Compare } from './compare/Compare'
import { Features } from './features/Features'
import { Hero } from './hero/Hero'
import { HowItWorks } from './how/HowItWorks'
import { Footer } from './layout/Footer'
import { Nav } from './layout/Nav'
import { useQuarry } from './quarry/sim'
import { useEffect } from 'react'

export function App() {
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
    <>
      <a className="skip" href="#content">
        Skip to content
      </a>
      <Nav />
      <main id="content">
        <Hero />
        <HowItWorks />
        <Features />
        <Compare />
        <Install />
        <CliSection />
      </main>
      <Footer />
    </>
  )
}
