# KMP Mocks Behind Repository Interfaces — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the ported screens a data source they can be written against once — canonical models from §03, computation where §03 says «computed», and mocks sitting behind the exact interfaces the Ktor client will implement in M3–M10.

**Architecture:** Everything lands in `kmp/shared/`, which is the only module with a JVM host-test builder — so all of it is covered by `scripts/gate/kmp.sh` rather than by the iOS simulator. Three layers, in dependency order: `domain/` holds the §03 entities and the pure functions that derive from them (occurrences, remaining doses, statuses); `repository/` holds the interfaces the screens consume; `mock/` holds the in-memory implementations and the seed. A screen imports the interface and never learns which implementation it got.

**Tech Stack:** Kotlin Multiplatform 2.4.10, `kotlinx-datetime` for the clock and timezone, `kotlin.test` on the JVM host-test builder, ktlint + detekt through `scripts/gate/kmp.sh`.

## Global Constraints

- **§03 is the model, the prototype is the picture.** `source/architecture-overview-v1.1.md` §03 is canonical; `mobile/src/**/data.ts` supplies realistic *values*, never shapes. Where they disagree, §03 wins and §03 already names the seven disagreements — they are listed under «Prototype drift the schema resolves» and each one has a resolution.
- **The seven corrections, verbatim from §03**, all of which this step must implement rather than port around:
  1. One `target_weight_kg` — not 100 in trends and 102 in body.
  2. Real clocks. «Today» is `Clock.now()` in the patient's timezone; cycle position derives from `protocol.startDate`. No `TODAY = new Date(2026, 4, 31)`.
  3. One `vials` table; `remaining = totalDoses − count(doseEvents.vialId)`. Not two disconnected datasets, and logging decrements.
  4. Dose is `value: BigDecimal-ish + unit`, never a display string. «0,25 мг» is assembled by `app.cadence.format`.
  5. One `measurements` list for all eight metrics; per-surface projections are queries.
  6. A care *team*, via `careTeamAssignments` — not a single doctor.
  7. The Today protocol strip and the Schedule screen render the **same** generated occurrences.
- **Nothing derived is stored.** Vial status, remaining doses, reorder hints, adherence, schedule occurrences, «due today» — all computed on read. If a `status` field is about to be added, stop. This is a project rule *and* §03's L10.
- **Interfaces are consumer-owned.** The screen declares what it needs; `mock/` and, later, the Ktor client implement it. `domain/` never imports `repository/`, and neither ever imports `composeApp`.
- **Repository methods mirror the screen → data map (§11), not CRUD.** `GET /me/today` is one call returning one object, because that is what the server will send. Building fine-grained getters here would be designing a client the API does not have.
- **RU is the product language**, and none of it lives in `domain/`. Model fields carry `nameRu` where §03 does; the formatting of numbers, plurals and dates stays in `app.cadence.format` on the UI side.
- **The gate is `./scripts/gate/kmp.sh`** — and this time it runs the tests: `shared` has `withHostTestBuilder`, so `commonTest` executes on the JVM under `testAndroidHostTest`. Green at the end of every task.
- **detekt's `MagicNumber` exemptions do not cover `app.cadence.shared`.** Seed values are named constants or live in a seed object whose whole purpose is to hold them.
- **Scope, decided 2026-08-03:** all §03 models now, because transcribing a locked document is not designing blind. Repositories and seeds only for what step 4 needs — Today and Schedule. Every later section brings its own repository when its screens land, against the pattern this step sets.

---

## File Structure

**Created, all under `kmp/shared/src/commonMain/kotlin/app/cadence/shared/`:**

- `domain/Ids.kt` — the typed identifiers, so a `PatientId` cannot be passed where a `VialId` belongs
- `domain/Dose.kt` — `Dose(value, unit)` and `DoseUnit`
- `domain/Identity.kt` — `Profile`, `PatientProfile`, `ProviderProfile`, `CareTeamAssignment`, `UserPreferences`
- `domain/Protocol.kt` — `Compound`, `Protocol`, `ProtocolItem`, `ProtocolPhase`, and the cadence/kind enums
- `domain/Dosing.kt` — `DoseEvent`, `InjectionSite`, `SideEffect`
- `domain/Inventory.kt` — `Vial`
- `domain/Measurements.kt` — `Measurement`, `Metric`, `MeasurementSource`, `BodyPhoto`
- `domain/Nutrition.kt` — `Ingredient`, `Recipe`, `RecipeIngredient`, `Meal`, `MealItem`, `NutritionTargets`, `Macros`
- `domain/Journal.kt` — `JournalEntry`, `JournalTag`, `JournalSource`
- `domain/Messaging.kt` — `Thread`, `Message`, `MessageKind`, `MessageCard`
- `domain/Content.kt` — `Article`, `ArticleCategory`, `ArticleBlock`
- `domain/Occurrence.kt` — `Occurrence`, `OccurrenceStatus`, and `occurrencesFor(...)`: the generator §03 says replaces a schedule table
- `domain/InventoryMath.kt` — `remainingDoses`, `vialStatus`, `reorderHint`
- `domain/CadenceClock.kt` — the clock seam and `today(timeZone)`
- `repository/TodayRepository.kt`, `repository/ScheduleRepository.kt`
- `mock/MockSeed.kt` — one patient, one protocol, the vials, the meals
- `mock/MockTodayRepository.kt`, `mock/MockScheduleRepository.kt`
- `mock/CadenceMocks.kt` — the single place a screen's dependencies are assembled

**Test files mirroring each of the above under `commonTest/kotlin/app/cadence/shared/`.**

**Modified:**

- `kmp/gradle/libs.versions.toml` — `kotlinx-datetime`
- `kmp/shared/build.gradle.kts` — the dependency, and a `commonMain` source set block it does not have yet
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceShell.kt` — the three placeholder constants give way to the mock
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/ActionChooserSheet.kt` — unchanged in shape; it already takes its four values as parameters, which is the point

**Deliberately not built:** repositories for dosing, measurements, nutrition, journal, messaging or content. Their models exist, their interfaces do not, because no screen has asked yet — and step 1 of this block showed what designing against zero call sites costs (five components ported, all five unused).

---

### Task 1: A clock, and a place for the domain to live

`shared/` has no `commonMain` dependencies block and no notion of time. §03's second correction — «real clocks; cycle position derives from `protocols.start_date` + patient timezone» — is the whole reason this comes first.

**Files:**
- Modify: `kmp/gradle/libs.versions.toml`
- Modify: `kmp/shared/build.gradle.kts`
- Create: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/domain/CadenceClock.kt`
- Test: `kmp/shared/src/commonTest/kotlin/app/cadence/shared/domain/CadenceClockTest.kt`

**Interfaces:**
- Produces: `CadenceClock` (interface with `now(): Instant`), `SystemCadenceClock`, `FixedCadenceClock(instant)`, and `CadenceClock.today(zone: TimeZone): LocalDate`.

- [ ] **Step 1: Add the version**

In `kmp/gradle/libs.versions.toml`, under `[versions]`, then the matching `[libraries]` entry:

```toml
kotlinx-datetime = "0.7.1"
```

```toml
kotlinx-datetime = { module = "org.jetbrains.kotlinx:kotlinx-datetime", version.ref = "kotlinx-datetime" }
```

- [ ] **Step 2: Give `shared` a commonMain block**

In `kmp/shared/build.gradle.kts`, inside `sourceSets`, above the existing `commonTest`:

```kotlin
        commonMain.dependencies {
            implementation(libs.kotlinx.datetime)
        }
```

- [ ] **Step 3: Write the failing test**

Create `kmp/shared/src/commonTest/kotlin/app/cadence/shared/domain/CadenceClockTest.kt`:

```kotlin
package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals

class CadenceClockTest {
    @Test
    fun aFixedClockReportsTheInstantItWasGiven() {
        // Every test downstream of this one needs a «today» it chose, because
        // the prototype's hardcoded 31 May 2026 is exactly what §03 replaces.
        val clock = FixedCadenceClock.at("2026-05-31T04:00:00Z")

        assertEquals(LocalDate(2026, 5, 31), clock.today(TimeZone.UTC))
    }

    @Test
    fun theDateDependsOnTheZoneAndNotOnlyOnTheInstant() {
        // 04:00 UTC is already the 31st in Moscow and still the 30th in Los
        // Angeles. Cycle week, «due today» and the missed-dose sweep all hang
        // off this, so it is asserted rather than assumed.
        val clock = FixedCadenceClock.at("2026-05-30T22:00:00Z")

        assertEquals(LocalDate(2026, 5, 31), clock.today(TimeZone.of("Europe/Moscow")))
        assertEquals(LocalDate(2026, 5, 30), clock.today(TimeZone.of("America/Los_Angeles")))
    }

    @Test
    fun theSystemClockMoves() {
        val clock = SystemCadenceClock

        val first = clock.now()
        val second = clock.now()

        // Not an equality assertion — two reads can land in the same
        // millisecond. What must not happen is a constant.
        assertNotEquals(0L, second.toEpochMilliseconds())
        assertEquals(true, second >= first)
    }
}
```

- [ ] **Step 4: Run it and watch it fail**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests "app.cadence.shared.domain.CadenceClockTest"`
Expected: FAIL — unresolved references.

- [ ] **Step 5: Write the clock**

Create `kmp/shared/src/commonMain/kotlin/app/cadence/shared/domain/CadenceClock.kt`:

```kotlin
package app.cadence.shared.domain

import kotlinx.datetime.Instant
import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlinx.datetime.toLocalDateTime

/**
 * Where «now» comes from.
 *
 * An interface rather than a call to the system clock, because §03 replaces the
 * prototype's three disagreeing «todays» (29, 31 and 28 May, depending on the
 * screen) with real clocks — and a real clock is exactly what a test cannot
 * assert against. Every derived thing in this module — cycle week, «due
 * today», a vial's days-to-expiry — is a function of this and of stored dates,
 * and none of them may read the system clock directly.
 */
interface CadenceClock {
    fun now(): Instant
}

/** The clock the app runs on. */
object SystemCadenceClock : CadenceClock {
    override fun now(): Instant = kotlinx.datetime.Clock.System.now()
}

/** The clock the tests run on. */
class FixedCadenceClock(
    private val instant: Instant,
) : CadenceClock {
    override fun now(): Instant = instant

    companion object {
        fun at(iso: String): FixedCadenceClock = FixedCadenceClock(Instant.parse(iso))
    }
}

/**
 * The patient's calendar date.
 *
 * Takes the zone rather than assuming the device's: §03 sends the device
 * timezone to the server at sign-in precisely so occurrences, reminders and the
 * sweep agree, and a client that quietly used its own default would be the one
 * disagreeing.
 */
fun CadenceClock.today(zone: TimeZone): LocalDate = now().toLocalDateTime(zone).date
```

If `kotlinx.datetime.Clock` is deprecated in favour of `kotlin.time.Clock` on this Kotlin version, follow the deprecation — the seam is the interface, not which stdlib type it wraps. Record the choice in a comment.

- [ ] **Step 6: Run the test**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests "app.cadence.shared.domain.CadenceClockTest"`
Expected: PASS, 3 tests.

- [ ] **Step 7: Run the gate**

Run: `cd /Users/dmitriiporollo/programming/projects/cadence-app && ./scripts/gate/kmp.sh`
Expected: `kmp gate: green` — and note that the gate now runs these tests, unlike `composeApp`'s.

- [ ] **Step 8: Commit**

```bash
git add kmp/gradle/libs.versions.toml kmp/shared/build.gradle.kts kmp/shared/src
git commit -m "feat(kmp): time comes from a clock the tests can hold still"
```

---

### Task 2: The §03 entities

Transcription, not design. Every field below is in §03; nothing is invented, and where the prototype has a field §03 does not, it does not come over.

**Files:**
- Create: the eleven `domain/*.kt` files listed in File Structure
- Test: `kmp/shared/src/commonTest/kotlin/app/cadence/shared/domain/DomainTest.kt`

**Interfaces:**
- Consumes: nothing but `kotlinx-datetime`.
- Produces: the model types every later task and every later step reads.

- [ ] **Step 1: Write the failing test**

Create `kmp/shared/src/commonTest/kotlin/app/cadence/shared/domain/DomainTest.kt`. It asserts the handful of things a model can get wrong that a compiler cannot catch:

```kotlin
package app.cadence.shared.domain

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class DomainTest {
    @Test
    fun aDoseKeepsItsValueAndUnitApart() {
        // §03: «Mobile's structured {value, unit} wins over the dashboard's
        // display string». The whole correction is that nothing downstream can
        // be handed «0,25 мг» and have to parse it back.
        val dose = Dose(value = 0.25, unit = DoseUnit.MG)

        assertEquals(0.25, dose.value)
        assertEquals(DoseUnit.MG, dose.unit)
    }

    @Test
    fun theTenInjectionSitesAreAllThere() {
        // §03 names ten zones; the wizard's rotation suggestion is meaningless
        // if the set is short.
        assertEquals(10, InjectionSite.entries.size)
    }

    @Test
    fun theSevenSideEffectsAreTheOnesTheProtocolTracks() {
        assertEquals(
            setOf("nausea", "fatigue", "headache", "bloating", "insomnia", "site", "appetite"),
            SideEffect.entries.map { it.code }.toSet(),
        )
    }

    @Test
    fun theEightMetricsCoverBothSurfaces() {
        // Mobile shows all eight, the dashboard shows a subset — §03's «same
        // rows, different projections». A missing metric is a screen that
        // cannot be built.
        assertEquals(
            setOf("weight", "hrv", "rhr", "sleep", "bodyfat", "waist", "hip", "chest"),
            Metric.entries.map { it.code }.toSet(),
        )
    }

    @Test
    fun aPatientHasOneTargetWeight() {
        // §03's first correction: the prototype carries 100 kg in trends and
        // 102 kg in body. There is one field, so there is one answer.
        val profile =
            PatientProfile(
                userId = UserId("p-1"),
                dateOfBirth = null,
                sex = null,
                heightCm = 188,
                targetWeightKg = 92.0,
                joinedAt = null,
            )

        assertEquals(92.0, profile.targetWeightKg)
    }

    @Test
    fun aProtocolPhaseCarriesItsDoseAsData() {
        val phase = ProtocolPhase(fromWeek = 1, toWeek = 4, dose = Dose(0.25, DoseUnit.MG))

        assertTrue(phase.covers(week = 3))
        assertTrue(!phase.covers(week = 5))
    }
}
```

- [ ] **Step 2: Run it and watch it fail**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests "app.cadence.shared.domain.DomainTest"`
Expected: FAIL — unresolved references.

- [ ] **Step 3: Write the identifiers**

Create `domain/Ids.kt`:

```kotlin
package app.cadence.shared.domain

import kotlin.jvm.JvmInline

// Identifiers are typed. §03 hangs eleven contexts off the same handful of
// foreign keys, and a String parameter list is where a patient id ends up in a
// vial id's place with everything still compiling.

@JvmInline
value class UserId(val raw: String)

@JvmInline
value class CompoundId(val raw: String)

@JvmInline
value class ProtocolId(val raw: String)

@JvmInline
value class ProtocolItemId(val raw: String)

@JvmInline
value class VialId(val raw: String)

@JvmInline
value class DoseEventId(val raw: String)

@JvmInline
value class MeasurementId(val raw: String)

@JvmInline
value class MealId(val raw: String)

@JvmInline
value class RecipeId(val raw: String)

@JvmInline
value class IngredientId(val raw: String)

@JvmInline
value class ThreadId(val raw: String)

@JvmInline
value class MessageId(val raw: Long)

@JvmInline
value class ArticleId(val raw: String)
```

- [ ] **Step 4: Write the dose**

Create `domain/Dose.kt`:

```kotlin
package app.cadence.shared.domain

/** §03: `default_unit мг|мкг`. */
enum class DoseUnit(
    val code: String,
) {
    MG("мг"),
    MCG("мкг"),
}

/**
 * A dose, as §03 resolves it: «numeric + unit; RU formatting ("0,25 мг") is
 * presentation».
 *
 * `Double` rather than a decimal type: the values are protocol doses with two
 * significant figures, they are never summed, and Kotlin Multiplatform has no
 * common `BigDecimal`. If a total is ever needed the type changes here, once.
 */
data class Dose(
    val value: Double,
    val unit: DoseUnit,
)
```

- [ ] **Step 5: Write the remaining nine domain files**

Transcribe §03 «BOUNDED CONTEXTS & DATA MODEL» field by field into `domain/Identity.kt`, `domain/Protocol.kt`, `domain/Dosing.kt`, `domain/Inventory.kt`, `domain/Measurements.kt`, `domain/Nutrition.kt`, `domain/Journal.kt`, `domain/Messaging.kt` and `domain/Content.kt`. Rules for the transcription:

- Every entity is a `data class`; every enumerated column is an `enum class` carrying its wire `code`.
- Optional columns (`?` in §03) are nullable; everything else is not.
- Dates that are dates are `LocalDate`; timestamps are `Instant`; nothing is a `String` that §03 types otherwise.
- `dose_value numeric + dose_unit` collapses to one `Dose`.
- **No status fields.** `Vial` has no `status`, `Protocol` has none beyond §03's own `active|completed|cancelled`, and nothing has `remaining`.
- `ProtocolPhase` gets `fun covers(week: Int) = week in fromWeek..toWeek`.
- `Metric`, `InjectionSite` and `SideEffect` carry exactly the sets §03 lists — 8, 10 and 7 members.

- [ ] **Step 6: Run the test**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests "app.cadence.shared.domain.DomainTest"`
Expected: PASS, 6 tests.

- [ ] **Step 7: Check the transcription against the source, field by field**

Open `~/Documents/prll-vault/20-Projects/cadence/source/architecture-overview-v1.1.md` at §03 and walk each entity against its Kotlin counterpart. Write down, in the commit message, any field deliberately left out and why (for example: `avatar_bg/fg`, which is presentation the design system already owns). A field silently missing is the failure mode here, and no test can see it.

- [ ] **Step 8: Run the gate and commit**

```bash
cd /Users/dmitriiporollo/programming/projects/cadence-app && ./scripts/gate/kmp.sh
git add kmp/shared/src && git commit -m "feat(kmp): the eleven contexts of §03, transcribed rather than designed"
```

---

### Task 3: Occurrences, generated

§03: «There is no materialized schedule table. The calendar is generated on demand from `protocol_items × protocol_phases` in the patient's timezone.» This is the single largest piece of real logic in the step, it is pure, and the gate runs its tests.

**Files:**
- Create: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/domain/Occurrence.kt`
- Test: `kmp/shared/src/commonTest/kotlin/app/cadence/shared/domain/OccurrenceTest.kt`

**Interfaces:**
- Consumes: `Protocol`, `ProtocolItem`, `ProtocolPhase`, `Dose`, `DoseEvent`, `CadenceClock` (Tasks 1–2).
- Produces: `Occurrence(itemId, date, time, dose, kind, status)`, `OccurrenceStatus`, and
  `fun occurrencesFor(protocol: Protocol, items: List<ProtocolItem>, phases: Map<ProtocolItemId, List<ProtocolPhase>>, events: List<DoseEvent>, date: LocalDate, today: LocalDate): List<Occurrence>`
  plus `fun cycleWeek(protocol: Protocol, date: LocalDate): Int?`.

- [ ] **Step 1: Write the failing test**

Create `kmp/shared/src/commonTest/kotlin/app/cadence/shared/domain/OccurrenceTest.kt`:

```kotlin
package app.cadence.shared.domain

import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

private val CYCLE_START = LocalDate(2026, 5, 10)

class OccurrenceTest {
    @Test
    fun theCycleWeekCountsFromTheProtocolStartAndNotFromAConstant() {
        // The prototype hardcodes «week 4» against a frozen 31 May. §03 derives
        // it, which is why a protocol that started elsewhere gives another
        // answer for the same date.
        val protocol = protocolStarting(CYCLE_START)

        assertEquals(1, cycleWeek(protocol, LocalDate(2026, 5, 10)))
        assertEquals(1, cycleWeek(protocol, LocalDate(2026, 5, 16)))
        assertEquals(2, cycleWeek(protocol, LocalDate(2026, 5, 17)))
        assertEquals(4, cycleWeek(protocol, LocalDate(2026, 5, 31)))
        assertEquals(12, cycleWeek(protocol, LocalDate(2026, 8, 1)))
    }

    @Test
    fun aDateOutsideTheCycleHasNoWeekAndNoOccurrences() {
        val protocol = protocolStarting(CYCLE_START)

        assertNull(cycleWeek(protocol, LocalDate(2026, 5, 9)))
        assertNull(cycleWeek(protocol, LocalDate(2026, 8, 2)))
        assertTrue(occurrencesOn(LocalDate(2026, 5, 9)).isEmpty())
    }

    @Test
    fun aWeeklyItemFallsOnItsWeekdayAndNowhereElse() {
        // Семаглутид, Sundays. Six of the seven days must be free of it, which
        // an implementation that emitted every day would fail.
        val sundays = occurrencesOn(LocalDate(2026, 5, 17)).filter { it.itemId == SEMA }
        val monday = occurrencesOn(LocalDate(2026, 5, 18)).filter { it.itemId == SEMA }

        assertEquals(1, sundays.size)
        assertTrue(monday.isEmpty())
    }

    @Test
    fun aTwiceDailyItemEmitsBothTimes() {
        // BPC-157 at 08:00 and 20:00 — §03's `times[]`, and the reason an
        // occurrence is keyed by (item, date, time) rather than (item, date).
        val bpc = occurrencesOn(LocalDate(2026, 5, 18)).filter { it.itemId == BPC }

        assertEquals(2, bpc.size)
        assertEquals(listOf(LocalTime(8, 0), LocalTime(20, 0)), bpc.map { it.time })
    }

    @Test
    fun theDoseFollowsTheTitrationPhaseForThatWeek() {
        // 0,25 in weeks 1–4, 0,5 in 5–8, 1,0 in 9–12 — and the boundary weeks
        // are what an off-by-one gets wrong, so all six are asserted.
        assertEquals(Dose(0.25, DoseUnit.MG), semaDoseOn(LocalDate(2026, 5, 10)))
        assertEquals(Dose(0.25, DoseUnit.MG), semaDoseOn(LocalDate(2026, 5, 31)))
        assertEquals(Dose(0.5, DoseUnit.MG), semaDoseOn(LocalDate(2026, 6, 7)))
        assertEquals(Dose(0.5, DoseUnit.MG), semaDoseOn(LocalDate(2026, 6, 28)))
        assertEquals(Dose(1.0, DoseUnit.MG), semaDoseOn(LocalDate(2026, 7, 5)))
        assertEquals(Dose(1.0, DoseUnit.MG), semaDoseOn(LocalDate(2026, 8, 1)))
    }

    @Test
    fun aLoggedEventMarksItsOccurrenceDoneAndLeavesTheOthersAlone() {
        // The status is computed by comparing generated occurrences against
        // dose_events — §03's missed-dose sweep, client-side. Storing it is
        // what the project rule forbids.
        val date = LocalDate(2026, 5, 17)
        val logged = doseEventFor(SEMA, date)

        val occurrences = occurrencesOn(date, events = listOf(logged))

        assertEquals(OccurrenceStatus.DONE, occurrences.first { it.itemId == SEMA }.status)
        assertTrue(occurrences.filter { it.itemId == BPC }.all { it.status != OccurrenceStatus.DONE })
    }

    @Test
    fun aPastOccurrenceWithNoEventIsMissedAndAFutureOneIsScheduled() {
        val past = occurrencesOn(LocalDate(2026, 5, 17), today = LocalDate(2026, 5, 31))
        val future = occurrencesOn(LocalDate(2026, 6, 7), today = LocalDate(2026, 5, 31))
        val todays = occurrencesOn(LocalDate(2026, 5, 31), today = LocalDate(2026, 5, 31))

        assertTrue(past.all { it.status == OccurrenceStatus.MISSED })
        assertTrue(future.all { it.status == OccurrenceStatus.SCHEDULED })
        assertTrue(todays.all { it.status == OccurrenceStatus.PENDING })
    }
}
```

The helpers (`protocolStarting`, `occurrencesOn`, `semaDoseOn`, `doseEventFor`, `SEMA`, `BPC`) are private to the test file and build the minimal protocol §03 describes: Семаглутид weekly on the cycle's start weekday with three titration phases, BPC-157 daily at two times.

- [ ] **Step 2: Run it and watch it fail**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests "app.cadence.shared.domain.OccurrenceTest"`
Expected: FAIL — unresolved references.

- [ ] **Step 3: Write the generator**

Create `domain/Occurrence.kt` with:

```kotlin
package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime
import kotlinx.datetime.daysUntil

/** What the calendar shows for one item, on one date, at one time. */
data class Occurrence(
    val itemId: ProtocolItemId,
    val kind: ProtocolItemKind,
    val date: LocalDate,
    val time: LocalTime,
    val dose: Dose?,
    val status: OccurrenceStatus,
)

/**
 * Computed by comparing generated occurrences against logged events, never
 * stored — §03's L10 and the project's «nothing derived is stored».
 */
enum class OccurrenceStatus { DONE, PENDING, MISSED, SCHEDULED }

/** Week 1 is the seven days from the protocol's start; null outside the cycle. */
fun cycleWeek(protocol: Protocol, date: LocalDate): Int? {
    val offset = protocol.startDate.daysUntil(date)
    if (offset < 0) return null
    val week = offset / DAYS_PER_WEEK + 1
    return if (week > protocol.weeks) null else week
}
```

plus `occurrencesFor(...)`, which for each item in the protocol: skips the date if `cycleWeek` is null; decides whether the item falls on that date from its `cadence` and `daysOfWeek`; emits one `Occurrence` per entry in `times`; resolves the dose from the phase whose `covers(week)` is true; and sets the status from `events` first, then from the date's relation to `today`. Named constants for `DAYS_PER_WEEK`; no literal `7` in the body.

- [ ] **Step 4: Run the test**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests "app.cadence.shared.domain.OccurrenceTest"`
Expected: PASS, 7 tests.

- [ ] **Step 5: Mutate and confirm**

One at a time, reverting each:

1. `week = offset / DAYS_PER_WEEK + 1` → drop the `+ 1` — the cycle-week test must fail on day one.
2. In the phase lookup, take the first phase unconditionally — the titration test must fail at week 5.
3. Emit only `times.first()` — the twice-daily test must fail.
4. Return `SCHEDULED` for a past date with no event — the missed test must fail.
5. Ignore `events` entirely — the done test must fail.

Any mutation that leaves the suite green means the assertion is decorative; fix the test before continuing.

- [ ] **Step 6: Run the gate and commit**

```bash
cd /Users/dmitriiporollo/programming/projects/cadence-app && ./scripts/gate/kmp.sh
git add kmp/shared/src && git commit -m "feat(kmp): the schedule is generated, because §03 says there is no table for it"
```

---

### Task 4: Remaining doses, vial status, reorder

§03: «The prototype ships two disconnected vial datasets and logging never decrements stock. Fixed structurally: `remaining = total_doses − count(dose_events.vial_id)`. Status (sealed / active / low <25% / expiring ≤14 d) and reorder hints (0 sealed spares & ≤4 weeks supply) are computed.»

**Files:**
- Create: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/domain/InventoryMath.kt`
- Test: `kmp/shared/src/commonTest/kotlin/app/cadence/shared/domain/InventoryMathTest.kt`

**Interfaces:**
- Consumes: `Vial`, `DoseEvent`, `LocalDate` (Tasks 1–2).
- Produces: `remainingDoses(vial, events)`, `VialStatus`, `vialStatus(vial, events, today)`, `ReorderHint`, `reorderHint(vials, events, today, dosesPerWeek)`.

- [ ] **Step 1: Write the failing test**

Create `kmp/shared/src/commonTest/kotlin/app/cadence/shared/domain/InventoryMathTest.kt`, asserting:

```kotlin
package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals

private val TODAY = LocalDate(2026, 5, 31)

class InventoryMathTest {
    @Test
    fun loggingADoseDecrementsTheVialItCameFrom() {
        // The prototype's central inventory bug: logging never touched stock.
        // There is no stored counter to go wrong here — the count is the
        // subtraction, every time it is asked for.
        val vial = vial(totalDoses = 4)

        assertEquals(4, remainingDoses(vial, events = emptyList()))
        assertEquals(2, remainingDoses(vial, events = List(2) { doseFrom(vial) }))
        assertEquals(0, remainingDoses(vial, events = List(4) { doseFrom(vial) }))
    }

    @Test
    fun eventsFromOtherVialsDoNotCount() {
        // Two vials of the same compound is the ordinary case, and a
        // subtraction that ignored `vialId` would drain both at once.
        val mine = vial(totalDoses = 4)
        val other = vial(totalDoses = 4)

        assertEquals(4, remainingDoses(mine, events = List(3) { doseFrom(other) }))
    }

    @Test
    fun remainingNeverGoesNegative() {
        // More events than doses is data corruption, not a number to render.
        val vial = vial(totalDoses = 2)

        assertEquals(0, remainingDoses(vial, events = List(5) { doseFrom(vial) }))
    }

    @Test
    fun anUnopenedVialIsSealedAndAnOpenedOneIsActive() {
        assertEquals(VialStatus.SEALED, vialStatus(vial(openedAt = null), emptyList(), TODAY))
        assertEquals(VialStatus.ACTIVE, vialStatus(vial(openedAt = LocalDate(2026, 5, 1)), emptyList(), TODAY))
    }

    @Test
    fun aVialBelowAQuarterIsLow() {
        // §03's «low <25%». Exactly a quarter is not low; below it is.
        val v = vial(totalDoses = 4, openedAt = LocalDate(2026, 5, 1))

        assertEquals(VialStatus.ACTIVE, vialStatus(v, List(2) { doseFrom(v) }, TODAY))
        assertEquals(VialStatus.LOW, vialStatus(v, List(3) { doseFrom(v) }, TODAY))
    }

    @Test
    fun expiryOutranksEverythingElse() {
        // «expiring ≤14 d». A vial that is both low and expiring reads as
        // expiring, because that is the one with a deadline attached.
        val v = vial(totalDoses = 4, openedAt = LocalDate(2026, 5, 1), expiresOn = LocalDate(2026, 6, 10))

        assertEquals(VialStatus.EXPIRING, vialStatus(v, List(3) { doseFrom(v) }, TODAY))
        assertEquals(
            VialStatus.LOW,
            vialStatus(v.copy(expiresOn = LocalDate(2026, 7, 20)), List(3) { doseFrom(v) }, TODAY),
        )
    }

    @Test
    fun theReorderHintNeedsBothNoSparesAndLittleSupply() {
        // §03: «0 sealed spares & ≤4 weeks supply». Either one alone is not a
        // reason to tell a patient to order more.
        val open = vial(totalDoses = 4, openedAt = LocalDate(2026, 5, 1))
        val sealed = vial(totalDoses = 4)

        assertEquals(null, reorderHint(listOf(open, sealed), emptyList(), TODAY, dosesPerWeek = 1.0))
        assertEquals(null, reorderHint(listOf(open), emptyList(), TODAY, dosesPerWeek = 0.2))

        val hint = reorderHint(listOf(open), List(1) { doseFrom(open) }, TODAY, dosesPerWeek = 1.0)
        assertEquals(3, hint?.weeksLeft)
    }
}
```

- [ ] **Step 2: Run it and watch it fail**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests "app.cadence.shared.domain.InventoryMathTest"`
Expected: FAIL — unresolved references.

- [ ] **Step 3: Write the arithmetic**

Create `domain/InventoryMath.kt`. `remainingDoses` is `(vial.totalDoses - events.count { it.vialId == vial.id }).coerceAtLeast(0)`. `VialStatus` is `SEALED | ACTIVE | LOW | EXPIRING | DISPOSED`, decided in that precedence with named thresholds (`LOW_FRACTION = 0.25`, `EXPIRING_DAYS = 14`). `reorderHint` returns `ReorderHint(weeksLeft, compound)` or null, and requires both conditions §03 names.

- [ ] **Step 4: Run the test, then mutate**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests "app.cadence.shared.domain.InventoryMathTest"`
Expected: PASS, 7 tests. Then, one at a time:

1. Drop the `it.vialId == vial.id` filter — the other-vials test must fail.
2. Drop `coerceAtLeast(0)` — the negative test must fail.
3. Change `<` to `<=` on the low threshold — the quarter-boundary test must fail.
4. Check expiry after low rather than before — the precedence test must fail.
5. Make `reorderHint` fire on either condition — the two null assertions must fail.

- [ ] **Step 5: Run the gate and commit**

```bash
cd /Users/dmitriiporollo/programming/projects/cadence-app && ./scripts/gate/kmp.sh
git add kmp/shared/src && git commit -m "feat(kmp): a vial's remaining doses are a subtraction, not a column"
```

---

### Task 5: The two interfaces the next step needs, and the mock behind them

The seam itself. §11's screen → data map is the shape: `GET /me/today` is one call returning one object, so `TodayRepository` has one method returning one object.

**Files:**
- Create: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/repository/TodayRepository.kt`
- Create: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/repository/ScheduleRepository.kt`
- Create: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/mock/MockSeed.kt`
- Create: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/mock/MockTodayRepository.kt`
- Create: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/mock/MockScheduleRepository.kt`
- Create: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/mock/CadenceMocks.kt`
- Test: `kmp/shared/src/commonTest/kotlin/app/cadence/shared/mock/MockRepositoryTest.kt`

**Interfaces:**
- Consumes: everything from Tasks 1–4.
- Produces: `TodayRepository` with `suspend fun today(): TodaySummary`; `ScheduleRepository` with `suspend fun month(month: LocalDate): List<ScheduleDay>` and `suspend fun day(date: LocalDate): List<Occurrence>`; `TodaySummary(nextDose, doseLoggedToday, meals, targets, weightGlance, reorder, cycleWeek)`; `CadenceMocks(clock, zone)` assembling both.

- [ ] **Step 1: Write the failing test**

Create `kmp/shared/src/commonTest/kotlin/app/cadence/shared/mock/MockRepositoryTest.kt`:

```kotlin
package app.cadence.shared.mock

import app.cadence.shared.domain.DoseUnit
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.OccurrenceStatus
import kotlinx.coroutines.test.runTest
import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

private val ZONE = TimeZone.of("Europe/Moscow")

/** The mock, wound to a date inside the seeded protocol's fourth week. */
private fun mocks(iso: String = "2026-05-31T09:00:00Z") =
    CadenceMocks(clock = FixedCadenceClock.at(iso), zone = ZONE)

class MockRepositoryTest {
    @Test
    fun todayReportsTheNextDoseAsDataRatherThanAsAString() =
        runTest {
            val summary = mocks().today.today()

            val next = assertNotNull(summary.nextDose, "the seeded protocol has a dose due today")
            assertEquals(0.25, next.dose?.value)
            assertEquals(DoseUnit.MG, next.dose?.unit)
        }

    @Test
    fun theCycleWeekMovesWhenTheClockDoes() =
        runTest {
            // The whole point of the clock seam: nothing is pinned to 31 May.
            assertEquals(4, mocks("2026-05-31T09:00:00Z").today.today().cycleWeek)
            assertEquals(5, mocks("2026-06-07T09:00:00Z").today.today().cycleWeek)
        }

    @Test
    fun loggingADoseChangesWhatTodayReportsAndWhatTheVialHasLeft() =
        runTest {
            // The acceptance criterion of the whole block, in one test: a write
            // through the interface is visible through the interface, with no
            // screen involved.
            val m = mocks()
            val before = m.today.today()
            assertTrue(!before.doseLoggedToday)
            val remainingBefore = before.reorder?.weeksLeft

            m.dosing.logDose(before.nextDose!!.itemId, site = null)

            val after = m.today.today()
            assertTrue(after.doseLoggedToday)
            assertTrue(after.reorder?.weeksLeft != remainingBefore || remainingBefore == null)
        }

    @Test
    fun theScheduleAndTodayAgreeAboutTheSameDay() =
        runTest {
            // §03's seventh correction: the Today strip and the Schedule screen
            // render the same generated occurrences. The prototype's diverge.
            val m = mocks()
            val fromSchedule = m.schedule.day(LocalDate(2026, 5, 31))
            val fromToday = m.today.today().nextDose

            assertEquals(
                fromSchedule.first { it.status == OccurrenceStatus.PENDING }.itemId,
                fromToday?.itemId,
            )
        }

    @Test
    fun aMonthOfDaysCarriesTheDotsTheCalendarDraws() =
        runTest {
            val days = mocks().schedule.month(LocalDate(2026, 5, 1))

            assertEquals(31, days.size)
            assertTrue(days.any { it.hasInjection })
            assertTrue(days.none { it.date.monthNumber != 5 })
        }
}
```

`kotlinx-coroutines-test` is needed for `runTest`; add it to `libs.versions.toml` and to `shared`'s `commonTest` in this task.

- [ ] **Step 2: Run it and watch it fail**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests "app.cadence.shared.mock.MockRepositoryTest"`
Expected: FAIL — unresolved references.

- [ ] **Step 3: Write the interfaces**

`repository/TodayRepository.kt` and `repository/ScheduleRepository.kt` declare `suspend` methods returning domain types. They carry KDoc naming the endpoint from §11 they stand in for, so the Ktor implementation has its contract written down beside it. `TodaySummary` and `ScheduleDay` live here, next to the interfaces that return them, because they are the shape of a response and not a §03 entity.

The dosing write lives on a third, minimal interface — `DoseLogRepository` with `suspend fun logDose(itemId, site): DoseEventId` — because Today has to be able to change and the test above is what proves the seam works at all.

- [ ] **Step 4: Write the seed and the mocks**

`mock/MockSeed.kt` holds one patient, one 12-week protocol starting 2026-05-10 with Семаглутид weekly (three titration phases: 0,25 / 0,5 / 1,0) and BPC-157 twice daily, two vials, `NutritionTargets` from §03 (kcal 1800, protein 140, carbs 200, fat 60), a handful of measurements, and today's meals. Values are the prototype's where the prototype has them; shapes are §03's.

`MockTodayRepository` and `MockScheduleRepository` compute everything from the seed through Task 3's and Task 4's functions — they must not hold a precomputed answer, because the point is that the real repository will not either. Dose writes append to an in-memory list shared with both, so the test above passes for the right reason.

`CadenceMocks(clock, zone)` exposes `today`, `schedule` and `dosing`. It is the only place a construction happens, so swapping in the Ktor client later is one file.

- [ ] **Step 5: Run the test, then mutate**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests "app.cadence.shared.mock.MockRepositoryTest"`
Expected: PASS, 5 tests. Then:

1. Make `MockTodayRepository` return a stored `cycleWeek` constant — the moving-clock test must fail.
2. Make `logDose` a no-op — the write-visible-through-read test must fail.
3. Give `MockTodayRepository` its own copy of the events list — the same test must fail.
4. Have `today()` pick its next dose from the seed directly rather than from `occurrencesFor` — the agreement test must fail.

- [ ] **Step 6: Run the gate and commit**

```bash
cd /Users/dmitriiporollo/programming/projects/cadence-app && ./scripts/gate/kmp.sh
git add kmp/gradle/libs.versions.toml kmp/shared && git commit -m "feat(kmp): mocks behind the interfaces the Ktor client will implement"
```

---

### Task 6: The shell stops making numbers up

Three hardcoded constants in `CadenceShell.kt` are the block's acceptance criterion in miniature: replacing them with the mock must not change `ActionChooserSheet` at all, because it already takes its four values as parameters.

**Files:**
- Modify: `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceShell.kt`
- Modify: `kmp/composeApp/build.gradle.kts` (only if `:shared`'s new dependencies need exposing)
- Test: `kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/CadenceShellDataTest.kt`

- [ ] **Step 1: Write the failing test**

Create `kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/CadenceShellDataTest.kt`, asserting that the action sheet shows what the injected repository says and not a constant: render `CadenceApp` with a `CadenceMocks` wound to a clock where the day already holds two meals, open the sheet, and assert the subtitle reads the mock's numbers. Then render it with a different seed state and assert the subtitle changed.

- [ ] **Step 2: Run it and watch it fail**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.shell.CadenceShellDataTest"`
Expected: FAIL.

- [ ] **Step 3: Take the parameters in**

`CadenceApp` gains a `mocks: CadenceMocks` parameter with a default of `CadenceMocks(SystemCadenceClock, TimeZone.currentSystemDefault())`, collects `today()` into state with `LaunchedEffect`, and passes the four values to `ActionChooserSheet` and the toast's target. `PLACEHOLDER_MEAL_NAME`, `PLACEHOLDER_DAY_KCAL` and `PLACEHOLDER_KCAL_TARGET` are deleted.

**`ActionChooserSheet.kt` must not change.** If it does, the interface was wrong — that is the acceptance criterion failing, and it is worth stopping over rather than patching the sheet.

- [ ] **Step 4: Run the whole suite**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test && ./gradlew :shared:testAndroidHostTest`
Expected: PASS on both.

- [ ] **Step 5: Confirm the acceptance criterion with `git diff`**

Run: `git diff --stat`
Expected: `ActionChooserSheet.kt` is **not** in the list. If it is, say so and explain why in the commit message — an unplanned change here is the signal the seam leaks.

- [ ] **Step 6: Run the gate and commit**

```bash
cd /Users/dmitriiporollo/programming/projects/cadence-app && ./scripts/gate/kmp.sh
git add kmp && git commit -m "feat(kmp): the shell asks a repository instead of holding three constants"
```

---

## Self-Review

**Spec coverage.** The subtask names `state/AppState.tsx` plus the scattered seeds, and three requirements: consumer-owned interfaces in `shared/`, mocks implementing them, screens knowing only the interface. Tasks 5 and 6 are those three. «Модели берутся из §03, а не из прототипа» with its five named corrections maps to Task 2 (dose as `{value, unit}`, one target weight), Task 3 (time from real clocks, one set of occurrences for both surfaces), Task 4 (one vial dataset, remaining derived) and Task 1 (the clock those rest on). The parent task's «подмена мока на настоящий источник не требует правки экрана — доказано хотя бы на одном разделе» is Task 6, Step 5, and it is checked by `git diff` rather than asserted.

**Known gaps, deliberately.** Nine of the eleven contexts get models and no repository, per the scope decision — dosing gets a minimal write interface only because Today has to be able to change for Task 5's seam test to mean anything. The mock is in-memory and single-user; multi-user, offline queueing and the idempotency key of `kmp-app` invariant 6 belong to M3 when there is a server to be idempotent against.

**Type consistency.** `Dose(value: Double, unit: DoseUnit)` is constructed identically in Tasks 2, 3 and 5. `ProtocolItemId` is the key type in `occurrencesFor`'s `phases` map, in `Occurrence.itemId`, and in `DoseLogRepository.logDose`. `cycleWeek(protocol, date): Int?` is nullable in Task 3 and read as nullable by `TodaySummary.cycleWeek` in Task 5. `remainingDoses(vial, events)` takes the same argument order in Task 4 and in `MockTodayRepository`.
