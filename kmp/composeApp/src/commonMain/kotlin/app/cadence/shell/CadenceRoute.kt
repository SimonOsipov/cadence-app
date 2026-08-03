package app.cadence.shell

import app.cadence.design.CadenceDestination
import kotlinx.serialization.Serializable

/**
 * The screen graph, ported one for one from `RootStackParamList` in
 * mobile/src/navigation/AppNavigator.tsx.
 *
 * Names are the prototype's, in English, because a route is not product copy —
 * nothing here is ever shown to a patient. The four routes carrying a parameter
 * carry exactly the parameter the prototype passes, and no more: `TrendDetail`
 * gets a biomarker id, not a biomarker.
 *
 * This is the area *after* sign-in. Block 7 adds the area before it; that
 * boundary lands above this graph rather than inside it.
 */
sealed interface CadenceRoute {
    @Serializable
    data object Today : CadenceRoute

    @Serializable
    data object Trends : CadenceRoute

    @Serializable
    data class TrendDetail(
        val biomarkerId: String,
    ) : CadenceRoute

    @Serializable
    data object Nutrition : CadenceRoute

    @Serializable
    data object Vials : CadenceRoute

    @Serializable
    data object Schedule : CadenceRoute

    @Serializable
    data object Learn : CadenceRoute

    @Serializable
    data class Article(
        val articleId: String,
    ) : CadenceRoute

    @Serializable
    data object Journal : CadenceRoute

    @Serializable
    data object Body : CadenceRoute

    @Serializable
    data object Recipes : CadenceRoute

    @Serializable
    data class RecipeDetail(
        val recipeId: String,
    ) : CadenceRoute

    @Serializable
    data object Profile : CadenceRoute

    @Serializable
    data object ChatList : CadenceRoute

    @Serializable
    data class ChatThread(
        val threadId: String,
    ) : CadenceRoute

    /**
     * The four the prototype presents as full-screen modals rather than
     * pushing — `Stack.Group` with `presentation: 'fullScreenModal'`. They
     * slide up, not in from the right.
     *
     * A marker interface rather than a comment over four declarations, because
     * the argument for grouping them at all — that four repeated overrides
     * invite one omission nobody would see — applies just as much to a group
     * the compiler does not check. Registering `Schedule` as a modal, or
     * `LogDose` as an ordinary push, now fails to compile.
     */
    sealed interface Modal : CadenceRoute

    @Serializable
    data object LogDose : Modal

    @Serializable
    data object LogMeal : Modal

    @Serializable
    data object AddVial : Modal

    @Serializable
    data object RecipeBuilder : Modal
}

/**
 * The root of the after-sign-in graph.
 *
 * Named once because two places need to agree on it — `NavHost`'s
 * `startDestination` and [popToTop] — and block 7 will move the boundary above
 * this graph. Two literals that drifted apart would turn every tab tap into a
 * silent no-pop.
 */
val CADENCE_ROOT: CadenceRoute = CadenceRoute.Today

/**
 * Where a bottom-bar destination lands.
 *
 * The prototype writes this as a ternary chain inside `changeTab`; here it is a
 * `when` over a closed enum, so a fifth destination fails to compile instead of
 * quietly falling through to Nutrition — which is what the chain's trailing
 * `: 'Nutrition'` does today.
 */
val CadenceDestination.route: CadenceRoute
    get() =
        when (this) {
            CadenceDestination.TODAY -> CadenceRoute.Today
            CadenceDestination.INVENTORY -> CadenceRoute.Vials
            CadenceDestination.TRENDS -> CadenceRoute.Trends
            CadenceDestination.NUTRITION -> CadenceRoute.Nutrition
        }
