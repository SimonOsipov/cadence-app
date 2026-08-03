package app.cadence.screens.today

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
import app.cadence.shared.repository.TodaySummary
import kotlinx.coroutines.runBlocking
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private val ZONE = TimeZone.of("Europe/Moscow")

/** The seeded day, read through the repository the screen will be handed. */
private fun summary(iso: String = "2026-05-31T04:00:00Z"): TodaySummary =
    runBlocking { CadenceMocks(FixedCadenceClock.at(iso), ZONE).today.today() }

@OptIn(ExperimentalTestApi::class)
class TodayScreenTest {
    @Test
    fun theHeaderGreetsWithTheDayTheHourAndTheCycleWeek() =
        runComposeUiTest {
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            // The prototype freezes «Воскресенье, утро · 4-я неделя»; all three
            // parts derive now, and 04:00 UTC is 07:00 in Moscow.
            onNodeWithText("Воскресенье, утро · 4-я неделя").assertIsDisplayed()
            onNodeWithText("Марина").assertIsDisplayed()
        }

    @Test
    fun theGreetingFollowsTheClockIntoTheAfternoon() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    TodayScreen(
                        summary = summary("2026-05-31T09:00:00Z"),
                        patientName = "Марина",
                    )
                }
            }

            onNodeWithText("Воскресенье, день · 4-я неделя").assertIsDisplayed()
        }

    @Test
    fun theHeroNamesTheCompoundAndTheWeeksDose() =
        runComposeUiTest {
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            // One node, not two: the prototype nests the dose inside the
            // compound's `Text` so the pair wraps as one paragraph, and the
            // port keeps that — «0,25 мг» is a styled run, not a sibling.
            // The dose is the phase's, not the compound's default: week 4.
            onNodeWithText("Семаглутид\n0,25 мг").assertIsDisplayed()
        }

    @Test
    fun theHeroAsksForTheDoseUntilItIsLogged() =
        runComposeUiTest {
            var logged = 0
            setContent {
                CadenceTheme {
                    TodayScreen(summary = summary(), patientName = "Марина", onLogDose = { logged++ })
                }
            }

            onNodeWithText("Записать →").assertIsDisplayed().performClick()

            assertEquals(1, logged)
            assertTrue(onAllNodesWithText("Открыть детали").fetchSemanticsNodes().isEmpty())
            // And the check-in prompt is not offered before there is a dose to
            // check in about. Asserting only its appearance let `if (true)`
            // through.
            assertTrue(onAllNodesWithText("Как перенесли дозу? Отметить").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun theWellbeingNudgeAppearsOnlyOnceTheDoseIsLogged() =
        runComposeUiTest {
            // The prototype shows «Как перенесли дозу?» inside the hero after
            // logging, and the standing journal nudge below it either way.
            val m = CadenceMocks(FixedCadenceClock.at("2026-05-31T04:00:00Z"), ZONE)
            val after =
                runBlocking {
                    m.dosing.logDose(
                        m.today
                            .today()
                            .nextDose!!
                            .itemId,
                        null,
                    )
                    m.today.today()
                }
            var nudged = 0

            setContent {
                CadenceTheme {
                    TodayScreen(summary = after, patientName = "Марина", onOpenQuickFeel = { nudged++ })
                }
            }

            onNodeWithText("Открыть детали").assertIsDisplayed()
            onNodeWithText("Как перенесли дозу? Отметить").performClick()

            assertEquals(1, nudged)
        }

    @Test
    fun theGlanceShowsTheLatestWeightAndItsMovement() =
        runComposeUiTest {
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            onNodeWithText("98,4").assertIsDisplayed()
            onNodeWithText("кг").assertIsDisplayed()
            // 98,8 a week earlier: the delta is computed from the series, not
            // hardcoded the way «↓ 0,6 кг» is in the prototype.
            onNodeWithText("↓ 0,4 кг").assertIsDisplayed()
        }

    @Test
    fun theReorderCardNamesTheCompoundAndTheWeeksLeft() =
        runComposeUiTest {
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            // Four doses at one a week, no sealed spare — the seeded state.
            onNodeWithText("Семаглутид закончится через ~4 недели").assertIsDisplayed()
            onNodeWithText("Запасного флакона нет").assertIsDisplayed()
        }

    @Test
    fun noReorderMeansNoCard() =
        runComposeUiTest {
            // The seeded day always has a hint — four doses, no spare — so the
            // absence has to be constructed. Without this, a card rendered
            // unconditionally passes every other assertion on the screen.
            setContent {
                CadenceTheme {
                    TodayScreen(summary = summary().copy(reorder = null), patientName = "Марина")
                }
            }

            assertTrue(onAllNodesWithText("Запасного флакона нет").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun everyControlInTheHeaderReportsItself() =
        runComposeUiTest {
            val seen = mutableListOf<String>()
            setContent {
                CadenceTheme {
                    TodayScreen(
                        summary = summary(),
                        patientName = "Марина",
                        onOpenChat = { seen += "chat" },
                        onOpenSchedule = { seen += "schedule" },
                        onOpenLearn = { seen += "learn" },
                        onOpenProfile = { seen += "profile" },
                    )
                }
            }

            listOf("Чат", "Расписание", "Обучение", "Профиль").forEach {
                onNodeWithContentDescription(it).performClick()
            }

            assertEquals(listOf("chat", "schedule", "learn", "profile"), seen)
        }
}
