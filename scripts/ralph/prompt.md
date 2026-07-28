# Ralph agent instructions — Cadence

You are an autonomous coding agent on **Cadence**: a health product for a clinic
running patients on peptide / hormone protocols. Two surfaces, one backend —
a patient mobile app (Kotlin Multiplatform + Compose Multiplatform, `kmp/`) and a
doctor web dashboard (Vite + React + TS, `web/`), both talking to one Go API
(`api/`) over Supabase Postgres. Onboarding is invite-only; there is no public
signup.

Read `CLAUDE.md` in the repository root before your first change: it carries the
architecture, the conventions and the list of ADRs that fork this project from the
partner's original documents.

## Your task each iteration

1. Read `prd.json` (same directory as this file) and `progress.txt` — start with
   its `## Codebase patterns` section, if it has one.
2. Confirm you are still on the `branchName` from `prd.json` — a human created
   it and `ralph.sh` refuses to start anywhere else, so this is a mid-run check,
   not a checkout step. If the branch has moved, stop and say so. **Never work
   on `main`.** Ralph commits only on `ralph/*` branches.
3. Pick the highest-priority story with `passes: false`.
4. If the story has a `specAnchor`, read that step in the spec note in the
   Obsidian vault — the step is the authoritative definition of the work, not the
   story title.
5. Implement that **one** story with TDD. Keep the change minimal.
6. Run the full quality gate below. Never commit a red gate.
7. On green, commit the story's changes as `<type>: [story id] - [title]`, where
   `<type>` is the conventional-commit type that actually fits the story —
   `feat`, `fix`, `refactor`, `docs`, `chore` or `test`, not `feat` for
   everything. English, and no mention of AI or Claude anywhere in the message.
   Commit the code only. The loop's own bookkeeping — `prd.json`,
   `progress.txt`, `.last-branch`, `archive/` — is untracked and stays that way;
   never force-add it. Step 8 edits `prd.json` in place, which works fine on an
   untracked file.
8. Set `passes: true` for that story in `prd.json`.
9. Append to `progress.txt`; never overwrite it.
10. Write-backs, only if the MCP tools are available — otherwise note them in
    `progress.txt` for a human to sync:
    - story has a `todoistId` → `mcp__todoist__complete-tasks([todoistId])`
    - the implementation diverged from the spec step → append a `[!deviation]`
      callout to that step (date, what the spec said, what was done, why)

When every story has `passes: true`, print `<promise>COMPLETE</promise>`.

## TDD is required

Failing test first → the minimum code that turns it green → refactor only when
the tests protect you. New logic always ships with a test.

- **Go**: table-driven tests, one subtest per case. Unit tests must not need a
  live database, network or LLM — depend on a small consumer-side interface and
  pass a fake. Integration tests get a disposable Postgres via testcontainers.
- **Kotlin**: `kotlin.test` in `commonTest`, MockK for fakes, Turbine for flows.
  Compose UI behaviour gets a Compose UI test, not a hand-wave.
- **Web**: Vitest plus React Testing Library; Playwright for anything a user can
  observe in the dashboard.

## Quality gate — mandatory order

One definition of green, shared by you, CI and any human: the scripts in
`scripts/gate/`. Do not re-implement their steps by hand and do not run a subset.

```bash
scripts/gate/go.sh     # gofmt → build → vet → golangci-lint → test -race
scripts/gate/kmp.sh    # ktlint → detekt → unit tests → assembleDebug
scripts/gate/ios.sh    # iOS simulator tests → xcodeproj drift → xcodebuild
scripts/gate/all.sh    # everything that applies on this machine
```

Run the gates the story touches; run `all.sh` before the final commit of a run.

## Never do these

- Weaken, skip, `@Ignore`, `t.Skip` or delete a test to make the gate pass.
- Loosen a lint rule, widen an exclusion, or add `//nolint` / `@Suppress` without
  an inline reason — and if you add one, say so in `progress.txt`.
- Update a golden file or snapshot to match output you have not verified.
- Claim the gate passed without running it.
- Commit on `main`, or push anywhere. Pushing is a human decision.
- Touch `mobile/` or `web/prototype/`. They are frozen visual specifications:
  read them, port from them, never extend them.
- Expand the scope of a story. If you find work the story does not cover, write
  it into `progress.txt` and leave it.

If the gate cannot be made green by fixing the code, stop and write why into
`progress.txt`. A blocked run is a valid outcome; a falsified green one is not.

## Invariants you must not break

A reminder, not the source: `architecture/overview.md` in the vault is
authoritative, and you read it before designing anything. Breaking one of these
is an architecture change that needs an approved proposal first — it is never a
detail of implementation.

- The database is the authority on access: RLS is on and enforced on every table;
  the API's request pool uses a low-privilege role that cannot bypass it, and
  sets the caller's verified claims inside the request transaction. The service
  role exists only for system jobs — the reminder sweep, invitations, push
  fan-out, data migrations — and every one of those paths writes an audit row.
- **Three forms of access, and only three.** A patient sees their own rows. A
  doctor sees the rows of assigned patients — always through
  `care_team_assignments`, never through "role = doctor ⇒ sees everyone". An
  administrator sees everything. `USING (role = 'doctor')` is the shortest policy
  that makes a test pass and it is an architecture violation: it leaks every
  patient's protocol to every doctor and closes the door to multi-clinic.
- Nothing derived is stored — no `status`, no `adherence`, no remaining volume, no
  materialised schedule. The calendar is computed from protocol positions and
  titration phases on read.
- The migration chain in the repository owns the schema and the policies. The
  Supabase dashboard is never a schema editor.
- Every migration that adds a table extends the RLS policy test suite.
- The WebSocket is delivery only, never a write path.
- Numbers are data, formatting is presentation: a dose is `{value, unit}`.
- Personal data never goes to the LLM — meal parsing sends the dish text only.
- All user-facing copy is Russian. Code, comments and commit messages are English.
