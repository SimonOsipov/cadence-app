package app.cadence

import androidx.compose.ui.window.ComposeUIViewController
import platform.UIKit.UIViewController

/** The bridge iosApp's Swift host calls to get its root view controller. */
fun mainViewController(): UIViewController = ComposeUIViewController { App() }
