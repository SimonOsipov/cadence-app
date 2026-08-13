package app.cadence

import androidx.compose.runtime.Composable
import app.cadence.design.CadenceTheme
import app.cadence.shell.CadenceApp

/**
 * The single Compose entry point both platforms render. The theme is provided here rather than
 * in [CadenceApp] because [CadenceApp] is only the area after sign-in, and block 7 adds an area
 * before it that needs the same tokens.
 */
@Composable
fun App() {
    CadenceTheme {
        CadenceApp()
    }
}
