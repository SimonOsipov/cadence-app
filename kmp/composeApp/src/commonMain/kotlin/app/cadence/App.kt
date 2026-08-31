package app.cadence

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import app.cadence.design.CadenceTheme
import app.cadence.design.CadenceTitle
import app.cadence.shared.auth.SessionState
import app.cadence.shell.CadenceApp

/** What the pre-sign-in area shows until steps 3–5 give it its screens. */
const val SIGN_IN_MARKER: String = "Вход в Cadence"

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
    modifier: Modifier = Modifier,
) {
    CadenceTheme {
        when (session) {
            // Neither area. Rendering the sign-in screen here flashes it on every launch of a
            // signed-in app, which reads as having been signed out.
            SessionState.Deciding -> Box(modifier.fillMaxSize())

            SessionState.SignedOut -> Box(modifier.fillMaxSize()) { CadenceTitle(SIGN_IN_MARKER) }

            SessionState.SignedIn -> CadenceApp(modifier = modifier)
        }
    }
}
