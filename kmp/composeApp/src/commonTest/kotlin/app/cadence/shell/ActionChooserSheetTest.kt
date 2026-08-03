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

/** The sheet with the day it is describing, and whatever it reports back. */
@OptIn(ExperimentalTestApi::class)
private fun ComposeUiTest.openSheet(
    open: Boolean = true,
    doseLogged: Boolean = false,
    mealCount: Int = 0,
    mealKcal: Int = 0,
    picked: (String) -> Unit = { },
) {
    setContent {
        CadenceTheme {
            ActionChooserSheet(
                open = open,
                doseLogged = doseLogged,
                mealCount = mealCount,
                mealKcal = mealKcal,
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

            // The two rows sit one above the other and carry the same shape; a
            // port that wires both to the same lambda looks identical.
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

            onNodeWithText("Семаглутид · 0,25 мг ждёт").assertIsDisplayed()
            onNodeWithText("Пока ничего сегодня · начнём ритм").assertIsDisplayed()
        }

    @Test
    fun theSubtitlesSayWhatTheDayAlreadyHolds() =
        runComposeUiTest {
            openSheet(doseLogged = true, mealCount = 2, mealKcal = 1240)

            onNodeWithText("Уже записано сегодня · открыть или поправить").assertIsDisplayed()
            // Both formatting rules in one string: the plural and the
            // grouping. The separator here is U+00A0, and it has to be —
            // the first version of this line carried a plain space and
            // failed against a correct implementation, which is what an
            // invisible character does to a literal.
            onNodeWithText("2 приёма сегодня · 1 240 ккал").assertIsDisplayed()
        }

    @Test
    fun theMealNounFollowsTheCount() =
        runComposeUiTest {
            // A second count, in the other plural form. Without it the row
            // passes with «приёма» hardcoded — which it did, until this
            // test was added because a mutation survived.
            openSheet(mealCount = 5, mealKcal = 2100)

            onNodeWithText("5 приёмов сегодня · 2 100 ккал").assertIsDisplayed()
        }

    @Test
    fun aClosedSheetComposesNothing() =
        runComposeUiTest {
            openSheet(open = false)

            assertTrue(onAllNodesWithText("Записать дозу").fetchSemanticsNodes().isEmpty())
            assertTrue(onAllNodesWithText("Отмена").fetchSemanticsNodes().isEmpty())
        }
}
