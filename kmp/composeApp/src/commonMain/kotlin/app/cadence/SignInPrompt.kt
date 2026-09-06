package app.cadence

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import app.cadence.shared.auth.SignIn
import kotlinx.coroutines.launch

/** What the sign-in screen shows, and what it calls back into. */
data class SignInPrompt(
    val problem: SignIn? = null,
    val busy: Boolean = false,
    val onSignIn: (String, String) -> Unit = { _, _ -> },
)

/**
 * Drives one sign-in attempt at a time.
 *
 * The call arrives as a seam rather than a client for [rememberInvitation]'s reason: `composeApp`
 * has no Android host-test builder, so a driver that needed a live GoTrue would run nowhere.
 */
@Composable
fun rememberSignIn(signIn: suspend (String, String) -> SignIn): SignInPrompt {
    var problem by remember { mutableStateOf<SignIn?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    return SignInPrompt(
        problem = problem,
        busy = busy,
        onSignIn = { address, password ->
            // Guarded, because a second tap while the first is in flight spends the rate limit on
            // the same credentials and the patient watches a form that looks stuck.
            if (!busy) {
                busy = true
                scope.launch {
                    problem = null

                    // In a finally for the cancellation-shaped escape only: `signIn` rethrows
                    // CancellationException, which completes this coroutine without touching the
                    // composition, and a busy left set is then a dead button with nothing written
                    // under it. An ordinary throw past the client's catches is not that case — the
                    // scope's job is a plain child of the recomposer's, so it takes the whole tree
                    // down and there is no frame to be dead in.
                    try {
                        val answer = signIn(address, password)

                        problem = answer.takeIf { it != SignIn.Accepted }
                    } finally {
                        busy = false
                    }
                }
            }
        },
    )
}
