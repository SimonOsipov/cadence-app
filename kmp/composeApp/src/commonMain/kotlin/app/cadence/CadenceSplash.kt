package app.cadence

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceTitle

/** What the splash shows, and what a test asks for it by. */
const val SPLASH_MARKER: String = "Cadence"

/**
 * Shown while neither area has been chosen.
 *
 * Not a frame between two screens: measured, a patient holding a session waits here for a network
 * round trip on any cold start past the vendor's refresh threshold, so this is a screen people see
 * on an ordinary launch. It says the app's name and nothing that would be wrong a moment later —
 * «signing in» would be a lie to somebody about to be sent to the sign-in screen.
 */
@Composable
fun CadenceSplash(modifier: Modifier = Modifier) {
    Column(
        modifier = modifier.fillMaxSize(),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        CadenceTitle(SPLASH_MARKER)
        CadenceEyebrow("клиника")
    }
}
