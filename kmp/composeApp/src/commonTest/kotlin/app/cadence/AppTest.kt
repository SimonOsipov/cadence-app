package app.cadence

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import kotlin.test.Test
import kotlin.test.assertTrue

@OptIn(ExperimentalTestApi::class)
class AppTest {
    @Test
    fun theAppOpensOnToday() =
        runComposeUiTest {
            setContent { App() }

            // The ported screen, not the placeholder. The greeting is
            // assembled from a TodaySummary, so reaching it proves :shared is
            // linked into the UI and not merely into the module graph — which
            // is what the deleted platform-name assertion used to do, and what
            // PlaceholderScreen's note asked to be re-homed.
            // The patient's name, not the greeting: `App()` runs on the system
            // clock, and asserting a weekday would make this test pass or fail
            // by the day it is run on. What it needs to prove is that the
            // ported screen is what the app opens on — the placeholder had no
            // name on it.
            onNodeWithText("Марина").assertIsDisplayed()
            onNodeWithText("Сегодня").assertIsSelected()
        }

    @Test
    fun theSheetCanBeDismissedWithoutChoosingAnything() =
        runComposeUiTest {
            setContent { App() }

            onNodeWithContentDescription("Записать").performClick()
            onNodeWithText("Отмена").performClick()
            waitForIdle()

            // Wiring onDismiss to nothing made the sheet unclosable — the only
            // way out was into one of the two wizards — and every test stayed
            // green, because the sheet's own suite checks that it *reports* the
            // tap, not that the shell acts on it.
            assertTrue(onAllNodesWithText("Отмена").fetchSemanticsNodes().isEmpty())
            onNodeWithText("Сегодня").assertIsSelected()
        }

    @Test
    fun theSheetSendsTheUserIntoTheMealWizard() =
        runComposeUiTest {
            setContent { App() }

            onNodeWithContentDescription("Записать").performClick()
            onNodeWithText("Записать приём пищи").performClick()
            waitForIdle()

            // The mirror of the dose case, and it was missing: the fix round
            // that pinned the dose row deleted the only test clicking this one,
            // so `onPickMeal = { }` passed the whole suite.
            onNodeWithText("Экран «Записать приём пищи»").assertIsDisplayed()
            assertTrue(onAllNodesWithText("Отмена").fetchSemanticsNodes().isEmpty())
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

            onNodeWithText("Сегодня").assertIsSelected()
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

            // The *destination*, not merely «some screen with a back button».
            // Swapping the two rows' targets — dose opening the meal wizard —
            // left the whole suite green, because both tests named after this
            // wiring asserted only that a «Назад» existed.
            onNodeWithText("Экран «Записать дозу»").assertIsDisplayed()
            // And the sheet closed behind the modal rather than staying under
            // it, as the prototype closes it before it navigates.
            assertTrue(onAllNodesWithText("Отмена").fetchSemanticsNodes().isEmpty())
        }
}
