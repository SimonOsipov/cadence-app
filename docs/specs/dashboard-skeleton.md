---
type: spec
project: cadence
status: approved
priority: p3
created: 2026-07-28
todoist_parent: "6h8w86HRwchgxPGq"
components: [web-dashboard]
proposal: ""
---
<!-- SNAPSHOT (read-only copy). Master: 20-Projects/cadence/specs/dashboard-skeleton.md in vault prll-vault. Edit the vault note, then re-export — never edit here. -->

# Dashboard Skeleton

> [!decision] 2026-08-17 — steps 1, 2 and 3 are implemented and merged to `main` as
> `ee219cb` (PR #13, squashed from 25 commits). Each was driven to an independent
> reviewer PASS: step 1 in four rounds, step 2 in four, step 3 in six. The status stays
> `approved` rather than `done` because steps 4 and 5 have not been started — they need
> the deployment contour, the manual task SKL-06, which ADR-008 renamed from a Railway
> project to a Timeweb one. Two deviations from this note were recorded during the work:
> «Сообщения» ships as a disabled span until that screen exists, and dose became
> `{value, unit}` rather than a preformatted string.

> [!deviation] 2026-08-06
> Spec said: the dashboard's static assets deploy to Railway. Actually: per
> ADR-008 deployment goes to Timeweb App Platform, and every mention of Railway
> below should be read as that. The correction was applied to
> `dashboard-first-live-read` and not to this note, its sibling; recorded here
> rather than rewritten because this spec is still `approved` and unbuilt, so
> the deployment target is settled when the work starts.

## Описание

Stand `web/` up as a real Vite application: port the design tokens from the prototype so they stop diverging, build the Overview page on fixtures, deploy it to Railway dev, and cover it with a smoke test that verifies the deployed build is the one being checked.

The point of this step is to put up a frame into which the M6 screens are added as routes rather than as a new build. The data stays fixtures: `/v1` arrives after the "API Skeleton" block, and doctor authentication is M2.

The first draft of this spec got NEEDS-REWORK from an independent judge: all three numbers in the "already implemented" section were wrong, the stated stack would not install, the side menu silently fell out of scope, and a "verbatim port" would have reproduced a violation of a component invariant. This draft was written after checking every claim against the code.

## User Story

**As a** clinic doctor (and before them, the developer porting screens from the prototype)
**I want** a dashboard that builds, deploys, and draws the Overview in the same colours and the same typography as the approved prototype
**So that** the next screen is a route and a component, not one more build setting

## Acceptance Criteria

**The frame**
- [ ] `web/` is a Vite + React + TypeScript application in strict mode; `tsc --noEmit`, ESLint, and Vitest are green
- [ ] The Node version is pinned (`.nvmrc` plus `engines`), the way Go, the JDK, and XcodeGen are pinned in the other stacks
- [ ] `web/prototype/**` is excluded from `tsconfig`, from the ESLint configuration, and from Vitest — it is frozen and must reach neither the build nor the linter
- [ ] The runners do not overlap: Vitest sees only `src/**`, Playwright only `tests/**`. Otherwise Vitest picks up the smoke spec and the gate goes red permanently, starting at step 5
- [ ] A Vitest run on an empty application does not fail (`passWithNoTests`, or a real test already in step 1) — otherwise the gate is red at the very first step
- [ ] `.gitignore` is extended: `node_modules`, `playwright-report/`, `test-results/`, `.vite/`. Today it does not contain `node_modules` at all
- [ ] `scripts/gate/web.sh` exists and is invoked from `scripts/gate/all.sh`, and a `web` job with a new change-filter output is created in `ci.yml`. All three places currently note explicitly that there is no web gate, and all three get updated

**Tokens**
- [ ] `colors_and_type.css` is ported from the prototype **byte for byte, except one line** — the Google Fonts `@import` is replaced by a local `@font-face`. The gate compares the ported file against the frozen original and fails on any other divergence
- [ ] A typed object is generated from that CSS, committed, and the gate fails on drift. Aliases of the form `--success: var(--forest-700)` resolve to a final value; the variable name is exported separately for the DOM
- [ ] A **reconciliation against `kmp/…/CadenceColors.kt`** is done: divergences are resolved explicitly and written down. At least four are known — `border` (in CSS `#e4dac6`, while the prototype's components and KMP use `#cdc1a8` from `--border-strong` under that name), `ink700` (absent from CSS, yields `undefined` in the prototype, has a value in KMP), untokenized hex values in components, and `--s-*`/`--r-*`, which do not appear in the prototype's markup at all
- [ ] A typo in a token name consumed from TypeScript is a compile error. For CSS and inline styles this is **not guaranteed**: either stylelint is added, or `var(--` is forbidden outside `tokens/`
- [ ] Three families are linked locally, the files live in the repository, and there are no runtime calls to Google Fonts: **Cormorant Garamond** (display), **Golos Text** (body text, replacing DM Sans), **JetBrains Mono**. All three have Cyrillic

**Icons**
- [ ] The subset in use is **derived by a script**, not retyped by hand. Two distinct roles. The **seed** — a one-off derivation from `dd-app.jsx`, `dd-components.jsx`, and `dd-data.jsx`: without `dd-chat*` (chat is out of scope) and without the `design-system` components the dashboard does not render (`AppHeader`, `IconBtn`, `TabBar`, `Spark`), including names from `icon:` fields, from `iconMap`, and from ternaries — around 22 icons expected. The **gate** — a standing reconciliation of the set against a grep over `web/src/**`, that is, against the new application: otherwise both sides of the comparison are produced by one script from one source and the check becomes a tautology
- [ ] An unknown icon name is a compile error

**Overview**
- [ ] The page draws the **side menu**, the stats bar, triage, the roster, today's schedule, and the patient card
- [ ] The side menu contains only "Обзор" and "Сообщения". The prototype's four items that were taken out of MVP scope ("Аналитика", "Протоколы", "Расписание", "Пациенты") are **removed**, not shown disabled: invariant 4 forbids dead controls, and honouring it from day one is cheaper than fixing it in M10
- [ ] **Every aggregate arrives precomputed**: the stats bar (average adherence included), the roster tab counters, the percentage to goal, the triage composition. In the prototype they are computed on the client — that violates invariant 2, and it does not get ported
- [ ] The ban on computing aggregates in components is a mechanism, not an intention: aggregates arrive as a separate `OverviewAggregates` type from the data layer, and the rule forbids **deriving them from `items`** — that is, it closes off the source rather than the arithmetic. A ban on `Math.*` will not do: it fires on sparkline geometry while letting `Math.max(...data)` through. The specific selectors are chosen at step 3; the criterion is that the rule does not get in the way of drawing charts and does catch any access to the raw patient array for the sake of a number
- [ ] Biomarker thresholds come from data rather than being hardcoded in components
- [ ] Components have loading, empty, and error states — otherwise they will have to be written into each of them during the move to `/v1`
- [ ] The roster fixture has the shape of a page (`items`, `total`, a cursor), not a bare array: the API will paginate and filter on the server
- [ ] A visual comparison against the prototype passes section by section across the Overview. The prototype is served locally over http for this — it does not open over `file://`

**Deployment and smoke**
- [ ] The static build is built and served in a Railway project, environment `dev`; build variables are set per environment, and a missing required one fails the build
- [ ] `web/.env.example` exists, and the gate and CI have defaults — otherwise required variables from the deployment step turn the previous step's gate red
- [ ] The build publishes its own commit (say `/version.json`), and the smoke test **reconciles the deployed SHA** against the one under test — otherwise the test hits the previous build and guards nothing
- [ ] The smoke test waits for the deployment to be ready, has a single retry, and installs browsers in CI
- [ ] A red smoke test **rolls back** the deployment to the previous one. A test after release is a detector, not a gate; the rollback is what makes it a gate

## Scope / Non-scope

**In scope:** the build frame, porting the tokens and the font mechanism, the derived icon subset, the Overview with a side menu on fixtures, the data access layer, deployment to Railway dev, the smoke test with SHA verification and rollback, the web gate and the CI job.

**Out of scope, named explicitly:**
- Real data from `/v1`. The access layer is prepared, the transport stays on fixtures.
- Sign-in and roles. Doctor authentication is M2; the page is open for now.
- Screens other than the Overview, including "Сообщения". Icons needed only by chat are not in the subset.
- The "Аналитика", "Протоколы", "Расписание", and "Пациенты" pages — out of MVP scope per the component note.
- A hermetic Playwright harness for PRs — M2. Here it is a post-deployment smoke test.
- The production environment. SKL-13 says "in the merge pipeline", and production appears in M11; here it is `dev` only.
- Generating a TS client from `openapi.json` — it is not in the SKL-09 description; the question will surface when real data is wired in.

**Blocking.** Steps 4 and 5 require a Railway project — the manual task SKL-06. Steps 1–3 do not depend on it. `/implement` will stop at step 4, and that is the correct behaviour.

## Что уже реализовано (DONE)

Verified against the code on 2026-07-28; the first draft's numbers were wrong and are corrected here.

- `web/prototype/` — the archived prototype (BST-07, commit `71e4a84`): `Doctor Dashboard.html`, five `dd-*.jsx` files, `design-system/`, and three logo SVGs. The README marks it FROZEN.
- `colors_and_type.css` contains **89** custom properties (not 121 — the earlier number came from a grep counting every occurrence of `--`). About a third of them are aliases to other variables.
- `heroicons.js` contains **41** paths (not 48).
- How many icons the dashboard uses, two different greps of mine answered differently (16 and 15), and the judge got 23. That is precisely why the list is not written into the spec: the subset is derived by a script and the gate verifies the match. By hand, this list will not be correct.
- `web/` outside `prototype/` is empty; there is no `package.json`.
- `.github/workflows/ci.yml` and `scripts/gate/all.sh` contain verbatim comments saying there is no web gate until SKL-09; both are updated here.
- `.gitignore` **does not contain** `node_modules`.

## Технические детали

**Versions and their compatibility** (verified by installing):

| Package | Version | Note |
|---|---|---|
| Vite | 8.1.5 | |
| React / React DOM | 19.2.8 | React 18 is **impossible**: `react-router@8.3.0` has peer `react >=19.2.7` |
| TypeScript | **6.0.3**, not 7.x | `typescript-eslint@8.65.0` holds peer `typescript >=4.8.4 <6.1.0`, and `typescript@7` does not expose the classic compiler API at all — linting `.tsx` will not work even with `--legacy-peer-deps` |
| ESLint / typescript-eslint | 10.8.0 / 8.65.0 | |
| @tanstack/react-query | 5.101.4 | |
| react-router | 8.3.0 | requires Node ≥ 22.22 |
| Vitest | 4.1.10 | |
| @playwright/test | 1.62.0 | |

**Tokens.** The CSS is ported byte for byte except the Google Fonts `@import` line: that is replaced by a local `@font-face`, and it is the only permissible deviation from verbatim — the gate checks the rest by comparison against the frozen original. Without such a check, "the CSS remains the source of truth" is backed by nothing.

The generator parses `:root`, resolves aliases to final values, and separately exports the variable names for the DOM. Values with commas, quotes, and functions (`--font-body`, `clamp(...)`, double shadows) require a real parser, not a regular expression.

**Reconciliation with the mobile app is mandatory.** KMP has already shipped `CadenceColors.kt`, and in places it follows the prototype's JS components rather than the CSS. Generating "from the CSS" without reconciling would split the two surfaces at the very first token. The known divergences are listed in the acceptance criteria; each is resolved by an explicit decision rather than silently.

**Icons.** The subset is derived by a script from the prototype and verified by the gate in both directions. A hand-written list was tried — it turned out to be simultaneously excessive (icons from components the dashboard does not render, and from the "Сообщения" screen) and incomplete (missing the side menu, triage, and activity feed icons).

**Aggregates.** In the prototype the stats bar is assembled by an expression over the patient array — including the average adherence named in invariant 2 of the component note. The roster tab counters and the percentage to goal are computed in the same place. All of it arrives precomputed from the fixture, and a lint rule prevents the computation from moving back into a component. An invariant with no mechanism lives until the first deadline.

**The access layer.** React Query is wired in immediately, with a transport behind it reading fixtures. One file is not enough: the fixtures have to be asynchronous, otherwise loading and error states never appear, and the roster has to be paginated, otherwise the prototype's client-side filtering moves in with it and falls apart against the real API.

**The smoke test.** A test against the deployed environment's URL is a detector, not a gate: by the time it goes red, the bad build is already serving traffic. The rollback is what makes it a gate. Plus the SHA check: Railway deploys asynchronously, and a test started right after the merge will hit the previous build and pass.

**Fonts — a decision shared with the mobile app.** `--font-display` already has a Cyrillic fallback in the CSS, and the display font is already chosen in the mobile app. Only the body text face is unresolved: DM Sans has no Cyrillic. The mechanism is built here, and the files are added once the decision lands — the same single substitution point KMP has in `cadenceTypography()`.

**Files:**

```
web/
  package.json, vite.config.ts, tsconfig.json, eslint.config.js, .nvmrc, .env.example
  scripts/generate-tokens.ts        CSS → typed object
  scripts/derive-icons.ts           the subset from the prototype
  src/
    tokens/colors_and_type.css      byte for byte, except @import
    tokens/tokens.ts                generated, committed
    fonts/                          @font-face; files land after the decision
    icons/                          the derived subset plus the component
    data/transport.ts               the seam: fixtures today, HTTP later
    data/fixtures/overview.ts       aggregates precomputed, roster paginated
    features/overview/…             menu, stats bar, triage, roster, schedule, card
    app.tsx, main.tsx, router.tsx
  tests/smoke.spec.ts
scripts/gate/web.sh
```

## Архитектурное решение

The component note already describes Vite + React + TS + React Query + react-router and requires the tokens to be ported verbatim into one typed source — the architecture does not change, so there is no proposal note.

Direction of generation: CSS is the source, TypeScript is the output. A verbatim port is only possible while the CSS remains what was ported. But "the single source of truth" is a stronger claim than reality: the prototype keeps part of its visual truth in inline hex, and KMP has already diverged from the CSS on two tokens. So generation comes together with reconciliation, and divergences are resolved by a decision rather than by a choice of tool.

The transport seam under React Query goes in from day one, together with asynchrony and pagination, otherwise the move to `/v1` becomes a rewrite of the components.

**A conflict of invariants, named explicitly.** Invariant 7 of the overview says biomarker thresholds live in a single constants module read by both surfaces; invariant 3 of the component note says they arrive in the API response. Those are two different architectures, not two phrasings of one. Here we follow the component note; on steps 1–3 that costs nothing, because the data is fixtures. The final decision is made in the "API Skeleton" block, where the thresholds are designed, and by the project's rules a system invariant requires a proposal note — the side cannot be chosen silently.

## Дельты компонентов

### web-dashboard.md
- MODIFIED: "Shape" — React 18 → React 19 (forced: the `react-router` peer); TypeScript 6, not 7 (otherwise linting does not work); tokens live as a CSS source plus a generated typed object, with drift caught by the gate; fonts are linked locally, with no runtime calls to Google Fonts.
- ADDED: to "Shape" — a transport seam under React Query (asynchronous fixtures today, HTTP when `/v1` is wired in), a typed icon component over the derived subset.
- MODIFIED: invariant 2 — **the wording is clarified, the meaning is unchanged**: "adherence" is already named in it, and average adherence is adherence. The aggregates are listed explicitly (stats bar, tab counters, percentages) and it is added that enforcement is provided by a rule rather than by an agreement.

## Decomposition

Mapping to the original tasks: steps 1–3 close SKL-09, step 4 closes SKL-10, step 5 closes SKL-13.

### step-1: The build frame and the web gate

Vite + React 19 + TypeScript 6 in strict mode, ESLint with typescript-eslint, Vitest. `.nvmrc` and `engines`. Exclusion of `web/prototype/**` from tsconfig, ESLint, and Vitest. Extending `.gitignore`. `scripts/gate/web.sh` and a `web` job in `ci.yml` appear, with a new change-filter output.

The tree builds and the gate is green on an empty application.

> [!deviation] 2026-08-17 — the stack filter moved out of `ci.yml` into a script of its own
> Spec said: «`scripts/gate/web.sh` and a `web` job in `ci.yml` appear, with a new change-filter output». Actually done: the `web` output and the `Web gate` job are there as written, and the filter's four expressions moved verbatim from the inline `run:` block into `scripts/gate/changed-stacks.sh`, with `scripts/gate/changed-stacks_test.sh` run by the Shell gate and by `all.sh`. Why: the new output was written with a `grep -q` on the far side of a pipe, and under `pipefail` that answers **false for an input it matched** once the *matching* paths pass the pipe buffer — measured here at roughly 30KB. What crosses that pipe is the matched subset rather than the change, and on a push to main that subset is under a kilobyte, so the case which reaches the threshold is a pull request touching thousands of files under one stack — the case the test carries. The direction is what makes it severe: a wrong `false` skips the job, and GitHub counts a skipped job as a satisfied required check, so the change merges having been gated by nothing. Inline, the block was reachable by neither shellcheck nor any test. The precedent for the location is this repository's own `ruleset.sh` — also a check *about* `ci.yml`, also in `scripts/gate/`, also run from the Shell gate.

todoist: "6h8w88mf3pfWxfFH"

### step-2: Tokens, fonts, and icons

A byte-for-byte port of the CSS except the `@import`; the generator with alias resolution; committing the generated object and checking for drift; comparing the ported CSS against the frozen original. Reconciliation with `CadenceColors.kt` and resolution of the known divergences.

`@font-face` with local files: Cormorant Garamond, Golos Text, JetBrains Mono. `--font-body` changes from DM Sans to Golos Text — the only value edit during the port, and it is recorded as a decision below. The gate fails if Vite could not resolve a font file.

The icon subset derivation script, the component, and the two-way check in the gate.

> [!deviation] 2026-08-17 — the icons' second direction is checked at step 3, not here
> Spec said: «the icon subset derivation script, the component, and the two-way check in the gate». Actually done: the script, the component and one direction of the check — the committed subset is re-derived from the prototype on every gate run, so it cannot drift from the screens it was taken off. The other direction, «every icon in the set is drawn by `web/src/**`», is not wired, because at this step the application draws nothing: the check would report twenty-two unused icons and have to be switched off to let the gate pass, and a check that must be switched off is not a check. It belongs with the Overview and is written into step-3's paragraph below, which is where `/implement` will read it — a check deferred to a step that does not name it is a check nobody builds. What already holds today is the stronger half of the same property — `IconName` is a union of the derived names, so a name the subset lacks does not compile, asserted with `@ts-expect-error`.
> The derivation itself diverged too, and for a measured reason: the sweep is over every quoted string in the three files intersected with what `heroicons.js` carries, rather than over the syntax that names an icon. Reading `Icon name="…"` and `icon: '…'` finds 21; the sweep finds 22. The missing one is `icon="check-circle"` in `dd-app.jsx:294` — an attribute rather than a field. Intersecting with the source of truth bounds the net rather than making it safe: a quoted string matching no `heroicons.js` key cannot become an icon, but one that matches a key does — `item.kind === 'checkin'` in `dd-components.jsx:222` is one character from the key `check`. Only the second direction closes that, which is another reason it belongs in step 3.

> [!deviation] 2026-08-17 — `var(--…)` is confined by a lint rule, and stylelint is not added
> Spec said the criterion's second half as a choice: «either stylelint is added, or `var(--` is forbidden outside `tokens/`». Actually done: the second, as an ESLint `no-restricted-syntax` rule over `src/**` with `src/tokens/**` exempt, matching both string literals and template chunks. Why: stylelint is a second linter, a second config and a second thing for the gate to run, for a rule this project needs in exactly one shape — and the shape it needs is about TypeScript files, which ESLint already parses. The rule is added now, while it is trivially green: step 3 writes the first component that could break it, and a rule arriving after its violations gets an exception instead of obedience. Probed against a file styling with `var(--paper)`: refused, and probed against `src/tokens/**` to confirm the exemption is local to that rule rather than switching the directory off. The half ESLint cannot reach — a `.css` file outside `tokens/`, and `src/fonts/fonts.css` is already one — is a grep in `web.sh`. One grep per file, its own status read, and the count of files read compared against the count found. Two shapes were measured broken first: a word-split variable with `|| true`, where a path containing a space split into two unreadable ones and the check went green having read nothing — this repository carries such a path a directory away, `web/prototype/Doctor Dashboard.html` — and then `find -exec grep {} +`, which fixed the splitting but returns **find's** status, and find collapses grep's 2 into its own 1, so a healthy run and an unreadable file answered alike.

todoist: "6h8w88wr2qpwvvRH"

### step-3: The Overview on fixtures

The side menu (with the accepted decision on the four out-of-scope items), the stats bar, triage, the roster, the schedule, the patient card. The transport seam, asynchronous fixtures, a paginated roster, loading/empty/error states. Every aggregate arrives precomputed; a lint rule forbids computing them in components. Biomarker thresholds come from data.

Carried here from step-2, which could not hold it: the second direction of the icon check — every icon in the derived subset is drawn by `web/src/**` — and it will not pass on arrival, because `book-open`, `calendar` and `chart-bar` reach the subset only through the four side-menu items this spec removes from the MVP. Deriving from the prototype and drawing the MVP are two different sets, and the difference is decided here rather than assumed away.

> [!deviation] 2026-08-17 — what the section-by-section comparison found, and what it leaves standing
> Both surfaces were served over http and screenshotted at 1440px. Three real defects came out of it, all fixed: weights rendered with a decimal point («104.2 кг») where the product writes «104,2 кг», which is the formatting rule with no formatter behind it — `src/format.ts` is that module now, and every number on the screen goes through it; the schedule showed raw patient ids for two of its eight entries, because the screen resolved names out of the roster page it happened to be showing and the roster is one page of twenty-five — the entry carries the name now, composed on the far side of the seam; and the prototype's opening greeting was absent entirely. The greeting reads «Здравствуйте, …» and not the prototype's «Доброе утро, …»: a hard-coded time of day is wrong for most of the working day, and the clock this would need is not something this block has.
> What the comparison does **not** pass on, deliberately, and each for a reason: the prototype's search field and its «Новый пациент» button are not drawn, on the same rule that removes four destinations from the side menu — searching the roster and creating a patient are things this block cannot do, and a control that does nothing is the dead control invariant 4 forbids. The same goes for «Все флаги» and «Открыть календарь», which lead to screens that are out of MVP scope, and for the triage cards' «Открыть»/«Написать» buttons, of which only «Открыть» has anywhere to go — the patient's name on the tile is the button instead.
> What is **cosmetic and outstanding**: the roster has no avatars, no per-row sparkline and no sort control; the triage cards omit the avatar and the protocol line; the sidebar has neither the brand mark nor the doctor's own footer card; and the «Внимание» stat card is not given the prototype's forest treatment. None changes what the screen says, all are ported pixels rather than decisions, and they are named here rather than left to be discovered — the criterion says the comparison passes section by section, and today it passes on structure and content and not on ornament.

> [!deviation] 2026-08-17 — «Сообщения» is in the menu and does not navigate
> The criterion mandates exactly two destinations, and the second one leads to a screen another block builds. Rendered as an `<a href>` it reloaded the page and served the Overview straight back with «Обзор» marked as the current page — the only control on the screen promising something it could not do, while the comparison callout justifies removing five others on precisely that ground. It renders as an `aria-disabled` span until the Messages screen exists; the item stays because the criterion names it, and the promise is withdrawn rather than the item. `react-router` was installed for the routing this would have needed and imported by nothing, so it is removed too — step 4 adds it when there is a second screen to route to.

> [!deviation] 2026-08-17 — the dose became a quantity, and the fixture stopped carrying it written out
> The prototype writes `dose: '1,0 мг'`, and the seeded fixture carried that string through to the screen. This project's own rule uses that exact example — «Dose is `{value, unit}`; «0,25 мг» is a render concern» — so the type is `{ value, unit }` now and the screen formats it. Measured before the change: a fixture edited to `"1.0 мг"` put a decimal point in front of a doctor and passed all 285 tests, because a pre-rendered string is invisible to a formatter that is never called. It also gave `quantity(v, u, 2)` its first production caller — the case `format.test.ts` pins with 0,25 had none.

todoist: "6h8w895m4HMqr9jH"

### step-4: Deployment to Railway dev

⏸ **Requires SKL-06.**

The face is chosen (Golos Text) and the files land in step 2 — but verifying that with the gate has to happen separately: `@font-face` without files builds **green**, and Vite leaves the reference to be resolved at runtime. So a missing file will not go red at build time or in step 2; it surfaces as a 404 on Railway. The gate is obliged to fail on that Vite warning.

Static assets in the `dev` environment, build variables per environment, `web/.env.example` and defaults for the gate. The build publishes its own commit. The page is compared against the prototype served locally.

todoist: "6h8w897hhGJHmfHq"

### step-5: The smoke test as a gate, with rollback

Waiting for the deployment to be ready, reconciling the deployed SHA, checking the Overview page and the roster, one retry, browser installation in CI. A red smoke test rolls the deployment back. The config is written to be extensible — in M2 a hermetic PR harness grows out of it.

todoist: "6h8w89CJJWM2x3cH"

## Открытые вопросы

> [!decision] 2026-08-17 — the dashboard links exactly the faces the mobile app bundles.
> Implied by the decision below and made an invariant by `fonts.test.ts`, which reads the set out of `kmp/composeApp/src/commonMain/composeResources/font/` rather than listing it. A fifth face added for a mobile screen is therefore a face the dashboard has to link too, and the web gate runs on that change — `changed-stacks.sh` covers that directory for exactly this reason. The cost is real and accepted: the alternative is two sets drifting, which is the divergence the whole token port exists to close.

> [!decision] 2026-07-28 — the body text face: **Golos Text** instead of DM Sans, on both surfaces.
> Reason: DM Sans has no Cyrillic (`latin`, `latin-ext`), and all product copy is Russian. In the prototype this check was done for the display face (Instrument Serif → Cormorant Garamond) and not done for the text face. Golos Text was designed for Cyrillic rather than extended into it, has an open licence, and is close to DM Sans in width and tone — the prototype's layouts will not shift.
> Consequences: in the dashboard `--font-body` changes during the port (the only value edit); in the mobile app the body face changes, and the files have to be placed in `composeApp` — a separate task, since BST-05 is already closed. (Done 2026-08-03: the three faces are bundled and the scale is built by `cadenceTypography()`; the `CadenceFonts` object that once held them no longer exists.)

> [!decision] 2026-08-17 — `border` is `--border-strong`, and neither side is renamed.
> Measured: the CSS declares `--border: #e4dac6` and `--border-strong: #cdc1a8`, while the prototype's own JS palette (`design-system/cadence-components.jsx:10`) declares `border: '#cdc1a8'`. Both surfaces that shipped followed the JS — `CadenceColors.border` is `#cdc1a8`, and every `C.border` in `dd-*.jsx` draws with it. The port may not edit the CSS, so the dashboard reads `borderStrong` wherever the prototype read `C.border`. Nothing is renamed on either side; what is written down is that the two are one colour under two names. One more line for step 3: CSS `--border` is byte-for-byte `--bone` (`#e4dac6`), which is how the JS palette could drop it rather than lose it — so a place the prototype drew with `C.bone` maps to either name, and the choice is made once. `reconciliation.test.ts` asserts the disagreement still exists — a recorded divergence that has quietly gone away fails as loudly as a new one.
> The other three of the four the acceptance criteria name resolve without an edit. `ink700` is the mobile app's alone (`#3f3b35`; the prototype yields `undefined` for it, so it never rendered on the web) and is not invented here — the CSS is the source, and a token it lacks is a design question. The untokenized hexes are eight names Kotlin carries from literals in *mobile* screens, listed in the same test. And `hairline` is not a divergence at all: Kotlin writes it as an ink with an alpha applied and the CSS as `rgba(...)`, the same colour in two spellings, so the test normalises rather than excuses it.

> [!decision] 2026-08-17 — `--s-*` and `--r-*` are kept, and the Overview is to use them.
> Measured: neither scale is referenced anywhere in the prototype's markup — it spaces and rounds with literals. They are ported anyway, because the port is verbatim, and they reach `tokens.ts` like everything else. The consequence is for step 3: a component needing a gap or a radius takes it from these scales rather than writing a number, which is the whole reason a scale nobody used is still worth carrying.

> [!decision] 2026-07-28 — the side menu shows only "Обзор" and "Сообщения".
> The prototype's four items that fall outside MVP scope are removed rather than shown disabled. Invariant 4 of the component note forbids dead controls by M10; honouring it immediately is cheaper than fixing it later, and a doctor at a demo will not see promises the MVP does not keep. The roster in the Overview *is* the patient list — a separate "Пациенты" item is unnecessary per the same note.
