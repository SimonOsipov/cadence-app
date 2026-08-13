package app.cadence.shell

import androidx.navigation.NavDestination
import androidx.navigation.NavDestination.Companion.hasRoute
import app.cadence.design.CadenceDestination
import kotlinx.serialization.Serializable

/**
 * Ported one for one from `RootStackParamList` in mobile/src/navigation/AppNavigator.tsx.
 * Names are the prototype's, in English — never shown to a patient. The area after sign-in;
 * block 7 adds the area before it, above this graph rather than inside it.
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
     * Presented as full-screen modals (`Stack.Group`, `fullScreenModal`), sliding up not in.
     * A marker interface, not a comment, so registering one as an ordinary push fails to compile.
     */
    sealed interface Modal : CadenceRoute

    /**
     * [vialId] is null from the tab bar and Today hero (picks the fullest open vial); set
     * only from a vial's own sheet. Carried on the route, not shell state, so it can't outlive
     * the screen.
     */
    @Serializable
    data class LogDose(
        val vialId: String? = null,
    ) : Modal

    @Serializable
    data object LogMeal : Modal

    @Serializable
    data object AddVial : Modal

    @Serializable
    data object RecipeBuilder : Modal
}

/**
 * Four explicit checks, not a test against [CadenceRoute.Modal]: the marker interface isn't
 * `@Serializable`, so `hasRoute` can't match it. A forgotten fifth modal is caught by
 * `CadenceNavigationTest.theScreenBeneathAModalDoesNotMove`, not the compiler.
 */
internal fun NavDestination.isModal(): Boolean =
    hasRoute<CadenceRoute.LogDose>() ||
        hasRoute<CadenceRoute.LogMeal>() ||
        hasRoute<CadenceRoute.AddVial>() ||
        hasRoute<CadenceRoute.RecipeBuilder>()

/** Named once so `NavHost`'s `startDestination` and [popToTop] can't drift apart into a silent no-pop. */
val CADENCE_ROOT: CadenceRoute = CadenceRoute.Today

/**
 * A `when` over a closed enum, unlike the prototype's ternary chain, so a fifth destination
 * fails to compile instead of falling through to Nutrition.
 */
val CadenceDestination.route: CadenceRoute
    get() =
        when (this) {
            CadenceDestination.TODAY -> CadenceRoute.Today
            CadenceDestination.INVENTORY -> CadenceRoute.Vials
            CadenceDestination.TRENDS -> CadenceRoute.Trends
            CadenceDestination.NUTRITION -> CadenceRoute.Nutrition
        }
