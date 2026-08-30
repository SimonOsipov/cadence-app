package app.cadence

import androidx.compose.runtime.remember
import androidx.compose.ui.window.ComposeUIViewController
import app.cadence.debug.CadenceDebug
import app.cadence.debug.CadenceDebugScreen
import platform.UIKit.UIViewController

/**
 * The debug screen's composition root on Apple, and the reason the framework carries the screen
 * at all: Kotlin/Native links what is reachable, so a screen nothing calls is a screen no
 * `strings` on the binary can find.
 *
 * This file is only on the compile path under `-Pcadence.debugTools`, which is the Apple stand-in
 * for Android's `debugImplementation` — there are no variants here to hang it on. Without the
 * property the framework has neither this function nor the module behind it.
 */
fun debugViewController(): UIViewController =
    ComposeUIViewController {
        val wiring = remember { CadenceDebug() }
        CadenceDebugScreen(probe = wiring::me, health = wiring::health, signIn = wiring::signIn)
    }
