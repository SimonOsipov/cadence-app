package app.cadence.shared.repository

import app.cadence.shared.domain.Macros
import app.cadence.shared.domain.Meal
import app.cadence.shared.domain.MealDraft
import app.cadence.shared.domain.MealId
import app.cadence.shared.domain.NutritionTargets
import kotlinx.datetime.LocalDate

/**
 * One day on «Питание»: what was eaten, its exact fold, and the day's targets.
 *
 * [totals] is rounded once — the same boundary `TodaySummary.mealMacros` is,
 * both going through `List<Meal>.dayTotals()` in `domain/Nutrition.kt` rather
 * than each rounding its own sum.
 */
data class NutritionDay(
    val date: LocalDate,
    val meals: List<Meal>,
    val totals: Macros,
    val targets: NutritionTargets,
)

/** One column of `CadenceWeekBars` — a date and that day's rounded totals. */
data class NutritionWeekDay(
    val date: LocalDate,
    val kcal: Int,
    val proteinG: Int,
)

/** Seven days ending on the date [NutritionRepository.week] was asked for, oldest first. */
data class NutritionWeek(
    val days: List<NutritionWeekDay>,
)

/** What logging a meal did. */
sealed interface MealLogResult {
    /**
     * The written meal's id and the day's totals **after** the write.
     *
     * Taken from a fresh read, not from a `NutritionDay` fetched before the
     * write: a toast built off the snapshot from before logging would name
     * the sum without the meal it just confirmed (step-13's ripple).
     */
    data class Written(
        val id: MealId,
        val dayTotals: Macros,
    ) : MealLogResult

    /** The draft cannot make a meal — see `MealDraft.canLog`. */
    data object Rejected : MealLogResult
}

/**
 * §11's «Питание»: a day's meals against target, the week's bar chart, and
 * the write both feed off.
 *
 * Meals live on `CadenceMocks`, not inside this repository's mock — its KDoc
 * explains why: `TodayRepository` reads the same list, and a private copy
 * here would be the two-open-tabs defect the architecture note already names.
 */
interface NutritionRepository {
    /** Meals, totals and targets for [date]. */
    suspend fun day(date: LocalDate): NutritionDay

    /** The seven days ending on [endingOn], oldest first. */
    suspend fun week(endingOn: LocalDate): NutritionWeek

    suspend fun log(draft: MealDraft): MealLogResult
}
