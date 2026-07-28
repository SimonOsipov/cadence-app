package app.cadence.android

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import app.cadence.App

/**
 * MainActivity is the whole Android host: it hands the window to the shared
 * Compose tree and owns nothing else. Everything a user sees lives in
 * :composeApp, so the two platforms cannot drift apart.
 */
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent { App() }
    }
}
