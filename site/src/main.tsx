import 'mokei/theme/quarry'
import { applyTheme, setDefaultThemeId } from 'mokei/theme'
import { registerScene, setDefaultSceneId } from 'mokei/scene'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import { quarryScene } from './quarry/scene'
import './styles.css'

if (typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)) {
  document.documentElement.classList.add('is-apple')
}

setDefaultSceneId('quarry')
setDefaultThemeId('quarry')
registerScene(quarryScene)
applyTheme('quarry')

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
