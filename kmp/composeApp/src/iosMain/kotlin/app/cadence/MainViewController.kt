package app.cadence

import androidx.compose.ui.window.ComposeUIViewController
import platform.UIKit.UIViewController

/**
 * mainViewController is the bridge the Swift host calls: it wraps the shared
 * Compose tree in a UIViewController that iosApp presents as its root.
 */
fun mainViewController(): UIViewController = ComposeUIViewController { App() }
