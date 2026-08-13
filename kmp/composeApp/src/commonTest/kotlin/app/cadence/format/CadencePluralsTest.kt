package app.cadence.format

import kotlin.test.Test
import kotlin.test.assertEquals

class CadencePluralsTest {
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
    fun itemsTakeTheSameRuleAsMealsAndWeeks() {
        // `NutritionMealFeed.kt` and `LogMealItemsList.kt` both call
        // `russianPlural(count, "позиция", "позиции", "позиций")` inline;
        // `pluralItems` is the named version `TodayMeals` reads.
        assertEquals("позиция", pluralItems(1))
        assertEquals("позиции", pluralItems(2))
        assertEquals("позиции", pluralItems(4))
        assertEquals("позиций", pluralItems(5))
        assertEquals("позиций", pluralItems(11))
        assertEquals("позиция", pluralItems(21))
    }
}
