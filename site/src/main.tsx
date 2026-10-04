import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import { activateQuarry } from './quarry/scene'
import './styles.css'

if (typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)) {
  document.documentElement.classList.add('is-apple')
}

activateQuarry()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
