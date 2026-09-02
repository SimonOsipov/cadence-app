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

        // The intent on a first creation, what was kept on every later one. A recreated activity is
        // handed the same VIEW intent again, and the composition keeps its answer to a link under
        // that link — so the link is what has to come back, not the intent it arrived in.
        links.value =
            if (savedInstanceState == null) {
                intent?.dataString
            } else {
                savedInstanceState.getString(LINK)
            }

        setContent { CadenceRoot(links) }
    }

    override fun onSaveInstanceState(outState: Bundle) {
        super.onSaveInstanceState(outState)
        outState.putString(LINK, links.value)
    }

    /**
     * The second and every later link, which is why the activity is `singleTop`: without it the
     * system builds another copy of this one and the running tree never hears about the link.
     */
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        links.value = intent.dataString
    }

    private companion object {
        const val LINK = "link"
    }
}
