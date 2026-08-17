import '@testing-library/jest-dom/vitest'

import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// Registered by hand because this project does not run Vitest with globals: without it every test
// leaves its DOM behind, the next one finds two of everything, and `getBy…` fails with «found
// multiple» on a screen that is perfectly correct.
afterEach(cleanup)
