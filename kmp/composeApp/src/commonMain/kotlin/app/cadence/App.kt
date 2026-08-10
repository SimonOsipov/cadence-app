package app.cadence

import androidx.compose.runtime.Composable
import app.cadence.design.CadenceTheme
import app.cadence.shell.CadenceApp

/**
 * App is the single Compose entry point both platforms render.
 *
 * It was a showcase of the design system until the shell landed. Everything it
 * displayed is covered by the design system's own tests — the coverage was
 * audited component by component before the showcase was deleted, and the one
 * gap it left, `CadencePill`, got a test of its own. The app itself is where a
 * regression shows now.
 *
 * The theme is provided here rather than in [CadenceApp] because [CadenceApp]
 * is only the area after sign-in, and block 7 adds an area before it that needs
 * the same tokens.
 */
@Composable
fun App() {
    CadenceTheme {
        CadenceApp()
    }
}
