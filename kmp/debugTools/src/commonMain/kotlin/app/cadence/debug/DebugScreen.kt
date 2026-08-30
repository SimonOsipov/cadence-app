package app.cadence.debug

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.BasicText
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch

/** The class the release artifact is grepped for; see the acceptance in `scripts/gate/kmp.sh`. */
const val DEBUG_SCREEN_MARKER: String = "CadenceDebugScreen"

/**
 * The live answers from the dev contour, and the only thing in this module a person sees.
 *
 * Sign-in is on the screen rather than in a build property because the three states are reached
 * by moving between them: a developer signs in, sees the token work, and is the one who decides
 * which account to try. A credential compiled into the module would be a credential in the
 * artifact.
 *
 * Nothing is asked on composition. The probes run when a button is pressed, so a screen that is
 * merely open does not talk to the contour — and so the effect cannot outlive the press.
 *
 * Not translated: this is the one surface no patient reaches, and the strings it prints are the
 * API's own. Everything a patient can see is Russian, and a rule in the gate enforces that on
 * the modules a patient can reach.
 */
@Composable
fun CadenceDebugScreen(
    probe: suspend () -> ProbeState,
    health: suspend () -> Boolean,
    signIn: suspend (String, String) -> SignIn,
    modifier: Modifier = Modifier,
) {
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var attempt by remember { mutableStateOf<SignIn>(SignIn.Untried) }
    var state by remember { mutableStateOf<ProbeState?>(null) }
    var up by remember { mutableStateOf<Boolean?>(null) }
    val scope = rememberCoroutineScope()

    Column(modifier.fillMaxSize().padding(16.dp)) {
        Field(email, "email", onChange = { email = it })
        Field(password, "password", secret = true, onChange = { password = it })

        Row {
            Button("sign in") {
                scope.launch { attempt = signIn(email, password) }
            }
            Button("ask") {
                scope.launch {
                    up = health()
                    state = probe()
                }
            }
        }

        BasicText(
            "sign-in: " +
                when (val outcome = attempt) {
                    SignIn.Untried -> "not tried"
                    SignIn.Accepted -> "accepted"
                    is SignIn.Refused -> "refused — ${outcome.why}"
                },
        )
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

@Composable
private fun Field(
    value: String,
    label: String,
    secret: Boolean = false,
    onChange: (String) -> Unit,
) {
    Row(Modifier.fillMaxWidth().padding(vertical = 4.dp)) {
        BasicText("$label: ")
        BasicTextField(
            value = value,
            onValueChange = onChange,
            visualTransformation = if (secret) PasswordVisualTransformation() else VisualTransformation.None,
            modifier = Modifier.fillMaxWidth().background(Color(0x11000000)),
        )
    }
}

@Composable
private fun Button(
    label: String,
    onClick: () -> Unit,
) {
    BasicText(
        "[ $label ]",
        modifier = Modifier.clickable(onClick = onClick).padding(8.dp),
    )
}
