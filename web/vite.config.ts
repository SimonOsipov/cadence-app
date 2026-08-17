import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: ['src/test-setup.ts'],
    // Rooted in src/ rather than merely excluding what it must not reach: an exclusion can be
    // out-argued by a wider include, and step 5's Playwright spec in tests/ would then be run by a
    // runner that is not Playwright — red permanently, on a branch that changed nothing about it.
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
    exclude: ['prototype/**', 'tests/**', 'node_modules/**', 'dist/**'],
  },
})
