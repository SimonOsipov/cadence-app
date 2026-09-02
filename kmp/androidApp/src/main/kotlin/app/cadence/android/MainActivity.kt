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
     * Only when there is one: an intent carrying no data says nothing about which link the app is
     * answering.
     */
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        intent.dataString?.let { links.value = it }
    }

    // Not the link the system replayed: a task keeps its launching VIEW intent as the task's base
    // intent, and a restart from Recents redelivers it flagged — after a reboot, with the saved
    // state gone, that is a spent token arriving as a fresh one.
    private fun linkTheUserActedOn(): String? {
        val replayed = intent?.let { it.flags and Intent.FLAG_ACTIVITY_LAUNCHED_FROM_HISTORY != 0 }

        return if (replayed == false) intent?.dataString else null
    }

    private companion object {
        const val LINK = "link"
    }
}
