package app.cadence.shell

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CadenceTheme
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.mock.CadenceMocks
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertTrue

private val ZONE = TimeZone.of("Europe/Moscow")

/** The app on a day the test chose, reading a repository the test wound. */
@OptIn(ExperimentalTestApi::class)
private fun mocks(iso: String = "2026-05-31T09:00:00Z") = CadenceMocks(FixedCadenceClock.at(iso), ZONE)

@OptIn(ExperimentalTestApi::class)
class CadenceShellDataTest {
    @Test
    fun theSheetShowsTheDayTheRepositoryReportsAndNotAConstant() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()

            // Two seeded meals of 320 and 520 kcal. Both numbers come from the
            // repository and are formatted by app.cadence.format on the way to
            // the screen — «840» is not a string anything upstream holds.
            onNodeWithText("2 приёма сегодня · 840 ккал").assertIsDisplayed()
            onNodeWithText("Семаглутид · 0,25 мг ждёт").assertIsDisplayed()
        }

    @Test
    fun anotherDayGivesTheSheetAnotherAnswer() =
        runComposeUiTest {
            // Week 5 of the seeded protocol, and a day with no meals on it.
            // Without this the shell could hold the seeded day's numbers as
            // literals and every assertion above would still pass — a mutation
            // doing exactly that survived until this test was added.
            setContent { CadenceTheme { CadenceApp(mocks = mocks("2026-06-07T09:00:00Z")) } }

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()

            onNodeWithText("Пока ничего сегодня · начнём ритм").assertIsDisplayed()
        }

    @Test
    fun loggingADoseChangesWhatTheSheetSaysNextTime() =
        runComposeUiTest {
            // The acceptance criterion of the whole block, on screen: a write
            // goes through the repository and the next read reflects it, with
            // ActionChooserSheet unchanged — it already took its four values as
            // parameters, which is the point.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()
            onNodeWithText("Записать дозу").performClick()
            waitForIdle()

            onNodeWithText("Экран «Записать дозу»").assertIsDisplayed()
            onNodeWithText("Записать").performClick()
            waitForIdle()

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()

            onNodeWithText("Уже записано сегодня · открыть или поправить").assertIsDisplayed()
            assertTrue(
                onAllNodesWithText("Семаглутид · 0,25 мг ждёт").fetchSemanticsNodes().isEmpty(),
                "the sheet is still reading its old answer",
            )
        }
}
