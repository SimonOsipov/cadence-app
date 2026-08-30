package app.cadence.android

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.runtime.remember
import app.cadence.debug.CadenceDebug
import app.cadence.debug.CadenceDebugScreen

/**
 * The debug screen's composition root on Android, and the reason the screen is reachable rather
 * than merely compiled.
 *
 * It lives in `src/debug` — the variant `debugImplementation(project(":debugTools"))` matches —
 * so both the class and its manifest entry are absent from a release APK by construction rather
 * than by a rule somebody has to remember. The gate greps both artifacts for the screen.
 *
 * `exported=false`: reachable with `adb shell am start -n app.cadence/.DebugActivity`, and by
 * nothing else on the device. The screen signs a real account in against the dev contour.
 */
class DebugActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            val wiring = remember { CadenceDebug() }
            CadenceDebugScreen(probe = wiring::me, health = wiring::health, signIn = wiring::signIn)
        }
    }
}
