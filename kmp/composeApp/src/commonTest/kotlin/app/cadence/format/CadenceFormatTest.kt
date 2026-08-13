package app.cadence.format

import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.DoseUnit
import app.cadence.shared.domain.Measurement
import app.cadence.shared.domain.MeasurementId
import app.cadence.shared.domain.MeasurementSource
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.Protocol
import app.cadence.shared.domain.ProtocolId
import app.cadence.shared.domain.ProtocolPlan
import app.cadence.shared.domain.ProtocolStatus
import app.cadence.shared.domain.TrendWindow
import app.cadence.shared.domain.UserId
import app.cadence.shared.domain.rangeOn
import app.cadence.shared.domain.trendSeries
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalDateTime
import kotlinx.datetime.LocalTime
import kotlinx.datetime.TimeZone
import kotlinx.datetime.toInstant
import kotlin.test.Test
import kotlin.test.assertEquals

class CadenceFormatTest {
    @Test
    fun integersGroupInThreesWithARussianNonBreakingSpace() {
        // `toLocaleString('ru-RU')` in the prototype. The separator is U+00A0,
        // not a plain space: a kcal count must not wrap between its thousands
        // and its hundreds.
        assertEquals("0", formatInteger(0))
        assertEquals("999", formatInteger(999))
        assertEquals("1\u00A0000", formatInteger(1000))
        assertEquals("1\u00A0240", formatInteger(1240))
        assertEquals("12\u00A0345", formatInteger(12345))
        assertEquals("1\u00A0234\u00A0567", formatInteger(1234567))
    }

    @Test
    fun negativeIntegersKeepTheirSignOutsideTheGrouping() {
        // No call site passes one today; the guard exists because the obvious
        // implementation groups the minus sign into the first triple, and
        // whether that renders right is then a matter of digit count.
        assertEquals("-1\u00A0234", formatInteger(-1234))
        assertEquals("-999", formatInteger(-999))
        assertEquals("-100", formatInteger(-100))
    }

    @Test
    fun theExtremesDoNotWrapAround() {
        // -Int.MIN_VALUE is Int.MIN_VALUE. An implementation that negates in
        // Int returns the value unchanged and formats a very negative number
        // as a very positive one.
        assertEquals("2\u00A0147\u00A0483\u00A0647", formatInteger(Int.MAX_VALUE))
        assertEquals("-2\u00A0147\u00A0483\u00A0648", formatInteger(Int.MIN_VALUE))
    }

    @Test
    fun aDecimalUsesTheCommaAndKeepsItsTrailingZero() {
        // «98,4», «110,0» — the comma is the locale's, and a weight that
        // dropped its trailing zero would jump between one and two glyphs as
        // the patient loses weight.
        assertEquals("98,4", formatDecimal(98.4, digits = 1))
        assertEquals("110,0", formatDecimal(110.0, digits = 1))
        assertEquals("0,25", formatDecimal(0.25, digits = 2))
        assertEquals("1\u00A0240,5", formatDecimal(1240.5, digits = 1))
        assertEquals("-0,6", formatDecimal(-0.6, digits = 1))
    }

    @Test
    fun aDecimalRoundsRatherThanTruncates() {
        assertEquals("0,3", formatDecimal(0.25, digits = 1))
        assertEquals("99,0", formatDecimal(98.96, digits = 1))
    }

    @Test
    fun aDoseIsItsValueAndItsUnitWithNoTrailingZeroes() {
        // «0,25 мг», «250 мкг», «1 мг» — the protocol's own numbers, and a
        // dose of 1,0 mg reads «1 мг» the way a clinician says it.
        assertEquals("0,25" to "мг", formatDose(Dose(0.25, DoseUnit.MG)))
        assertEquals("0,5" to "мг", formatDose(Dose(0.5, DoseUnit.MG)))
        assertEquals("1" to "мг", formatDose(Dose(1.0, DoseUnit.MG)))
        assertEquals("250" to "мкг", formatDose(Dose(250.0, DoseUnit.MCG)))
    }

    @Test
    fun aKcalValueCarriesItsOwnUnit() {
        assertEquals("960 ккал", formatKcal(960))
        assertEquals("1 800 ккал", formatKcal(1800))
        assertEquals("0 ккал", formatKcal(0))
    }

    @Test
    fun aGramValueCarriesItsOwnUnit() {
        assertEquals("80 г", formatGrams(80))
        assertEquals("0 г", formatGrams(0))
    }

    @Test
    fun aTenthsKcalFieldRoundsForDisplayLikeToMacrosDoes() {
        // 295,2 kcal — the same round-half-up boundary `MacrosTenths.toMacros`
        // uses, reached here without a second `Macros` conversion.
        assertEquals("295 ккал", formatKcalTenths(2952))
        // The half-up tie: 240,5 rounds up, not to even.
        assertEquals("241 ккал", formatKcalTenths(2405))
        assertEquals("0 ккал", formatKcalTenths(0))
    }

    @Test
    fun aDeltaNeedsTwoReadingsAndSaysWhichWayItWent() {
        // One reading is where every patient starts, and points[size - 2] on a
        // one-point list throws. The guard was written twice inline and
        // measured by nothing.
        assertEquals(null, formatDeltaSincePrevious(emptyList(), "кг"))
        assertEquals(null, formatDeltaSincePrevious(listOf(98.4), "кг"))
        assertEquals("↓ 0,4 кг", formatDeltaSincePrevious(listOf(98.8, 98.4), "кг"))
        assertEquals("↑ 0,6 кг", formatDeltaSincePrevious(listOf(98.4, 99.0), "кг"))
        // A plateau is not a gain. Rendering two identical readings as «↑ 0,0»
        // is a claim the data does not make.
        assertEquals("→ 0,0 кг", formatDeltaSincePrevious(listOf(98.4, 98.4), "кг"))
        // Only the *last pair* counts, whatever came before it — which is the
        // whole difference from a window's delta, where the first reading is
        // the base. Over this list a window would answer «↓ 2,8 кг».
        assertEquals("↓ 0,4 кг", formatDeltaSincePrevious(listOf(101.2, 99.0, 98.8, 98.4), "кг"))
    }

    @Test
    fun aDeltaThatWasAlreadyComputedIsRenderedWithoutBeingRecomputed() {
        // What the trends screens will hand over: `TrendSeries.delta` is a
        // number taken across a window, and there is no pair of points left for
        // a formatter to subtract. A second arrow-and-magnitude rule written
        // beside this one is how the two deltas start disagreeing about what
        // «↓» means.
        assertEquals("↓ 2,8 кг", formatSignedDelta(-2.8, "кг"))
        assertEquals("↑ 13 мс", formatSignedDelta(13.0, "мс", digits = 0))
        assertEquals("→ 0,0 кг", formatSignedDelta(0.0, "кг"))
    }

    @Test
    fun aDeltaTooSmallToPrintIsNotAnArrow() {
        // A window delta is an arbitrary subtraction — imported weights do not
        // land on tenths — so 98,79 − 98,75 reaches this function. «↓ 0,0 кг»
        // beside a true plateau's «→ 0,0 кг» would be two glyphs for one
        // printed number.
        assertEquals("→ 0,0 кг", formatSignedDelta(-0.04, "кг"))
        assertEquals("→ 0,0 кг", formatSignedDelta(0.04, "кг"))
        assertEquals("→ 0 мс", formatSignedDelta(0.4, "мс", digits = 0))
        // The threshold is where the rendering rounds, not somewhere of its own.
        assertEquals("↑ 0,1 кг", formatSignedDelta(0.05, "кг"))
    }

    @Test
    fun theDigitsSurviveTheHopFromOneFormatterToTheOther() {
        // `formatDeltaSincePrevious` delegates, and a delegation that dropped
        // its third argument would render an HRV move as «↓ 3,0 мс» while
        // every other assertion here stayed green on the default.
        assertEquals("↓ 3 мс", formatDeltaSincePrevious(listOf(60.0, 57.0), "мс", digits = 0))
    }

    @Test
    fun theTwoDeltasAnswerDifferentQuestionsAboutOneSetOfReadings() {
        // The claim both KDocs make, joined here rather than asserted in halves:
        // over these four readings «Сегодня» says −0,4 and a window says −2,8.
        val values = listOf(101.2, 99.0, 98.8, 98.4)
        val days =
            listOf(LocalDate(2026, 5, 25), LocalDate(2026, 5, 27), LocalDate(2026, 5, 29), LocalDate(2026, 5, 31))
        val today = LocalDate(2026, 5, 31)
        val plan =
            ProtocolPlan(
                protocol =
                    Protocol(
                        ProtocolId("pr"),
                        UserId("p"),
                        LocalDate(2026, 5, 10),
                        12,
                        ProtocolStatus.ACTIVE,
                        null,
                        null,
                    ),
                items = emptyList(),
                phases = emptyMap(),
            )
        val readings =
            values.zip(days) { value, day ->
                Measurement(
                    id = MeasurementId("m-$day"),
                    patientId = UserId("p"),
                    metric = Metric.WEIGHT,
                    value = value,
                    unit = "kg",
                    measuredAt = LocalDateTime(day, LocalTime(6, 0)).toInstant(TimeZone.UTC),
                    source = MeasurementSource.MANUAL,
                    externalId = null,
                    note = null,
                )
            }

        val window = requireNotNull(TrendWindow.WEEK.rangeOn(plan, today))
        val series = trendSeries(readings, Metric.WEIGHT, window, TimeZone.UTC)

        assertEquals("↓ 0,4 кг", formatDeltaSincePrevious(values, "кг"))
        assertEquals("↓ 2,8 кг", formatSignedDelta(requireNotNull(series.delta), "кг"))
    }

    @Test
    fun everyUnitTheAppStoresHasARussianForm() {
        assertEquals("кг", unitRu("kg"))
        assertEquals("мс", unitRu("ms"))
        assertEquals("уд/мин", unitRu("bpm"))
        assertEquals("см", unitRu("cm"))
        // An unknown unit passes through: a number with no unit beside it is
        // worse than one with an unfamiliar unit.
        assertEquals("%", unitRu("%"))
    }
}
