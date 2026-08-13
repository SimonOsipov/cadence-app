package app.cadence.shared.domain

import app.cadence.shared.mock.MockSeed
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * Checked against the three things that can contradict it: the prototype it was copied
 * from, the seed it's rendered beside, and §03's set of metrics. That every metric *has* a
 * row isn't asserted — `Metric.meta` is a `when`, so the compiler owns that.
 */
class MetricMetaTest {
    @Test
    fun theCopyIsTheTrendsPrototypesWhereThatModuleHasIt() {
        // Units aren't on this list: the prototype writes «кг», this table holds the wire's
        // «kg» — pinned in theUnitIsTheOneTheReadingsAreStoredIn instead.
        assertEquals("Вес" to "Масса тела", Metric.WEIGHT.meta.label to Metric.WEIGHT.meta.eyebrow)
        assertEquals("HRV" to "Вариабельность сердца", Metric.HRV.meta.label to Metric.HRV.meta.eyebrow)
        assertEquals("ЧСС покоя" to "Пульс в покое", Metric.RHR.meta.label to Metric.RHR.meta.eyebrow)
        assertEquals("Сон" to "Качество сна", Metric.SLEEP.meta.label to Metric.SLEEP.meta.eyebrow)
        assertEquals("% жира" to "Состав тела", Metric.BODY_FAT.meta.label to Metric.BODY_FAT.meta.eyebrow)
        assertEquals("Талия" to "Обхват талии", Metric.WAIST.meta.label to Metric.WAIST.meta.eyebrow)
    }

    @Test
    fun theDecimalsAreThePrototypesAndTheyAreNotAllTheSame() {
        // Rounding everything to one place would draw «58,0 мс» where the watch reported a
        // whole number.
        assertEquals(
            mapOf(
                Metric.WEIGHT to 1,
                Metric.HRV to 0,
                Metric.RHR to 0,
                Metric.SLEEP to 0,
                Metric.BODY_FAT to 1,
                Metric.WAIST to 1,
            ),
            listOf(Metric.WEIGHT, Metric.HRV, Metric.RHR, Metric.SLEEP, Metric.BODY_FAT, Metric.WAIST)
                .associateWith { it.meta.decimals },
        )
    }

    @Test
    fun theTapeTheTrendsModuleNeverDrewStillCarriesItsOwnLabel() {
        // The trends module has no `hip` or `chest`; the *body* screen does, and these two
        // labels are its own strings, not inventions.
        assertEquals("Бёдра", Metric.HIP.meta.label)
        assertEquals("Грудь", Metric.CHEST.meta.label)

        // Invented: the body screen has no eyebrow field. Follows «Талия»/«Обхват талии»,
        // not the trends module's thigh (whose label and eyebrow are one string).
        assertEquals("Обхват бёдер", Metric.HIP.meta.eyebrow)
        assertEquals("Обхват груди", Metric.CHEST.meta.eyebrow)

        // Absolute, not "same as WAIST": a relative assertion would let all three move
        // together and stay green. Body screen says dec:0 for the tape, trends says 1 for
        // waist — this table follows trends, pinned here.
        listOf(Metric.WAIST, Metric.HIP, Metric.CHEST).forEach { tape ->
            assertEquals("cm", tape.meta.unit, "${tape.code} is measured with a tape")
            assertEquals(1, tape.meta.decimals)
        }
    }

    @Test
    fun progressRunsUpwardForTwoMetricsAndDownwardForTheRest() {
        // Both sides asserted — with two constants they're equivalent today, and stop being
        // so the moment a third exists.
        assertEquals(
            setOf(Metric.HRV, Metric.SLEEP),
            Metric.entries.filter { it.meta.direction == MetricDirection.UP }.toSet(),
        )
        assertEquals(
            Metric.entries.toSet() - setOf(Metric.HRV, Metric.SLEEP),
            Metric.entries.filter { it.meta.direction == MetricDirection.DOWN }.toSet(),
        )
    }

    @Test
    fun sleepIsTheOnlyMetricThatLeavesTheForestPalette() {
        assertEquals(
            setOf(Metric.SLEEP),
            Metric.entries.filter { it.meta.accent == MetricAccent.SAND }.toSet(),
        )
        assertEquals(
            Metric.entries.toSet() - setOf(Metric.SLEEP),
            Metric.entries.filter { it.meta.accent == MetricAccent.FOREST }.toSet(),
        )
    }

    @Test
    fun theThreeTableEnumsAreTheValuesTheTableActuallyUses() {
        // None of these comes from §03; the set, not the count, same guard as `DomainTest`.
        assertEquals(setOf(MetricDirection.UP, MetricDirection.DOWN), MetricDirection.entries.toSet())
        assertEquals(setOf(MetricEntry.BY_HAND, MetricEntry.DEVICE_ONLY), MetricEntry.entries.toSet())
        assertEquals(setOf(MetricAccent.FOREST, MetricAccent.SAND), MetricAccent.entries.toSet())
    }

    @Test
    fun theUnitIsTheOneTheReadingsAreStoredIn() {
        // Checked against the seed, not a second literal: a table and the data it labels
        // drifting apart is the failure this catches. CHEST is unmeasured on purpose;
        // MeasurementSeedTest.everyMetricButChestHasBeenMeasured stops others joining it here.
        Metric.entries.forEach { metric ->
            val stored =
                MockSeed.measurements
                    .filter { it.metric == metric }
                    .map { it.unit }
                    .toSet()
            if (metric != Metric.CHEST) {
                assertEquals(setOf(metric.meta.unit), stored, "${metric.code}'s table unit and its readings")
            }
        }
        assertEquals("kg", Metric.WEIGHT.meta.unit)
        assertEquals("/100", Metric.SLEEP.meta.unit)
    }

    @Test
    fun aMetricThePatientTypesInHasReadingsSomebodyTypedIn() {
        // Membership, not equality: measurements.md invariant 6 keeps manual entry
        // first-class even where a device also reports, so an imported reading beside typed
        // ones is expected. Asserting `sources == {MANUAL}` would go red on the first
        // smart-scale weight and point at the table, taking the keyboard away from patients.
        Metric.entries.filter { it.meta.entry == MetricEntry.BY_HAND }.forEach { metric ->
            val sources =
                MockSeed.measurements
                    .filter { it.metric == metric }
                    .map { it.source }
                    .toSet()
            if (metric != Metric.CHEST) {
                assertTrue(MeasurementSource.MANUAL in sources, "${metric.code} is typed in")
            }
        }
        // The tape is where equality *is* the rule: measurements.md invariant 5, «Tape
        // measurements are manual only and in centimetres only.»
        listOf(Metric.WAIST, Metric.HIP).forEach { tape ->
            val sources =
                MockSeed.measurements
                    .filter { it.metric == tape }
                    .map { it.source }
                    .toSet()
            assertEquals(setOf(MeasurementSource.MANUAL), sources, "${tape.code} is manual only")
        }

        // The other direction is absolute.
        Metric.entries.filter { it.meta.entry == MetricEntry.DEVICE_ONLY }.forEach { metric ->
            val manual = MockSeed.measurements.filter { it.metric == metric && it.source == MeasurementSource.MANUAL }
            assertEquals(emptyList(), manual, "${metric.code} is not typed in")
        }
    }

    @Test
    fun theMetricsThePatientTypesInAreThePrototypesEditableOnes() {
        assertEquals(
            setOf(Metric.WEIGHT, Metric.BODY_FAT, Metric.WAIST, Metric.HIP, Metric.CHEST),
            Metric.entries.filter { it.meta.entry == MetricEntry.BY_HAND }.toSet(),
        )
        // Sleep is the contestable one: the API scores it, but it's still not something a
        // hand writes.
        assertEquals(MetricEntry.DEVICE_ONLY, Metric.SLEEP.meta.entry)
    }

    @Test
    fun aCodeFromTheRouteFindsItsMetricAndAnUnknownOneDoesNot() {
        assertEquals(Metric.WEIGHT, Metric.fromCode("weight"))
        assertEquals(Metric.BODY_FAT, Metric.fromCode("bodyfat"))
        // The prototype's seventh trend metric, which §03 doesn't have.
        assertNull(Metric.fromCode("thigh"))
        // The body screen's derived row, not a metric either.
        assertNull(Metric.fromCode("bmi"))
        assertNull(Metric.fromCode(""))
        assertNull(Metric.fromCode("WEIGHT"), "codes are the wire's lowercase, not the enum's name")
    }

    @Test
    fun everyCodeSurvivesTheRoundTrip() {
        // Also catches two metrics sharing a code: `firstOrNull` would resolve the second to the first.
        assertEquals(Metric.entries, Metric.entries.map { Metric.fromCode(it.code) })
    }

    @Test
    fun aRowWithNothingToShowIsRefusedRatherThanRendered() {
        val ok = Metric.WEIGHT.meta
        assertFailsWith<IllegalArgumentException> { ok.copy(label = " ") }
        assertFailsWith<IllegalArgumentException> { ok.copy(eyebrow = "") }
        assertFailsWith<IllegalArgumentException> { ok.copy(unit = "") }
        assertFailsWith<IllegalArgumentException> { ok.copy(decimals = -1) }
        // Sanity cap, not `formatDecimal`'s own limit.
        assertFailsWith<IllegalArgumentException> { ok.copy(decimals = 5) }
        assertEquals(4, ok.copy(decimals = 4).decimals, "four places is still a table, not a typo")
    }
}
