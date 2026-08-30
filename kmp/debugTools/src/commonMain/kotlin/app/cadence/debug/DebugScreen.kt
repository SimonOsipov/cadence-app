package app.cadence.debug

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

/** The class the release artifact is grepped for; see the acceptance in `scripts/gate/kmp.sh`. */
const val DEBUG_SCREEN_MARKER: String = "CadenceDebugScreen"

/**
 * The live answers from the dev contour, and the only thing in this module a person sees.
 *
 * Not translated: this is the one surface no patient reaches, and the strings it prints are the
 * API's own. Everything a patient can see is Russian, and a rule in the gate enforces that on
 * the modules a patient can reach.
 */
@Composable
fun CadenceDebugScreen(
    probe: suspend () -> ProbeState,
    health: suspend () -> Boolean,
    modifier: Modifier = Modifier,
) {
    var state by remember { mutableStateOf<ProbeState?>(null) }
    var up by remember { mutableStateOf<Boolean?>(null) }

    LaunchedEffect(Unit) {
        up = health()
        state = probe()
    }

    Column(modifier.fillMaxSize().padding(16.dp)) {
        BasicText("healthz: " + (up?.let { if (it) "up" else "no answer" } ?: "…"))
        BasicText(
            "GET /v1/me: " +
                when (val answer = state) {
                    null -> "…"
                    is ProbeState.SignedIn -> "signed in — ${answer.body}"
                    is ProbeState.Unavailable -> "unavailable — ${answer.why}"
                    ProbeState.SignedOut -> "signed out — the token was refused and not renewed"
                },
        )
    }
}
