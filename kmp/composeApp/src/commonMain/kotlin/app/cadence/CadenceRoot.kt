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
import app.cadence.shared.auth.Recovery
import app.cadence.shared.auth.SessionState
import app.cadence.shared.auth.SignIn
import app.cadence.shared.auth.acceptInvitation
import app.cadence.shared.auth.acceptRecovery
import app.cadence.shared.auth.cadenceAuthFor
import app.cadence.shared.auth.choosePassword
import app.cadence.shared.auth.invitationToken
import app.cadence.shared.auth.recover
import app.cadence.shared.auth.recoveryToken
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
        acceptRecovery = { client.acceptRecovery(it) },
        recover = { client.recover(it) },
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
    acceptRecovery: suspend (String) -> Acceptance = { Acceptance.Unreachable },
    recover: suspend (String) -> Recovery = { Recovery.Unreachable },
    modifier: Modifier = Modifier,
) {
    val link by links.collectAsState(null)
    val scope = rememberCoroutineScope()
    // One slot for both links rather than one each, and it cost a review round to see why: two
    // slots are both filled once a patient has followed both, and the driver of the one the screen
    // does not show still spends its token — silently, single-use, with nothing ever drawn for it.
    //
    // Saveable, and that is the whole of why the invitation's own saved state works: the link
    // arrives through a flow, so the frame a recreated screen comes back on has none, and the token
    // is the reset input of everything keyed on it. Measured — held in a plain remember, the
    // exchange ran a second time.
    var landed by rememberSaveable { mutableStateOf<String?>(null) }

    LaunchedEffect(link) {
        link?.let(::invitationToken)?.let { landed = AN_INVITATION + it }
        link?.let(::recoveryToken)?.let { landed = A_RECOVERY + it }
    }

    App(
        session = session,
        invitation = rememberInvitation(landed.tokenOf(AN_INVITATION), accept, choose),
        recoveryReturn = rememberInvitation(landed.tokenOf(A_RECOVERY), acceptRecovery, choose),
        signIn = rememberSignIn(signIn),
        recovery = rememberRecovery(recover),
        onSignOut = { scope.launch { signOut() } },
        modifier = modifier,
    )
}

// Which link the app is answering, kept as one string because that is what survives a recreation
// without a saver of its own. The tokens are hex, so neither tag can appear inside one.
private const val AN_INVITATION = "invite:"

private const val A_RECOVERY = "recover:"

private fun String?.tokenOf(kind: String) = this?.takeIf { it.startsWith(kind) }?.removePrefix(kind)
