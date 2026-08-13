package app.cadence.shared.mock

import app.cadence.shared.domain.Compound
import app.cadence.shared.domain.CompoundId
import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.DoseEvent
import app.cadence.shared.domain.DoseEventId
import app.cadence.shared.domain.DoseUnit
import app.cadence.shared.domain.Macros
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.Meal
import app.cadence.shared.domain.MealId
import app.cadence.shared.domain.MealItem
import app.cadence.shared.domain.MealSource
import app.cadence.shared.domain.Measurement
import app.cadence.shared.domain.MeasurementId
import app.cadence.shared.domain.MeasurementSource
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.NutritionTargets
import app.cadence.shared.domain.PatientProfile
import app.cadence.shared.domain.Protocol
import app.cadence.shared.domain.ProtocolCadence
import app.cadence.shared.domain.ProtocolId
import app.cadence.shared.domain.ProtocolItem
import app.cadence.shared.domain.ProtocolItemId
import app.cadence.shared.domain.ProtocolItemKind
import app.cadence.shared.domain.ProtocolPhase
import app.cadence.shared.domain.ProtocolPlan
import app.cadence.shared.domain.ProtocolStatus
import app.cadence.shared.domain.UserId
import app.cadence.shared.domain.Vial
import app.cadence.shared.domain.VialId
import app.cadence.shared.domain.occurrencesFor
import app.cadence.shared.domain.suggestNextSite
import kotlinx.datetime.DatePeriod
import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalDateTime
import kotlinx.datetime.LocalTime
import kotlinx.datetime.TimeZone
import kotlinx.datetime.plus
import kotlinx.datetime.toInstant
import kotlin.math.round
import kotlin.math.sin
import kotlin.time.Instant

/**
 * Values are the prototype's where it has them; shapes are §03's — the prototype's frozen
 * «31 May» is a computed start date here, and its two disconnected vial datasets are one
 * list that dose events subtract from.
 */
object MockSeed {
    val patientId = UserId("patient-1")

    val semaglutide =
        Compound(
            id = CompoundId("sema"),
            code = "sema",
            nameRu = "Семаглутид",
            defaultUnit = DoseUnit.MG,
            route = "п/к",
            icon = "beaker",
        )

    val bpc =
        Compound(
            id = CompoundId("bpc"),
            code = "bpc",
            nameRu = "BPC-157",
            defaultUnit = DoseUnit.MCG,
            route = "п/к",
            icon = "beaker",
        )

    /**
     * A SUPPLEMENT in §03's terms: no phases, so no dose on the row — the strip's dose
     * column is meaningfully optional, not always present.
     */
    val glycine =
        Compound(
            id = CompoundId("glycine"),
            code = "glycine",
            nameRu = "Глицин + магний",
            defaultUnit = DoseUnit.MG,
            route = "внутрь",
            icon = "moon",
        )

    val compounds = listOf(semaglutide, bpc, glycine)

    /** Sunday, 10 May 2026 — the prototype's cycle start, now an actual field. */
    val cycleStart = LocalDate(2026, 5, 10)

    /**
     * Everything below hangs off [cycleStart]; reading the system clock instead ages it all
     * out at once — `cycleWeek` returns null past `lastPrescribedDay`, a hard gate in several
     * functions, so the surface goes blank rather than degrading. It did: the course ended on
     * 1 August 2026 and no screen said anything. Week 4 is the day every test fixture already
     * winds to, so the demo shows what the suite asserts.
     */
    const val DEMO_NOW = "2026-05-31T09:00:00Z"

    private val protocolId = ProtocolId("protocol-1")
    val semaItemId = ProtocolItemId("item-sema")
    val bpcItemId = ProtocolItemId("item-bpc")
    val glycineItemId = ProtocolItemId("item-glycine")

    val plan =
        ProtocolPlan(
            protocol =
                Protocol(
                    id = protocolId,
                    patientId = patientId,
                    startDate = cycleStart,
                    weeks = 12,
                    status = ProtocolStatus.ACTIVE,
                    createdBy = null,
                    notes = null,
                ),
            items =
                listOf(
                    ProtocolItem(
                        id = semaItemId,
                        protocolId = protocolId,
                        kind = ProtocolItemKind.INJECTION,
                        compoundId = semaglutide.id,
                        cadence = ProtocolCadence.WEEKLY,
                        daysOfWeek = listOf(DayOfWeek.SUNDAY),
                        times = listOf(LocalTime(7, 0)),
                        loggable = true,
                    ),
                    ProtocolItem(
                        id = bpcItemId,
                        protocolId = protocolId,
                        kind = ProtocolItemKind.INJECTION,
                        compoundId = bpc.id,
                        cadence = ProtocolCadence.DAILY,
                        daysOfWeek = emptyList(),
                        times = listOf(LocalTime(8, 0), LocalTime(20, 0)),
                        loggable = true,
                    ),
                    ProtocolItem(
                        id = glycineItemId,
                        protocolId = protocolId,
                        kind = ProtocolItemKind.SUPPLEMENT,
                        compoundId = glycine.id,
                        cadence = ProtocolCadence.DAILY,
                        daysOfWeek = emptyList(),
                        times = listOf(LocalTime(21, 30)),
                        loggable = false,
                    ),
                ),
            phases =
                mapOf(
                    semaItemId to
                        listOf(
                            ProtocolPhase(1, 4, Dose(0.25, DoseUnit.MG)),
                            ProtocolPhase(5, 8, Dose(0.5, DoseUnit.MG)),
                            ProtocolPhase(9, 12, Dose(1.0, DoseUnit.MG)),
                        ),
                    bpcItemId to listOf(ProtocolPhase(1, 12, Dose(250.0, DoseUnit.MCG))),
                ),
        )

    /**
     * History stops the day before DEMO_NOW so the app opens on a day not yet logged (hero
     * offers «Записать →»). Measurements run a day further, to [MEASURED_THROUGH] — the two
     * are meant to disagree: this morning's readings have happened, this evening's dose hasn't.
     */
    val seededThrough = LocalDate(2026, 5, 30)

    /**
     * Two BPC vials open at once from the 25th, deliberately: §03 allows it, and it's the
     * only shape in which "which vial did this come out of" has more than one answer.
     */
    private val BPC_SPARE_OPENED = LocalDate(2026, 5, 22)
    private val BPC_LOW_OPENED = LocalDate(2026, 5, 25)

    /**
     * §03: `remaining = total_doses − count(dose_events.vial_id)`, and [Vial] has no field
     * to store the remainder — so [history] is the other half of every number this cabinet
     * shows, not decoration. Semaglutide: one vial, three of four doses taken, so the reorder
     * hint fires. Sits last on purpose: a reader answering «the patient's vial» with
     * `vials.first()` is right only by list order. BPC-157 carries the other four statuses:
     * disposed, low, sealed-and-expiring, sealed.
     */
    val vials =
        listOf(
            Vial(
                id = VialId("vial-bpc-1"),
                patientId = patientId,
                compoundId = bpc.id,
                concentrationLabel = "5 мг/мл",
                totalDoses = 30,
                openedAt = cycleStart,
                expiresOn = LocalDate(2026, 8, 1),
                lot = "B-2204",
                locationRu = "Холодильник, полка 1",
                // Disposed with six doses still in it: only case where drawing from a
                // disposed vial is distinguishable from drawing correctly.
                disposedAt = BPC_SPARE_OPENED,
                labelPhotoPath = null,
            ),
            Vial(
                id = VialId("vial-bpc-2"),
                patientId = patientId,
                compoundId = bpc.id,
                concentrationLabel = "5 мг/мл",
                totalDoses = 14,
                openedAt = BPC_LOW_OPENED,
                expiresOn = LocalDate(2026, 9, 1),
                lot = "B-2510",
                locationRu = "Холодильник, полка 1",
                disposedAt = null,
                labelPhotoPath = null,
            ),
            Vial(
                id = VialId("vial-bpc-3"),
                patientId = patientId,
                compoundId = bpc.id,
                concentrationLabel = "5 мг/мл",
                totalDoses = 10,
                openedAt = BPC_SPARE_OPENED,
                // Within a fortnight of the seeded day.
                expiresOn = LocalDate(2026, 6, 10),
                lot = "B-2601",
                locationRu = "Холодильник, полка 1",
                disposedAt = null,
                labelPhotoPath = null,
            ),
            Vial(
                id = VialId("vial-bpc-4"),
                patientId = patientId,
                compoundId = bpc.id,
                concentrationLabel = "5 мг/мл",
                totalDoses = 30,
                openedAt = null,
                expiresOn = LocalDate(2026, 10, 15),
                lot = "B-2610",
                locationRu = "Холодильник, полка 1",
                disposedAt = null,
                labelPhotoPath = null,
            ),
            Vial(
                id = VialId("vial-sema-1"),
                patientId = patientId,
                compoundId = semaglutide.id,
                concentrationLabel = "1 мг/мл",
                totalDoses = 4,
                openedAt = cycleStart,
                expiresOn = LocalDate(2026, 9, 1),
                lot = "A-2261",
                locationRu = "Холодильник, полка 2",
                disposedAt = null,
                labelPhotoPath = null,
            ),
        )

    /**
     * Generated, not typed: forty-five hand-written rows is forty-five chances to disagree
     * with the schedule, and the zones come from [suggestNextSite] so the history follows
     * the rotation rule rather than second-guessing it.
     */
    val history: List<DoseEvent> = buildHistory()

    private fun buildHistory(): List<DoseEvent> {
        val written = mutableListOf<DoseEvent>()
        var date = cycleStart

        while (date <= seededThrough) {
            occurrencesFor(plan, emptyList(), date, seededThrough)
                .filter { it.itemId == semaItemId || it.itemId == bpcItemId }
                .sortedBy { it.time }
                .forEach { occurrence ->
                    written +=
                        DoseEvent(
                            id = DoseEventId("seed-${written.size}"),
                            patientId = patientId,
                            protocolItemId = occurrence.itemId,
                            vialId = vialDrawnFrom(occurrence.itemId, date),
                            scheduledForDate = date,
                            scheduledForTime = occurrence.time,
                            injectedAt = LocalDateTime(date, occurrence.time).toInstant(TimeZone.UTC),
                            dose = occurrence.dose ?: error("a loggable occurrence with no dose on $date"),
                            // Each event asks the wizard's own function, against everything logged before it.
                            site = suggestNextSite(written),
                            mood = null,
                            sideEffects = emptyList(),
                            note = null,
                            photoPath = null,
                            createdAt = LocalDateTime(date, occurrence.time).toInstant(TimeZone.UTC),
                        )
                }
            date = LocalDate.fromEpochDays(date.toEpochDays() + 1)
        }

        return written
    }

    /** Which vial a dose on [date] came out of. */
    private fun vialDrawnFrom(
        itemId: ProtocolItemId,
        date: LocalDate,
    ): VialId =
        when {
            itemId == semaItemId -> VialId("vial-sema-1")
            date < BPC_SPARE_OPENED -> VialId("vial-bpc-1")
            date < BPC_LOW_OPENED -> VialId("vial-bpc-3")
            else -> VialId("vial-bpc-2")
        }

    val profile =
        PatientProfile(
            userId = patientId,
            dateOfBirth = LocalDate(1988, 3, 14),
            sex = null,
            heightCm = 188,
            targetWeightKg = 92.0,
            joinedAt = LocalDate(2026, 4, 20),
        )

    /** §03's `nutrition_targets` — the one place these four constants exist. */
    val targets =
        NutritionTargets(
            patientId = patientId,
            macros = Macros(kcal = 1800, proteinG = 140, carbsG = 200, fatG = 60),
            waterMl = null,
        )

    /**
     * Must be a Sunday: weekly metrics are measured on Sundays, and a mid-week horizon would
     * quietly end their series early while the daily ones ran on.
     */
    private val MEASURED_THROUGH = LocalDate(2026, 5, 31)

    /**
     * Six weeks of readings for every generated metric. Chest is left unmeasured on purpose —
     * «a metric with no readings says so» needs one that has none. Weight isn't generated:
     * its eight literals below run from 12 April, before the intake, and stay untouched
     * because two mutation traps stand on them. Everything generated starts at
     * [profile]`.joinedAt` (the intake) so all four trend windows (7/28/84/22 days) end up
     * with different point counts, two of them partial — described rather than hidden.
     */
    val measurements: List<Measurement> = buildMeasurements()

    private fun buildMeasurements(): List<Measurement> {
        // Not a fallback: a profile with no joinedAt has no first day to measure from, and
        // substituting one would put a single reading at the *end* value of a course never taken.
        val joined = profile.joinedAt ?: error("the seeded profile has no joinedAt to measure from")
        val daily =
            generateSequence(joined) { it.plus(DatePeriod(days = 1)) }
                .takeWhile { it <= MEASURED_THROUGH }
                .toList()
        // Filtered out of `daily`, not prepended, so a horizon earlier than the intake can't
        // leave a measurement dated in the future.
        val weekly = daily.filter { it == joined || it.dayOfWeek == DayOfWeek.SUNDAY }
        val watch = MeasurementSource.HEALTH_KIT
        val hand = MeasurementSource.MANUAL

        return listOf(
            // Eight readings for a seven-point series (so take/takeLast disagree) and one out
            // of list order (so the sort matters) — both mutations survived a seed that was
            // already sorted and exactly seven long.
            weight("2026-04-26T06:00:00Z", 100.8),
            weight("2026-04-12T06:00:00Z", 101.9),
            weight("2026-04-19T06:00:00Z", 101.2),
            weight("2026-05-03T06:00:00Z", 100.1),
            weight("2026-05-10T06:00:00Z", 99.9),
            weight("2026-05-17T06:00:00Z", 99.2),
            weight("2026-05-24T06:00:00Z", 98.8),
            weight("2026-05-31T06:00:00Z", 98.4),
        ) +
            // HRV is minutes later than the last weight on purpose, so «latest weight» can't
            // be «latest reading» and pass — every other reading of the day stays earlier.
            ramp(Metric.HRV, daily, "ms", watch, Ramp(50.0, 58.0, WHOLE, 2.0, LocalTime(6, 5))) +
            ramp(Metric.RHR, daily, "bpm", watch, Ramp(66.0, 58.0, WHOLE, 1.0, LocalTime(6, 1))) +
            // Seeded as HEALTH_KIT because `MeasurementSource` has no "derived" value, the
            // nearest true thing for a metric scored server-side from imported sessions.
            ramp(Metric.SLEEP, daily, "/100", watch, Ramp(72.0, 86.0, WHOLE, 3.0, LocalTime(6, 2))) +
            ramp(Metric.BODY_FAT, weekly, "%", hand, Ramp(26.5, 24.6, TENTHS, 0.1, LocalTime(6, 3))) +
            // The prototype's waist series is in inches — its own copy gives it away (the
            // fall the caption describes is 6,35cm, not the 2,5cm the numbers imply) — so
            // porting it as written would draw a 37cm waist on a 188cm body. HIP isn't the
            // prototype's thigh rebadged; §03 has no thigh metric.
            ramp(Metric.WAIST, weekly, "cm", hand, Ramp(104.0, 99.0, TENTHS, 0.3, LocalTime(6, 3))) +
            ramp(Metric.HIP, weekly, "cm", hand, Ramp(108.0, 105.0, TENTHS, 0.2, LocalTime(6, 3)))
    }

    /** Values land on whole numbers. */
    private const val WHOLE = 1.0

    /** Values land on one decimal place. */
    private const val TENTHS = 10.0

    /**
     * [ripple] stops a chart from being a straight line; fixed rather than random so an
     * assertion doesn't become a coin toss. [grid] is what the value rounds onto.
     */
    private class Ramp(
        val from: Double,
        val to: Double,
        val grid: Double,
        val ripple: Double,
        val atTime: LocalTime,
    )

    /**
     * Endpoints are left exact so «this metric moved in its own direction» is a statement
     * about the protocol, not about where the wave landed. A single day has no two endpoints
     * to run between, so it's refused rather than resolved to one.
     */
    private fun ramp(
        metric: Metric,
        days: List<LocalDate>,
        unit: String,
        source: MeasurementSource,
        shape: Ramp,
    ): List<Measurement> {
        require(days.size >= 2) {
            "${metric.code} was handed ${days.size} day(s): a ramp needs two endpoints to run between"
        }

        return days.mapIndexed { i, day ->
            val t = i.toDouble() / (days.size - 1)
            val wave = if (i == 0 || i == days.lastIndex) 0.0 else sin(i * 0.7) * shape.ripple
            Measurement(
                id = MeasurementId("m-${metric.code}-$day"),
                patientId = patientId,
                metric = metric,
                value = round((shape.from + (shape.to - shape.from) * t + wave) * shape.grid) / shape.grid,
                unit = unit,
                measuredAt = LocalDateTime(day, shape.atTime).toInstant(TimeZone.UTC),
                source = source,
                externalId = null,
                note = null,
            )
        }
    }

    private fun weight(
        at: String,
        kg: Double,
    ) = Measurement(
        id = MeasurementId("m-$at"),
        patientId = patientId,
        metric = Metric.WEIGHT,
        value = kg,
        unit = "kg",
        measuredAt = Instant.parse(at),
        source = MeasurementSource.MANUAL,
        externalId = null,
        note = null,
    )

    /**
     * Today (2026-05-31, `meal-1`/`meal-2`) is untouched: 840 kcal across two meals is what
     * `ConfirmToastTest`, `CadenceShellDataTest` and `TodayScreenTest` already pin. The other
     * six days carry the variety: an under-target day, a three-meal day, an over-target day,
     * the rest plain two-meal days — seven pairwise-distinct totals rather than six empty
     * days around one real one. Every `eatenAt` sits inside 03:00Z–20:00Z on purpose:
     * `CadenceMocks` filters by *local* date (Europe/Moscow, UTC+3, no DST), and a time
     * outside that band would cross midnight in one zone but not the other.
     * `MealSource.MANUAL` throughout is this seed's fixture value, not a path the port writes.
     */
    val meals =
        listOf(
            // 2026-05-25, Monday — under target: one meal, well under 1800 kcal.
            Meal(
                id = MealId("meal-3"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-25T06:30:00Z"),
                name = "Завтрак",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Творог с мёдом", 200, MacrosTenths(4150, 280, 380, 90))),
            ),
            // 2026-05-26, Tuesday — plain two-meal day.
            Meal(
                id = MealId("meal-4"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-26T05:30:00Z"),
                name = "Завтрак",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Омлет с овощами", 220, MacrosTenths(3400, 220, 120, 220))),
            ),
            Meal(
                id = MealId("meal-5"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-26T10:00:00Z"),
                name = "Обед",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Паста с индейкой", 350, MacrosTenths(6600, 380, 780, 180))),
            ),
            // 2026-05-27, Wednesday — three meals.
            Meal(
                id = MealId("meal-6"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-27T05:00:00Z"),
                name = "Завтрак",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Овсянка с бананом", 250, MacrosTenths(3000, 100, 520, 60))),
            ),
            Meal(
                id = MealId("meal-7"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-27T09:30:00Z"),
                name = "Обед",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Салат с курицей", 300, MacrosTenths(6000, 420, 300, 300))),
            ),
            Meal(
                id = MealId("meal-8"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-27T15:30:00Z"),
                name = "Ужин",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Рыба с рисом", 280, MacrosTenths(5000, 360, 480, 140))),
            ),
            // 2026-05-28, Thursday — over target: two large meals, 2100 kcal.
            Meal(
                id = MealId("meal-9"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-28T10:00:00Z"),
                name = "Обед",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Бургер домашний", 350, MacrosTenths(9000, 400, 700, 460))),
            ),
            Meal(
                id = MealId("meal-10"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-28T15:00:00Z"),
                name = "Ужин",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Паста Карбонара", 400, MacrosTenths(12000, 460, 1100, 620))),
            ),
            // 2026-05-29, Friday — plain two-meal day.
            Meal(
                id = MealId("meal-11"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-29T06:00:00Z"),
                name = "Завтрак",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Сырники", 220, MacrosTenths(7000, 320, 640, 260))),
            ),
            Meal(
                id = MealId("meal-12"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-29T11:00:00Z"),
                name = "Обед",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Плов с курицей", 350, MacrosTenths(8500, 400, 980, 260))),
            ),
            // 2026-05-30, Saturday — plain two-meal day.
            Meal(
                id = MealId("meal-13"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-30T06:30:00Z"),
                name = "Завтрак",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Гранола с йогуртом", 250, MacrosTenths(8000, 280, 900, 260))),
            ),
            Meal(
                id = MealId("meal-14"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-30T12:30:00Z"),
                name = "Обед",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Стейк с овощами", 320, MacrosTenths(9200, 520, 340, 480))),
            ),
            // 2026-05-31, Sunday — DEMO_NOW. Untouched: 840 kcal across two meals.
            Meal(
                id = MealId("meal-1"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-31T06:30:00Z"),
                name = "Завтрак",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Овсянка на воде", 250, MacrosTenths(3200, 120, 540, 60))),
            ),
            Meal(
                id = MealId("meal-2"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-31T10:00:00Z"),
                name = "Обед",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Куриная грудка с гречкой", 320, MacrosTenths(5200, 480, 460, 120))),
            ),
        )
}
