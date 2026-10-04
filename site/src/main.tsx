import { applyTheme } from 'mokei/theme'
import { registerScene } from 'mokei/scene'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import { applyPagePalette } from './quarry/palette'
import { quarryScene } from './quarry/scene'
import './styles.css'

if (typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)) {
  document.documentElement.classList.add('is-apple')
}

registerScene(quarryScene)
applyTheme('quarry')
applyPagePalette()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
