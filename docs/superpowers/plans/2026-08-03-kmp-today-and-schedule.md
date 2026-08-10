# Today and Schedule — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the first two real sections — `today/TodayScreen.tsx` (1037), `today/BiomarkerSheet.tsx` (130) and `schedule/ScheduleScreen.tsx` (776) — against the repositories step 3 built, and make the two screens render the same generated occurrences instead of the prototype's two disagreeing copies.

**Architecture:** The screens are Compose functions in `composeApp` that take plain values and lambdas and know nothing about repositories — the shell reads and passes down, exactly as it does for `ActionChooserSheet`. Anything computed goes in `shared`, under the gate: the week's protocol rows and the month grid are projections over `occurrencesFor`, not screen logic. What the screens own is layout.

**Tech Stack:** Compose Multiplatform 1.11.1 with the ported design system, `shared`'s domain and repositories, `app.cadence.format` for every rendered number.

## Global Constraints

- **The prototype is the picture, §03 is the model.** Layout, spacing, type and colour come from `mobile/src/features/{today,schedule}/`; what the numbers *are* comes from `shared`.
- **Two corrections this step must make**, both named in the subtask:
  1. **The protocol strip is not hardcoded.** `TodayScreen.tsx:100-133` is a literal array of three rows. It becomes a projection of the same `occurrencesFor` the Schedule screen renders — §03's seventh correction, and the reason both are in one step.
  2. **«Today» is the clock**, not 31 May 2026. The header's «Воскресенье, утро · 4-я неделя», the cycle position and every «past/today/future» decision derive from `CadenceClock` and `protocol.startDate`.
- **No new colours, fonts or primitives in a screen.** Anything missing goes into `app.cadence.design` with a test, never into `today/`.
- **Every number is formatted by `app.cadence.format`.** «0,25 мг», «1 240 ккал», «4-я неделя», weekday and month names. If a formatter is missing, it is written there with a table test, not inline.
- **RU copy is the prototype's, verbatim.** Copy that depends on data the mock cannot yet supply is a divergence and gets recorded.
- **Compose UI tests run on the iOS simulator; `shared` tests run under the gate.** Put logic in `shared` when there is a choice — it is the half the gate actually checks.
- **The gate is `./scripts/gate/kmp.sh`**, green at the end of every task, and run by itself rather than chained behind a `grep` that can mask its exit status.
- **Scripted edits assert their pattern matched.** Three silent no-ops so far this session; `Edit` is preferred over find-and-replace for anything structural.

---

## File Structure

**Created in `shared`:**
- `domain/WeekProtocol.kt` — `ProtocolRow` and `weekProtocolRows(plan, events, today)`: one row per protocol item for the current week, which is what the Today strip draws
- `repository/MeasurementsRepository.kt` — §11's «latest measurement + 7-pt series per metric»
- `mock/MockMeasurementsRepository` inside `CadenceMocks`
- tests for both

**Created in `composeApp`:**
- `design/CadenceListRow.kt` — the icon-tile + title/sub + trailing-value row the protocol strip and the schedule list both draw (extracted on its second call site, not its first)
- `screens/today/TodayScreen.kt`, `screens/today/TodayHero.kt`, `screens/today/BiomarkerGlance.kt`, `screens/today/BiomarkerSheet.kt`, `screens/today/MealHero.kt`
- `screens/schedule/ScheduleScreen.kt`, `screens/schedule/CycleCalendar.kt`
- one test file per screen file

**Modified:**
- `shared/repository/TodayRepository.kt` — `TodaySummary` gains `weekProtocol` and `weightSeries`
- `composeApp/shell/CadenceShell.kt` — the two placeholders give way to the screens
- `mock/MockSeed.kt` — the prototype's third protocol item (glycine + magnesium, a `SUPPLEMENT`) and a seven-point weight series

**Read, never edited:** everything under `mobile/`.

**Deliberately not built:** `QuickFeelSheet`. The wellbeing nudge on Today is part of this screen and gets ported; the sheet it opens belongs to the journal section (step 7), so the nudge takes an `onOpenQuickFeel` lambda the shell leaves unwired, and there is a test that the nudge reports the tap.

---

### Task 1: The week's protocol, computed

The correction the whole step exists for. `TodayScreen.tsx:100-133` hardcodes three rows; the Schedule screen builds its own from `eventsForDate`. One projection replaces both.

**Files:**
- Create: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/domain/WeekProtocol.kt`
- Test: `kmp/shared/src/commonTest/kotlin/app/cadence/shared/domain/WeekProtocolTest.kt`
- Modify: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/mock/MockSeed.kt`

**Interfaces:**
- Produces: `ProtocolRow(itemId, kind, compoundId, dose, cadenceLabel, timesLabel, todayStatus)` and `weekProtocolRows(plan, events, today): List<ProtocolRow>`.

- [ ] **Step 1: Write the failing test.** Assert that a row exists per protocol item; that the semaglutide row carries the week's *titrated* dose rather than the protocol's first; that the row's `todayStatus` is `DONE` once the day's occurrence is logged and `PENDING` before; that an item not scheduled today still gets a row (it is the week's protocol, not the day's); and that a cancelled protocol produces none.

- [ ] **Step 2: Run it and watch it fail.**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests "*WeekProtocolTest"`

- [ ] **Step 3: Write the projection.** `weekProtocolRows` walks `plan.items`, resolves the phase covering `cycleWeek(plan.protocol, today)`, and derives `todayStatus` from `occurrencesFor(plan, events, today, today)` — so the strip and the calendar cannot disagree by construction.

- [ ] **Step 4: Seed the third item.** The prototype's third row is «Глицин + магний · на ночь». Add it to `MockSeed.plan` as a `SUPPLEMENT` with `times = [21:30]`, and a `Compound` for it. Note in the commit that it exists to make the strip three rows deep as the prototype draws it.

- [ ] **Step 5: Mutate.** A projection ignoring the phase; a `todayStatus` that ignores events; a row list built from the day's occurrences rather than the week's items (which would drop the supplement on days it is not due).

- [ ] **Step 6: Gate, then commit.**

---

### Task 2: Measurements, and the seven points the glance draws

§11: the Biomarker Sheet reads «latest measurement + 7-pt series per metric», and Today's headline biomarker draws the same series as a sparkline.

**Files:**
- Create: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/repository/MeasurementsRepository.kt`
- Modify: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/mock/CadenceMocks.kt`, `MockSeed.kt`
- Modify: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/repository/TodayRepository.kt`
- Test: `kmp/shared/src/commonTest/kotlin/app/cadence/shared/mock/MockRepositoryTest.kt`

**Interfaces:**
- Produces: `MetricSeries(metric, points: List<Measurement>, latest: Measurement?)`; `MeasurementsRepository.series(metric, points: Int = 7)`; `TodaySummary.weightSeries: List<Double>`.

- [ ] **Step 1: Write the failing test.** The series is ordered by `measuredAt` ascending and truncated to the *last* seven, not the first; `latest` is the last point; a metric with no rows returns an empty series and a null latest rather than throwing.

- [ ] **Step 2–4: Run red, implement, run green.**

- [ ] **Step 5: Seed a real series.** Seven weight readings descending from 101,2 to 98,4 over seven weeks — the prototype's weight journey, which its sparkline draws.

- [ ] **Step 6: Mutate.** `take(7)` instead of `takeLast(7)`; an unsorted series; `latest` as `first()`.

- [ ] **Step 7: Gate, then commit.**

---

### Task 3: The Today screen, top half

Header, hero dose card, biomarker glance, wellbeing nudge, reorder reminder. Read `TodayScreen.tsx:135-538` as you go; the hero's numbers are `TodaySummary.nextDose`, its copy is the prototype's.

**Files:**
- Create: `screens/today/TodayHero.kt`, `screens/today/BiomarkerGlance.kt`, `screens/today/TodayScreen.kt`
- Test: `screens/today/TodayScreenTest.kt`

- [ ] **Step 1: Write the failing test.** The hero names the compound and the week's dose as two runs («Семаглутид» over «0,25 мг» in the drawn italic); its button reads «Записать →» before the dose is logged and «Открыть детали» after; the wellbeing nudge appears only once the dose is logged, and reports its tap; the reorder card appears only when `TodaySummary.reorder` is non-null and names the weeks left.

- [ ] **Step 2–4: Run red, implement, run green.** Every string that carries a number goes through `app.cadence.format`.

- [ ] **Step 5: Mutate.** The button label ignoring `doseLogged`; the reorder card rendering unconditionally; the dose taken from the compound's default rather than the phase.

- [ ] **Step 6: Gate, then commit.**

---

### Task 4: The Today screen, bottom half

The protocol strip (from Task 1), the meal hero, the meals card. Read `TodayScreen.tsx:539-667` and the two components at `668-1037`.

**Files:**
- Create: `screens/today/MealHero.kt`, `design/CadenceListRow.kt`
- Modify: `screens/today/TodayScreen.kt`
- Test: `screens/today/TodayScreenTest.kt`, `design/DesignSystemTest.kt`

- [ ] **Step 1: Write the failing test.** The strip draws one row per protocol item, in order; the semaglutide row shows this week's dose and «записано»/«ждёт» following the logged state; «Весь график» reports a tap. The meals card shows the day's count and energy against the target.

- [ ] **Step 2–4: Run red, implement, run green.** `CadenceListRow` is extracted **only** because the schedule list draws the same shape in Task 6 — if it turns out not to, it stays local to Today.

- [ ] **Step 5: Mutate.** A strip built from a literal list; a row that drops its dose; the meals card ignoring the target.

- [ ] **Step 6: Gate, then commit.**

---

### Task 5: The biomarker sheet

`BiomarkerSheet.tsx`, 130 lines, over the existing `CadenceSheet`.

**Files:**
- Create: `screens/today/BiomarkerSheet.kt`
- Test: `screens/today/BiomarkerSheetTest.kt`

- [ ] **Step 1: Write the failing test.** The sheet names the metric, its latest value and unit as separate runs, and draws a spark over the seven points; a metric with no readings says so rather than drawing an empty chart.

- [ ] **Step 2–4: Run red, implement, run green.**

- [ ] **Step 5: Mutate, then gate and commit.**

---

### Task 6: The schedule screen

`ScheduleScreen.tsx`, 776 lines: the month grid with day dots, the titration callout, and the day sheet. Read it in three passes — grid, callout, sheet.

**Files:**
- Create: `screens/schedule/CycleCalendar.kt`, `screens/schedule/ScheduleScreen.kt`
- Test: `screens/schedule/ScheduleScreenTest.kt`

- [ ] **Step 1: Write the failing test.** The grid renders a month of cells with the first aligned to its weekday, Monday-first; days outside the protocol carry no dot; a day with an injection is marked; tapping a day opens its occurrences; the titration callout names the next step and its date, computed from the phases rather than from a constant.

- [ ] **Step 2–4: Run red, implement, run green.** Weekday and month names come from a formatter in `app.cadence.format` with a table test — the prototype's `MONTHS_GEN`/`WD_SHORT`.

- [ ] **Step 5: Mutate.** A grid ignoring the first weekday; dots on days outside the cycle; a titration callout reading a literal date.

- [ ] **Step 6: Gate, then commit.**

---

### Task 7: The shell stops drawing placeholders

**Files:**
- Modify: `shell/CadenceShell.kt`
- Test: `shell/CadenceShellDataTest.kt`

- [ ] **Step 1: Write the failing test.** `App()` opens on the real Today screen; the tab bar reaches Schedule and both show the same next dose; «Весь график» navigates to Schedule; the hero's button opens the log-dose route.

- [ ] **Step 2–4: Run red, wire, run green.** The two `PlaceholderScreen` calls for `Today` and `Schedule` are replaced; the other seventeen routes keep theirs.

- [ ] **Step 5: Check the acceptance criterion by diff.** `git diff --name-only` for this commit must not contain `ActionChooserSheet.kt` or any file under `design/` — the screens should have needed nothing new from the design system that Task 4 did not already add deliberately.

- [ ] **Step 6: Gate, run both suites, commit.**

---

### Task 8: Divergences and the registry

- [ ] **Step 1: Record every divergence this step made** in `docs/prototype-divergences.md`, at minimum: the protocol strip now computed (with what the prototype hardcoded); «today» from the clock; anything in the two screens whose copy depends on data the mock cannot supply.

- [ ] **Step 2: Update `docs/superpowers/plans/` if the shape changed**, and the component note `kmp-app.md` only at block finalization, not here.

- [ ] **Step 3: Gate, commit.**

---

## Self-Review

**Spec coverage.** The subtask names three files and two prohibitions. `TodayScreen.tsx` → Tasks 3 and 4; `BiomarkerSheet.tsx` → Task 5; `ScheduleScreen.tsx` → Task 6. «Не переносится захардкоженная полоса протокола» → Task 1, which is why it is first. «Сегодня от настоящих часов» → the clock seam from step 3, exercised by every task and asserted in Task 6's grid. §11's reads for both screens are covered by `TodayRepository` (step 3) plus `MeasurementsRepository` (Task 2).

**Known gaps, deliberately.** `QuickFeelSheet` is the journal section's. The dose wizard behind the hero's button is step 5's — the hero navigates to the route the shell already has. Nothing here proves visual fidelity; that is step 11's side-by-side run.

**Type consistency.** `ProtocolRow` is produced in Task 1 and consumed in Task 4 under that name. `MetricSeries` is produced in Task 2 and consumed in Tasks 3 and 5. `TodaySummary.weekProtocol` and `.weightSeries` are added in Tasks 1–2 and read in Tasks 3–4.
