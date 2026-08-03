package app.cadence.shell

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.navigation.NavGraphBuilder
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import app.cadence.design.CadenceDestination
import kotlinx.coroutines.delay

/**
 * Stand-ins until the meal wizard lands in step 8 of the block.
 *
 * The kcal figure is the *day's* total after the meal, not the meal's own —
 * see [ConfirmToastState].
 */
private const val PLACEHOLDER_MEAL_NAME = "Обед"
private const val PLACEHOLDER_DAY_KCAL = 1240

/** The day's target — `MEAL_TARGETS.kcal` in the prototype's meal data. */
private const val PLACEHOLDER_KCAL_TARGET = 2100

/**
 * The whole after-sign-in surface: the graph, the sheet the `+` opens, and the
 * card a logged meal raises.
 *
 * Two pieces of state, both about what is on screen right now and neither
 * derived from anything: whether the sheet is open, and what the toast is
 * showing. Both are the prototype's — `actionSheetOpen` in the navigator,
 * `confirmSheet` in the app state.
 *
 * The timer lives here rather than in [ConfirmToast] because how long something
 * stays on screen is a property of the screen. `showConfirm` in the prototype
 * puts it in the same place, for the same reason.
 */
@Composable
fun CadenceApp(
    navController: NavHostController = rememberNavController(),
    modifier: Modifier = Modifier,
) {
    var actionsOpen by remember { mutableStateOf(false) }
    var toast by remember { mutableStateOf<ConfirmToastState?>(null) }
    // Keyed on a counter rather than on the toast itself. ConfirmToastState is a
    // data class and mutableStateOf compares structurally, so raising an equal
    // toast inside the window would be no assignment at all: no state change,
    // no restart, and the second confirmation would inherit the remainder of
    // the first one's life. The prototype clears its timeout before re-arming,
    // for the same reason.
    //
    // No UI test reaches this, and that is not an oversight: the overlay
    // swallows touches for the whole window, so a second meal cannot be logged
    // by tapping until the first toast is gone — see
    // ConfirmToastTest.theToastSwallowsEveryTouchWhileItIsUp. The counter stays
    // because the moment anything raises a toast without a tap behind it — a
    // repository push, a recipe added to the day — the equality trap is live
    // again, and it costs one Int.
    var raisedAt by remember { mutableStateOf(0) }

    LaunchedEffect(raisedAt) {
        if (toast != null) {
            delay(CADENCE_CONFIRM_TOAST_MS)
            toast = null
        }
    }

    Box(modifier.fillMaxSize()) {
        CadenceShell(
            navController = navController,
            onOpenActions = { actionsOpen = true },
            onMealLogged = { name, dayKcal ->
                toast = ConfirmToastState(name, dayKcal)
                raisedAt++
            },
        )

        ActionChooserSheet(
            open = actionsOpen,
            // The zero-state, until the repositories land with the next
            // subtask. Wire these three to them and nothing else here changes.
            doseLogged = false,
            mealCount = 0,
            mealKcal = 0,
            onDismiss = { actionsOpen = false },
            onPickDose = {
                actionsOpen = false
                navController.openRoute(CadenceRoute.LogDose)
            },
            onPickMeal = {
                actionsOpen = false
                navController.openRoute(CadenceRoute.LogMeal)
            },
        )

        ConfirmToast(state = toast, targetKcal = PLACEHOLDER_KCAL_TARGET)
    }
}

/**
 * The after-sign-in host: the screen graph and the overlays above it.
 *
 * Screens get named lambdas and never the controller, which is how the
 * prototype wires its `Stack.Screen` children and what keeps a screen testable
 * without a navigator. `onOpenActions` is the one callback that is not
 * navigation: the `+` in the bar opens a sheet, and a sheet is not a place.
 *
 * Takes its controller as a parameter so a test can assert on the back stack —
 * «did that tap navigate» is not answerable from what is on screen when two
 * routes render the same word.
 */
@Composable
fun CadenceShell(
    navController: NavHostController = rememberNavController(),
    modifier: Modifier = Modifier,
    onOpenActions: () -> Unit = { },
    onMealLogged: (String, Int) -> Unit = { _, _ -> },
) {
    NavHost(
        navController = navController,
        startDestination = CADENCE_ROOT,
        modifier = modifier.fillMaxSize(),
        // The transitions live here, on the NavHost, because Compose reads
        // each side from the destination it belongs to: a screen's exit is
        // taken from the screen being *left*, not from the one arriving. Two
        // overrides on the modal itself therefore could not hold the screen
        // beneath it still — they fired when leaving the modal forward, which
        // is not the same event and not the one that drifts.
        enterTransition = { if (targetState.destination.isModal()) modalEnter() else pushEnter() },
        exitTransition = { if (targetState.destination.isModal()) modalUnderlayExit() else pushExit() },
        popEnterTransition = { if (initialState.destination.isModal()) modalUnderlayEnter() else popEnter() },
        popExitTransition = { if (initialState.destination.isModal()) modalExit() else popExit() },
    ) {
        tabRoutes(navController, onOpenActions)
        pushedRoutes(navController)
        modalRoutes(navController, onMealLogged)
    }
}

/**
 * The four the bottom bar reaches. Only these carry the bar.
 *
 * Written as a `when` over the enum rather than four literal `composable<…>`
 * calls so that adding a fifth destination fails to compile here too, not only
 * in [CadenceDestination.route].
 */
private fun NavGraphBuilder.tabRoutes(
    nav: NavHostController,
    onOpenActions: () -> Unit,
) {
    CadenceDestination.entries.forEach { destination ->
        val body: @Composable () -> Unit = {
            PlaceholderScreen(
                title = destination.label,
                destination = destination,
                // Today is the root: there is nothing behind it to go back to.
                // The other three use goBack, as the prototype does — identical
                // to popToTop only while a tab always sits at depth 1, which
                // stops being true the moment a real screen reaches one from
                // deeper (RecipeDetail → Nutrition).
                onBack = if (destination == CadenceDestination.TODAY) null else back(nav),
                onSelectTab = nav::selectDestination,
                onLog = onOpenActions,
            )
        }
        when (destination) {
            CadenceDestination.TODAY -> composable<CadenceRoute.Today> { body() }
            CadenceDestination.INVENTORY -> composable<CadenceRoute.Vials> { body() }
            CadenceDestination.TRENDS -> composable<CadenceRoute.Trends> { body() }
            CadenceDestination.NUTRITION -> composable<CadenceRoute.Nutrition> { body() }
        }
    }
}

/**
 * The way out of every screen that is not the root.
 *
 * One definition rather than one per graph section: `popBackStack()` spelled
 * inline in three places is three chances for one of them to become `{ }` and
 * strand a screen, which is what the tests could not see until they clicked it.
 */
private fun back(nav: NavHostController): () -> Unit = { nav.popBackStack() }

/** Everything reached by a push: slides in from the right, backed out of. */
private fun NavGraphBuilder.pushedRoutes(nav: NavHostController) {
    val back = back(nav)

    composable<CadenceRoute.TrendDetail> { entry ->
        PlaceholderScreen("Биомаркер · ${entry.toRoute<CadenceRoute.TrendDetail>().biomarkerId}", onBack = back)
    }
    composable<CadenceRoute.Schedule> { PlaceholderScreen("Расписание", onBack = back) }
    composable<CadenceRoute.Learn> { PlaceholderScreen("Обучение", onBack = back) }
    composable<CadenceRoute.Article> { entry ->
        PlaceholderScreen("Статья · ${entry.toRoute<CadenceRoute.Article>().articleId}", onBack = back)
    }
    composable<CadenceRoute.Journal> { PlaceholderScreen("Дневник", onBack = back) }
    composable<CadenceRoute.Body> { PlaceholderScreen("Тело", onBack = back) }
    composable<CadenceRoute.Recipes> { PlaceholderScreen("Рецепты", onBack = back) }
    composable<CadenceRoute.RecipeDetail> { entry ->
        PlaceholderScreen("Рецепт · ${entry.toRoute<CadenceRoute.RecipeDetail>().recipeId}", onBack = back)
    }
    composable<CadenceRoute.Profile> { PlaceholderScreen("Профиль", onBack = back) }
    composable<CadenceRoute.ChatList> { PlaceholderScreen("Чаты", onBack = back) }
    composable<CadenceRoute.ChatThread> { entry ->
        PlaceholderScreen("Переписка · ${entry.toRoute<CadenceRoute.ChatThread>().threadId}", onBack = back)
    }
}

/**
 * The prototype's `Stack.Group` with `presentation: 'fullScreenModal'`: these
 * slide up rather than in, and every one of them ends by dismissing itself.
 */
private fun NavGraphBuilder.modalRoutes(
    nav: NavHostController,
    onMealLogged: (String, Int) -> Unit,
) {
    val back = back(nav)

    modal<CadenceRoute.LogDose> {
        PlaceholderScreen("Записать дозу", onBack = back, action = "Записать" to back)
    }
    modal<CadenceRoute.LogMeal> {
        PlaceholderScreen(
            title = "Записать приём пищи",
            onBack = back,
            // The prototype's LogMeal hands the meal to the app, which raises
            // the toast and dismisses. The placeholder hands over a fixed one
            // so the toast is reachable before the wizard is ported.
            action =
                "Записать" to {
                    onMealLogged(PLACEHOLDER_MEAL_NAME, PLACEHOLDER_DAY_KCAL)
                    back()
                },
        )
    }
    modal<CadenceRoute.AddVial> { PlaceholderScreen("Добавить флакон", onBack = back) }
    modal<CadenceRoute.RecipeBuilder> { PlaceholderScreen("Новый рецепт", onBack = back) }
}

/**
 * A route in the modal group: up rather than in, and back down again.
 *
 * Named so the four entries carry the transition by membership, the way the
 * prototype's `Stack.Group` does, instead of each repeating the same two
 * overrides — where one omission would be a screen that slides the wrong way
 * and no test would say so.
 */
private inline fun <reified T : CadenceRoute.Modal> NavGraphBuilder.modal(noinline content: @Composable () -> Unit) {
    // No transition overrides: they belong on the NavHost, which is the only
    // place that can see both sides of a transition at once. What this builder
    // still buys is the type constraint — registering an ordinary route as a
    // modal, or a modal as an ordinary push, does not compile.
    composable<T> { content() }
}
