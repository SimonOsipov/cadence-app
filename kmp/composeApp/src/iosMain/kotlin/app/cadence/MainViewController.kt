package app.cadence

import androidx.compose.ui.window.ComposeUIViewController
import kotlinx.coroutines.flow.MutableStateFlow
import platform.UIKit.UIViewController

private val links = MutableStateFlow<String?>(null)

/** The bridge iosApp's Swift host calls to get its root view controller. */
fun mainViewController(): UIViewController = ComposeUIViewController { CadenceRoot(links) }

/**
 * What the Swift host's `onOpenURL` hands over, for the launch link and every later one.
 *
 * Outside the controller because the controller is built once and the links keep arriving: a
 * parameter would only ever carry the first, and on a cold start there is none yet to carry.
 */
fun openedWith(link: String) {
    links.value = link
}
