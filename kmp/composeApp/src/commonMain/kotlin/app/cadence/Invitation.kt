package app.cadence

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.Saver
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import app.cadence.shared.auth.Acceptance
import app.cadence.shared.auth.PasswordSet
import io.github.jan.supabase.auth.exception.AuthErrorCode
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
 * The exchange is re-run on a retry and on nothing else — not on recomposition, and not on the
 * screen being recreated. A second spend of the same token answers `otp_expired`, so a patient
 * mid-acceptance would be told the link they are holding is used up.
 */
@Composable
fun rememberInvitation(
    token: String?,
    accept: suspend (String) -> Acceptance,
    choose: suspend (String) -> PasswordSet,
): Invitation? {
    var outcome by rememberSaveable(token, stateSaver = ANSWER_SAVER) { mutableStateOf<Acceptance?>(null) }
    var problem by remember(token) { mutableStateOf<PasswordSet?>(null) }
    var busy by remember(token) { mutableStateOf(false) }
    var attempt by remember(token) { mutableIntStateOf(0) }
    // The invitation is over the moment the password is set: left on screen it strands a patient
    // on a form they have already completed, with the app they were invited to behind it.
    var finished by rememberSaveable(token) { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    // An answer already given is not asked for again: Android recreates the activity for a
    // font-scale or locale change, and asking twice answers otp_expired over the very session this
    // link created. Only a retry clears it, and that is offered only where nothing was spent.
    LaunchedEffect(token, attempt) {
        if (token == null || outcome != null) return@LaunchedEffect

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
        onRetry = {
            outcome = null
            attempt += 1
        },
    )
}

private const val ACCEPTED = "accepted"

private const val UNREACHABLE = "unreachable"

private const val REFUSED = "refused:"

/**
 * The exchange's answer as something a recreated screen can be handed back.
 *
 * The refusal's code travels with it: dropped, a spent link comes back as the sentence that
 * promises nothing, which is the one refusal a patient can do nothing about.
 */
private val ANSWER_SAVER: Saver<Acceptance?, String> =
    Saver(
        save = { answer ->
            when (answer) {
                null -> null
                Acceptance.Accepted -> ACCEPTED
                Acceptance.Unreachable -> UNREACHABLE
                is Acceptance.Refused -> REFUSED + answer.code?.name.orEmpty()
            }
        },
        restore = { saved ->
            when {
                saved == ACCEPTED -> Acceptance.Accepted
                saved == UNREACHABLE -> Acceptance.Unreachable
                saved.startsWith(REFUSED) -> refusalNamed(saved.removePrefix(REFUSED))
                else -> null
            }
        },
    )

// An empty name is a refusal that named none, and so is one the vendor has since renamed.
private fun refusalNamed(code: String) = Acceptance.Refused(AuthErrorCode.entries.firstOrNull { it.name == code })
