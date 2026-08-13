package app.cadence.screens.schedule

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.getBoundsInRoot
import androidx.compose.ui.test.onAllNodesWithContentDescription
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CadenceTheme
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.mock.CadenceMocks
import kotlinx.coroutines.runBlocking
import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private val ZONE = TimeZone.of("Europe/Moscow")

/** May 2026: the cycle starts on the 10th, so the first nine days are outside it. */
private fun mayState(): ScheduleState =
    runBlocking {
        val m = CadenceMocks(FixedCadenceClock.at("2026-05-31T04:00:00Z"), ZONE)
        val summary = m.today.today()
        ScheduleState(
            today = summary.date,
            cycleWeek = summary.cycleWeek,
            cycleWeeks = 12,
            days = m.schedule.month(LocalDate(2026, 5, 1)),
            titration = summary.nextTitration,
        )
    }

@OptIn(ExperimentalTestApi::class)
class ScheduleScreenTest {
    @Test
    fun theGridIsMondayFirstAndCoversTheMonth() =
        runComposeUiTest {
            setContent { CadenceTheme { ScheduleScreen(state = mayState()) } }

            // 1 May 2026 is a Friday, so the row carries four blanks before it — ignoring the
            // weekday would put it under «Пн» and shift every dot by four days.
            listOf("Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс").forEach {
                onNodeWithText(it).assertIsDisplayed()
            }
            // Position, not presence: 1 May must sit to the *right* of 4 May. Drop the leading
            // blanks and the order inverts — invisible to a test that only checks existence.
            val first = onNodeWithContentDescription("1 мая · вне курса").getBoundsInRoot().left
            val fourth = onNodeWithContentDescription("4 мая · вне курса").getBoundsInRoot().left

            assertTrue(first > fourth, "1 May was not aligned to its weekday")
        }

    @Test
    fun theBandSaysWhereInTheCourseThePatientIs() =
        runComposeUiTest {
            setContent { CadenceTheme { ScheduleScreen(state = mayState()) } }

            onNodeWithText("ТЕКУЩИЙ КУРС").assertIsDisplayed()
            onNodeWithText("неделя 4 из 12").assertIsDisplayed()
        }

    @Test
    fun theTitrationCalloutNamesTheStepAndItsDate() =
        runComposeUiTest {
            setContent { CadenceTheme { ScheduleScreen(state = mayState()) } }

            // Computed from the phases: 0,25 → 0,5 on 7 June, week 5.
            onNodeWithText("ШАГ ТИТРАЦИИ").assertExists()
            onNodeWithText("Доза растёт: 0,25 мг → 0,5 мг").assertExists()
            onNodeWithText("7 июня · 5-я неделя").assertExists()
        }

    @Test
    fun tappingADayOpensWhatIsOnIt() =
        runComposeUiTest {
            var opened: LocalDate? = null
            setContent { CadenceTheme { ScheduleScreen(state = mayState(), onOpenDay = { opened = it }) } }

            // 31 May is a Sunday inside the course, so its cell announces the injection too —
            // the content description is what a screen reader says, since a dot has no text.
            onNodeWithContentDescription("31 мая · сегодня · инъекция").performScrollTo().performClick()

            assertEquals(LocalDate(2026, 5, 31), opened)
        }

    @Test
    fun daysBeforeTheCourseBeganCarryNoMarks() =
        runComposeUiTest {
            setContent { CadenceTheme { ScheduleScreen(state = mayState()) } }

            // The 1st through the 9th are outside the protocol; the 10th is its first day.
            onNodeWithContentDescription("1 мая · вне курса").assertExists()
            onNodeWithContentDescription("10 мая · инъекция").assertExists()
        }

    @Test
    fun theInjectionDaysCarryADotAndTheOthersDoNot() =
        runComposeUiTest {
            setContent { CadenceTheme { ScheduleScreen(state = mayState()) } }

            // Four Sundays in May 2026's cycle. `describe` is built from the same field, so it
            // cannot witness the dot: deleting the Box left every assertion green. useUnmergedTree
            // because a clickable cell merges its descendants, hiding the dot's tag otherwise.
            assertEquals(
                4,
                onAllNodesWithTag(INJECTION_DOT_TAG, useUnmergedTree = true).fetchSemanticsNodes().size,
            )
        }

    @Test
    fun todayIsMarkedAndSaysSo() =
        runComposeUiTest {
            setContent { CadenceTheme { ScheduleScreen(state = mayState()) } }

            // The ring around today draws nothing a test can read, so the day announces itself.
            onNodeWithContentDescription("31 мая · сегодня · инъекция").assertExists()
            assertEquals(1, onAllNodesWithContentDescription("30 мая").fetchSemanticsNodes().size)
        }

    @Test
    fun aCourseWithNoNextStepDrawsNoCallout() =
        runComposeUiTest {
            setContent { CadenceTheme { ScheduleScreen(state = mayState().copy(titration = null)) } }

            assertTrue(onAllNodesWithText("ШАГ ТИТРАЦИИ").fetchSemanticsNodes().isEmpty())
        }
}
