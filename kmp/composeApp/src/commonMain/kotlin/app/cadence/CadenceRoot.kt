package app.cadence

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import app.cadence.shared.auth.Acceptance
import app.cadence.shared.auth.PasswordSet
import app.cadence.shared.auth.SessionState
import app.cadence.shared.auth.SignIn
import app.cadence.shared.auth.acceptInvitation
import app.cadence.shared.auth.cadenceAuthFor
import app.cadence.shared.auth.choosePassword
import app.cadence.shared.auth.invitationToken
import app.cadence.shared.auth.sessionStates
import app.cadence.shared.auth.signIn
import app.cadence.shared.auth.signOut
import app.cadence.shared.net.AUTH_BASE
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.launch

/** Everything a platform root has to hand over: the links it is opened with, and nothing else. */
@Composable
fun CadenceRoot(
    links: Flow<String?>,
    modifier: Modifier = Modifier,
) {
    val client = remember { cadenceAuthFor(AUTH_BASE) }
    val sessions = remember(client) { client.sessionStates() }

    CadenceRoot(
        session = sessions.collectAsSessionState(),
        links = links,
        accept = { client.acceptInvitation(it) },
        choose = { client.choosePassword(it) },
        signIn = { address, password -> client.signIn(address, password) },
        signOut = { client.signOut() },
        modifier = modifier,
    )
}

/**
 * The same with the client's calls as seams — the whole of what a test can reach here.
 *
 * A link that is not an invitation does not replace the one being answered: the roots hear every
 * address the patient opened the app with, and dropping the screen for one of them strands someone
 * who has spent their token and not yet chosen a password.
 */
@Composable
fun CadenceRoot(
    session: SessionState,
    links: Flow<String?>,
    accept: suspend (String) -> Acceptance,
    choose: suspend (String) -> PasswordSet,
    signIn: suspend (String, String) -> SignIn = { _, _ -> SignIn.Unreachable },
    signOut: suspend () -> Unit = { },
    modifier: Modifier = Modifier,
) {
    val link by links.collectAsState(null)
    val scope = rememberCoroutineScope()
    // The token is the reset input of everything the invitation saves, and the flow leaves it null
    // for frame one: the restore lands, then the token arrives and resets it. Measured — held in a
    // plain remember, the exchange ran a second time.
    var token by rememberSaveable { mutableStateOf<String?>(null) }

    LaunchedEffect(link) {
        link?.let(::invitationToken)?.let { token = it }
    }

    App(
        session = session,
        invitation = rememberInvitation(token, accept, choose),
        signIn = rememberSignIn(signIn),
        onSignOut = { scope.launch { signOut() } },
        modifier = modifier,
    )
}
