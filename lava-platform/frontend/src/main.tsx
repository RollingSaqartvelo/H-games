import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './audio/mute'   // patch Audio() before any sound module instantiates
import './index.css'
import { App } from './App'

const root = document.getElementById('root')!
createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
