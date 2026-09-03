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
import app.cadence.shared.auth.SignIn

/**
 * The way back in for a patient who has a password — which, since accepting an invitation
 * requires setting one, is every patient the clinic has invited.
 */
@Composable
fun SignInScreen(
    onSignIn: (String, String) -> Unit,
    onForgot: () -> Unit,
    problem: SignIn? = null,
    busy: Boolean = false,
    modifier: Modifier = Modifier,
) {
    var address by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }

    Column(
        modifier = modifier.fillMaxSize().padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp, Alignment.CenterVertically),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        CadenceTitle(SignInCopy.TITLE)

        // Named rather than found by their text: an empty field carries none, and a screen reader
        // needs the name anyway.
        CadenceTextField(
            value = address,
            onValueChange = { address = it },
            placeholder = SignInCopy.ADDRESS_FIELD,
            fieldModifier = Modifier.semantics { contentDescription = SignInCopy.ADDRESS_FIELD },
            singleLine = true,
        )
        CadenceTextField(
            value = password,
            onValueChange = { password = it },
            placeholder = SignInCopy.PASSWORD_FIELD,
            fieldModifier = Modifier.semantics { contentDescription = SignInCopy.PASSWORD_FIELD },
            singleLine = true,
            masked = true,
        )

        if (problem != null) {
            CadenceTitle(titleFor(problem))
            CadenceBody(hintFor(problem))
        }

        CadenceButton(
            label = SignInCopy.ENTER,
            onClick = { onSignIn(address, password) },
            // Not «valid»: an address this screen judges is an address the server never sees, and
            // the shapes it would reject are the server's to name. Only «typed at all».
            enabled = address.isNotBlank() && password.isNotEmpty() && !busy,
        )
        CadenceButton(label = RecoveryCopy.FORGOT, onClick = onForgot)
    }
}

// Exhaustive rather than `else`: a fourth answer added later would otherwise compile straight into
// the refusal copy, which is the direction this screen calls the expensive mistake. Accepted never
// reaches here — the driver stores only what is not it — and the type is what still permits it.
private fun titleFor(problem: SignIn) =
    when (problem) {
        SignIn.Unreachable -> SignInCopy.OFFLINE
        SignIn.Refused, SignIn.Accepted -> SignInCopy.REFUSED
    }

private fun hintFor(problem: SignIn) =
    when (problem) {
        SignIn.Unreachable -> SignInCopy.OFFLINE_HINT
        SignIn.Refused, SignIn.Accepted -> SignInCopy.REFUSED_HINT
    }
