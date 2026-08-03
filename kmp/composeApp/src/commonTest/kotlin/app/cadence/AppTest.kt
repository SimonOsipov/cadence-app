package app.cadence

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.shared.currentPlatform
import kotlin.test.Test
import kotlin.test.assertTrue

@OptIn(ExperimentalTestApi::class)
class AppTest {
    @Test
    fun theAppOpensOnToday() =
        runComposeUiTest {
            setContent { App() }

            onNodeWithText("Экран «Сегодня»").assertIsDisplayed()
            onNodeWithText("Сегодня").assertIsSelected()
        }

    @Test
    fun theAppShowsThePlatformItRunsOn() =
        runComposeUiTest {
            setContent { App() }

            // Proves :shared is linked into the UI and not merely into the
            // module graph. It rides on the placeholder's footer today; see the
            // note on PlaceholderScreen about where this has to go when the
            // last placeholder is deleted.
            onNodeWithText("заглушка · ${currentPlatform().name}").assertIsDisplayed()
        }

    @Test
    fun theAppIsWrappedInItsOwnTheme() =
        runComposeUiTest {
            // CadenceTheme is App's job, not the shell's — the shell is only
            // the area after sign-in, and block 7 adds an area before it that
            // needs the same tokens. If App stopped providing them, every
            // composable underneath would throw on the typography local rather
            // than fall back, so reaching any screen at all is the assertion.
            setContent { App() }

            onNodeWithText("Экран «Сегодня»").assertIsDisplayed()
        }

    @Test
    fun thePlusOpensTheSheetRatherThanChangingDestination() =
        runComposeUiTest {
            setContent { App() }

            onNodeWithContentDescription("Записать").performClick()

            onNodeWithText("Отмена").assertIsDisplayed()
            // And the bar did not treat the action as a place.
            onNodeWithText("Сегодня").assertIsSelected()
        }

    @Test
    fun theSheetSendsTheUserIntoTheDoseWizard() =
        runComposeUiTest {
            setContent { App() }

            onNodeWithContentDescription("Записать").performClick()
            onNodeWithText("Записать дозу").performClick()
            waitForIdle()

            // The sheet row and the wizard carry the same words, so the back
            // affordance is what tells them apart — and the sheet closing
            // behind the modal is the other half of it.
            onNodeWithContentDescription("Назад").assertIsDisplayed()
            assertTrue(onAllNodesWithText("Отмена").fetchSemanticsNodes().isEmpty())
        }
}
