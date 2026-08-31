package app.cadence

import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.window.ComposeUIViewController
import app.cadence.shared.auth.SessionState
import app.cadence.shared.auth.cadenceAuthFor
import app.cadence.shared.auth.sessionStates
import app.cadence.shared.net.AUTH_BASE
import platform.UIKit.UIViewController

/** The bridge iosApp's Swift host calls to get its root view controller. */
fun mainViewController(): UIViewController =
    ComposeUIViewController {
        val sessions = remember { cadenceAuthFor(AUTH_BASE).sessionStates() }
        val session by sessions.collectAsState(SessionState.Deciding)

        App(session)
    }
