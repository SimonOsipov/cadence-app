package app.cadence.android

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import app.cadence.CadenceRoot
import kotlinx.coroutines.flow.MutableStateFlow

/**
 * Owns nothing beyond handing the window and the incoming links to the shared Compose tree —
 * everything a user sees lives in :composeApp.
 */
class MainActivity : ComponentActivity() {
    private val links = MutableStateFlow<String?>(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        links.value = savedInstanceState?.getString(LINK) ?: linkTheUserActedOn()

        setContent { CadenceRoot(links) }
    }

    override fun onSaveInstanceState(outState: Bundle) {
        super.onSaveInstanceState(outState)
        outState.putString(LINK, links.value)
    }

    /**
     * The second and every later link, which is why the activity is `singleTop` — see the manifest.
     *
     * Only when there is one: every intent redelivered to a running activity arrives here, and a
     * null would drop the acceptance screen out from under a patient who has spent their token and
     * not yet chosen a password.
     */
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        intent.dataString?.let { links.value = it }
    }

    /**
     * The launch link, unless the system replayed it.
     *
     * A task the mail client started keeps that `VIEW` intent as its base intent, and restarting the
     * task from Recents hands it over again carrying `FLAG_ACTIVITY_LAUNCHED_FROM_HISTORY` — after a
     * reboot, with no saved state left, that is a spent token arriving as a fresh one. What comes
     * back is `otp_expired` over a session that same link created, on a screen offering no way out.
     */
    private fun linkTheUserActedOn(): String? {
        val replayed = intent?.flags?.and(Intent.FLAG_ACTIVITY_LAUNCHED_FROM_HISTORY) != 0

        return if (replayed) null else intent?.dataString
    }

    private companion object {
        const val LINK = "link"
    }
}
