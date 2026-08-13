package app.cadence.android

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import app.cadence.App

/**
 * Owns nothing beyond handing the window to the shared Compose tree — everything a user sees
 * lives in :composeApp.
 */
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent { App() }
    }
}
