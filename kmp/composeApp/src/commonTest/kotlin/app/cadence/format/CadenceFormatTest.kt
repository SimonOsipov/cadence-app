package app.cadence.format

import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.DoseUnit
import app.cadence.shared.domain.MacrosTenths
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
import app.cadence.shared.domain.toMacros
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
        // `toLocaleString('ru-RU')` in the prototype. U+00A0, not a plain space: a kcal count
        // must not wrap between its thousands and hundreds.
        assertEquals("0", formatInteger(0))
        assertEquals("999", formatInteger(999))
        assertEquals("1\u00A0000", formatInteger(1000))
        assertEquals("1\u00A0240", formatInteger(1240))
        assertEquals("12\u00A0345", formatInteger(12345))
        assertEquals("1\u00A0234\u00A0567", formatInteger(1234567))
    }

    @Test
    fun negativeIntegersKeepTheirSignOutsideTheGrouping() {
        // No call site passes a negative today; guards the obvious-but-wrong implementation that
        // groups the minus sign into the first triple.
        assertEquals("-1\u00A0234", formatInteger(-1234))
        assertEquals("-999", formatInteger(-999))
        assertEquals("-100", formatInteger(-100))
    }

    @Test
    fun theExtremesDoNotWrapAround() {
        // -Int.MIN_VALUE is Int.MIN_VALUE: negating in Int would format a very negative number
        // as a very positive one.
        assertEquals("2\u00A0147\u00A0483\u00A0647", formatInteger(Int.MAX_VALUE))
        assertEquals("-2\u00A0147\u00A0483\u00A0648", formatInteger(Int.MIN_VALUE))
    }

    @Test
    fun aDecimalUsesTheCommaAndKeepsItsTrailingZero() {
        // A weight that dropped its trailing zero would jump between one and two glyphs as the
        // patient loses weight.
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
        // The protocol's own numbers; 1,0 mg reads «1 мг» the way a clinician says it.
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
        // Checked against production `toMacros()` directly, not literals that merely happen to
        // agree today, so the two rounding paths cannot drift apart unnoticed.
        listOf(2952, 2405, 0, 9, 15).forEach { tenths ->
            val viaToMacros = formatKcal(MacrosTenths(tenths, 0, 0, 0).toMacros().kcal)
            assertEquals(viaToMacros, formatKcalTenths(tenths), "tenths=$tenths")
        }
        // Pinned literals too, so the table itself stays legible.
        assertEquals("295 ккал", formatKcalTenths(2952))
        // The half-up tie: 240,5 rounds up, not to even.
        assertEquals("241 ккал", formatKcalTenths(2405))
        assertEquals("0 ккал", formatKcalTenths(0))
    }

    @Test
    fun aDeltaNeedsTwoReadingsAndSaysWhichWayItWent() {
        // One reading is where every patient starts, and points[size - 2] on a one-point list
        // throws — previously unmeasured.
        assertEquals(null, formatDeltaSincePrevious(emptyList(), "кг"))
        assertEquals(null, formatDeltaSincePrevious(listOf(98.4), "кг"))
        assertEquals("↓ 0,4 кг", formatDeltaSincePrevious(listOf(98.8, 98.4), "кг"))
        assertEquals("↑ 0,6 кг", formatDeltaSincePrevious(listOf(98.4, 99.0), "кг"))
        // A plateau is not a gain: two identical readings must not render «↑ 0,0».
        assertEquals("→ 0,0 кг", formatDeltaSincePrevious(listOf(98.4, 98.4), "кг"))
        // Only the *last pair* counts, unlike a window's delta (base = first reading) — over
        // this list a window would answer «↓ 2,8 кг».
        assertEquals("↓ 0,4 кг", formatDeltaSincePrevious(listOf(101.2, 99.0, 98.8, 98.4), "кг"))
    }

    @Test
    fun aDeltaThatWasAlreadyComputedIsRenderedWithoutBeingRecomputed() {
        // What trends screens hand over: `TrendSeries.delta` is already a number, with no pair of
        // points left to subtract — a second arrow-and-magnitude rule here would let the two
        // deltas disagree about what «↓» means.
        assertEquals("↓ 2,8 кг", formatSignedDelta(-2.8, "кг"))
        assertEquals("↑ 13 мс", formatSignedDelta(13.0, "мс", digits = 0))
        assertEquals("→ 0,0 кг", formatSignedDelta(0.0, "кг"))
    }

    @Test
    fun aDeltaTooSmallToPrintIsNotAnArrow() {
        // Imported weights do not land on tenths, so a window delta like 98,79 − 98,75 reaches
        // this; «↓ 0,0 кг» beside a true plateau's «→ 0,0 кг» would be two glyphs for one number.
        assertEquals("→ 0,0 кг", formatSignedDelta(-0.04, "кг"))
        assertEquals("→ 0,0 кг", formatSignedDelta(0.04, "кг"))
        assertEquals("→ 0 мс", formatSignedDelta(0.4, "мс", digits = 0))
        // The threshold is where the rendering rounds, not somewhere of its own.
        assertEquals("↑ 0,1 кг", formatSignedDelta(0.05, "кг"))
    }

    @Test
    fun theDigitsSurviveTheHopFromOneFormatterToTheOther() {
        // A delegation that dropped the third argument would render an HRV move as «↓ 3,0 мс»
        // while every other assertion here stayed green on the default.
        assertEquals("↓ 3 мс", formatDeltaSincePrevious(listOf(60.0, 57.0), "мс", digits = 0))
    }

    @Test
    fun theTwoDeltasAnswerDifferentQuestionsAboutOneSetOfReadings() {
        // Joined here rather than asserted in halves: over these four readings «Сегодня» says
        // −0,4 and a window says −2,8.
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
        // An unknown unit passes through: a number with no unit is worse than an unfamiliar one.
        assertEquals("%", unitRu("%"))
    }
}
