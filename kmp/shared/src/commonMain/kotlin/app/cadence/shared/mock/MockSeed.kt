package app.cadence.shared.mock

import app.cadence.shared.domain.Compound
import app.cadence.shared.domain.CompoundId
import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.DoseEvent
import app.cadence.shared.domain.DoseEventId
import app.cadence.shared.domain.DoseUnit
import app.cadence.shared.domain.Macros
import app.cadence.shared.domain.Meal
import app.cadence.shared.domain.MealId
import app.cadence.shared.domain.MealItem
import app.cadence.shared.domain.MealSource
import app.cadence.shared.domain.Measurement
import app.cadence.shared.domain.MeasurementId
import app.cadence.shared.domain.MeasurementSource
import app.cadence.shared.domain.Metric
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
import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalDateTime
import kotlinx.datetime.LocalTime
import kotlinx.datetime.TimeZone
import kotlinx.datetime.toInstant
import kotlin.time.Instant

/**
 * One patient, mid-protocol.
 *
 * The values are the prototype's where the prototype has them — Семаглутид
 * titrating 0,25 → 0,5 → 1,0 over twelve weeks, BPC-157 twice a day, a target
 * of 1800 kcal. The *shapes* are §03's, which is the whole difference: the
 * prototype's frozen «31 May» is gone, replaced by a start date the cycle week
 * is computed from, and its two disconnected vial datasets are one list that
 * dose events subtract from.
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
     * The prototype's third strip row, «Глицин + магний · на ночь».
     *
     * A SUPPLEMENT in §03's terms: no phases, so no dose on the row — which is
     * what makes the strip's dose column meaningfully optional rather than
     * always present.
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
     * The last day the seeded history covers.
     *
     * The whole seed is written against Sunday 31 May 2026 — week four of the
     * cycle — and the history stops the day before, because the app has to open
     * on a day the patient has not logged yet: the hero offers «Записать →» and
     * `doseLoggedToday` is false.
     */
    val seededThrough = LocalDate(2026, 5, 30)

    /**
     * When each BPC vial took over.
     *
     * Two of them are open at once from the 25th, which is not untidiness: §03
     * allows it, the prototype's own `VIALS` contains it, and it is the only
     * shape in which «which vial did this come out of» is a question with more
     * than one answer — so it is the shape the picker exists for.
     */
    private val BPC_SPARE_OPENED = LocalDate(2026, 5, 22)
    private val BPC_LOW_OPENED = LocalDate(2026, 5, 25)

    /**
     * Five vials, and every remaining count is a subtraction.
     *
     * §03's third correction says `remaining = total_doses − count(dose_events
     * .vial_id)`, and [Vial] has no field to store one in — so the only way a
     * count can be wrong here is for the history behind it to be missing. That
     * is what [history] is: not decoration, but the other half of every number
     * this cabinet shows.
     *
     * Semaglutide is one vial with no spare, three of its four doses taken. The
     * reorder hint fires because the patient is nearly out — the state this
     * replaced was four doses left in week four of a weekly protocol with
     * nothing ever logged, which is the incoherent one.
     *
     * The semaglutide vial sits last on purpose: a reader that answers
     * «the patient's vial» with `vials.first()` is right only by the order of
     * a list, and a list's order is not a fact about a cabinet.
     *
     * BPC-157 carries the other four statuses: one disposed when it ran out,
     * one low, one sealed and expiring, one sealed. No compound is invented for
     * the sake of a status the patient's protocol does not prescribe.
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
                // Thrown out on the 22nd with six doses still in it — a vial is
                // disposed of for reasons other than running dry, and one that
                // still had doses is the only case in which drawing from a
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
     * What the patient has already done — every slot from the cycle's start to
     * [seededThrough].
     *
     * Generated rather than typed, for two reasons. Forty-five hand-written
     * rows is forty-five chances to disagree with the schedule they are
     * supposed to satisfy; and the zones come from [suggestNextSite], so the
     * history is an example of the rotation rule the app follows rather than a
     * second opinion about it. A seeded zone list would drift from the rule the
     * first time the rule changed.
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
                            // The rotation, walked. Each event asks the same
                            // function the wizard will ask, against everything
                            // logged before it.
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

    /** §03: «kcal 1800 · protein 140 · carbs 200 · fat 60». */
    val targets = Macros(kcal = 1800, proteinG = 140, carbsG = 200, fatG = 60)

    /**
     * Seven weekly weights and one HRV.
     *
     * Seven because §11 asks for a «7-pt series per metric»; weekly because the
     * protocol weighs in weekly. The HRV sits later than the last weight on
     * purpose, so «latest weight» cannot be «latest reading» and pass.
     */
    val measurements =
        listOf(
            // Eight readings for a seven-point series, so `take` and `takeLast`
            // cannot agree; and one of them out of list order, so the sort has
            // something to do. Both mutations survived a seed that was already
            // sorted and exactly seven long.
            weight("2026-04-26T06:00:00Z", 100.8),
            weight("2026-04-12T06:00:00Z", 101.9),
            weight("2026-04-19T06:00:00Z", 101.2),
            weight("2026-05-03T06:00:00Z", 100.1),
            weight("2026-05-10T06:00:00Z", 99.9),
            weight("2026-05-17T06:00:00Z", 99.2),
            weight("2026-05-24T06:00:00Z", 98.8),
            weight("2026-05-31T06:00:00Z", 98.4),
            Measurement(
                id = MeasurementId("m-hrv"),
                patientId = patientId,
                metric = Metric.HRV,
                value = 58.0,
                unit = "ms",
                measuredAt = Instant.parse("2026-05-31T06:05:00Z"),
                source = MeasurementSource.HEALTH_KIT,
                externalId = null,
                note = null,
            ),
        )

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

    val meals =
        listOf(
            Meal(
                id = MealId("meal-1"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-31T06:30:00Z"),
                name = "Завтрак",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Овсянка на воде", 250, Macros(320, 12, 54, 6))),
            ),
            Meal(
                id = MealId("meal-2"),
                patientId = patientId,
                eatenAt = Instant.parse("2026-05-31T10:00:00Z"),
                name = "Обед",
                source = MealSource.MANUAL,
                recipeId = null,
                items = listOf(MealItem("Куриная грудка с гречкой", 320, Macros(520, 48, 46, 12))),
            ),
        )
}
