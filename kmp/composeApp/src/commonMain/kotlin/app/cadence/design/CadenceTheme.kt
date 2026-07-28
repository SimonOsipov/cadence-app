package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Modifier

/**
 * The palette in force. staticCompositionLocalOf, not compositionLocalOf: the
 * palette does not change while the app runs, and a static local skips the
 * invalidation bookkeeping a changing one would need.
 */
val LocalCadencePalette = staticCompositionLocalOf { CadenceLightPalette }

val LocalCadenceTypography = staticCompositionLocalOf { CadenceDefaultTypography }

/**
 * CadenceTheme publishes the design tokens to everything beneath it and paints
 * the background. It replaces MaterialTheme rather than wrapping it: the
 * product has its own design language, ported from a locked prototype, and a
 * Material default leaking through would be a bug, not a fallback.
 */
@Composable
fun CadenceTheme(
    palette: CadencePalette = CadenceLightPalette,
    typography: CadenceTypography = CadenceDefaultTypography,
    content: @Composable () -> Unit,
) {
    CompositionLocalProvider(
        LocalCadencePalette provides palette,
        LocalCadenceTypography provides typography,
    ) {
        Box(Modifier.fillMaxSize().background(palette.bg)) {
            content()
        }
    }
}

/** Shorthand for the tokens in scope, so screens read `Cadence.palette.ink`. */
object Cadence {
    val palette: CadencePalette
        @Composable get() = LocalCadencePalette.current

    val typography: CadenceTypography
        @Composable get() = LocalCadenceTypography.current
}
