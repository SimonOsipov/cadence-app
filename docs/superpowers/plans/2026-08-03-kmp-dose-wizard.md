# The Dose Wizard — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port `log-dose/LogDoseScreen.tsx` (480) and `log-dose/components.tsx` (863) — the five-step wizard that is the product's critical path — against the repositories steps 3–4 built, and write the two facts §03 says one check-in writes.

**Architecture:** The wizard's state is a value in `shared`, not a pile of `remember`s in the screen: five steps that can be moved between, validated and reviewed is logic, and logic goes where the gate runs. `composeApp` renders it and reports taps. The write goes through a widened `DoseLogRepository`, and §03's «one action, two facts» — a dose event *and* a journal entry with `source = DOSE` — is one repository call, not two the screen makes in sequence.

**Tech Stack:** Compose Multiplatform, the ported design system, `shared`'s domain and repositories, `app.cadence.format`.

## Global Constraints

- **The prohibition, verbatim from the subtask:** «Доза хранится `{value, unit}`, а не строкой "1,0 мг". Форматтер один на поверхность.» The prototype's `COMPOUNDS` carry `default: '0.25'` as a **string** and its stepper does arithmetic on parsed strings. `Dose(value: Double, unit: DoseUnit)` already exists; nothing in this step may hold a rendered dose.
- **§03's «one action, two facts».** The dosing reconciliation says: «The wizard's mood/sides check-in also writes today's journal_entry with source dose — one action, two facts, matching the prototype's journal seed data.» `JournalTag` is already a `typealias` for `SideEffect` so the two vocabularies cannot drift. A screen making two calls can half-fail; the repository makes one.
- **The ten zones are the prototype's**, already in `InjectionSite` — `r-delt`, `l-delt`, `r-abdomen`, `l-abdomen`, `r-thigh`, `l-thigh`, `l-lback`, `r-lback`, `l-glute`, `r-glute`. Two of them were once invented here and matched no zone the body map draws; assert the set, never the count.
- **The rotation suggestion is derived**, not seeded: «предложим следующую зону по ротации» is a function of the patient's recent `DoseEvent.site` values, and it belongs in `shared`.
- **No new colours, fonts or primitives in a screen.** Anything missing goes to `app.cadence.design` with a test.
- **The gate is `./scripts/gate/kmp.sh`**, run on its own line, green at the end of every task. `shared` tests run under it; Compose tests run on the iOS simulator.
- **Scripted edits assert their pattern and the trace is read.** Five silent no-ops in the previous session; prefer `Edit` for anything structural, and never restore-in-a-loop with an escaped `$(basename)`.

---

## File Structure

**Created in `shared`:**
- `domain/InjectionRotation.kt` — `suggestNextSite(recent: List<DoseEvent>): InjectionSite`
- `domain/DoseDraft.kt` — the wizard's state: selected item, dose, site, mood, side effects, note, photo; plus `canAdvance(step)` and `steps`
- `repository/DoseLogRepository.kt` — widened to take a `DoseDraft` and return both ids
- tests for each

**Created in `composeApp`:**
- `screens/dose/DoseWizard.kt` — the chrome: progress, back/next, the five bodies
- `screens/dose/CompoundStep.kt`, `DoseStep.kt`, `SiteStep.kt`, `ContextStep.kt`, `ReviewStep.kt`
- `design/CadenceBodyMap.kt` — the ten-zone diagram, in the design system because it is a drawn primitive with no product logic
- `design/CadenceStepper.kt`, `design/CadenceSlider.kt` — the dose stepper and the mood slider, same argument
- one test file per screen file, plus design tests

**Modified:**
- `mock/CadenceMocks.kt` — the widened write, and a journal entry beside the dose event
- `shell/CadenceShell.kt` — the `LogDose` placeholder gives way to the wizard

**Deliberately not built:** the photo upload. §03 routes photos direct to object storage under a path convention (path C, the documented client→data exception); the slot renders and reports a tap, and the upload lands with the storage work. Recorded as a divergence.

---

### Task 1: The rotation suggestion

«Предложим следующую зону по ротации» — the one piece of clinical logic in the wizard.

**Files:** `shared/domain/InjectionRotation.kt`, its test.

- [ ] **Step 1: Write the failing test.** With no history, the suggestion is the first zone. With a history, it is the least-recently-used zone — and never the one used last. Zones used equally often break ties by which was used longest ago. A history containing a zone not in the current set does not crash.
- [ ] **Step 2–4:** run red, implement, run green.
- [ ] **Step 5: Mutate.** A suggestion that ignores history; one that returns the *most* recent; one that ties by declaration order rather than by recency.
- [ ] **Step 6:** gate, commit.

---

### Task 2: The wizard's state, and what may advance

Five steps with a rule each. This is why the state is in `shared`.

**Files:** `shared/domain/DoseDraft.kt`, its test.

- [ ] **Step 1: Write the failing test.** A draft with no compound cannot leave step 1; with no dose cannot leave step 2; with no site cannot leave step 3; step 4 is entirely optional and always advances; step 5 is the review and has no «next». Changing the compound resets the dose to that compound's current phase, not to the previous compound's number. The dose is a `Dose`, and nothing in the draft is a rendered string.
- [ ] **Step 2–4:** run red, implement, run green.
- [ ] **Step 5: Mutate.** Each guard dropped in turn; a compound change that keeps the old dose.
- [ ] **Step 6:** gate, commit.

> [!deviation] 2026-08-03
> Spec said: the draft holds selected item, dose, site, mood, side effects, note,
> photo, plus `canAdvance(step)` and `steps`. Actually done: it also holds the
> item's `kind`, and gained `canSubmit()`. Why: `DoseEvent.site` is nullable
> because a supplement has no zone, so `canAdvance(SITE)` has to know what kind
> of item it is or the wizard cannot log one; and a review step that «has no
> next» makes `canAdvance(REVIEW)` false, so a screen wiring its last button to
> it would render a wizard nobody can finish. `canSubmit()` is derived from the
> other four steps rather than restating them.
>
> Two further guards were added that the prototype does not have: a dose must be
> greater than zero, and re-tapping the already-chosen compound is a no-op. Both
> are prototype bugs the block is instructed not to port. Recorded in
> `docs/prototype-divergences.md`.

---

### Task 3: One action, two facts

**Files:** `shared/repository/DoseLogRepository.kt`, `mock/CadenceMocks.kt`, `MockRepositoryTest`.

- [ ] **Step 1: Write the failing test.** Submitting a draft writes a `DoseEvent` *and* a `JournalEntry` for the same date with `source = DOSE`, carrying the draft's mood and its side effects as tags. The vial is decremented. Submitting twice for one occurrence does not write two events. The journal entry upserts — §03 has `UNIQUE(patient, date)`.
- [ ] **Step 2–4:** run red, implement, run green.
- [ ] **Step 5: Mutate.** The journal write dropped; the tags dropped; the vial not carried; two submissions accepted.
- [ ] **Step 6:** gate, commit.

> [!deviation] 2026-08-03
> Spec said: three files — `DoseLogRepository.kt`, `CadenceMocks.kt`,
> `MockRepositoryTest`; the repository «returns both ids». Actually done: two
> more files, and the second id is a date.
>
> `JournalRepository.kt` is new because the second fact has to be observable —
> a write nothing can read back is a write nobody can show to work — and the
> journal entry has no id of its own, since §03 keys it `UNIQUE(patient,
> date)`. `TodaySummary.suggestedSite` is new because `submit` refuses an
> injection without a zone, so the interim one-tap write needs one; it is task
> 5's input arriving early, and it is also the only thing that makes
> `DoseEvent.site` observable.
>
> `AlreadyLogged` carries no id and fires only when *every* slot the item has
> today is logged: §03 gives BPC-157 `times = [08:00, 20:00]`, and resolving
> «the first occurrence» rather than «the first still open» meant a twice-daily
> item could only ever record its morning dose.

---

### Task 4: The drawn primitives

`BodyDiagram` (208 lines), `DoseStepper` (78), `SyringeBar` (75), `MoodSlider` (65). They are drawn, they carry no product logic, and three of them will be wanted again — so they go in `design/`, with the handles a test needs, because a `Canvas` has no text.

**Files:** `design/CadenceBodyMap.kt`, `design/CadenceStepper.kt`, `design/CadenceSlider.kt`, design tests.

- [ ] **Step 1: Write the failing test.** The body map draws ten zones, each named for a screen reader and each reporting its own id when tapped; the selected one says it is selected. The stepper reports increments and decrements and never goes below zero. The slider reports the value it lands on. The syringe fill is a fraction of its max, clamped.
- [ ] **Step 2–4:** run red, implement, run green.
- [ ] **Step 5: Mutate.** A zone reporting its neighbour's id; a stepper that ignores its step size; a fill that is not clamped.
- [ ] **Step 6:** gate, commit.

> [!deviation] 2026-08-03
> Spec said: three files in `design/`. Actually done: three files, plus
> `CadenceBodyMap` takes a `lastUsed` list the plan does not mention.
>
> The prototype draws a muted dot for `state.lastUsed`, and without it the
> rotation suggestion reads as arbitrary — nothing on screen says why *this*
> zone is next. Task 1 already computes the history, so the parameter costs one
> line and closes the affordance rather than deferring it.
>
> `stepFor(unit)` became the default of a `step` parameter rather than a rule
> inside the control: a compound whose titration does not move in 0,05 мг is a
> fact about the protocol, and the caller should not have to change the design
> system.

---

### Task 5: The five steps

**Files:** the five `screens/dose/*Step.kt`, `DoseWizard.kt`, tests.

- [ ] **Step 1: Write the failing test.** Each step renders its eyebrow («Шаг 1 · Препарат») and its emphasised title. «Дальше» is disabled until the step's rule is met and enabled once it is. The last step's button reads «Сохранить дозу». Back returns to the previous step and keeps what was entered. The review names the compound, the dose as two runs, and the zone in Russian.
- [ ] **Step 2–4:** run red, implement, run green.
- [ ] **Step 5: Mutate.** The next button ignoring `canAdvance`; back losing the draft; the review reading the compound's default rather than the draft's dose.
- [ ] **Step 6:** gate, commit.

---

### Task 6: The shell stops drawing the placeholder

**Files:** `shell/CadenceShell.kt`, `shell/CadenceShellDataTest.kt`.

- [ ] **Step 1: Write the failing test.** The hero's «Записать →» opens the wizard; completing it returns to Today with the dose logged, the strip saying «записано», and the vial down by one.
- [ ] **Step 2–4:** run red, wire, run green.
- [ ] **Step 5: Check the acceptance criterion by diff.** `git diff --name-only` must not contain `ActionChooserSheet.kt`; `TodayScreen.kt` should appear only if the hero genuinely needed a new parameter.
- [ ] **Step 6:** gate, both suites, commit.

---

### Task 7: Divergences

- [ ] Record: the photo upload deferred to the storage work; the prototype's dose-as-string replaced by `{value, unit}` (the subtask's prohibition, so it is a *correction* rather than a divergence, but the registry is where the next reader looks); anything the five steps drop.
- [ ] Gate, commit.

---

## Self-Review

**Spec coverage.** The subtask names two files and one prohibition. `LogDoseScreen.tsx` → Tasks 2 and 5; `components.tsx` → Task 4. «Доза `{value, unit}`, форматтер один на поверхность» → Task 2's assertion that nothing in the draft is a rendered string, and Task 5's review rendering through `formatDose`. §03's «one action, two facts» is Task 3 and is the reason the write is one repository call.

**Known gaps, deliberately.** The photo upload. Nothing here proves visual fidelity — step 11's side-by-side run does.

**Type consistency.** `DoseDraft` is produced in Task 2 and consumed in Tasks 3, 5 and 6. `suggestNextSite` is produced in Task 1 and called by Task 2's compound-change reset and Task 5's site step. `InjectionSite` is already in `shared/domain/Dosing.kt` with the ten prototype codes.
