// Runs the build inspection against dist/. See check-build-assets.ts for what it asserts and why.
import { REQUIRED_FACES, inspect } from './check-build-assets.ts'

const verdict = inspect('dist', REQUIRED_FACES)

if (!verdict.ok) {
  for (const problem of verdict.problems) console.error(problem)
  process.exit(1)
}

console.log(`the build carries every asset its stylesheets name, the ${REQUIRED_FACES.length} faces included`)
