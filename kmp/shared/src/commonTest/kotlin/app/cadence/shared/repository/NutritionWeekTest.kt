package app.cadence.shared.repository

import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

private fun weekDay(daysAgo: Int) = NutritionWeekDay(date = LocalDate(2026, 5, 31 - daysAgo), kcal = 900, proteinG = 90)

class NutritionWeekTest {
    /**
     * The guard `TrendRange`/`MetricMeta` set the precedent for: before this `init` existed,
     * "a week always carries seven days" was a comment on `NutritionScreen.kt`'s
     * `weekProteinAverage`, not enforced — a short or empty `days` list reached that division
     * and threw `IllegalArgumentException: Cannot round NaN value` instead of failing at the
     * boundary that actually built the malformed value.
     */
    @Test
    fun aWeekShorterOrLongerThanSevenDaysIsRefusedRatherThanBuilt() {
        assertFailsWith<IllegalArgumentException> { NutritionWeek(days = emptyList()) }
        assertFailsWith<IllegalArgumentException> { NutritionWeek(days = (0..5).map { weekDay(it) }) }
        assertFailsWith<IllegalArgumentException> { NutritionWeek(days = (0..7).map { weekDay(it) }) }

        val ok = NutritionWeek(days = (0..6).map { weekDay(it) })
        assertEquals(7, ok.days.size, "seven days is still a week, not a typo")
    }
}
