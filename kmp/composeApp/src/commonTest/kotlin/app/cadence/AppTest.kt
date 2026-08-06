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

/**
 * The tab, not the word. `ProtocolStripRow` renders «Сегодня» once per due item
 * as well, so `onNodeWithText` finds several nodes whenever the seeded protocol
 * covers the day the app is wound to — which, since the clock was pinned, is
 * always.
 */
private const val TODAY_TAB = "Сегодня"

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
            // The patient's name: the placeholder had none on it, so reaching
            // it is what separates the ported screen from the stub.
            onNodeWithText("Марина").assertIsDisplayed()
            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
        }

    @Test
    fun theAppOpensInsideTheSeededCourseRatherThanPastItsEnd() =
        runComposeUiTest {
            // The one assertion that fails when the fixture ages out. The seed's
            // course ran twelve weeks from 10 May 2026 and ended on 1 August;
            // `App()` read the system clock, so from 2 August every screen went
            // blank at once — `cycleWeek` returns null past the last prescribed
            // day, and that null is a hard gate in `occurrencesFor`,
            // `weekProtocolRows` and `phaseDose`. Nothing failed, because every
            // other test in the tree winds its own clock; only `App()` used the
            // real one, and only this file constructs `App()`.
            //
            // Keyed on the strip rather than on a greeting: `ProtocolStrip`
            // returns early on an empty row list, so its heading is present
            // exactly when the protocol is in force. Upper-case because
            // `CadenceEyebrow` renders it that way.
            setContent { App() }

            onNodeWithText("ПРОТОКОЛ ЭТОЙ НЕДЕЛИ").assertIsDisplayed()
            assertTrue(
                onAllNodesWithText("Семаглутид", substring = true).fetchSemanticsNodes().isNotEmpty(),
                "the seeded protocol has to reach the strip, not merely exist",
            )
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
            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
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

            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
        }

    @Test
    fun thePlusOpensTheSheetRatherThanChangingDestination() =
        runComposeUiTest {
            setContent { App() }

            onNodeWithContentDescription("Записать").performClick()

            onNodeWithText("Отмена").assertIsDisplayed()
            // And the bar did not treat the action as a place.
            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
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
            onNodeWithText("Шаг 1 из 5").assertIsDisplayed()
            // And the sheet closed behind the modal rather than staying under
            // it, as the prototype closes it before it navigates. Keyed on the
            // sheet's other row: «Отмена» is now the wizard's own header.
            assertTrue(onAllNodesWithText("Записать приём пищи").fetchSemanticsNodes().isEmpty())
        }
}
