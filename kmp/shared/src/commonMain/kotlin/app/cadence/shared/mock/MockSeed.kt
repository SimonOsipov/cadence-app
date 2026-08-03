package app.cadence.shared.mock

import app.cadence.shared.domain.Compound
import app.cadence.shared.domain.CompoundId
import app.cadence.shared.domain.Dose
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
import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime
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
     * One open vial and nothing behind it — which is what makes the reorder
     * hint fire, and the prototype's «0 sealed spares» state.
     */
    val vials =
        listOf(
            Vial(
                id = VialId("vial-1"),
                patientId = patientId,
                compoundId = semaglutide.id,
                concentrationLabel = "1 мг/мл",
                // Four weekly doses and nothing behind it: the prototype's
                // «0 sealed spares, running low» state, which is what makes the
                // reorder hint fire at all.
                totalDoses = 4,
                openedAt = LocalDate(2026, 5, 10),
                expiresOn = LocalDate(2026, 9, 1),
                lot = "A-2261",
                locationRu = "Холодильник, полка 2",
                disposedAt = null,
                labelPhotoPath = null,
            ),
        )

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

    val measurements =
        listOf(
            Measurement(
                id = MeasurementId("m-1"),
                patientId = patientId,
                metric = Metric.WEIGHT,
                value = 98.4,
                unit = "kg",
                measuredAt = Instant.parse("2026-05-31T06:00:00Z"),
                source = MeasurementSource.MANUAL,
                externalId = null,
                note = null,
            ),
            // An *earlier* weight, sitting later in the list, so «latest» cannot
            // be «last in the list» and pass by position.
            Measurement(
                id = MeasurementId("m-0"),
                patientId = patientId,
                metric = Metric.WEIGHT,
                value = 99.6,
                unit = "kg",
                measuredAt = Instant.parse("2026-05-24T06:00:00Z"),
                source = MeasurementSource.MANUAL,
                externalId = null,
                note = null,
            ),
            // A later reading of another metric, so «latest weight» cannot be
            // «last of any metric» either.
            Measurement(
                id = MeasurementId("m-2"),
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
