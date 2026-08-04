# The Vial Cabinet — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port `inventory/VialsScreen.tsx` (822), `VialDetailSheet.tsx` (420) and `AddVialScreen.tsx` (475) — «Аптечка» — against one vial set whose remaining count, statuses and reorder hints are all computed, and close the vial picker the dose wizard was left owing.

**Architecture:** The clinical maths already exists in `shared`: `remainingDoses`, `vialStatus` and `reorderHint` in `domain/InventoryMath.kt`, written for the Today hero in step 4. The cabinet is a reader of those, not a second implementation. What is new is one repository, one summary function that aggregates across compounds, three screens, and the seed growing from one vial to the prototype's five — which is what makes any of it visible, and what unblocks the picker.

**Tech Stack:** Compose Multiplatform, the ported design system, `shared`'s domain and repositories, `app.cadence.format`.

## Global Constraints

- **The prohibition, verbatim from the subtask:** «Один набор флаконов, а не два рассогласованных, как в прототипе. Остаток выводится из залогированных доз, а не хранится; статусы и подсказки докупки вычисляются.» The prototype ships `VIAL_INVENTORY` in `inventory/data.ts` **and** a different `VIALS` in `log-dose/data.ts`; §03 calls this its third correction. `MockSeed.vials` is the one set, and `Vial` has no `remaining` and no `status` field to store — see its KDoc.
- **Remaining is a subtraction.** `remaining = total_doses − count(dose_events.vial_id)`. A seeded vial that says «8 из 12» has twelve doses and four logged events behind it, not an `8`.
- **No new colours, fonts or primitives in a screen.** Anything missing goes to `app.cadence.design` with a test.
- **Numbers are data, formatting is presentation.** A dose is `{value, unit}`; «0,25 мг» is `formatDose`'s output. Dates are `LocalDate`; «14 сен» is a formatter's.
- **All user copy is Russian**; code, comments and commit messages English.
- **The gate is `./scripts/gate/kmp.sh`**, green at the end of every task. `shared` tests run under it; Compose tests run on the iOS simulator (`:composeApp:iosSimulatorArm64Test`).
- **Mutants are re-run after every fix.** A fix moves the coincidence rather than removing it — the dose wizard paid for that twice.
- **The mutation harness runs both suites.** One that ran only the simulator reported a `shared` guard as a gap.

---

## File Structure

**Created in `shared`:**
- `domain/InventorySummary.kt` — `inventorySummary(vials, events, plan, compounds, today)`: the counts by status and the reorder hints across every compound, which `reorderHint` currently answers for one item at a time
- `repository/InventoryRepository.kt` — `vials()`, `vial(id)`, `addVial(draft)`
- tests for each

**Modified in `shared`:**
- `mock/MockSeed.kt` — five vials and the dose events behind their remaining counts
- `mock/CadenceMocks.kt` — the inventory repository, and `vialFor` gaining back its disposal filter now that the seed can reach it

**Created in `composeApp`:**
- `screens/inventory/VialsScreen.kt` — header, summary card, filter chips, the active list, the sealed accordion, the empty state
- `screens/inventory/VialCard.kt` — the card and the compact row, both drawn from one vial
- `screens/inventory/VialDetailSheet.kt` — facts, recent doses, usage, the action rows
- `screens/inventory/AddVialScreen.kt` — the form
- `design/CadenceGauge.kt` — the remaining-doses meter the card draws
- one test file per screen file, plus a design test

**Modified in `composeApp`:**
- `shell/CadenceShell.kt` — `Vials` and `AddVial` stop drawing placeholders
- `screens/dose/DoseSteps.kt` — the dose step gains the vial picker
- `screens/dose/DoseOption.kt` — `syringeUnits` becomes computable, because a vial says what the concentration is

**Deliberately not built:** the label photo (same storage exception as the dose wizard's), and «Изменить лот, дату или дозу», which the prototype's own sheet wires to `onClose`.

---

### Task 1: One vial set, and a remaining count nothing stores

The seed has one vial. Everything the cabinet shows — sealed spares, an expiring vial, a low one — is invisible until it has the prototype's five, and the picker the dose wizard owes cannot exist while a compound has at most one.

**Files:** `shared/mock/MockSeed.kt`, `shared/mock/CadenceMocks.kt`, `MockRepositoryTest`.

- [ ] **Step 1: Write the failing test.** The seed holds five vials across four compounds. Semaglutide has an open one and a sealed spare; BPC-157 has an open one expiring within fourteen days and a sealed spare; TB-500 has one below a quarter remaining; Tesamorelin has one opened yesterday with no spare. Every remaining count is `totalDoses` minus the seeded events for that vial — no vial carries a number that was typed. `vialStatus` answers SEALED, EXPIRING, LOW and ACTIVE across the set, one each at least.
- [ ] **Step 2–4:** run red, seed, run green.
- [ ] **Step 5: Fix what the growth breaks.** `MockTodayRepository` reads `MockSeed.vials.first()` for `vialDosesLeft` — make it read the open vial of the item's compound, which is `vialFor`'s job. Restore `vialFor`'s `disposedAt` filter, now that a seed with five vials can carry a disposed one, and test it.
- [ ] **Step 6: Mutate.** A remaining count read from a stored field; `vialFor` returning the first vial in the list; the disposal filter dropped; a status boundary moved by a day.
- [ ] **Step 7:** gate, both suites, commit.

---

### Task 2: The cabinet's own arithmetic

`reorderHint` answers for one protocol item. The cabinet asks across every compound at once, and splits the set four ways.

**Files:** `shared/domain/InventorySummary.kt`, its test.

- [ ] **Step 1: Write the failing test.** `inventorySummary` returns the vials grouped as the prototype groups them: active (opened, including an opened one that is expiring), sealed (never opened, including an unopened one that is expiring), expiring (either), low (opened and under a quarter). A vial appears in more than one group where the prototype puts it in more than one, and the groups are named rather than counted. Reorder hints fire for a compound with **no** sealed spare and four weeks or less of total stock, and for no other; the weeks are `total doses ÷ doses per week`, floored, and the rate is `ProtocolItem.dosesPerWeek()` rather than a table.
- [ ] **Step 2–4:** run red, implement, run green.
- [ ] **Step 5: Mutate.** The sealed-spare condition dropped; the four-week threshold moved; a compound with no protocol item counted anyway; the groups made disjoint.
- [ ] **Step 6:** gate, commit.

---

### Task 3: The meter

`VialCard` draws a remaining-doses gauge; the detail sheet draws a weekly-usage bar row. Both are drawn, both are wanted twice.

**Files:** `design/CadenceGauge.kt`, its test.

- [ ] **Step 1: Write the failing test.** The gauge reports the fraction it fills, measured rather than described — a `Canvas` asserts nothing, and a meter painted full regardless survived that exact mistake in the dose wizard. It is clamped at both ends and draws nothing rather than dividing when the total is zero. It names its own numbers for a screen reader («8 из 12 доз»). The usage row draws one bar per week with the tallest at full height and the rest in proportion.
- [ ] **Step 2–4:** run red, implement, run green.
- [ ] **Step 5: Mutate.** A fill that ignores its fraction; a fill that is not clamped; a usage row whose bars are all the same height; a zero total dividing.
- [ ] **Step 6:** gate, commit.

---

### Task 4: «Ваша аптечка»

**Files:** `screens/inventory/VialsScreen.kt`, `screens/inventory/VialCard.kt`, their tests.

- [ ] **Step 1: Write the failing test.** The header counts the vials in Russian («5 флаконов в холодильнике» — the plural rule, not the prototype's approximation) and titles «Ваша аптечка» with «аптечка» emphasised. The summary card shows the four counts. The chips are «Все», «Активные», «Истекают», «Запас» and one per compound, each with its count, and tapping one filters the list to exactly the vials of that group — asserted by which vials are on screen, not by the chip's own state. «Истекают» carries a danger tone only when the count is above zero. The sealed section is collapsed and opens. An empty cabinet draws the empty state with its «Добавить флакон» action rather than an empty list. Each card names its compound, its dose through the formatter, its remaining count and its status pill; a card reports its own vial's id when tapped, and each is asserted separately.
- [ ] **Step 2–4:** run red, implement, run green.
- [ ] **Step 5: Mutate.** A chip that filters to the wrong group; a card reporting its neighbour's id; the status pill fixed to one label; the sealed section open from the start; the count read from a stored field.
- [ ] **Step 6:** gate, both suites, commit.

---

### Task 5: The detail sheet

**Files:** `screens/inventory/VialDetailSheet.kt`, its test.

- [ ] **Step 1: Write the failing test.** The sheet names the compound and the dose as two runs, shows «Открыт», «Истекает», «Лот» and «Хранится» as four fact cards with the dates through the formatter, lists the recent doses for **this vial** with their zone in Russian and a relative day label («сегодня», «вчера», «3 дн назад», «2 нед назад»), and draws the usage row. Its actions report: «Записать дозу» carries this vial's id; «Перенести в запас» and «Прикрепить фото» are present and disabled with a reason, because neither is built. A vial with no logged doses says so rather than drawing an empty list.
- [ ] **Step 2–4:** run red, implement, run green.
- [ ] **Step 5: Mutate.** The recent list unfiltered by vial; the day label off by one; «Записать дозу» reporting a fixed id; the dose rendered as one run.
- [ ] **Step 6:** gate, both suites, commit.

---

### Task 6: «Добавить флакон»

**Files:** `screens/inventory/AddVialScreen.kt`, `shared/repository/InventoryRepository.kt` (the write), `MockRepositoryTest`.

- [ ] **Step 1: Write the failing test.** The form takes a compound, a dose, a total-doses count, a lot, an expiry date and a location. «Сохранить» is dead until the compound, the dose and the expiry are given — the three a vial cannot exist without — and live once. The dose is entered as a number and a unit and stored as `Dose`, never as «0,25 мг». Saving writes through the repository and the new vial comes back out of `vials()` with `remaining == totalDoses`, because nothing has been drawn from it. An expiry already past is refused with a reason rather than saved.
- [ ] **Step 2–4:** run red, implement, run green.
- [ ] **Step 5: Mutate.** The save guard dropped; the expiry guard dropped; the dose stored as a string; a saved vial arriving with a remaining count of its own.
- [ ] **Step 6:** gate, both suites, commit.

---

### Task 7: The shell, and the debt the dose wizard left

**Files:** `shell/CadenceShell.kt`, `screens/dose/DoseSteps.kt`, `screens/dose/DoseOption.kt`, `CadenceShellDataTest`, `DoseWizardTest`.

- [ ] **Step 1: Write the failing test.** The «Аптечка» tab draws the cabinet rather than a placeholder; a card opens the detail sheet; the `+` opens «Добавить флакон» and saving returns to a cabinet with one more vial. The dose wizard's step 2 gains the vial picker: it offers the open vials of the chosen compound, defaults to the one with the most doses left, and the chosen vial reaches `DoseEvent.vialId` — asserted by which vial's remaining count moves. With a vial chosen, the syringe barrel draws, because a vial says what the concentration is.
- [ ] **Step 2–4:** run red, wire, run green.
- [ ] **Step 5: Check the acceptance criterion by diff.** `git diff --name-only` must not contain `ActionChooserSheet.kt`, `TodayScreen.kt` or `CadenceBodyMap.kt`.
- [ ] **Step 6: Mutate.** The picker offering sealed vials; the default picking the emptiest; the chosen vial not reaching the write; the barrel drawn without a concentration.
- [ ] **Step 7:** gate, both suites, commit.

---

### Task 8: Divergences

- [ ] Record: the label photo deferred with the storage work; «Изменить лот, дату или дозу», which the prototype itself wires to `onClose`; the prototype's two disconnected vial datasets replaced by one (a correction, and the subtask's prohibition, so the registry is where the next reader looks); anything the three screens drop. Strike the vial picker off «What the dose wizard still owes».
- [ ] Gate, commit.

---

## Self-Review

**Spec coverage.** The subtask names three files and one prohibition. `VialsScreen.tsx` → Tasks 3, 4; `VialDetailSheet.tsx` → Task 5; `AddVialScreen.tsx` → Task 6. «Один набор флаконов» → Task 1, which deletes the second dataset by never creating it and grows the one that exists. «Остаток выводится из залогированных доз» → Task 1's assertion that no seeded vial carries a typed number, guarded by `Vial` having no field to put one in. «Статусы и подсказки докупки вычисляются» → Task 2, over `vialStatus` and `dosesPerWeek` rather than a second table.

**Known gaps, deliberately.** The label photo and the edit action. Nothing here proves visual fidelity — step 11's side-by-side run does.

**Type consistency.** `inventorySummary` is produced in Task 2 and consumed in Task 4. `InventoryRepository` is produced in Task 6's write and read in Tasks 4, 5 and 7. `CadenceGauge` is produced in Task 3 and called in Tasks 4 and 5. `Vial`, `vialStatus`, `remainingDoses` and `reorderHint` already exist in `shared/domain/InventoryMath.kt` and are not redefined.
