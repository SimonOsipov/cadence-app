package app.cadence.format

import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.DoseUnit
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
    fun weeksTakeTheSameRuleAsMeals() {
        // The prototype's reorder card repeats the meal card's approximation —
        // `weeksLeft < 5 ? 'недели' : 'недель'` — and is wrong from 21 up in
        // the same way. One rule, two nouns.
        assertEquals("неделю", pluralWeeks(1))
        assertEquals("недели", pluralWeeks(3))
        assertEquals("недель", pluralWeeks(5))
        assertEquals("недель", pluralWeeks(11))
        assertEquals("неделю", pluralWeeks(21))
        assertEquals("недели", pluralWeeks(22))
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
    fun mealsTakeTheRussianPluralAndNotThePrototypesApproximationOfIt() {
        assertEquals("приём", pluralMeals(1))
        assertEquals("приёма", pluralMeals(2))
        assertEquals("приёма", pluralMeals(4))
        assertEquals("приёмов", pluralMeals(5))
        assertEquals("приёмов", pluralMeals(11))
        assertEquals("приёмов", pluralMeals(14))
        // Exactly where the prototype's `count < 5 ? 'приёма' : 'приёмов'` is
        // wrong.
        assertEquals("приём", pluralMeals(21))
        assertEquals("приёма", pluralMeals(22))
        assertEquals("приёмов", pluralMeals(25))
        assertEquals("приём", pluralMeals(101))
        assertEquals("приёмов", pluralMeals(111))
        // Never rendered — the sheet takes its zero-state branch — but a
        // plural function that is wrong at zero is wrong.
        assertEquals("приёмов", pluralMeals(0))
    }

    @Test
    fun aDeltaNeedsTwoReadingsAndSaysWhichWayItWent() {
        // One reading is where every patient starts, and points[size - 2] on a
        // one-point list throws. The guard was written twice inline and
        // measured by nothing.
        assertEquals(null, formatDelta(emptyList(), "кг"))
        assertEquals(null, formatDelta(listOf(98.4), "кг"))
        assertEquals("↓ 0,4 кг", formatDelta(listOf(98.8, 98.4), "кг"))
        assertEquals("↑ 0,6 кг", formatDelta(listOf(98.4, 99.0), "кг"))
        // A plateau is not a gain. Rendering two identical readings as «↑ 0,0»
        // is a claim the data does not make.
        assertEquals("→ 0,0 кг", formatDelta(listOf(98.4, 98.4), "кг"))
        // Only the last pair counts, whatever came before it.
        assertEquals("↓ 0,4 кг", formatDelta(listOf(101.2, 99.0, 98.8, 98.4), "кг"))
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
