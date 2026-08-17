# Cadence

A health product for a clinic running patients on peptide and hormone protocols.
Two surfaces over one backend: a patient mobile app (Android + iOS) and a doctor
web dashboard. Onboarding is invite-only — there is no public signup.

**Stack:** Go (API) · Kotlin Multiplatform + Compose Multiplatform (mobile) ·
Vite + React + TS (dashboard) · self-hosted Postgres + GoTrue on Timeweb Cloud.

Not Supabase and not Railway. ADR-008 moved the data layer into Russia on
2026-07-29; anything you find still naming those two is stale, and this file is
the place to correct it.

## Why this file exists

The project's working instructions live in `CLAUDE.md` and `AGENTS.md`, and both
are gitignored — so until now a fresh clone contained no orientation of any kind,
while `scripts/ralph/prompt.md` instructed an autonomous agent to read a file
that cannot exist on a CI runner. What follows is the part a checkout needs.

The architecture and the specs live outside the repository, in an Obsidian vault
(`20-Projects/cadence`): the system overview, one note per bounded context, the
ADRs, and the feature specs. `docs/specs/` holds snapshots exported from it. The
vault is master; the snapshots are derived and are never edited here.

## Layout

```
api/                 Go — internal/<bounded context>, migrations/, and six commands:
                       cmd/api              the server
                       cmd/migrate          the chain
                       cmd/provisioner      the trusted boundary in front of GoTrue's admin key
                       cmd/bootstrap-admin  the only way to create the clinic's first administrator
                       cmd/openapi          regenerates the committed contract
                       cmd/sqlauthorship    the gate that refuses interpolated SQL
kmp/                 Kotlin Multiplatform — shared/, composeApp/, androidApp/, iosApp/
web/                 not built yet (SKL-09); web/prototype/ is a frozen visual spec
mobile/              FROZEN. The Expo prototype: the design specification for 24 screens
docs/specs/          snapshots of approved specs (the vault is master)
docs/prototype-divergences.md   every deliberate departure from the prototype, with why
scripts/gate/        the definition of green — see below
scripts/ralph/       the autonomous loop
.github/             CI workflows and the branch ruleset — see "CI status"
```

`mobile/` and `web/prototype/` are read-only design specifications. Port from
them; never ship them and never extend them. Where a screen's behaviour is
ambiguous the prototype is the answer — but its bugs are not to be ported, and
every departure is recorded in `docs/prototype-divergences.md`.

## Prerequisites

| Tool | Version | Why |
|---|---|---|
| JDK | 21 | The KMP build |
| Go | see `api/go.mod` | The API |
| `gofumpt` | latest | Formatting — not `gofmt` |
| `golangci-lint` | **2.8.0** | Pinned in `ci.yml`; a different 2.x gives a different answer |
| Xcode | latest | The iOS half of the KMP gate — macOS only |
| XcodeGen | **2.46.0** | Pinned in `kmp/iosApp/.xcodegen-version` |
| Docker | any | The API integration suite brings up a real Postgres |
| `shellcheck`, `jq` | any | The gate scripts |

Gradle comes from the wrapper; do not install it.

## Running it

```sh
cd api && make dev-up     # Postgres and GoTrue in Docker, chain applied
cd api && make migrate-up # apply the chain on its own
cd api && make openapi    # regenerate the committed contract after any type change
```

`dev-up` does not start the API, and running it afterwards takes more than the
compose file provides: `config.Load` refuses to start without `AUTH_JWT_ISSUER`,
`AUTH_JWT_SESSION_KIDS`, `AUTH_JWT_ADMIN_KID`, `PROVISIONER_URL` and
`PROVISIONER_SHARED_SECRET`, and there is no provisioner service in that file yet.
`dev-up` prints the same list when it finishes.

`api/` uses three connection strings, deliberately: `DATABASE_URL` for the request
path (a role that owns nothing), `DATABASE_MIGRATION_URL` for the role that owns
the schema, and `DATABASE_SERVICE_URL` for the service path — which is not a
reserve for future jobs but how every profile, card, care team, preference row and
audit row is written, because no request-path policy admits any of them.

The clinic's first administrator cannot come from the API — the database refuses
`role='admin'` on the service path — so it comes from `cmd/bootstrap-admin`, run
once against the migration role. Its `-h` says the rest.

## The gate

```sh
./scripts/gate/all.sh     # the command to run before opening a pull request
```

It runs the ruleset check, `go.sh`, `kmp.sh`, the API integration suite when a
Docker daemon is up, and `ios.sh` when Xcode is present. It never says a bare
"all green": whatever it could not run on this machine is named.

What green does and does not cover:

- **`kmp.sh`** runs ktlint, detekt, `:shared:testAndroidHostTest` and the Android
  assemble. That is **244 of the tree's 537 tests**. `composeApp` declares no
  Android host-test builder, so its **293 Compose UI tests** — navigation, the
  shell, the dose wizard, inventory, trends — run under `ios.sh` and nowhere
  else. On a Linux host they are not even compiled.
- **`go.sh`** runs `go test ./... -race` with no build tag, so it reports
  `[no test files]` for `internal/platform/database`. Forced RLS, the
  low-privilege request role and the migration chain are proven only by
  `make test-integration`, which `all.sh` runs when Docker is available.
- **`ios.sh`** needs macOS and Xcode.

## CI status

CI is committed but has never executed. `.github/` is not on `main`, GitHub
reports zero registered workflows, and no ruleset is applied — so **`main` is
currently unprotected and `ci.yml` has never been validated against a real
runner.** Applying the ruleset needs admin; the command and the ordering
dependency are in `.github/rulesets/README.md`.

Until that changes, `scripts/gate/all.sh` on your own machine is the only gate
there is. Run it.

## Conventions

Language: dialogue in Russian, code and commit messages in English. The product
language is Russian — every user-facing string, on every surface.

Four project rules, with what actually enforces each:

| Rule | Enforced by |
|---|---|
| Every migration that adds a table extends the RLS policy test suite | Partly: `TestForcedRowLevelSecurityInvariant` sweeps `pg_class` and has a negative twin, but nothing checks that a *new* table gained policy tests. Behind `-tags=integration`. |
| Nothing derived is stored — no `status` column, no cached remaining count | Nothing. Reviewer's responsibility. |
| RU is the product language | Nothing. `BundledFontTest` guards the typeface, not the copy. |
| Formatters live in one module per surface | Nothing. `app.cadence.format` is the only one today by convention. |

Go, React/TypeScript and Kotlin standards are in the global `~/.claude/CLAUDE.md`.
The short version: gofumpt, errors wrapped with `%w`, `context.Context` first,
consumer-owned interfaces; functional components and TS strict; no `!!`, `val`
over `var`, `StateFlow` for UI state.

Numbers are data and formatting is presentation: a dose is `{value, unit}` and
«0,25 мг» is a render concern.
