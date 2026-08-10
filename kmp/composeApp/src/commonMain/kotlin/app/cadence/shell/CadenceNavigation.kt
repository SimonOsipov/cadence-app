package app.cadence.shell

import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavGraph.Companion.findStartDestination
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

        // Below the current screen. React Navigation slices the stack back to
        // the screen *and* writes the new arguments onto it; the two halves
        // need different handling here.
        existing.destination.id != current?.id -> {
            if (existing.destination.arguments.isEmpty()) {
                // Nothing to update, so keep the instance and whatever state it
                // holds. The common case — every bottom-bar destination is
                // argument-less — and the recipe-to-Nutrition one.
                popBackStack(existing.destination.id, inclusive = false)
            } else {
                // Popping to it non-inclusively lands on the arguments it was
                // created with, not the ones asked for: `TrendDetail("ldl")`
                // with `TrendDetail("hrv")` buried in the stack put the patient
                // back on hrv. Measured — and worse than the duplicate-pushing
                // version this replaced, which at least showed the right one.
                // Rebuilding costs the entry's state, which no screen holds yet.
                popBackStack(existing.destination.id, inclusive = true)
                navigate(route)
            }
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
 * Pops by destination id rather than by `destination.route`. Not because the
 * pattern would fail to match — `popUpTo(String)` is resolved against
 * `destination.route`, which is the very pattern being passed, so it holds for
 * any argument type. The id is simply identity, and using it removes the null
 * branch in which the whole call degraded to a plain push: the finished screen
 * left behind you, which is the one thing this function exists to prevent.
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
 * Returns whether it popped anything, and [selectDestination] reads it.
 *
 * Pops to the graph's own start destination rather than to a named route. The
 * typed `popBackStack(CADENCE_ROOT, …)` fills arguments into the route string
 * before matching — the trap documented on [openRoute] — so the first root that
 * carried one would return false for a root that *is* on the stack, and every
 * tab tap would become a silent no-pop. Block 7 moves this boundary; asking the
 * graph is argument-proof and needs no second literal to stay in step.
 */
fun NavHostController.popToTop(): Boolean = popBackStack(graph.findStartDestination().id, inclusive = false)

/**
 * The bottom bar's `changeTab`, minus the `'log'` branch — logging is not a
 * destination, so it never reaches here (see [CadenceShell]).
 *
 * Pops to the root before navigating, exactly as the prototype does. Tapping a
 * tab is «go there», not «go there on top of everything I was doing».
 */
fun NavHostController.selectDestination(destination: CadenceDestination) {
    // popToTop returns false in two different situations: «already standing on
    // the root», which is ordinary, and «the root is not on the stack at all»,
    // which cannot happen today and is what block 7 changes. Only the second is
    // a reason to stop — stacking a tab on an unknown stack would make the bar
    // accumulate depth and turn back into tab history.
    val onRoot = currentBackStackEntry?.destination?.id == graph.findStartDestination().id
    if (!popToTop() && !onRoot) return

    // The guard is load-bearing, not cosmetic: without it, tapping «Сегодня»
    // while on Today takes openRoute's third branch and rebuilds the root.
    if (destination != CadenceDestination.TODAY) openRoute(destination.route)
}
