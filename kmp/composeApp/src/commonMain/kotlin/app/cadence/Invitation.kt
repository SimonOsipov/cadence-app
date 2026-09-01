package app.cadence

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import app.cadence.shared.auth.Acceptance
import app.cadence.shared.auth.PasswordSet
import kotlinx.coroutines.launch

/**
 * An invitation being accepted: what to show, and what the screen calls back into.
 *
 * Its presence is what says the app is accepting one, and [outcome] being null is what says the
 * exchange is still in flight. One value rather than a flag beside a nullable answer, which would
 * let «accepting, and no answer either way» be written twice over.
 */
data class Invitation(
    val outcome: Acceptance?,
    val problem: PasswordSet? = null,
    val busy: Boolean = false,
    val onPasswordChosen: (String) -> Unit = {},
    val onRetry: () -> Unit = {},
)

/**
 * Drives one invitation from the link to a password.
 *
 * Both calls arrive as seams rather than a client, which is what lets this be measured at all:
 * `composeApp` has no Android host-test builder, so anything decided here runs on the iOS target
 * only, and a driver that needed a live GoTrue would run nowhere.
 *
 * The exchange is re-run on [retry] and not on recomposition: [token] alone as the key would spend
 * the same token twice on any change around it, and the second spend answers `otp_expired` — the
 * screen would then tell a patient mid-acceptance that their link was used up.
 */
@Composable
fun rememberInvitation(
    token: String?,
    accept: suspend (String) -> Acceptance,
    choose: suspend (String) -> PasswordSet,
): Invitation? {
    var outcome by remember(token) { mutableStateOf<Acceptance?>(null) }
    var problem by remember(token) { mutableStateOf<PasswordSet?>(null) }
    var busy by remember(token) { mutableStateOf(false) }
    var attempt by remember(token) { mutableIntStateOf(0) }
    // The invitation is over the moment the password is set: left on screen it strands a patient
    // on a form they have already completed, with the app they were invited to behind it.
    var finished by remember(token) { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    LaunchedEffect(token, attempt) {
        if (token == null) return@LaunchedEffect

        outcome = null
        outcome = accept(token)
    }

    if (token == null || finished) return null

    return Invitation(
        outcome = outcome,
        problem = problem,
        busy = busy,
        onPasswordChosen = { password ->
            // Guarded, because a second tap while the first is in flight sets the password twice
            // and the patient watches a form that looks stuck.
            if (!busy) {
                busy = true
                scope.launch {
                    problem = null

                    val answer = choose(password)

                    problem = answer.takeIf { it != PasswordSet.Done }
                    finished = answer == PasswordSet.Done
                    busy = false
                }
            }
        },
        onRetry = { attempt += 1 },
    )
}
