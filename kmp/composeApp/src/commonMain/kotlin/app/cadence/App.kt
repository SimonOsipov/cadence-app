package app.cadence

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import app.cadence.design.CadenceTheme
import app.cadence.shared.auth.SessionState
import app.cadence.shell.CadenceApp
import kotlinx.coroutines.flow.Flow

/**
 * The two areas, and the whole of the transition between them.
 *
 * The theme is provided here rather than in [CadenceApp] because [CadenceApp] is only the area
 * after sign-in, and the area before it needs the same tokens.
 *
 * **The areas are alternatives, not destinations in one graph.** That is what closes the three
 * leak paths without guarding a single route: composed as a `when`, the area after sign-in is
 * absent from the tree for a patient without a session, so there is nothing to navigate to,
 * nothing on a back stack to return into, and nothing for a deep link to resolve against. A
 * guard on each route would have to be right every time a route is added.
 *
 * [session] is derived in `:shared` rather than here: `composeApp` has no Android host-test
 * builder, so anything decided in this file is measured on one platform only.
 */
@Composable
fun App(
    session: SessionState,
    invitation: Invitation? = null,
    signIn: SignInPrompt = SignInPrompt(),
    onSignOut: () -> Unit = { },
    modifier: Modifier = Modifier,
) {
    CadenceTheme {
        // An invitation outranks the session, and deliberately: a patient who followed a link is
        // answering it, and dropping them into whichever area their session names would leave the
        // link unexplained — including the case where it is the reason they have no session.
        if (invitation != null) {
            AcceptanceScreen(
                outcome = invitation.outcome,
                onPasswordChosen = invitation.onPasswordChosen,
                onRetry = invitation.onRetry,
                problem = invitation.problem,
                busy = invitation.busy,
                modifier = modifier,
            )

            return@CadenceTheme
        }

        when (session) {
            // Neither area, and not nothing: rendering the sign-in screen here flashes it on
            // every launch of a signed-in app, and rendering an empty box leaves a patient
            // looking at a blank one for a whole round trip.
            SessionState.Deciding -> {
                CadenceSplash(modifier)
            }

            SessionState.SignedOut -> {
                SignInScreen(
                    onSignIn = signIn.onSignIn,
                    problem = signIn.problem,
                    busy = signIn.busy,
                    modifier = modifier,
                )
            }

            SessionState.SignedIn -> {
                CadenceApp(modifier = modifier, onSignOut = onSignOut)
            }
        }
    }
}

/**
 * [SessionState.Deciding] until the stream speaks, and chosen here rather than at each root:
 * the value before the first emission is what decides whether a signed-in launch flashes the
 * sign-in screen, and a literal written once per root is a literal nothing measures.
 */
@Composable
fun Flow<SessionState>.collectAsSessionState(): SessionState {
    val session by collectAsState(SessionState.Deciding)

    return session
}
