package app.cadence.shell

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.navigation.NavGraphBuilder
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import app.cadence.design.CadenceDestination

/** Stand-ins until the meal wizard lands in step 8 of the block. */
private const val PLACEHOLDER_MEAL_NAME = "Обед"
private const val PLACEHOLDER_MEAL_KCAL = 520

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
        startDestination = CadenceRoute.Today,
        modifier = modifier.fillMaxSize(),
        enterTransition = { pushEnter() },
        exitTransition = { pushExit() },
        popEnterTransition = { popEnter() },
        popExitTransition = { popExit() },
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
                onBack = if (destination == CadenceDestination.TODAY) null else nav::popToTop,
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

/** Everything reached by a push: slides in from the right, backed out of. */
private fun NavGraphBuilder.pushedRoutes(nav: NavHostController) {
    val back: () -> Unit = { nav.popBackStack() }

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
    val back: () -> Unit = { nav.popBackStack() }

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
                    onMealLogged(PLACEHOLDER_MEAL_NAME, PLACEHOLDER_MEAL_KCAL)
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
private inline fun <reified T : CadenceRoute> NavGraphBuilder.modal(noinline content: @Composable () -> Unit) {
    composable<T>(
        enterTransition = { modalEnter() },
        popExitTransition = { modalExit() },
    ) { content() }
}
