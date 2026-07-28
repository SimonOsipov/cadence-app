package app.cadence.design

import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/** Corner radii, ported from `R` in mobile/src/theme/index.ts. */
object CadenceRadius {
    val xs: Dp = 4.dp
    val sm: Dp = 8.dp
    val md: Dp = 12.dp
    val lg: Dp = 18.dp
    val xl: Dp = 24.dp
    val xxl: Dp = 32.dp

    /** Fully rounded. The prototype writes 999; any value past half the height does. */
    val pill: Dp = 999.dp
}

/**
 * The spacing scale. The prototype has no named scale — it writes the numbers
 * inline — so this is the set those numbers actually fall into, collected once
 * here rather than re-typed on every screen.
 */
object CadenceSpacing {
    val xxs: Dp = 4.dp
    val xs: Dp = 6.dp
    val sm: Dp = 8.dp
    val md: Dp = 12.dp
    val lg: Dp = 16.dp
    val xl: Dp = 20.dp
    val xxl: Dp = 24.dp
    val huge: Dp = 32.dp
}

/**
 * Elevation steps, ported from `SHADOW`. The prototype uses a warm umbra
 * (rgba(46,38,24,…)); Compose's shadow takes an elevation and derives the rest
 * per platform, so the tint does not survive the port. What survives is the
 * step: xs barely lifts, lg is a sheet floating over a scrim.
 */
object CadenceElevation {
    val none: Dp = 0.dp
    val xs: Dp = 1.dp
    val sm: Dp = 2.dp
    val md: Dp = 6.dp
    val lg: Dp = 12.dp
}
