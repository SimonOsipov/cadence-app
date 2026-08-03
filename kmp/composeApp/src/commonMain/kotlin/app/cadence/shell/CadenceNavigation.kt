package app.cadence.shell

import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavHostController
import app.cadence.design.CadenceDestination

/**
 * React Navigation's `navigate`: go to the screen if the stack already holds
 * it, otherwise push it.
 *
 * This is not what `NavController.navigate` does, and not what `launchSingleTop`
 * does either — `launchSingleTop` de-duplicates only when the route is already
 * on top. The difference shows the moment a recipe hands back to Nutrition from
 * three screens deep: the faithful behaviour shrinks the stack, a plain
 * navigate grows it, and back then walks a path the user never took.
 *
 * **Matching is by screen, not by arguments**, because React Navigation keys
 * its stack by screen name and lets the params ride along. Compose does the
 * opposite, and finding that out took a measurement rather than a reading: the
 * typed `popBackStack(route)` overload calls `generateRouteFilled` and matches
 * `…/a-2` against `…/a-1`, so it misses and pushes a duplicate. This function
 * had documented React Navigation's rule while implementing Compose's, and the
 * three branches below are what the rule actually costs.
 *
 * The third branch is the one that matters. Opening `TrendDetail("ldl")` while
 * standing on `TrendDetail("hrv")` used to return early and do nothing at all —
 * a patient taps a neighbouring biomarker and the screen does not change, with
 * nothing anywhere to say why. React Navigation updates the params; so does
 * this, by swapping the entry.
 *
 * One divergence remains and is deliberate: re-opening the current screen with
 * *identical* arguments rebuilds its entry where React Navigation would no-op.
 * Comparing arguments needs the filled route string, which the library does not
 * expose, and no screen holds state worth preserving yet. When one does, this
 * is the line that has to change — recorded in `docs/prototype-divergences.md`.
 */
fun NavHostController.openRoute(route: CadenceRoute) {
    val current = currentBackStackEntry?.destination
    val existing = currentBackStack.value.lastOrNull { it.destination.hasRoute(route::class) }

    when {
        // Not on the stack: push it.
        existing == null -> {
            navigate(route)
        }

        // Below the current screen: return to that instance, keeping it and
        // whatever state it holds. This is the recipe-to-Nutrition case.
        existing.destination.id != current?.id -> {
            popBackStack(existing.destination.id, inclusive = false)
        }

        // It *is* the current screen, with arguments that may differ. Swap the
        // entry rather than leaving the old arguments on screen.
        else -> {
            navigate(route) { popUpTo(existing.destination.id) { inclusive = true } }
        }
    }
}

/**
 * React Navigation's `push`: always a new entry, even for the same screen.
 *
 * The distinction from [openRoute] is real and the prototype relies on it once:
 * an article linking to an article calls `push` by name, because returning to
 * the article the reader just left is not what a link means.
 */
fun NavHostController.pushRoute(route: CadenceRoute) {
    navigate(route)
}

/**
 * React Navigation's `replace`: the finished screen must not be behind you.
 *
 * The prototype uses it twice — a chat thread swapping itself for the thread
 * list, and the recipe builder swapping itself for the recipe it just saved.
 * In both, backing out to the screen that just completed would be wrong.
 *
 * Pops by destination id rather than by `destination.route`. The route is a
 * *pattern* — `…CadenceRoute.ChatThread/{threadId}` — matched as a string, so
 * it happens to work for the four routes carrying a `String` and would quietly
 * stop the first time one carries an `Int`. The id is identity, and it also
 * removes the null branch, in which the whole call degraded to a plain push:
 * the finished screen left behind you, which is the one thing this function
 * exists to prevent.
 */
fun NavHostController.replaceRoute(route: CadenceRoute) {
    val leaving = currentBackStackEntry?.destination?.id
    navigate(route) {
        if (leaving != null) popUpTo(leaving) { inclusive = true }
    }
}

/**
 * React Navigation's `popToTop`.
 *
 * Returns whether it popped anything, and [selectDestination] reads it. The
 * root is [CADENCE_ROOT] rather than `CadenceRoute.Today` spelled twice: block
 * 7 puts an area above this graph, and a root that moved would turn every tab
 * tap into a silent no-pop while the tests, which all start on Today, stayed
 * green.
 */
fun NavHostController.popToTop(): Boolean = popBackStack(CADENCE_ROOT, inclusive = false)

/**
 * The bottom bar's `changeTab`, minus the `'log'` branch — logging is not a
 * destination, so it never reaches here (see [CadenceShell]).
 *
 * Pops to the root before navigating, exactly as the prototype does. Tapping a
 * tab is «go there», not «go there on top of everything I was doing».
 */
fun NavHostController.selectDestination(destination: CadenceDestination) {
    // popToTop returns false when the root is not on the stack — which cannot
    // happen while Today is the start destination, and is precisely what block
    // 7 changes. Stacking a tab on top of an unknown stack is worse than doing
    // nothing: the bar would start accumulating depth and back would walk a
    // path the patient never took.
    if (!popToTop() && currentBackStackEntry?.destination?.hasRoute(CADENCE_ROOT::class) != true) return

    // The guard is load-bearing, not cosmetic: without it, tapping «Сегодня»
    // while on Today takes openRoute's third branch and rebuilds the root.
    if (destination != CadenceDestination.TODAY) openRoute(destination.route)
}
