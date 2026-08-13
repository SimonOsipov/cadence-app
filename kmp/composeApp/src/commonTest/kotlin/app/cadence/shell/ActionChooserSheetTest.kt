package app.cadence.shell

import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CadenceTheme
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** What a live summary would produce — «Семаглутид · 0,25 мг ждёт» was a literal in production until it was not. */
private const val DUE_LINE = "Семаглутид · 0,25 мг ждёт"

/** The four facts about the day the sheet describes. */
private data class Day(
    val doseLogged: Boolean = false,
    val doseDue: String? = DUE_LINE,
    val mealCount: Int = 0,
    val mealKcal: Int = 0,
)

/** The sheet with the day it is describing, and whatever it reports back. */
@OptIn(ExperimentalTestApi::class)
private fun ComposeUiTest.openSheet(
    day: Day = Day(),
    open: Boolean = true,
    picked: (String) -> Unit = { },
) {
    setContent {
        CadenceTheme {
            ActionChooserSheet(
                open = open,
                doseLogged = day.doseLogged,
                doseDue = day.doseDue,
                mealCount = day.mealCount,
                mealKcal = day.mealKcal,
                onDismiss = { picked("dismiss") },
                onPickDose = { picked("dose") },
                onPickMeal = { picked("meal") },
            )
        }
    }
}

@OptIn(ExperimentalTestApi::class)
class ActionChooserSheetTest {
    @Test
    fun theSheetOffersBothWaysToRecordAndAWayOut() =
        runComposeUiTest {
            openSheet()

            onNodeWithText("ЧТО ЗАПИСЫВАЕМ?").assertIsDisplayed()
            onNodeWithText("Выберите ритм.").assertIsDisplayed()
            onNodeWithText("Записать дозу").assertIsDisplayed()
            onNodeWithText("Записать приём пищи").assertIsDisplayed()
            onNodeWithText("Отмена").assertIsDisplayed()
        }

    @Test
    fun eachRowReportsWhichOneWasTapped() =
        runComposeUiTest {
            var picked = ""
            openSheet(picked = { picked = it })

            // Same shape, same lambda risk — a port that wires both rows together looks identical.
            onNodeWithText("Записать приём пищи").performClick()
            assertEquals("meal", picked)

            onNodeWithText("Записать дозу").performClick()
            assertEquals("dose", picked)

            onNodeWithText("Отмена").performClick()
            assertEquals("dismiss", picked)
        }

    @Test
    fun anEmptyDaySaysSoRatherThanCountingZero() =
        runComposeUiTest {
            openSheet()

            onNodeWithText(DUE_LINE).assertIsDisplayed()
            onNodeWithText("Пока ничего сегодня · начнём ритм").assertIsDisplayed()
        }

    @Test
    fun theDueLineIsTheOneItWasGivenAndNotACompoundOfItsOwn() =
        runComposeUiTest {
            // Regressed to a literal for three blocks with the live summary already in scope.
            // Asserting the seed's own line wouldn't catch it — needs a line the seed can't produce.
            openSheet(Day(doseDue = "BPC-157 · 250 мкг ждёт"))

            onNodeWithText("BPC-157 · 250 мкг ждёт").assertIsDisplayed()
            assertTrue(
                onAllNodesWithText(DUE_LINE).fetchSemanticsNodes().isEmpty(),
                "a compound nobody passed in",
            )
        }

    @Test
    fun aDayThatPrescribesNothingSaysSoRatherThanNamingADose() =
        runComposeUiTest {
            openSheet(Day(doseDue = null))

            onNodeWithText("На сегодня доза не назначена").assertIsDisplayed()
        }

    @Test
    fun theSubtitlesSayWhatTheDayAlreadyHolds() =
        runComposeUiTest {
            openSheet(Day(doseLogged = true, mealCount = 2, mealKcal = 1240))

            onNodeWithText("Уже записано сегодня · открыть или поправить").assertIsDisplayed()
            // Separator is U+00A0, not a plain space: an earlier version of this line used a
            // plain space and failed against a correct implementation.
            onNodeWithText("2 приёма сегодня · 1\u00A0240 ккал").assertIsDisplayed()
        }

    @Test
    fun theMealNounFollowsTheCount() =
        runComposeUiTest {
            // Without this, the row passes with «приёма» hardcoded — which it did, until a
            // surviving mutation added this test.
            openSheet(Day(mealCount = 5, mealKcal = 2100))

            onNodeWithText("5 приёмов сегодня · 2\u00A0100 ккал").assertIsDisplayed()
        }

    @Test
    fun aClosedSheetComposesNothing() =
        runComposeUiTest {
            openSheet(open = false)

            assertTrue(onAllNodesWithText("Записать дозу").fetchSemanticsNodes().isEmpty())
            assertTrue(onAllNodesWithText("Отмена").fetchSemanticsNodes().isEmpty())
        }
}
