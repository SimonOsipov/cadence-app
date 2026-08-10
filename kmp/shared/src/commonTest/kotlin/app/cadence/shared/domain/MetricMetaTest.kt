package app.cadence.shared.domain

import app.cadence.shared.mock.MockSeed
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * The metric table, checked against the three things that can contradict it:
 * the prototype it was copied from, the seed it will be rendered beside, and
 * §03's set of metrics.
 *
 * That every metric *has* a row is not asserted here — `Metric.meta` is a
 * `when`, so the compiler owns it and a test would only restate the build.
 */
class MetricMetaTest {
    @Test
    fun theCopyIsTheTrendsPrototypesWhereThatModuleHasIt() {
        // `mobile/src/features/trends/data.ts` — label and eyebrow, taken as
        // written. Units are not on this list: the prototype writes «кг» and
        // this table holds the wire's «kg», which `theUnitIsTheOneTheReadingsAreStoredIn`
        // pins instead.
        assertEquals("Вес" to "Масса тела", Metric.WEIGHT.meta.label to Metric.WEIGHT.meta.eyebrow)
        assertEquals("HRV" to "Вариабельность сердца", Metric.HRV.meta.label to Metric.HRV.meta.eyebrow)
        assertEquals("ЧСС покоя" to "Пульс в покое", Metric.RHR.meta.label to Metric.RHR.meta.eyebrow)
        assertEquals("Сон" to "Качество сна", Metric.SLEEP.meta.label to Metric.SLEEP.meta.eyebrow)
        assertEquals("% жира" to "Состав тела", Metric.BODY_FAT.meta.label to Metric.BODY_FAT.meta.eyebrow)
        assertEquals("Талия" to "Обхват талии", Metric.WAIST.meta.label to Metric.WAIST.meta.eyebrow)
    }

    @Test
    fun theDecimalsAreThePrototypesAndTheyAreNotAllTheSame() {
        // A table that rounded everything to one place would draw «58,0 мс»
        // where the watch reported a whole number. Expected values are the
        // prototype's literals; only the key list is shared with the actual.
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
        // The trends module has no `hip` and no `chest`. The *body* screen does
        // — `mobile/src/features/body/data.ts:40-41` — and these two labels are
        // its own strings, not inventions.
        assertEquals("Бёдра", Metric.HIP.meta.label)
        assertEquals("Грудь", Metric.CHEST.meta.label)

        // The eyebrow is invented: the body screen has no such field. It
        // follows «Талия» / «Обхват талии» rather than the trends module's
        // thigh, whose label and eyebrow are one string.
        assertEquals("Обхват бёдер", Metric.HIP.meta.eyebrow)
        assertEquals("Обхват груди", Metric.CHEST.meta.eyebrow)

        // Absolute, not «same as WAIST»: the constants module will one day own
        // these fields, and a relative assertion would let all three move
        // together and stay green. The body screen says `dec: 0` for the tape
        // and the trends module says 1 for the waist — this table follows
        // trends, and that choice is what is pinned here.
        listOf(Metric.WAIST, Metric.HIP, Metric.CHEST).forEach { tape ->
            assertEquals("cm", tape.meta.unit, "${tape.code} is measured with a tape")
            assertEquals(1, tape.meta.decimals)
        }
    }

    @Test
    fun progressRunsUpwardForTwoMetricsAndDownwardForTheRest() {
        // «Прогресс» is not «выросло»: a table with one direction would call a
        // rising resting pulse an improvement. Both sides asserted — with two
        // constants they are equivalent today, and stop being so the moment a
        // third exists.
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
        // None of these comes from §03, so nothing upstream bounds them. The
        // set, not the count — the same guard `DomainTest` puts on the metric
        // and side-effect codes.
        assertEquals(setOf(MetricDirection.UP, MetricDirection.DOWN), MetricDirection.entries.toSet())
        assertEquals(setOf(MetricEntry.BY_HAND, MetricEntry.DEVICE_ONLY), MetricEntry.entries.toSet())
        assertEquals(setOf(MetricAccent.FOREST, MetricAccent.SAND), MetricAccent.entries.toSet())
    }

    @Test
    fun theUnitIsTheOneTheReadingsAreStoredIn() {
        // §03 records a `unit` per reading and the project rule leaves the
        // Russian to the surface, so this table holds «kg» and not «кг».
        // Checked against the seed rather than a second literal: a table and
        // the data it labels drifting apart is the failure this catches.
        //
        // `CHEST` is unmeasured on purpose and so has nothing to compare
        // against; `MeasurementSeedTest.everyMetricButChestHasBeenMeasured`
        // is what stops any other metric quietly joining it here.
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
        // Membership, not equality. `entry` answers «may a screen offer
        // «Добавить замер»», and `measurements.md` invariant 6 keeps manual
        // entry first-class even where a device also reports — §03 maps weight
        // and body fat 1:1 from HealthKit, so an imported reading beside the
        // typed ones is the expected state, not a contradiction. What BY_HAND
        // does require is that a hand can write: a metric marked BY_HAND with
        // no manual reading anywhere in the seed is a button that writes
        // nowhere.
        //
        // Asserting `sources == {MANUAL}` here instead would go red on the
        // first smart-scale weight and point at the table, whose one-keystroke
        // «fix» takes the keyboard away from the metric patients type most.
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
        // The tape is the one place equality *is* the rule, and it is not
        // `entry` that says so — `measurements.md` invariant 5: «Tape
        // measurements are manual only and in centimetres only.»
        listOf(Metric.WAIST, Metric.HIP).forEach { tape ->
            val sources =
                MockSeed.measurements
                    .filter { it.metric == tape }
                    .map { it.source }
                    .toSet()
            assertEquals(setOf(MeasurementSource.MANUAL), sources, "${tape.code} is manual only")
        }

        // The other direction is absolute: nothing a device alone produces may
        // carry a manual reading.
        Metric.entries.filter { it.meta.entry == MetricEntry.DEVICE_ONLY }.forEach { metric ->
            val manual = MockSeed.measurements.filter { it.metric == metric && it.source == MeasurementSource.MANUAL }
            assertEquals(emptyList(), manual, "${metric.code} is not typed in")
        }
    }

    @Test
    fun theMetricsThePatientTypesInAreThePrototypesEditableOnes() {
        // `mobile/src/features/body/data.ts:37-41` — `editable: true` on
        // weight, body fat, waist, hip and chest, and on nothing else. `bmi` is
        // there too and derived, which is why §03 has no such metric.
        assertEquals(
            setOf(Metric.WEIGHT, Metric.BODY_FAT, Metric.WAIST, Metric.HIP, Metric.CHEST),
            Metric.entries.filter { it.meta.entry == MetricEntry.BY_HAND }.toSet(),
        )
        // Sleep is the contestable one: the API scores it, and the seed calls
        // its `HEALTH_KIT` source a stand-in for a «derived» value that
        // `MeasurementSource` does not have. It is still not something a hand
        // writes.
        assertEquals(MetricEntry.DEVICE_ONLY, Metric.SLEEP.meta.entry)
    }

    @Test
    fun aCodeFromTheRouteFindsItsMetricAndAnUnknownOneDoesNot() {
        // `CadenceRoute.TrendDetail` carries a `String`, so this is the only
        // way back.
        assertEquals(Metric.WEIGHT, Metric.fromCode("weight"))
        assertEquals(Metric.BODY_FAT, Metric.fromCode("bodyfat"))
        // The prototype's seventh trend metric, which §03 does not have. A
        // screen opened on it says so rather than crashing.
        assertNull(Metric.fromCode("thigh"))
        // And the body screen's derived row, which is not a metric either.
        assertNull(Metric.fromCode("bmi"))
        assertNull(Metric.fromCode(""))
        assertNull(Metric.fromCode("WEIGHT"), "codes are the wire's lowercase, not the enum's name")
    }

    @Test
    fun everyCodeSurvivesTheRoundTrip() {
        // Also the only thing that would catch two metrics sharing a code:
        // `firstOrNull` would resolve the second to the first.
        assertEquals(Metric.entries, Metric.entries.map { Metric.fromCode(it.code) })
    }

    @Test
    fun aRowWithNothingToShowIsRefusedRatherThanRendered() {
        // The guard `TrendRange` set the precedent for. A blank label draws a
        // nameless card; a negative `decimals` indexes off the front of
        // `formatDecimal`'s scale sequence.
        val ok = Metric.WEIGHT.meta
        assertFailsWith<IllegalArgumentException> { ok.copy(label = " ") }
        assertFailsWith<IllegalArgumentException> { ok.copy(eyebrow = "") }
        assertFailsWith<IllegalArgumentException> { ok.copy(unit = "") }
        assertFailsWith<IllegalArgumentException> { ok.copy(decimals = -1) }
        // The upper bound is a sanity cap rather than something `formatDecimal`
        // imposes, and a cap nothing measures is a comment.
        assertFailsWith<IllegalArgumentException> { ok.copy(decimals = 5) }
        assertEquals(4, ok.copy(decimals = 4).decimals, "four places is still a table, not a typo")
    }
}
