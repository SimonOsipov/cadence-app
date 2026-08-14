package app.cadence.shell

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.navigation.NavGraphBuilder
import androidx.navigation.NavHostController
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import app.cadence.design.CadenceDestination
import app.cadence.screens.nutrition.LogMealScreen
import app.cadence.screens.nutrition.NutritionScreen
import app.cadence.screens.recipes.RecipeBuilderScreen
import app.cadence.screens.recipes.RecipeDetailScreen
import app.cadence.screens.recipes.RecipeDetailState
import app.cadence.screens.recipes.RecipesScreen
import app.cadence.shared.domain.MealDraft
import app.cadence.shared.domain.RecipeId
import app.cadence.shared.mock.CadenceMocks
import app.cadence.shared.repository.MealLogResult
import app.cadence.shared.repository.RecipeSaveResult
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
    // One write in flight at a time. «Сохранить» stays live until the answer lands (the
    // footer only knows whether there are items), and on M9's round trip that window is a
    // whole request — two taps would log the meal twice, each with its own toast.
    var saving by remember { mutableStateOf(false) }

    if (summary == null || now == null) {
        PlaceholderScreen(title = "Записать приём пищи", onBack = back)
        return
    }

    LogMealScreen(
        now = now,
        targets = summary.targets,
        parse = actions.parseMeal,
        onSave = { draft ->
            if (!saving) {
                saving = true
                scope.launch {
                    if (actions.onMealLogged(draft) is MealLogResult.Written) back()
                    saving = false
                }
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
        // Non-null by `MealDraft.canLog`, which every `Written` has already passed — an
        // unnamed confirmation would be a silent defect, not a tolerable default.
        toast.raise(requireNotNull(draft.name), result.dayTotals.kcal)
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
 * The library, one recipe's card and the builder. «Рецепты» is gated on all three of its
 * reads — the library, the day it scores against and the ingredient table it prices with,
 * an empty one of which is not «no ingredients» but a table that throws on the first row.
 */
internal fun NavGraphBuilder.recipeRoutes(
    nav: NavHostController,
    data: CadenceShellData,
    actions: CadenceShellActions,
) {
    val back = {
        nav.popBackStack()
        Unit
    }

    composable<CadenceRoute.Recipes> {
        val library = data.recipes
        val day = data.nutritionDay
        val ingredients = data.ingredients

        if (library == null || day == null || ingredients == null) {
            PlaceholderScreen("Рецепты", onBack = back)
        } else {
            RecipesScreen(
                library = library,
                ingredients = ingredients,
                consumed = day.totals,
                goals = day.targets.macros,
                onBack = back,
                onOpen = { nav.openRoute(CadenceRoute.RecipeDetail(it.raw)) },
                onCreate = { nav.openRoute(CadenceRoute.RecipeBuilder) },
            )
        }
    }

    composable<CadenceRoute.RecipeDetail> { entry ->
        RecipeDetailRoute(
            nav = nav,
            id = RecipeId(entry.toRoute<CadenceRoute.RecipeDetail>().recipeId),
            data = data,
            actions = actions,
            back = back,
        )
    }

    modal<CadenceRoute.RecipeBuilder> {
        val scope = rememberCoroutineScope()
        var saving by remember { mutableStateOf(false) }

        RecipeBuilderScreen(
            search = actions.searchIngredients,
            onCancel = back,
            // Replaces the builder rather than stacking on it: the patient came to make a
            // recipe, and going back from what they made belongs to the library they started
            // from, not to the form (`CadenceNavigation.kt`'s `replaceRoute` names this case).
            onSave = { draft ->
                if (!saving) {
                    saving = true
                    scope.launch {
                        val result = actions.onRecipeSaved(draft)
                        if (result is RecipeSaveResult.Saved) {
                            nav.replaceRoute(CadenceRoute.RecipeDetail(result.id.raw))
                        }
                        saving = false
                    }
                }
            },
        )
    }
}

/**
 * One recipe's card, read through [CadenceShellActions.loadRecipe] — the repository's own
 * `recipe(id)`, not the library snapshot the row was drawn from. The snapshot is a frame
 * behind its own writes: `replaceRoute` onto a just-saved recipe composes before
 * `LaunchedEffect(reloads)` has re-read the library, so a card resolved against it answers
 * «Рецепт не найден.» to the recipe the patient just made.
 *
 * Three states, not two: «ещё не ответили» is not «такого рецепта нет», and a deep link can
 * carry an id no recipe has. Same shape as `TrendDetail`'s own read one file over.
 */
@Composable
private fun RecipeDetailRoute(
    nav: NavHostController,
    id: RecipeId,
    data: CadenceShellData,
    actions: CadenceShellActions,
    back: () -> Unit,
) {
    val scope = rememberCoroutineScope()
    var state by remember(id) { mutableStateOf<RecipeDetailState>(RecipeDetailState.Loading) }
    // One write in flight at a time: the button stays live until the answer lands, and two
    // taps in the same frame would otherwise queue two coroutines and log the meal twice —
    // the defect `LogDoseModal`'s own `submitting` flag exists for.
    var adding by remember { mutableStateOf(false) }

    LaunchedEffect(id) {
        state = actions.loadRecipe(id)?.let(RecipeDetailState::Found) ?: RecipeDetailState.NotFound
    }

    RecipeDetailScreen(
        state = state,
        ingredients = data.ingredients.orEmpty(),
        onBack = back,
        // The same write and the same confirmation as the wizard's, and then «Питание» —
        // where the meal it just wrote is visible. The draft already carries
        // `source = RECIPE` and the recipe's id (`Recipe.toMealDraft`), so this path cannot
        // log a recipe as free text.
        onAddToDay = { draft ->
            if (!adding) {
                adding = true
                scope.launch {
                    if (actions.onMealLogged(draft) is MealLogResult.Written) {
                        nav.selectDestination(CadenceDestination.NUTRITION)
                    }
                    adding = false
                }
            }
        },
    )
}
