import js from '@eslint/js'
import reactHooks from 'eslint-plugin-react-hooks'
import tseslint from 'typescript-eslint'

// Reach for tokens.x instead of naming a custom property: a mistyped one renders as nothing, silently.
// Case- and space-insensitive because CSS is both.
const reachForTokens =
  'Reach for tokens.x instead: a mistyped custom property renders as nothing, silently.'

const cssVariables = [
  { selector: "Literal[value=/var\\s*\\(\\s*--/i]", message: reachForTokens },
  { selector: "TemplateElement[value.raw=/var\\s*\\(\\s*--/i]", message: reachForTokens },
]

// Invariant 2 of the component note, given a mechanism. Every number the Overview shows arrives
// computed; the way that stops being true is a component reaching into the rows it was handed for one
// — `patients.filter(p => p.status === 'attention').length` is what the prototype does four times over,
// and it reads like nothing at all.
//
// The rule closes the *source*, not the arithmetic: a ban on Math.* would fire on sparkline geometry
// while letting Math.max(...items) through. `map` stays, because that is how rows are drawn, and
// `length` stays, because it answers how big this page is rather than how many patients there are —
// that number is `total`, and it comes from the seam.
//
// Four shapes, because three of them were found walking through the first version: a chained
// `.map(…).reduce(…)` is a CallExpression on the object side, a `for…of` over the rows carries no array
// method at all for a selector to reach, and a spread makes the object an ArrayExpression. What no
// syntax rule can close is a rename — `const rows = page.items` — and that is the rule's stated limit
// rather than a gap nobody noticed.
const derivesFromRows = /^(reduce|reduceRight|filter|some|every|find|findIndex|flatMap|sort)$/.source
const rowsNamed = /^(items|patients|roster|triage|schedule)$/.source
const takeTheAggregate =
  'Take the number from the aggregates the seam returns — deriving it here makes the client a second source of truth for it.'

const derivedAggregates = [
  { selector: `MemberExpression[object.name=/${rowsNamed}/][property.name=/${derivesFromRows}/]`, message: takeTheAggregate },
  { selector: `MemberExpression[object.property.name=/${rowsNamed}/][property.name=/${derivesFromRows}/]`, message: takeTheAggregate },
  { selector: `MemberExpression[object.callee.object.property.name=/${rowsNamed}/][property.name=/${derivesFromRows}/]`, message: takeTheAggregate },
  { selector: `MemberExpression[object.type='ArrayExpression'][property.name=/${derivesFromRows}/]`, message: takeTheAggregate },
  { selector: `ForOfStatement[right.property.name=/${rowsNamed}/]`, message: takeTheAggregate },
  { selector: `ForOfStatement[right.name=/${rowsNamed}/]`, message: takeTheAggregate },
]

export default tseslint.config(
  // First and alone: `prototype/` is the frozen in-browser Babel prototype, and every rule below would
  // have something to say about it.
  // The probes violate both rules on purpose; the gate lints them with --no-ignore and requires the
  // refusals. Ignored here so an ordinary `eslint .` stays green.
  { ignores: ['prototype/**', 'dist/**', 'src/features/__probes__/**'] },
  js.configs.recommended,
  {
    // Type-checked rules are scoped to the files a TS project actually covers. Applied globally they
    // are handed this very file and fail on it, which reads like a broken rule rather than a
    // misconfiguration.
    files: ['**/*.{ts,tsx}'],
    extends: [tseslint.configs.recommendedTypeChecked],
    languageOptions: {
      parserOptions: { projectService: true, tsconfigRootDir: import.meta.dirname },
    },
  },
  // configs.flat, not configs['recommended-latest']: at v7 the top-level entries are still
  // eslintrc-shaped and ESLint 10 refuses them — `plugins` as an array of strings.
  reactHooks.configs.flat['recommended-latest'],
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/tokens/**'],
    rules: { 'no-restricted-syntax': ['error', ...cssVariables] },
  },
  {
    // Both selector sets, and the repetition is the point: ESLint flat config *replaces* a rule's
    // options by name rather than merging them, so a second `no-restricted-syntax` block over
    // src/features/** silently switched off the var(--…) guard exactly where the components are.
    // Measured — `var(--paper)` in a feature file linted clean while the same line outside did not.
    files: ['src/features/**/*.{ts,tsx}'],
    ignores: ['src/features/**/*.test.{ts,tsx}'],
    rules: { 'no-restricted-syntax': ['error', ...cssVariables, ...derivedAggregates] },
  },
  {
    // scripts/ runs under Node rather than in the browser, and the config above declares neither.
    // Named one by one instead of pulling in a globals package: two entries, and a third would be as
    // obvious to add as this comment is to read.
    files: ['scripts/**/*.{js,mjs}'],
    languageOptions: { globals: { console: 'readonly', process: 'readonly' } },
  },
  { files: ['**/*.{js,mjs}'], extends: [tseslint.configs.disableTypeChecked] },
)
