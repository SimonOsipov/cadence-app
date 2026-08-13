package app.cadence.shell

import androidx.compose.animation.AnimatedContentTransitionScope
import androidx.compose.animation.EnterTransition
import androidx.compose.animation.ExitTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.slideOutVertically
import androidx.navigation.NavBackStackEntry

/** `animationDuration: 380` on the prototype's stack — both push and modal. */
const val CADENCE_PUSH_DURATION_MS: Int = 380

/** How far the outgoing screen trails the incoming one. iOS's own parallax. */
private const val PARALLAX_DIVISOR = 3

// Compose has no native-stack preset, so the prototype's slide_from_right/slide_from_bottom
// are spelled out below; the parallax is what a native iOS push does under them.

internal fun AnimatedContentTransitionScope<NavBackStackEntry>.pushEnter(): EnterTransition =
    slideInHorizontally(tween(CADENCE_PUSH_DURATION_MS)) { it }

internal fun AnimatedContentTransitionScope<NavBackStackEntry>.pushExit(): ExitTransition =
    slideOutHorizontally(tween(CADENCE_PUSH_DURATION_MS)) { -it / PARALLAX_DIVISOR }

internal fun AnimatedContentTransitionScope<NavBackStackEntry>.popEnter(): EnterTransition =
    slideInHorizontally(tween(CADENCE_PUSH_DURATION_MS)) { -it / PARALLAX_DIVISOR }

internal fun AnimatedContentTransitionScope<NavBackStackEntry>.popExit(): ExitTransition =
    slideOutHorizontally(tween(CADENCE_PUSH_DURATION_MS)) { it }

internal fun AnimatedContentTransitionScope<NavBackStackEntry>.modalEnter(): EnterTransition =
    slideInVertically(tween(CADENCE_PUSH_DURATION_MS)) { it }

internal fun AnimatedContentTransitionScope<NavBackStackEntry>.modalExit(): ExitTransition =
    slideOutVertically(tween(CADENCE_PUSH_DURATION_MS)) { it }

/**
 * A native `fullScreenModal` leaves the screen beneath exactly where it was; NavHost's
 * defaults would slide it a third of the width sideways.
 */
internal fun modalUnderlayEnter(): EnterTransition = EnterTransition.None

internal fun modalUnderlayExit(): ExitTransition = ExitTransition.None
