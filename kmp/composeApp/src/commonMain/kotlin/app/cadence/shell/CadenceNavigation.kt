package app.cadence.shell

import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavHostController
import app.cadence.design.CadenceDestination

/**
 * React Navigation's `navigate`: return to the route if the stack already holds
 * it, otherwise push it.
 *
 * This is not what `NavController.navigate` does, and not what `launchSingleTop`
 * does either — `launchSingleTop` de-duplicates only when the route is already
 * on top. The difference shows the moment a recipe hands back to Nutrition from
 * three screens deep: the faithful behaviour shrinks the stack, a plain
 * navigate grows it, and back then walks a path the user never took.
 *
 * Matching is by route *type*, not by value, because that is what React
 * Navigation matches on — its stack is keyed by screen name and the params ride
 * along. So opening `Article("a-2")` while `Article("a-1")` is below returns to
 * a-1; the prototype knows this, which is exactly why article-to-article is the
 * one place it calls [pushRoute] by name instead.
 */
fun NavHostController.openRoute(route: CadenceRoute) {
    // Already here: React Navigation's navigate to the current screen is a
    // no-op, and popBackStack below would not fire for it either — it would
    // fall through to navigate() and stack a duplicate.
    if (currentBackStackEntry?.destination?.hasRoute(route::class) == true) return
    if (!popBackStack(route, inclusive = false)) navigate(route)
}

/** React Navigation's `push`: always a new entry, even for the same route. */
fun NavHostController.pushRoute(route: CadenceRoute) {
    navigate(route)
}

/**
 * React Navigation's `replace`: the finished screen must not be behind you.
 *
 * The prototype uses it twice — a chat thread swapping itself for the thread
 * list, and the recipe builder swapping itself for the recipe it just saved.
 * In both, backing out to the screen that just completed would be wrong.
 */
fun NavHostController.replaceRoute(route: CadenceRoute) {
    val leaving = currentBackStackEntry?.destination?.route
    navigate(route) {
        if (leaving != null) popUpTo(leaving) { inclusive = true }
    }
}

/** React Navigation's `popToTop`. */
fun NavHostController.popToTop() {
    popBackStack(CadenceRoute.Today, inclusive = false)
}

/**
 * The bottom bar's `changeTab`, minus the `'log'` branch — logging is not a
 * destination, so it never reaches here (see [CadenceShell]).
 *
 * Pops to the root before navigating, exactly as the prototype does. Tapping a
 * tab is «go there», not «go there on top of everything I was doing».
 */
fun NavHostController.selectDestination(destination: CadenceDestination) {
    popToTop()
    if (destination != CadenceDestination.TODAY) openRoute(destination.route)
}
