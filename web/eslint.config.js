import js from '@eslint/js'
import reactHooks from 'eslint-plugin-react-hooks'
import tseslint from 'typescript-eslint'

export default tseslint.config(
  // First and alone: `prototype/` is the frozen in-browser Babel prototype, and every rule below would
  // have something to say about it.
  { ignores: ['prototype/**', 'dist/**'] },
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
    // The acceptance criterion's second half: a typo in a token name is a compile error from
    // TypeScript, and nothing gives CSS-in-JS the same. So `var(--…)` is confined to src/tokens/, where
    // the names come from, and everywhere else a component reaches for `tokens.x` and gets checked.
    //
    // Added while it is trivially green — step 3 writes the first component that would break it, and a
    // rule that arrives after its violations is a rule that gets an exception instead.
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/tokens/**'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector: "Literal[value=/var\\(--/]",
          message: 'Reach for tokens.x instead: a mistyped custom property renders as nothing, silently.',
        },
        {
          selector: "TemplateElement[value.raw=/var\\(--/]",
          message: 'Reach for tokens.x instead: a mistyped custom property renders as nothing, silently.',
        },
      ],
    },
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
