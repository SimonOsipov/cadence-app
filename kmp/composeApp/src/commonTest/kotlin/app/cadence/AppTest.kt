package app.cadence

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.shared.currentPlatform
import kotlin.test.Test

@OptIn(ExperimentalTestApi::class)
class AppTest {
    @Test
    fun appShowsTheBrand() = runComposeUiTest {
        setContent { App() }

        onNodeWithText("Cadence").assertIsDisplayed()
    }

    @Test
    fun appShowsThePlatformItRunsOn() = runComposeUiTest {
        setContent { App() }

        // Proves :shared is linked into the UI, not just into the module graph.
        onNodeWithText(currentPlatform().name).assertIsDisplayed()
    }
}
