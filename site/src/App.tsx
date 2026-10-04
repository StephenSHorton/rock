import { CliSection } from './cli/CliSection'
import { Install } from './cli/Install'
import { Compare } from './compare/Compare'
import { Features } from './features/Features'
import { Hero } from './hero/Hero'
import { Jev } from './jev/Jev'
import { Footer } from './layout/Footer'
import { Nav } from './layout/Nav'
import { YardBand } from './yard/YardBand'

export function App() {
  return (
    <>
      <a className="skip" href="#content">
        Skip to content
      </a>
      <Nav />
      <main id="content">
        <Hero />
        <YardBand />
        <Jev />
        <Features />
        <Compare />
        <Install />
        <CliSection />
      </main>
      <Footer />
    </>
  )
}
