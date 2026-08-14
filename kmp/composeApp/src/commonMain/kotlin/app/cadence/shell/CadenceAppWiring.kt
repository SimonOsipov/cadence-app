package app.cadence.shell

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import app.cadence.screens.schedule.ScheduleState
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.InventorySummary
import app.cadence.shared.domain.Meal
import app.cadence.shared.domain.TrendWindow
import app.cadence.shared.domain.TrendsOverview
import app.cadence.shared.domain.VialDetail
import app.cadence.shared.mock.CadenceMocks
import app.cadence.shared.parsing.MealParser
import app.cadence.shared.repository.NutritionDay
import app.cadence.shared.repository.NutritionWeek
import app.cadence.shared.repository.RecipeFilter
import app.cadence.shared.repository.RecipeList
import app.cadence.shared.repository.RecipeSaveResult
import app.cadence.shared.repository.TodaySummary
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

// The composition root's own state, split out of `CadenceShell.kt`: that file carries the
// navigation graph, this one carries what the graph is fed and how a write refreshes it.

/**
 * Held above the trends screen, not inside it: list and detail both read it, so a window
 * remembered in one would reset on return from the other.
 */
internal class TrendsUiState {
    var window by mutableStateOf(TrendWindow.THREE_MONTHS)
    var overview by mutableStateOf<TrendsOverview?>(null)
}

/** Keyed on both [reloads] and the window: keying on [reloads] alone would leave the chips changing nothing. */
@Composable
internal fun rememberTrendsState(
    mocks: CadenceMocks,
    reloads: Int,
): TrendsUiState {
    val state = remember { TrendsUiState() }
    LaunchedEffect(state.window, reloads) {
        state.overview = mocks.trends.overview(state.window)
    }
    return state
}

/**
 * Everything one `reloads` bump re-reads, held together for the same reason [TrendsUiState]
 * is: eight `remember`s in [CadenceApp] and one effect that assigns them all is one fact —
 * "what the repositories last said" — spelled eight times.
 */
internal class DayUiState {
    var summary by mutableStateOf<TodaySummary?>(null)

    // TodaySummary carries only the count and the fold — this is TodayMeals' own list.
    var todayMeals by mutableStateOf<List<Meal>>(emptyList())
    var schedule by mutableStateOf<ScheduleState?>(null)
    var cabinet by mutableStateOf<InventorySummary?>(null)
    var nutritionDay by mutableStateOf<NutritionDay?>(null)
    var nutritionWeek by mutableStateOf<NutritionWeek?>(null)
    var recipes by mutableStateOf<RecipeList?>(null)
    var ingredients by mutableStateOf<List<Ingredient>>(emptyList())
}

@Composable
internal fun rememberDayState(
    mocks: CadenceMocks,
    reloads: Int,
): DayUiState {
    val state = remember { DayUiState() }
    LaunchedEffect(reloads) {
        val today = mocks.today.today()
        state.summary = today
        state.schedule = mocks.scheduleFor(today)
        state.cabinet = mocks.inventory.cabinet()

        val day = mocks.nutrition.day(today.date)
        state.nutritionDay = day
        state.todayMeals = day.meals
        state.nutritionWeek = mocks.nutrition.week(today.date)
        state.recipes = mocks.recipes.library(RecipeFilter())
        // The whole table, once: every row a patient can pick has to be priceable, and the
        // repository's search is a `contains` over the same list.
        state.ingredients = mocks.recipes.ingredients("")
    }
    return state
}

/**
 * The confirmation toast and the counter that restarts its timer, together — they are one
 * fact and were two `remember`s.
 *
 * Keyed on a counter, not the toast itself: [ConfirmToastState] is a data class and
 * `mutableStateOf` compares structurally, so an equal toast raised inside the window would
 * be no assignment at all — no restart, and the new confirmation would inherit the old
 * timer. No UI test reaches the identical-toast case directly (the overlay blocks a second
 * tap until the first toast clears — `ConfirmToastTest.theToastSwallowsEveryTouchWhileItIsUp`),
 * but the counter stays for any future non-tap trigger.
 */
internal class ToastUiState {
    var state by mutableStateOf<ConfirmToastState?>(null)
        private set
    var raisedAt by mutableStateOf(0)
        private set

    fun raise(
        mealName: String,
        dayKcal: Int,
    ) {
        state = ConfirmToastState(mealName, dayKcal)
        raisedAt++
    }

    fun clear() {
        state = null
    }
}

/** §03: `protocols.weeks (12)`. Read from the protocol once one is fetched. */
private const val CYCLE_WEEKS = 12

/** The month around [today], in the shape «График» draws. */
private suspend fun CadenceMocks.scheduleFor(today: TodaySummary): ScheduleState =
    ScheduleState(
        today = today.date,
        cycleWeek = today.cycleWeek,
        cycleWeeks = CYCLE_WEEKS,
        days = schedule.month(today.date),
        titration = today.nextTitration,
    )

/**
 * Every repository call the shell can make, bound to this composition's state.
 *
 * A function rather than ten lambdas inline in `CadenceApp`: each of them ends in the same
 * two moves — write, then [onReload] so the next read comes from the repository instead of
 * the snapshot the screen is holding — and having them together is what makes a missing
 * bump visible.
 */
@Suppress("LongParameterList")
internal fun shellActions(
    mocks: CadenceMocks,
    parser: MealParser,
    toast: ToastUiState,
    scope: CoroutineScope,
    onReload: () -> Unit,
    onOpenActions: () -> Unit,
    onOpenVial: (VialDetail?) -> Unit,
    onTrendWindow: (TrendWindow) -> Unit,
): CadenceShellActions =
    CadenceShellActions(
        onOpenActions = onOpenActions,
        onDoseLogged = { draft ->
            val result = mocks.dosing.submit(draft)
            onReload()
            result
        },
        onMealLogged = { draft -> logMeal(mocks, toast, draft, onReload) },
        onAddVial = { draft ->
            scope.launch {
                mocks.inventory.addVial(draft)
                onReload()
            }
        },
        onOpenVial = { id -> scope.launch { onOpenVial(mocks.inventory.vial(id)) } },
        onTrendWindow = onTrendWindow,
        onLoadMetric = { metric, window -> mocks.trends.metric(metric, window) },
        parseMeal = { text -> parser.parse(text) },
        searchIngredients = { query -> mocks.recipes.ingredients(query) },
        onRecipeSaved = { draft ->
            val result = mocks.recipes.save(draft)
            if (result is RecipeSaveResult.Saved) onReload()
            result
        },
    )
