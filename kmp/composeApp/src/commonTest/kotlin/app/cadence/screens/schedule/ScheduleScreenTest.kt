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

            // 1 May 2026 is a Friday, so the first row carries four blanks
            // before it — a grid that ignored the weekday would put it under
            // «Пн» and shift every dot in the month by four days.
            listOf("Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс").forEach {
                onNodeWithText(it).assertIsDisplayed()
            }
            // Position, not presence. 1 May is a Friday and 4 May a Monday,
            // so the first of the month must sit to the *right* of the fourth.
            // Drop the leading blanks and the order inverts — which a test
            // asserting only that «1» and «31» exist cannot see, and a mutation
            // proved it.
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

            // 31 May is a Sunday inside the course, so its cell announces the
            // injection too — the description is what a screen reader says and
            // what a test can grab, since a dot has no text.
            onNodeWithContentDescription("31 мая · сегодня · инъекция").performScrollTo().performClick()

            assertEquals(LocalDate(2026, 5, 31), opened)
        }

    @Test
    fun daysBeforeTheCourseBeganCarryNoMarks() =
        runComposeUiTest {
            setContent { CadenceTheme { ScheduleScreen(state = mayState()) } }

            // The 1st through the 9th are outside the protocol; the 10th is its
            // first day. A grid that marked them would be inventing a course.
            onNodeWithContentDescription("1 мая · вне курса").assertExists()
            onNodeWithContentDescription("10 мая · инъекция").assertExists()
        }

    @Test
    fun theInjectionDaysCarryADotAndTheOthersDoNot() =
        runComposeUiTest {
            setContent { CadenceTheme { ScheduleScreen(state = mayState()) } }

            // Four Sundays inside the cycle in May 2026 — the 10th, 17th, 24th
            // and 31st. `describe` is built from the same field, so it cannot
            // witness the dot: deleting the Box left every assertion green.
            // useUnmergedTree: the cell is clickable, and a clickable merges its
            // descendants, so the dot's tag is invisible in the merged tree.
            assertEquals(
                4,
                onAllNodesWithTag(INJECTION_DOT_TAG, useUnmergedTree = true).fetchSemanticsNodes().size,
            )
        }

    @Test
    fun todayIsMarkedAndSaysSo() =
        runComposeUiTest {
            setContent { CadenceTheme { ScheduleScreen(state = mayState()) } }

            // The ring around today draws nothing a test can read, so the day
            // announces itself instead — which the screen reader needed anyway.
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
