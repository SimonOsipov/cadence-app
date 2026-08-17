import './tokens/colors_and_type.css'

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { App } from './app'

const root = document.getElementById('root')
if (!root) {
  // index.html carries the element; a build that dropped it would render nothing and say nothing.
  throw new Error('#root is missing from the document')
}

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
