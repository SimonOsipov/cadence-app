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
import app.cadence.shared.auth.Recovery

/**
 * Asking for a recovery mail.
 *
 * The address is trimmed on its way out: a keyboard's trailing space is refused by the provider,
 * every refusal here is answered «sent» by design, and the form is gone by then — so the one
 * mistake a patient cannot see is also the one they could not correct.
 *
 * The form goes only once the letter is on its way — [Recovery.Sent] is the answer to every
 * address, including one the clinic has never seen, so leaving the form open invites a patient to
 * type it again and spend the per-address gap on a letter they already have.
 */
@Composable
fun RecoveryScreen(
    onRecover: (String) -> Unit,
    onBack: () -> Unit,
    outcome: Recovery? = null,
    busy: Boolean = false,
    modifier: Modifier = Modifier,
) {
    var address by remember { mutableStateOf("") }

    Column(
        modifier = modifier.fillMaxSize().padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp, Alignment.CenterVertically),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        CadenceTitle(RecoveryCopy.TITLE)

        if (outcome != null) {
            CadenceTitle(titleFor(outcome))
            CadenceBody(hintFor(outcome))
        }

        if (outcome != Recovery.Sent) {
            CadenceTextField(
                value = address,
                onValueChange = { address = it },
                placeholder = RecoveryCopy.ADDRESS_FIELD,
                fieldModifier = Modifier.semantics { contentDescription = RecoveryCopy.ADDRESS_FIELD },
                singleLine = true,
            )
            CadenceButton(
                label = RecoveryCopy.SEND,
                onClick = { onRecover(address.trim()) },
                enabled = address.isNotBlank() && !busy,
            )
        }

        CadenceButton(label = RecoveryCopy.BACK, onClick = onBack)
    }
}

private fun titleFor(outcome: Recovery) =
    when (outcome) {
        Recovery.Sent -> RecoveryCopy.SENT
        Recovery.Unreachable -> RecoveryCopy.OFFLINE
    }

private fun hintFor(outcome: Recovery) =
    when (outcome) {
        Recovery.Sent -> RecoveryCopy.SENT_HINT
        Recovery.Unreachable -> RecoveryCopy.OFFLINE_HINT
    }
