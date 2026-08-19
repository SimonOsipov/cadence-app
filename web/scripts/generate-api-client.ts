// Writes src/api/ from the API's committed contract, ../api/openapi.json.
//
//   node scripts/generate-api-client.ts          rewrites the modules
//   node scripts/generate-api-client.ts --check  fails if they have drifted
//
// The gate runs --check. Committing the output and regenerating it are not alternatives: the commit is
// what makes a contract change readable in a diff — an added field, a widened enum, a response that
// stopped being nullable — and the check is what keeps that copy honest.
//
// Types and no runtime. The generator will also emit a client of its own, and doing so vendors sixteen
// files of somebody else's fetch layer into this repository; what the contract actually decides is the
// shapes, and the twenty lines that call it are in src/data/api.ts where they can be read. The one
// thing a types-only output cannot catch — a route renamed under an unchanged shape — is asserted
// instead by src/data/api.test.ts against the same document.
import { createClient } from '@hey-api/openapi-ts'
import { mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

const CONTRACT = '../api/openapi.json'
const OUTPUT = 'src/api'

const staging = mkdtempSync(join(tmpdir(), 'cadence-api-types-'))

try {
  await createClient({
    input: CONTRACT,
    output: { path: staging, postProcess: [] },
    plugins: ['@hey-api/typescript'],
    logs: { level: 'silent' },
  })

  const generated = readdirSync(staging).sort()
  if (generated.length === 0) {
    console.error(`the generator wrote nothing from ${CONTRACT}, so this check measured nothing`)
    process.exit(1)
  }

  if (process.argv.includes('--check')) {
    const committed = readdirSync(OUTPUT).sort()

    // The set and not each file in turn: a module the generator stopped emitting is drift the
    // committed copy cannot show, because nothing looks at a file that is no longer generated.
    if (committed.join() !== generated.join()) {
      console.error(
        `${OUTPUT} holds [${committed.join(', ')}] and ${CONTRACT} generates [${generated.join(', ')}] — run: npm run api`,
      )
      process.exit(1)
    }

    for (const file of generated) {
      if (readFileSync(join(OUTPUT, file), 'utf8') !== readFileSync(join(staging, file), 'utf8')) {
        console.error(`${OUTPUT}/${file} is not what ${CONTRACT} generates — run: npm run api`)
        process.exit(1)
      }
    }

    console.log(`${OUTPUT} matches ${CONTRACT}`)
  } else {
    for (const file of readdirSync(OUTPUT)) {
      rmSync(join(OUTPUT, file))
    }

    for (const file of generated) {
      writeFileSync(join(OUTPUT, file), readFileSync(join(staging, file), 'utf8'))
    }

    console.log(`wrote ${OUTPUT}: ${generated.join(', ')}`)
  }
} finally {
  rmSync(staging, { recursive: true, force: true })
}
