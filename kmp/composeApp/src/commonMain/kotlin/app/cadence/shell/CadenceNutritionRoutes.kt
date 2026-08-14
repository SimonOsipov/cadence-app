package app.cadence.shell

import androidx.compose.runtime.Composable
import androidx.compose.runtime.rememberCoroutineScope
import androidx.navigation.NavGraphBuilder
import androidx.navigation.NavHostController
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import app.cadence.screens.nutrition.LogMealScreen
import app.cadence.screens.nutrition.NutritionScreen
import app.cadence.screens.recipes.RecipeDetailScreen
import app.cadence.screens.recipes.RecipeDetailState
import app.cadence.screens.recipes.RecipesScreen
import app.cadence.shared.domain.MealDraft
import app.cadence.shared.domain.RecipeId
import app.cadence.shared.mock.CadenceMocks
import app.cadence.shared.repository.MealLogResult
import app.cadence.shared.repository.TodaySummary
import kotlinx.coroutines.launch
import kotlinx.datetime.LocalDateTime

// The nutrition port's own routes, split out of `CadenceShell.kt` — that file had reached
// detekt's function ceiling, and these five screens (the wizard, «Питание», «Рецепты», the
// recipe card and the builder) are one feature's worth of wiring.

/**
 * Gated on the read like its siblings — targets come from the day, and the header's clock
 * reading from [CadenceShellData.now], never from a literal (the prototype's own `08:42`).
 *
 * The write is awaited, and only a [MealLogResult.Written] closes the modal: a rejected
 * draft leaves the patient on the screen with what they typed, rather than dismissing it
 * as if it had been recorded.
 */
@Composable
internal fun LogMealModal(
    summary: TodaySummary?,
    now: LocalDateTime?,
    actions: CadenceShellActions,
    back: () -> Unit,
) {
    val scope = rememberCoroutineScope()

    if (summary == null || now == null) {
        PlaceholderScreen(title = "Записать приём пищи", onBack = back)
        return
    }

    LogMealScreen(
        now = now,
        targets = summary.targets,
        parse = actions.parseMeal,
        onSave = { draft ->
            scope.launch {
                if (actions.onMealLogged(draft) is MealLogResult.Written) back()
            }
        },
        onCancel = back,
    )
}

/**
 * Writes the meal and confirms it with the day total **from the write's own answer**.
 *
 * Not from the `summary` the shell is holding: that is still the pre-write snapshot when
 * this returns, because `LaunchedEffect(reloads)` re-reads only afterwards, and a toast
 * built from it names the day without the meal it is confirming.
 */
internal suspend fun logMeal(
    mocks: CadenceMocks,
    toast: ToastUiState,
    draft: MealDraft,
    onWritten: () -> Unit,
): MealLogResult {
    val result = mocks.nutrition.log(draft)
    if (result is MealLogResult.Written) {
        toast.raise(draft.name.orEmpty(), result.dayTotals.kcal)
        onWritten()
    }
    return result
}

/**
 * «Питание» — gated on both reads, because the screen draws a day *and* its week: composed
 * against half of them it would state a zero week the patient never had.
 */
@Composable
internal fun NutritionRoute(
    nav: NavHostController,
    data: CadenceShellData,
    onOpenActions: () -> Unit,
    body: @Composable () -> Unit,
) {
    val day = data.nutritionDay
    val week = data.nutritionWeek

    if (day == null || week == null) {
        body()
        return
    }

    NutritionScreen(
        day = day,
        week = week,
        zone = data.zone,
        onBack = { nav.popBackStack() },
        onLogMeal = { nav.openRoute(CadenceRoute.LogMeal) },
        onOpenRecipes = { nav.openRoute(CadenceRoute.Recipes) },
        onSelectTab = nav::selectDestination,
        onOpenActions = onOpenActions,
    )
}

/**
 * The library and one recipe's card. Both are gated on the library read — the same list the
 * detail resolves its id against, so a card and the row that opened it can never disagree.
 */
internal fun NavGraphBuilder.recipeRoutes(
    nav: NavHostController,
    data: CadenceShellData,
) {
    val back = {
        nav.popBackStack()
        Unit
    }

    composable<CadenceRoute.Recipes> {
        val library = data.recipes
        val day = data.nutritionDay

        if (library == null || day == null) {
            PlaceholderScreen("Рецепты", onBack = back)
        } else {
            RecipesScreen(
                library = library,
                ingredients = data.ingredients,
                consumed = day.totals,
                goals = day.targets.macros,
                onBack = back,
                onOpen = { nav.openRoute(CadenceRoute.RecipeDetail(it.raw)) },
                onCreate = { nav.openRoute(CadenceRoute.RecipeBuilder) },
            )
        }
    }

    composable<CadenceRoute.RecipeDetail> { entry ->
        val id = RecipeId(entry.toRoute<CadenceRoute.RecipeDetail>().recipeId)
        val library = data.recipes

        // Three states, not two: «библиотека ещё не прочитана» is not «такого рецепта нет»,
        // and a deep link can carry an id no recipe has.
        val state =
            when {
                library == null -> {
                    RecipeDetailState.Loading
                }

                else -> {
                    library.recipes
                        .firstOrNull { it.id == id }
                        ?.let(RecipeDetailState::Found)
                        ?: RecipeDetailState.NotFound
                }
            }

        RecipeDetailScreen(
            state = state,
            ingredients = data.ingredients,
            onBack = back,
        )
    }
}
