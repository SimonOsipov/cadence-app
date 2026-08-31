package app.cadence.android

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import app.cadence.App
import app.cadence.shared.auth.SessionState
import app.cadence.shared.auth.cadenceAuthFor
import app.cadence.shared.auth.sessionStates
import app.cadence.shared.net.AUTH_BASE

/**
 * Owns nothing beyond handing the window to the shared Compose tree — everything a user sees
 * lives in :composeApp.
 *
 * `cadenceAuthFor` and not `cadenceAuth`: this activity is recreated for a font-scale, density
 * or locale change, none of which `configChanges` names, and a client built per instance is a
 * second owner of one refresh token.
 */
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            val sessions = remember { cadenceAuthFor(AUTH_BASE).sessionStates() }
            val session by sessions.collectAsState(SessionState.Deciding)

            App(session)
        }
    }
}
