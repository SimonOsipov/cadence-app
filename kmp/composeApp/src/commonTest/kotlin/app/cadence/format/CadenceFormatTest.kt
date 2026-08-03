package app.cadence.format

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
}
