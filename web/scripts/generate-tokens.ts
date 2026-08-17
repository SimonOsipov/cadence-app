// Writes src/tokens/tokens.ts from src/tokens/colors_and_type.css.
//
//   node scripts/generate-tokens.ts          rewrites the module
//   node scripts/generate-tokens.ts --check  fails if the module has drifted
//
// The gate runs --check. Committing the output and regenerating it are not alternatives: the commit is
// what makes the tokens readable in a diff and importable without a build step, and the check is what
// keeps that copy honest.
import { readFileSync, writeFileSync } from 'node:fs'

import { parseTokens, renderModule } from './tokens.ts'

const SOURCE = 'src/tokens/colors_and_type.css'
const MODULE = 'src/tokens/tokens.ts'

const rendered = renderModule(parseTokens(readFileSync(SOURCE, 'utf8')))

if (process.argv.includes('--check')) {
  const committed = readFileSync(MODULE, 'utf8')

  if (committed !== rendered) {
    console.error(`${MODULE} is not what ${SOURCE} generates — run: npm run tokens`)
    process.exit(1)
  }

  console.log(`${MODULE} matches ${SOURCE}`)
} else {
  writeFileSync(MODULE, rendered)
  console.log(`wrote ${MODULE}`)
}
