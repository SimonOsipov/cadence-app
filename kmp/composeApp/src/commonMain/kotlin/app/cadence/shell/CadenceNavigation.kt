package app.cadence.shell

import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavHostController
import app.cadence.design.CadenceDestination

/**
 * React Navigation's `navigate`: go to the screen if the stack already holds it, else push.
 * Matching is by screen, not arguments — measured that Compose's typed `popBackStack(route)`
 * fills the route string first and matches `…/a-2` against `…/a-1`, missing and duplicating.
 * Deliberate divergence: re-opening the current screen with identical arguments rebuilds the
 * entry where React Navigation would no-op — recorded in docs/prototype-divergences.md.
 */
fun NavHostController.openRoute(route: CadenceRoute) {
    val current = currentBackStackEntry?.destination
    val existing = currentBackStack.value.lastOrNull { it.destination.hasRoute(route::class) }

    when {
        existing == null -> {
            navigate(route)
        }

        existing.destination.id != current?.id -> {
            if (existing.destination.arguments.isEmpty()) {
                popBackStack(existing.destination.id, inclusive = false)
            } else {
                // Non-inclusive pop lands on the arguments the entry was created with, not the
                // ones asked for — measured: TrendDetail("ldl") over a buried TrendDetail("hrv")
                // landed on hrv. Rebuild instead, at the cost of the entry's state.
                popBackStack(existing.destination.id, inclusive = true)
                navigate(route)
            }
        }

        // Current screen, arguments may differ — swap the entry rather than leave the old ones up.
        else -> {
            navigate(route) { popUpTo(existing.destination.id) { inclusive = true } }
        }
    }
}

/**
 * React Navigation's `push`: always a new entry, even for the same screen — an article
 * linking to an article must not return to the one just left.
 */
fun NavHostController.pushRoute(route: CadenceRoute) {
    navigate(route)
}

/**
 * React Navigation's `replace`: pops by destination id (identity) rather than by route
 * pattern, so the finished screen (chat thread, saved recipe) never ends up behind you.
 */
fun NavHostController.replaceRoute(route: CadenceRoute) {
    val leaving = currentBackStackEntry?.destination?.id
    navigate(route) {
        if (leaving != null) popUpTo(leaving) { inclusive = true }
    }
}

/**
 * React Navigation's `popToTop`. Returns whether it popped anything, read by [selectDestination].
 * Pops to the graph's start destination rather than a named route: the typed
 * `popBackStack(CADENCE_ROOT, …)` fills arguments into the route string first — same trap as
 * [openRoute] — so a root carrying one would return false for a root that *is* on the stack.
 */
fun NavHostController.popToTop(): Boolean = popBackStack(graph.findStartDestination().id, inclusive = false)

/** The bottom bar's `changeTab`, minus `'log'` — logging isn't a destination (see [CadenceShell]). */
fun NavHostController.selectDestination(destination: CadenceDestination) {
    // popToTop() returns false both when already on root (ordinary) and when root isn't on the
    // stack at all (can't happen today). Only the latter should stop us from navigating.
    val onRoot = currentBackStackEntry?.destination?.id == graph.findStartDestination().id
    if (!popToTop() && !onRoot) return

    // Load-bearing: without this guard, tapping «Сегодня» while on Today rebuilds the root.
    if (destination != CadenceDestination.TODAY) openRoute(destination.route)
}
