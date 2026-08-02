---
type: spec
project: cadence
status: approved
priority: p3
created: 2026-07-28
todoist_parent: "6h8xwxR3mpXjmxMq"
components: [api, kmp-app, web-dashboard]
proposal: "[[20-Projects/cadence/architecture/proposals/data-layer-to-russia|architecture/proposals/data-layer-to-russia]]"
---

<!-- SNAPSHOT (read-only copy). Master: 20-Projects/cadence/specs/deploy-observability.md in vault prll-vault. Edit the vault note, then re-export — never edit here. -->

# Deployment and Observability

> [!decision] 2026-07-29 — the spec was revised for the data layer's move to Russia
> The 152-FZ gate from [[20-Projects/cadence/adr/ADR-006-supabase-and-152fz|ADR-006]] fired; the data layer moves to Timeweb — see the [[20-Projects/cadence/architecture/proposals/data-layer-to-russia|proposal note]].
> For this spec that is a **rename, not a rewrite**: App Platform provides exactly what it was built on — auto-deploy from the repository, a Dockerfile build, environment variables, a one-button rollback to any version, application logs in the panel.
> Three things changed: the platform's name, the **Sentry boundary** (exceptions only; logs stay inside the Russian perimeter), and the **source of the external ping** (a monitor from Sentry's free tier). The step breakdown and its mapping to Todoist tasks did not change.

## Summary

Take the pipeline to release quality and make failures visible on all three
surfaces — without medical data travelling to somebody else's cloud.

Mitigating the "a single operator" risk starts here. One person will not notice an
incident nobody told them about, and will not remember the rollback order at three
in the morning.

The first draft got NEEDS-REWORK, and two findings turned its mechanics over. The
first: `beforeSend` in the mobile SDK is not a hook for rebuilding the event but a
merge-back of a fixed subset of fields, and exceptions are not among them; a naive
implementation would have looked like it worked while silently letting the error
text out. The second: the diagnostic route "`request_id` → platform logs" led
nowhere, because a 5xx has no meaningful log at all. An allow-list without such a
log is not "strict", it is blind.

## User Story

**As the** system's only operator
**I want** deployment to happen by itself, failures to reach me, and diagnosis to be
possible without medical data in somebody else's cloud
**So that** an incident in a clinical product is found by me rather than by a
doctor, and fixed by a procedure rather than by reproducing it by hand

## Acceptance Criteria

**Sentry: an allow-list plus a curated code**
- [ ] Sentry is wired into the API, the dashboard, and the mobile app
- [ ] **Only exceptions leave, and within them only what is explicitly permitted**: the curated error code, the stack, the build version, the platform, `request_id`, and the route without parameters
- [ ] **Logs are never sent to Sentry.** This is not a setting but a compliance boundary: `logScrubbed` deliberately writes the reason for a 401 refusal, the panic value, and field locations into the log — that is, what the user sent. The logs live inside the Russian perimeter, and Sentry receives only the curated event. `request_id` stitches them together
- [ ] A curated code is mandatory, because the type is not enough: after `fmt.Errorf("…: %w", err)` almost every error in Go has the single type `*fmt.wrapError`, and in JS almost everything is an `Error`. An allow-list keyed on type would group all of production into one issue
- [ ] The raw error message does not go out: a direct `CaptureException(err)` is forbidden in favour of a wrapper that takes the code and the recognized type
- [ ] **On mobile the filter is installed natively** through `initWithPlatformOptions`, separately for Android and iOS. The shared KMP hook is unsuitable for this: it merges back a fixed set of fields, and `exceptions` is not among them — `event.exceptions.clear()` will give a green test and send the exception in full
- [ ] There is a canary test: an event with a field the shared hook does not carry over proves the scrubbing happened at the native level
- [ ] **Every** sending channel is enumerated and closed, not only errors: traces, logs, metrics, replay, screenshots, and view-hierarchy captures are disabled in the MVP. Each SDK has its own separate hooks, and `beforeSend` does not cover them
- [ ] `attachThreads` and `attachStackTrace` are considered explicitly: thread dumps are unfilterable by the shared hook
- [ ] The DSN is normalized to an empty string — `null` never reaches the SDK. On Android `null` crashes the app at startup, while an empty string disables the SDK. The test covers `null` specifically
- [ ] `sentry-go` does not silently pick up `SENTRY_DSN` from the environment: an empty configuration means disabled

**A meaningful log — without it the allow-list is useless**
- [ ] A structured log appears on the 5xx path: `request_id`, the curated code, the unwrapped `%w` chain. Today the logger writes only the method, path, status, and duration, and an ordinary 500 is not logged at all
- [ ] The log has **its own written rule** about what may go into it, and the rule is enforced: field values, request bodies, and user-supplied strings do not go into the log
- [ ] The existing places that write content into the log today are brought in line with that rule: the reason a `/healthz` probe failed and the panic value in the recoverer — both carry what the user sent

**Source maps**
- [ ] The release and the maps are created **where the real build happens**. The dashboard is built on App Platform, not in CI: maps uploaded from a separate job would belong to a different bundle and would not match — the stack would stay unreadable while the criterion looked satisfied

**Ping and the deployment artifact**
- [ ] A deployment artifact for the API appears in the repository: today there is neither a `Dockerfile` nor an App Platform manifest, and no other spec creates them
- [ ] Merging into `main` deploys **the API** to the `dev` environment; migrations run as a separate step before the deployment, with the `cmd/migrate` command
- [ ] The API has its own post-deployment smoke test and its own rollback. For the dashboard this is already done in its own block; the API holds medical data and was left without both
- [ ] An external monitor polls `/healthz` and notifies the operator; verified against a deliberately stopped service that the notification arrives
- [ ] The monitor comes **from Sentry's free tier** (one is included), and no separate service is created. It lives outside our infrastructure, and that is a requirement rather than a convenience: a monitor sitting alongside what it monitors goes silent precisely when it is needed. `/healthz` answers `{"status":"ok"}` and carries no personal data, so a foreign monitor creates no cross-border transfer here

**RUNBOOK**
- [ ] `RUNBOOK.md`: deployment, rollback, where which variables live, where to look at logs, what to do when the ping fires
- [ ] The rollback drill is run **on the API**, not on the dashboard: the dashboard's rollback is automated by the smoke test from its own block, and a drill there would measure the automation instead of the procedure
- [ ] The drill leaves an artifact: what was deployed, what broke it, which command rolled it back, how long it took

**Protecting main**
- [ ] `CODEOWNERS` and a ruleset file with the exact list of required checks live in the repository — the configuration is versioned rather than clicked
- [ ] We will not have `admin` rights: the partner cannot grant them (BST-10, confirmed on 2026-07-29). So the step ships **not a file but a ready-to-run command** — `gh api` with this ruleset — so that all the partner has to do is run one line rather than walk through settings tabs and make a single decision
- [ ] A short note accompanies the command: what it turns on, what will break in their habits afterwards (direct pushes to `main` stop working), and how to roll it back
- [ ] **Acceptance is ours, and it is verifiable rather than declarative.** After the partner applies it: an attempt to push directly to `main` and an attempt to merge with a red check must both be rejected. Both are done with our `write` access, so we can confirm the result ourselves

## Scope / Non-scope

**In scope:** Sentry with an allow-list and a curated code, native filters on mobile,
a structured 5xx log with its own rule, source maps where the build happens, the API
deployment artifact, deploying the API on merge, a smoke test and rollback for the
API, the external ping, `RUNBOOK.md` with a completed drill, and the groundwork for
protecting `main`.

**Out of scope:**
- The production environment, backups, point-in-time recovery — M11.
- A metrics stack — the overview says outright that the MVP has none.
- **Deploying the dashboard** — it belongs to the "Dashboard Skeleton" block, step 4. The first draft claimed it here too; there must be no double ownership.
- The Go, KMP, and web gates — done in BST-09 and in the API and dashboard blocks. What remains here is only making the checks required, and the deployment.
- Notifications about business events — the rollout is in M8.

**Blocking:**

| What is needed | From where | What is impossible without it |
|---|---|---|
| problem+json and `errors[].value` | "API Skeleton", step 2 | the criterion about echoing a submitted value |
| `cmd/migrate` | "API Skeleton", step 1 | migrations as a separate step |
| the Timeweb environment: clusters, S3, SMTP | SKL-01 (manual) | everything below |
| App Platform with the API and GoTrue | SKL-06 (manual) | deployment, ping, the rollback drill |
| the dashboard application | "Dashboard Skeleton", step 1 | Sentry and maps for the dashboard |
| **the partner applying the ruleset** | BST-10 — we will not have `admin` rights | `main` protection taking effect |
| a Sentry Cloud account, free tier | manual, ~10 minutes | real DSNs and the availability monitor |

Step 1 is **not entirely free**: some of its criteria depend on the error shape that
arrives with "API Skeleton". The first draft claimed otherwise.

## What already exists (DONE)

Verified against the repository on 2026-07-28.

- `.github/workflows/ci.yml`: six jobs — `changes`, `scripts`, `tests-not-weakened`, `api`, `kmp-android`, `kmp-ios`. There is no web job and no deployment
- `scripts/gate/`: `all.sh`, `go.sh`, `kmp.sh`, `ios.sh`
- Sentry is absent from the code — in Go, in Kotlin, and in the build configurations. It is mentioned in the text of the specs, so "nowhere" in the first draft was an inaccuracy
- `RUNBOOK.md` is absent. **There is no deployment artifact either**: no `Dockerfile`, no `docker-compose.yml`, no platform manifest of any kind
- Permissions: `admin: false`, `maintain: false`; we have `pull`, `push`, `triage`. `main` protection is entirely absent — rulesets is empty and `branches/main/protection` returns 404. Meanwhile the workflow already fires on pushes to `main`
- **The repository is public.** For a product whose repository will carry the entire migration chain and the RLS policies, that deserves its own decision — recorded in the risks below
- The request logger writes the method, path, status, duration, and `request_id`. An ordinary 500 is not logged. The `/healthz` failure reason and the panic value in the recoverer **do** go into the log — together with what the user sent

## Technical detail

**Versions** (all current as of 2026-07-28): `sentry-go` v0.48.0 · `@sentry/react` 10.68.0 · `sentry-kotlin-multiplatform` **0.27.0**.

**Rebuilding the event is not possible everywhere.** In Go, `BeforeSend` returns an event, and what it returns is what goes out. In JS the same. In KMP — no: the shared hook merges back a fixed set (release, level, message, tags, user, breadcrumbs, and a few more), while `exceptions`, `contexts`, `extras`, and `threads` are not carried over. So the mobile filter is installed natively through `initWithPlatformOptions`, and this is not "the same allow-list on two clients" but two implementations with two sets of tests. The step's estimate must account for that.

**Why a curated code is needed.** An allow-list that lets "the error type" through is useless in Go: the `%w` convention makes nearly everything a `*fmt.wrapError`. An allow-list that lets the message through is a block-list, which we rejected: `%w` drags along a fragment of SQL with its parameters. The way out is a finite set of codes the product declares itself, and a wrapper that forbids sending a raw error.

**The log is the second half of the mechanism.** The allow-list answers "something broke"; the log has to answer "what exactly": a constraint violation, a policy refusal, a pool timeout, and a wrong dose unit all produce the same stack. That is why a structured 5xx log with the unwrapped `%w` chain is in scope — along with its own rule about what may go into it. Otherwise the allow-list turns diagnosis into manual reproduction, and in a clinical product that is hours.

**Logs and medical data — decided 2026-07-29.** The code deliberately writes the reason into the log and hides it from the response, and that stays. The logs are declared a **trusted zone**, because they are now ours and in Russia: the App Platform panel, and Loki inside our own perimeter as volume grows. Access to the platform becomes part of the perimeter, and that is written down.
Hence the Sentry boundary too: a curated exception event goes there, and not a single log line does. The separation is not cosmetic — it is precisely the log that carries what the user sent, and for the dose-logging form that is medical data.

**Source maps — where the build happens.** By its approved spec the dashboard is built on the platform, not in CI. Sentry for Vite matches maps to a bundle by an identifier injected by the bundler, so maps from a different build will not fit. The plugin goes into the dashboard's build, and the token into App Platform's environment variables.

**An empty DSN.** In Go and JS the absence of a DSN simply disables the SDK. On Android `null` throws at startup, and only an empty string disables it — and the KMP wrapper passes `null` straight through. So normalization is mandatory, and the test must check `null`, not `""`.

**The rollback drill — on the API.** The dashboard's rollback is automated by the smoke test from its own block; a drill there would measure the automation. The API holds medical data and so far has neither a smoke test nor a rollback — so both are built here, and the procedure is verified on it.

## Architecture decision

The overview already defines observability: Sentry on three surfaces, `/healthz`
plus an external ping, platform logs, no metrics stack, and `RUNBOOK.md` from the
first milestone. The architecture does not change — only whose platform it is.

The decision made here is the shape of the filter: a strict allow-list plus a
curated error code, rather than scrubbing known fields. It is dictated by the domain:
medical data, a single operator, and 152-FZ, which as of 2026-07-29 is a **satisfied
requirement rather than an open risk**. That tightens things rather than loosening
them: the allow-list stopped being a matter of hygiene and became a compliance
boundary, so the canary test on a field the shared hook does not carry over goes from
desirable to mandatory. A block-list is a bet that nobody will ever forget a filter.

The judge fairly observed that the first draft built a filter and did not build the
channel the meaningful content goes into instead. Hence adding the structured 5xx log
to the scope: without it, the allow-list leaves the operator with no diagnosis.

**A divergence from the overview, named explicitly.** The "Environments" section says
that in dev, migrations are applied automatically on deployment. This spec introduces
a separate step before deployment — the same mechanism that will be needed in prod,
where automatic is not allowed. Compatible, but it is a refinement of the overview,
not compliance with it.

## Component deltas

### api.md
- ADDED: to "Shape" — Sentry with the event rebuilt from permitted fields and a wrapper forbidding the sending of a raw error; a structured 5xx log with the curated code and the unwrapped chain.
- ADDED: to "Invariants" — only what is explicitly permitted goes out; a raw error message leaves the process neither through Sentry nor through the log.

### kmp-app.md
- ADDED: to "Shape" — Sentry with the filter installed natively on each platform; the shared KMP hook is unsuitable for privacy control.

### web-dashboard.md
- ADDED: to "Shape" — Sentry with an allow-list; the release and the source maps are created in the build on App Platform, not in CI.

## Decomposition

Mapping to the original tasks: steps 1, 2, and 5 close SKL-12; steps 3, 4, and 6 close the remainder of SKL-07.

### step-1: Sentry in the API, the curated code, and the structured 5xx log

⏸ **Partially requires** "API Skeleton", step 2 — the criterion about `errors[].value` refers to problem+json, which does not exist yet.

The wrapper forbidding a raw error; the set of codes; rebuilding the event; disabling the other channels; DSN normalization with a test on `null`. The structured 5xx log with its own rule, and bringing `/healthz` and the recoverer in line with it. Tests on a synthetic event containing a dose, an email, and a fragment of SQL.

todoist: "6h8xx8cHV9Wq46Wq"

### step-2: Sentry in the mobile app and the dashboard

⏸ **The dashboard requires** "Dashboard Skeleton", step 1.

Mobile: native filters through `initWithPlatformOptions` separately on Android and iOS, and a canary test on a field the shared hook does not carry over. Dashboard: the allow-list plus the release and the maps in the App Platform build.

todoist: "6h8xx8gH33cxMqWq"

### step-3: The API deployment artifact

⏸ **Requires SKL-06.**

A `Dockerfile` and, if there is more than one service, a `docker-compose.yml` at the repository root — today neither appears in any spec. Variables per environment. The App Platform constraints the manifest has to account for: ports 80 and 443 are taken, only the first service is proxied automatically, and `volumes` are forbidden.

todoist: "6h8xx8hFMr7m84Vq"

### step-4: Merging deploys the API to dev, with a smoke test and rollback

⏸ **Requires step 3 and `cmd/migrate`** from "API Skeleton", step 1.

Migrations as a separate step before the deployment. A `/healthz` smoke test after deployment, reconciling the deployed version. Rollback on red.

todoist: "6h8xx8hX5HrMX6Mq"

### step-5: The availability ping and the RUNBOOK with a drill

⏸ **Requires step 4.**

An external monitor with a check that the notification arrives. `RUNBOOK.md`. A rollback drill on the API with an artifact: what was deployed, what broke it, what rolled it back, how long it took.

todoist: "6h8xx8jg2JqG83jH"

### step-6: Protecting main — preparation, application by the partner, verification by us

There will be no `admin` rights: the partner cannot grant them. The step is designed for that rather than waiting on them.

`CODEOWNERS` and the ruleset go into the repository as versioned configuration, and they come with a **ready-to-run `gh api` command** plus a note of a few lines: what gets turned on, what changes in their habits, how to roll it back. The point is that the partner runs one line and makes not a single decision — the decisions were made here.

Acceptance stays with us and stays verifiable: after it is applied we attempt a direct push to `main` and a merge with a red check, and both attempts must be rejected. `write` access is enough for that check.

> [!decision] 2026-07-29 — **neither a fork nor an organization: we stay in the partner's repository.**
> It turned out that the partner does not "not want" to grant `admin` but cannot: personal-account GitHub repositories have exactly two levels — owner and collaborator with write — and the `admin`, `maintain`, and `triage` roles exist only on organization repositories. GitHub's own recommendation for this case is to transfer to an organization.
> A fork and an organization were both considered. The fork was rejected: it grants `admin` **on a copy**, while the canonical `main` stays unprotected; beyond that it moves a clinic's commercial product into a contractor's personal account, splits the remotes in two, ties the deployment to the fork, and denies PRs from the fork access to CI secrets. An organization is the right shape for a two-person commercial product and would settle the matter permanently, but it requires an action from the partner that we decided not to request now.
> Accepted: we stay as we are, and applying the ruleset is a one-off action by the partner. The cost, named explicitly: **every future repository setting will go through them again** — deployment secrets, environments, Dependabot. Revisit once more than one such request has accumulated.

> [!deviation] 2026-07-29 — the step ships an executable check, not only configuration
> Spec said: `CODEOWNERS` and a ruleset file plus the command and the note. Actually done: the same, plus `scripts/gate/ruleset.sh`, included in the shared gate and in the `Shell gate` job, plus extending the `changes` filter in `ci.yml` from `.github/workflows/` to all of `.github/`.
> Why: the protection configuration is the only part of the repository whose error shows up in no test and is discovered only by things ceasing to merge. Both errors are quiet and point in opposite directions: a required check with no corresponding job will never report and will block every PR forever, while a working job outside the required list gates nothing and the branch merely looks protected. The script reconciles `main.json` against `ci.yml` in both directions and separately requires that `on:` never acquire a path filter — a job skipped by `if:` reports as successful, whereas a workflow skipped by `paths:` does not report at all.
> Extending the `changes` filter follows from that: without it, editing the ruleset would not trigger the job that checks it.
> Verified with ten mutations, all killed: a path filter on the trigger, a typo in a check name, a job dropped from the list, `enforcement: evaluate`, a non-empty `bypass_actors`, history-rewrite protection removed, and after review four more — `"on":` in quotes, a comment at column zero inside the `on:` block, a flow-style `on:`, and a `matrix` on a job. Review found the first three itself: the reader was silently returning an empty block, and the path-filter check turned into a no-op — that is, into exactly the quiet failure it was written to prevent.
>
> **Our half of the step is done; the step is not closed.** What remains is outside our control: merging the branch (BST-12 holds the push until the end of M0 and M1), one CI run reconciling the actual check names — they were derived from `ci.yml` but never once observed, and a required check with a name GitHub has never reported blocks every PR forever — application by the owner, and our two behaviour checks afterwards.

todoist: "6h8xx8rpMQ5QGM8q"

## Open questions

> [!decision] 2026-07-29 — **the logs are a trusted zone, Sentry is not.** The logs live inside the Russian perimeter and medical data is permissible in them; access to App Platform becomes part of the perimeter, and that is written down here. Sentry receives only the curated exception event, not a single log line. Diagnosis does not get poorer for it: `request_id` stitches the Sentry event to the meaningful log on our side. It also follows that the structured 5xx log from step 1 writes the `%w` chain unwrapped in full rather than truncated.

> [!decision] 2026-07-29 — **the notification channel: email plus a webhook.** Email is included in the Sentry tier and costs nothing; the webhook is what actually wakes you, and it also answers the objection "formally the criterion is met, actually it is not". The step 5 acceptance criterion is unchanged and is verified against a deliberately stopped service: the notification must arrive, not merely be sent. Webhook availability on the free tier is verified while doing the step.

> [!decision] 2026-07-29 — **the repository stays public for now.** The partner's decision as the product's owner; they are also the one who grants `admin` rights.
> The argument that did not overturn it but should be revisited is recorded: publication does not break the access model — Postgres enforces it, and knowing a policy's text does not let you get around it — but it makes finding our mistakes in its *implementation* cheaper. An attacker does not need to guess which endpoint forgot `WithCaller`; they read the diff. With one developer and zero external review, that hands out a map of terrain we have not fully surveyed ourselves.
> **Revisit in M2, before the first RLS policy merges.** Right now the repository holds architecture and a skeleton; the first policy makes the exact access map for medical data public, and closing the repository afterwards undoes nothing — forks, clones, and caches remain. The converse is not true: closing buries nothing, and individual pieces can be published deliberately at any time.
> A side benefit: a public repository has unlimited Actions minutes, so the `kmp-ios` job on `macos-latest` does not run into the free limit with its 10× multiplier.
