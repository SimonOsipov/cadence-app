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
 * The exchange is run once per token, and again only on a retry — not on recomposition and not on
 * the screen being recreated, mid-flight included. A second spend of one token answers
 * `otp_expired`, so a patient mid-acceptance would be told the link they are holding is used up.
 */
@Composable
fun rememberInvitation(
    token: String?,
    accept: suspend (String) -> Acceptance,
    choose: suspend (String) -> PasswordSet,
): Invitation? {
    var outcome by rememberSaveable(token, stateSaver = ANSWER_SAVER) { mutableStateOf<Acceptance?>(null) }
    var asked by rememberSaveable(token) { mutableStateOf(false) }
    var problem by remember(token) { mutableStateOf<PasswordSet?>(null) }
    var busy by remember(token) { mutableStateOf(false) }
    var attempt by remember(token) { mutableIntStateOf(0) }
    // The invitation is over the moment the password is set: left on screen it strands a patient
    // on a form they have already completed, with the app they were invited to behind it.
    var finished by rememberSaveable(token) { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    // Neither an answer already given nor an ask already made is repeated: Android recreates the
    // activity for a font-scale or locale change, and a second spend answers otp_expired over the
    // very session the first one created. Only a retry asks again for the same token.
    LaunchedEffect(token, attempt) {
        if (token == null || outcome != null) return@LaunchedEffect

        if (asked) {
            // Started by a composition that is gone, and whether it reached the server cannot be
            // known here. Unreachable is the honest shape of that: the one answer whose screen
            // offers another try, which is the patient's to make rather than this line's.
            outcome = Acceptance.Unreachable
            return@LaunchedEffect
        }

        asked = true
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
            asked = false
            attempt += 1
        },
    )
}

private const val ACCEPTED = "accepted"

private const val REFUSED = "refused:"

/**
 * The exchange's answer as something a recreated screen can be handed back.
 *
 * The refusal's code travels with it: dropped, a spent link comes back as the refusal that names
 * no reason, and only the named one can say that a new invitation would work.
 */
private val ANSWER_SAVER: Saver<Acceptance?, String> =
    Saver(
        save = { answer ->
            when (answer) {
                Acceptance.Accepted -> ACCEPTED

                is Acceptance.Refused -> REFUSED + answer.code?.name.orEmpty()

                // Neither is saved, and one line covers both: an answer that never arrived is not
                // one to hand back, and `asked` answers a recreation before the saved value is
                // read — measured, the mutation dropping an Unreachable branch here survived.
                null, Acceptance.Unreachable -> null
            }
        },
        restore = { saved ->
            when {
                saved == ACCEPTED -> Acceptance.Accepted
                saved.startsWith(REFUSED) -> refusalNamed(saved.removePrefix(REFUSED))
                else -> null
            }
        },
    )

// An empty name is a refusal that named none — what [Acceptance.Refused] carries for one.
private fun refusalNamed(code: String) = Acceptance.Refused(AuthErrorCode.entries.firstOrNull { it.name == code })
