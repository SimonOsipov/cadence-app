package app.cadence

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import app.cadence.shared.auth.Acceptance
import app.cadence.shared.auth.PasswordSet
import app.cadence.shared.auth.SessionState
import app.cadence.shared.auth.acceptInvitation
import app.cadence.shared.auth.cadenceAuthFor
import app.cadence.shared.auth.invitationToken
import app.cadence.shared.auth.sessionStates
import app.cadence.shared.auth.setInvitationPassword
import app.cadence.shared.net.AUTH_BASE
import kotlinx.coroutines.flow.Flow

/**
 * Everything a platform root needs to hand over: the links it is opened with, and nothing else.
 *
 * Written once for both roots rather than in each, because nothing measures this half — composeApp
 * has no Android host-test builder, and a live client belongs to no test on either platform. What
 * it assembles is measured; this is the assembly.
 */
@Composable
fun CadenceRoot(
    links: Flow<String?>,
    modifier: Modifier = Modifier,
) {
    val client = remember { cadenceAuthFor(AUTH_BASE) }
    val sessions = remember(client) { client.sessionStates() }
    val link by links.collectAsState(null)

    CadenceRoot(
        session = sessions.collectAsSessionState(),
        link = link,
        accept = { client.acceptInvitation(it) },
        choose = { client.setInvitationPassword(it) },
        modifier = modifier,
    )
}

/**
 * The same, with the link already caught and the two writes as seams.
 *
 * A link that is not an invitation is not a refusal to show: the launcher opens the app with none
 * at all, and every other address the system may hand over is somebody else's. The token is what
 * the invitation is keyed on rather than the link — two links carry two tokens, and answering the
 * second with the first tells a patient their live invitation is used up.
 */
@Composable
fun CadenceRoot(
    session: SessionState,
    link: String?,
    accept: suspend (String) -> Acceptance,
    choose: suspend (String) -> PasswordSet,
    modifier: Modifier = Modifier,
) {
    App(
        session = session,
        invitation = rememberInvitation(link?.let(::invitationToken), accept, choose),
        modifier = modifier,
    )
}
