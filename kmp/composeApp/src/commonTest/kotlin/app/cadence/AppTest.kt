package app.cadence

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.screens.nutrition.LOG_MEAL_CHAT_FIELD_TAG
import app.cadence.shared.auth.SessionState
import kotlin.test.Test
import kotlin.test.assertTrue

// The tab, not the word: `ProtocolStripRow` also renders «Сегодня» once per due item, so
// `onNodeWithText` would find several nodes on the pinned, always-covered seeded day.
private const val TODAY_TAB = "Сегодня"

// Every case here is about the area after sign-in, so every case hands App a session. Before
// the gate existed App had none to take; the state is the shell's input now, and a test that
// left it out would be measuring the sign-in screen. AppSessionTest owns the gate itself.
@OptIn(ExperimentalTestApi::class)
class AppTest {
    @Test
    fun theAppOpensOnToday() =
        runComposeUiTest {
            setContent { App(SessionState.SignedIn) }

            // The patient's name, assembled from a TodaySummary: reaching it proves :shared is
            // linked into the UI, not merely the module graph, and separates the ported screen
            // from the placeholder (which had no name on it).
            onNodeWithText("Марина").assertIsDisplayed()
            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
        }

    @Test
    fun theAppOpensInsideTheSeededCourseRatherThanPastItsEnd() =
        runComposeUiTest {
            // Guards against the fixture ageing out: the seeded course ends 1 August 2026, and
            // `App()` reads the real system clock (every other test winds its own), so past that
            // date `cycleWeek` goes null and every screen blanks silently. Keyed on the strip
            // rather than a greeting because `ProtocolStrip` only renders its heading when the
            // protocol is in force; upper-case because `CadenceEyebrow` renders it that way.
            setContent { App(SessionState.SignedIn) }

            onNodeWithText("ПРОТОКОЛ ЭТОЙ НЕДЕЛИ").assertIsDisplayed()
            assertTrue(
                onAllNodesWithText("Семаглутид", substring = true).fetchSemanticsNodes().isNotEmpty(),
                "the seeded protocol has to reach the strip, not merely exist",
            )
        }

    @Test
    fun theSheetCanBeDismissedWithoutChoosingAnything() =
        runComposeUiTest {
            setContent { App(SessionState.SignedIn) }

            onNodeWithContentDescription("Записать").performClick()
            onNodeWithText("Отмена").performClick()
            waitForIdle()

            // Regression guard: wiring onDismiss to nothing made the sheet unclosable (only a
            // wizard was an exit), and every test stayed green because the sheet's own suite
            // checks it *reports* the tap, not that the shell acts on it.
            assertTrue(onAllNodesWithText("Отмена").fetchSemanticsNodes().isEmpty())
            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
        }

    @Test
    fun theSheetSendsTheUserIntoTheMealWizard() =
        runComposeUiTest {
            setContent { App(SessionState.SignedIn) }

            onNodeWithContentDescription("Записать").performClick()
            onNodeWithText("Записать приём пищи").performClick()
            waitForIdle()

            // Was missing: the round that pinned the dose row deleted the only test clicking
            // this one, so `onPickMeal = { }` passed the whole suite. Since step-13 the route
            // draws the wizard itself, so the assertion is its own composer rather than the
            // placeholder's title.
            onNodeWithTag(LOG_MEAL_CHAT_FIELD_TAG, useUnmergedTree = true).assertExists()
            // The sheet closed behind it: none of the five ported screens draws «Записать
            // дозу», so the sheet's own row is still the witness that `onPickMeal` dismissed
            // it — without this, a handler that only navigates leaves the sheet waiting
            // underneath the modal.
            assertTrue(onAllNodesWithText("Записать дозу").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun theAppIsWrappedInItsOwnTheme() =
        runComposeUiTest {
            // If App stopped providing CadenceTheme, every composable underneath would throw on
            // the typography local rather than fall back — so reaching any screen is the assertion.
            setContent { App(SessionState.SignedIn) }

            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
        }

    @Test
    fun thePlusOpensTheSheetRatherThanChangingDestination() =
        runComposeUiTest {
            setContent { App(SessionState.SignedIn) }

            onNodeWithContentDescription("Записать").performClick()

            onNodeWithText("Отмена").assertIsDisplayed()
            // The bar did not treat the action as a place.
            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
        }

    @Test
    fun theSheetSendsTheUserIntoTheDoseWizard() =
        runComposeUiTest {
            setContent { App(SessionState.SignedIn) }

            onNodeWithContentDescription("Записать").performClick()
            onNodeWithText("Записать дозу").performClick()
            waitForIdle()

            // The *destination*, not merely «some screen with a back button»: swapping the two
            // rows' targets left the suite green before, since both tests only asserted a «Назад».
            onNodeWithText("Шаг 1 из 5").assertIsDisplayed()
            // The sheet closed behind the modal (prototype closes it before navigating), keyed
            // on the sheet's other row since «Отмена» is now the wizard's own header.
            assertTrue(onAllNodesWithText("Записать приём пищи").fetchSemanticsNodes().isEmpty())
        }
}
