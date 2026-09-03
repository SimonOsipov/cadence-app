package app.cadence

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import app.cadence.shared.auth.Recovery
import kotlinx.coroutines.launch

/** What the recovery screen shows, and what it calls back into. */
data class RecoveryPrompt(
    val outcome: Recovery? = null,
    val busy: Boolean = false,
    val onRecover: (String) -> Unit = { },
)

/** Drives one request for a recovery mail at a time — see [rememberSignIn] for the shape. */
@Composable
fun rememberRecovery(recover: suspend (String) -> Recovery): RecoveryPrompt {
    var outcome by remember { mutableStateOf<Recovery?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    return RecoveryPrompt(
        outcome = outcome,
        busy = busy,
        onRecover = { address ->
            // Guarded like the sign-in's: a second tap here spends the per-address gap, and the
            // patient is then told to wait a minute for a letter their own second tap delayed.
            if (!busy) {
                busy = true
                scope.launch {
                    outcome = null

                    try {
                        outcome = recover(address)
                    } finally {
                        busy = false
                    }
                }
            }
        },
    )
}
