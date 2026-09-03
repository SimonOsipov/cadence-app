package app.cadence

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceButton
import app.cadence.design.CadenceTextField
import app.cadence.design.CadenceTitle
import app.cadence.shared.auth.Acceptance
import app.cadence.shared.auth.PasswordSet
import io.github.jan.supabase.auth.exception.AuthErrorCode

/**
 * The invitation, from the link to a password.
 *
 * [outcome] is null while the exchange is in flight; the platform root drives it, exactly as it
 * drives the session — anything decided in this file is measured on one platform only, because
 * `composeApp` has no Android host-test builder.
 */
@Composable
fun AcceptanceScreen(
    outcome: Acceptance?,
    onPasswordChosen: (String) -> Unit,
    onRetry: () -> Unit,
    problem: PasswordSet? = null,
    busy: Boolean = false,
    words: PasswordWords = PasswordWords.OfAnInvitation,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier.fillMaxSize().padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp, Alignment.CenterVertically),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        when (outcome) {
            null -> {
                CadenceTitle(words.checking)
            }

            Acceptance.Accepted -> {
                PasswordForm(onPasswordChosen, problem, busy, words)
            }

            Acceptance.Unreachable -> {
                CadenceTitle(AcceptanceCopy.OFFLINE)
                CadenceBody(AcceptanceCopy.OFFLINE_HINT)
                CadenceButton(label = AcceptanceCopy.RETRY, onClick = onRetry)
            }

            is Acceptance.Refused -> {
                CadenceTitle(titleFor(outcome.code, words))
                CadenceBody(hintFor(outcome.code, words))
            }
        }
    }
}

// The provider's own floor, not ours, and the sentence says so: this arrives when the deployment
// raised its minimum without the app being rebuilt, so repeating our number would be wrong.
private fun problemText(problem: PasswordSet) =
    when {
        problem is PasswordSet.Refused && problem.code == AuthErrorCode.WeakPassword -> {
            AcceptanceCopy.TOO_WEAK
        }

        problem is PasswordSet.Unreachable -> {
            AcceptanceCopy.OFFLINE_HINT
        }

        else -> {
            AcceptanceCopy.UNNAMED
        }
    }

private fun titleFor(
    code: AuthErrorCode?,
    words: PasswordWords,
) = when (code) {
    AuthErrorCode.OtpExpired -> AcceptanceCopy.SPENT
    AuthErrorCode.UserBanned -> AcceptanceCopy.BANNED
    else -> words.unnamed
}

private fun hintFor(
    code: AuthErrorCode?,
    words: PasswordWords,
) = when (code) {
    AuthErrorCode.OtpExpired -> words.spentHint
    AuthErrorCode.UserBanned -> AcceptanceCopy.BANNED_HINT
    else -> words.unnamedHint
}

@Composable
private fun PasswordForm(
    onChosen: (String) -> Unit,
    problem: PasswordSet?,
    busy: Boolean,
    words: PasswordWords,
) {
    var password by remember { mutableStateOf("") }

    CadenceTitle(words.choosing)
    CadenceBody(AcceptanceCopy.PASSWORD_HINT)
    CadenceTextField(
        value = password,
        onValueChange = { password = it },
        placeholder = AcceptanceCopy.PASSWORD_FIELD,
        // The field carries no text of its own while empty, so it is named rather than found by
        // one — which a screen reader needs anyway.
        fieldModifier = Modifier.semantics { contentDescription = AcceptanceCopy.PASSWORD_FIELD },
        singleLine = true,
        masked = true,
    )
    // Required, not requested: an invitation completed without one leaves the patient depending
    // on email every time the session is lost, which is the whole reason the spec was reversed.
    // Held to the length the server holds it to, so the refusal arrives before the typing rather
    // than after — measured, GoTrue answers 422 weak_password with reasons ["length"].
    if (problem != null) CadenceBody(problemText(problem))

    CadenceButton(
        label = AcceptanceCopy.ENTER,
        onClick = { onChosen(password) },
        enabled = password.length >= AcceptanceCopy.PASSWORD_MIN_LENGTH && !busy,
    )
}
