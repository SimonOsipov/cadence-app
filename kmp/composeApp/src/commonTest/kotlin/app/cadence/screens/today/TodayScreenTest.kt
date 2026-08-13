package app.cadence.screens.today

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CadenceTheme
import app.cadence.shared.domain.DoseDraft
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.Meal
import app.cadence.shared.domain.MealId
import app.cadence.shared.domain.MealItem
import app.cadence.shared.domain.MealSource
import app.cadence.shared.domain.ProtocolItemKind
import app.cadence.shared.domain.UserId
import app.cadence.shared.mock.CadenceMocks
import app.cadence.shared.repository.TodaySummary
import kotlinx.coroutines.runBlocking
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue
import kotlin.time.Instant

private val ZONE = TimeZone.of("Europe/Moscow")

/** Семаглутид, BPC-157 and the nightly supplement — the seeded protocol. */
private const val PROTOCOL_ITEMS = 3

/** The seeded day, read through the repository the screen will be handed. */
private fun summary(iso: String = "2026-05-31T04:00:00Z"): TodaySummary =
    runBlocking { CadenceMocks(FixedCadenceClock.at(iso), ZONE).today.today() }

/** The same day's meals, through `NutritionRepository` — `TodayMeals`' own source. */
private fun meals(iso: String = "2026-05-31T04:00:00Z"): List<Meal> =
    runBlocking {
        val mocks = CadenceMocks(FixedCadenceClock.at(iso), ZONE)
        mocks.nutrition.day(mocks.today.today().date).meals
    }

@OptIn(ExperimentalTestApi::class)
class TodayScreenTest {
    @Test
    fun theHeaderGreetsWithTheDayTheHourAndTheCycleWeek() =
        runComposeUiTest {
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            // The prototype freezes «Воскресенье, утро · 4-я неделя»; all three parts derive
            // now, and 04:00 UTC is 07:00 in Moscow.
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

            // One node, not two: the prototype nests the dose inside the compound's `Text` so
            // the pair wraps as one paragraph — «0,25 мг» is a styled run, not a sibling.
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
            // The check-in prompt is not offered before there is a dose to check in about;
            // asserting only its appearance let `if (true)` through.
            assertTrue(onAllNodesWithText("Как перенесли дозу? Отметить").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun theWellbeingNudgeAppearsOnlyOnceTheDoseIsLogged() =
        runComposeUiTest {
            // The prototype shows «Как перенесли дозу?» in the hero after logging, and the
            // standing journal nudge below it either way.
            val m = CadenceMocks(FixedCadenceClock.at("2026-05-31T04:00:00Z"), ZONE)
            val after =
                runBlocking {
                    val before = m.today.today()
                    m.dosing.submit(
                        DoseDraft(
                            itemId = assertNotNull(before.nextDose).itemId,
                            kind = ProtocolItemKind.INJECTION,
                            dose = before.nextDose?.dose,
                            site = before.suggestedSite,
                        ),
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
            // The chart itself, which no text assertion can see.
            onNodeWithTag(GLANCE_SPARK_TAG, useUnmergedTree = true).assertIsDisplayed()
            // 98,8 a week earlier: the delta is computed from the series, not hardcoded the
            // way «↓ 0,6 кг» is in the prototype.
            onNodeWithText("↓ 0,4 кг").assertIsDisplayed()
        }

    @Test
    fun theReorderCardNamesTheCompoundAndTheWeeksLeft() =
        runComposeUiTest {
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            // One dose left at one a week, no sealed spare: the hint fires because the patient
            // is nearly out (three of four Sundays taken), not because the seed was tuned to it.
            onNodeWithText("Семаглутид закончится через ~1 неделю").assertExists()
            onNodeWithText("Запасного флакона нет").assertExists()
        }

    @Test
    fun noReorderMeansNoCard() =
        runComposeUiTest {
            // The seeded day always has a hint, so the absence has to be constructed —
            // otherwise an unconditionally-rendered card passes every other assertion.
            setContent {
                CadenceTheme {
                    TodayScreen(summary = summary().copy(reorder = null), patientName = "Марина")
                }
            }

            assertTrue(onAllNodesWithText("Запасного флакона нет").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun theProtocolStripDrawsOneRowPerItemOfTheWeek() =
        runComposeUiTest {
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            // The supplement has no phases, so its row has no dose — what makes the column
            // optional. assertExists, not assertIsDisplayed: these sit below the fold in the
            // test's viewport, so only rendering at all is being asserted.
            onNodeWithText("Протокол этой недели".uppercase()).assertExists()
            onNodeWithText("Семаглутид").assertExists()
            onNodeWithText("BPC-157").assertExists()
            onNodeWithText("Глицин + магний").assertExists()
            onNodeWithText("0,25 мг").assertExists()
        }

    @Test
    fun theStripSaysWhatIsStillOwedToday() =
        runComposeUiTest {
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            // All three items are due today (weekly injection on Sunday, two daily), so three
            // rows say «ждёт» — expecting one was the assertion being wrong, not the strip.
            assertEquals(PROTOCOL_ITEMS, onAllNodesWithText("ждёт").fetchSemanticsNodes().size)
        }

    @Test
    fun theStripFollowsTheDoseOnceItIsLogged() =
        runComposeUiTest {
            val m = CadenceMocks(FixedCadenceClock.at("2026-05-31T04:00:00Z"), ZONE)
            val after =
                runBlocking {
                    val before = m.today.today()
                    m.dosing.submit(
                        DoseDraft(
                            itemId = assertNotNull(before.nextDose).itemId,
                            kind = ProtocolItemKind.INJECTION,
                            dose = before.nextDose?.dose,
                            site = before.suggestedSite,
                        ),
                    )
                    m.today.today()
                }

            setContent { CadenceTheme { TodayScreen(summary = after, patientName = "Марина") } }

            // Only one row: the seeded history stops the day before, so today's BPC-157 slots
            // are still open and the just-logged semaglutide dose is the day's only «записано».
            assertEquals(1, onAllNodesWithText("записано").fetchSemanticsNodes().size)
        }

    @Test
    fun aRowSaysNothingAboutADayItIsNotDueOn() =
        runComposeUiTest {
            // Wednesday: the weekly injection is on the strip (it's the week's protocol) but
            // not due, so it carries no status. On the seeded Sunday all three items are due,
            // which is why a mutation putting a status on every row survived until this test.
            setContent {
                CadenceTheme {
                    TodayScreen(summary = summary("2026-05-20T04:00:00Z"), patientName = "Марина")
                }
            }

            onNodeWithText("Семаглутид").assertExists()
            // The evening supplement waits; BPC-157 is logged from the seed's past history;
            // semaglutide carries no status at all since it is not due today.
            assertEquals(1, onAllNodesWithText("ждёт").fetchSemanticsNodes().size)
            assertEquals(1, onAllNodesWithText("записано").fetchSemanticsNodes().size)

            // The hero says nothing about a dose there is none of. It used to name the
            // compound and offer «Записать →» — a tap opened the wizard and logged nothing,
            // because the shell's `nextDose?.let` had nothing to let.
            assertTrue(onAllNodesWithText("Записать →").fetchSemanticsNodes().isEmpty())
            assertTrue(
                onAllNodesWithText("Недельная инъекция, запланирована на сегодня.")
                    .fetchSemanticsNodes()
                    .isEmpty(),
            )
            onNodeWithText("Сегодня инъекции нет").assertIsDisplayed()
        }

    @Test
    fun aSingleReadingDrawsWithoutADelta() =
        runComposeUiTest {
            // One weigh-in is where a patient starts, and `it[it.size - 2]` on a one-point
            // series throws. The guard is `size >= 2`; no fixture had exactly one point before.
            setContent {
                CadenceTheme {
                    TodayScreen(
                        summary = summary().copy(weightSeries = listOf(98.4), weightKg = 98.4),
                        patientName = "Марина",
                    )
                }
            }

            onNodeWithText("98,4").assertIsDisplayed()
            assertTrue(onAllNodesWithText("↓ 0,4 кг").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun theWholeScheduleIsOneTapAway() =
        runComposeUiTest {
            var opened = 0
            setContent {
                CadenceTheme {
                    TodayScreen(summary = summary(), patientName = "Марина", onOpenSchedule = { opened++ })
                }
            }

            onNodeWithText("Весь график").performScrollTo().performClick()

            assertEquals(1, opened)
        }

    @Test
    fun theMealsCardCountsTheDayAgainstItsTarget() =
        runComposeUiTest {
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            // Two seeded meals: 320 + 520 kcal against a target of 1 800.
            onNodeWithText("ПРИЁМЫ СЕГОДНЯ").assertExists()
            onNodeWithText("840").assertExists()
            onNodeWithText("/ 1\u00A0800 ккал").assertExists()
        }

    @Test
    fun theMacroLegsCountEachMacroAgainstItsOwnTarget() =
        runComposeUiTest {
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            // Three near-identical call lines — the shape that invites a copy-paste swap — with
            // seeded values all distinct, so a transposed pair fails.
            onNodeWithText("Б 60/140").assertExists()
            onNodeWithText("Ж 18/60").assertExists()
            onNodeWithText("У 100/200").assertExists()
        }

    @Test
    fun theMealHeroSaysWhatIsLeftOfTheDay() =
        runComposeUiTest {
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            // 1 800 − 840 and 140 − 60, both computed rather than written.
            onNodeWithText("Осталось 960 ккал · 80 г белка").assertExists()
        }

    @Test
    fun theMealHintOnAZeroMealDayIsTheFirstOfFourStates() =
        runComposeUiTest {
            // A day the seed has no meals for — `suggestNextMeal`'s `meals.length == 0` branch.
            setContent {
                CadenceTheme {
                    TodayScreen(summary = summary("2026-06-07T09:00:00Z"), patientName = "Марина")
                }
            }

            // The eyebrow goes through `CadenceEyebrow`, which uppercases its text.
            onNodeWithText("Начнём день".uppercase()).assertExists()
            onNodeWithText("Завтрак?").assertExists()
            onNodeWithText("Целимся в 35 г белка.").assertExists()
        }

    @Test
    fun theMealHintOnATwoMealDayDiffersFromTheZeroMealState() =
        runComposeUiTest {
            // The seeded Sunday's two meals — `suggestNextMeal`'s `meals.length == 2` branch.
            // A fixed eyebrow/title would pass this or the zero-meal test, never both.
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            onNodeWithText("Следующий приём".uppercase()).assertExists()
            onNodeWithText("Перекус?").assertExists()
            onNodeWithText("Немного белка, немного фруктов.").assertExists()
            assertTrue(onAllNodesWithText("Начнём день".uppercase()).fetchSemanticsNodes().isEmpty())
            assertTrue(onAllNodesWithText("Завтрак?").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun theMealHintOnAOneMealDayIsTheSecondState() =
        runComposeUiTest {
            // 2026-05-25, Monday — the seed's one-meal day (`MockSeed.kt:588`). `suggestNextMeal`'s
            // `meals.length == 1` branch has no other witness in `commonTest`.
            setContent {
                CadenceTheme {
                    TodayScreen(summary = summary("2026-05-25T09:00:00Z"), patientName = "Марина")
                }
            }

            onNodeWithText("Следующий приём".uppercase()).assertExists()
            onNodeWithText("Скоро обед").assertExists()
            onNodeWithText("Подсказать, что собрать?").assertExists()
        }

    @Test
    fun theMealHintOnAThreeMealDayIsTheLastState() =
        runComposeUiTest {
            // 2026-05-27, Wednesday — the seed's three-meal day (`MockSeed.kt:617`).
            // `suggestNextMeal`'s `else` branch has no other witness in `commonTest`.
            setContent {
                CadenceTheme {
                    TodayScreen(summary = summary("2026-05-27T09:00:00Z"), patientName = "Марина")
                }
            }

            onNodeWithText("Последний шанс".uppercase()).assertExists()
            onNodeWithText("Лёгкий ужин?").assertExists()
            onNodeWithText("Запас есть — без излишеств.").assertExists()
        }

    @Test
    fun theRecentMealsListShowsALoggedMealWithItsTimeAndItemCount() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    TodayScreen(summary = summary(), patientName = "Марина", meals = meals(), zone = ZONE)
                }
            }

            onNodeWithText("Завтрак").assertExists()
            onNodeWithText("Обед").assertExists()
            // 06:30 UTC is 09:30 in Moscow — breakfast's own 320 kcal, not lunch's 520.
            onNodeWithText("09:30 · 1 позиция").assertExists()
            onNodeWithText("320 ккал").assertExists()
            // 10:00 UTC is 13:00 in Moscow — lunch's own row, not breakfast's again.
            onNodeWithText("13:00 · 1 позиция").assertExists()
            onNodeWithText("520 ккал").assertExists()
        }

    @Test
    fun theRecentMealsListShowsTheEmptyStateOnADayWithNoMeals() =
        runComposeUiTest {
            val iso = "2026-06-07T09:00:00Z"
            setContent {
                CadenceTheme {
                    TodayScreen(summary = summary(iso), patientName = "Марина", meals = meals(iso), zone = ZONE)
                }
            }

            onNodeWithText("Сегодня пока ничего — первый приём приземлится здесь.").assertExists()
        }

    @Test
    fun theRecentMealsListShowsOnlyTheLastThreeMeals() =
        runComposeUiTest {
            // The prototype's cap: `meals.slice(-3)` (`TodayScreen.tsx:857`). No seeded day
            // reaches four meals, so this needs a hand-built fixture — otherwise
            // `RECENT_MEALS_LIMIT = 3` could read `100` and nothing would notice.
            val fourMeals =
                listOf("Приём 1", "Приём 2", "Приём 3", "Приём 4").mapIndexed { index, name ->
                    Meal(
                        id = MealId("meal-cap-$index"),
                        patientId = UserId("patient-cap"),
                        eatenAt = Instant.parse("2026-05-31T0${6 + index}:00:00Z"),
                        name = name,
                        source = MealSource.MANUAL,
                        recipeId = null,
                        items = listOf(MealItem("Позиция", 100, MacrosTenths(1000, 100, 100, 100))),
                    )
                }

            setContent {
                CadenceTheme {
                    TodayScreen(summary = summary(), patientName = "Марина", meals = fourMeals, zone = ZONE)
                }
            }

            onNodeWithText("Приём 2").assertExists()
            onNodeWithText("Приём 3").assertExists()
            onNodeWithText("Приём 4").assertExists()
            assertTrue(onAllNodesWithText("Приём 1").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun theRecentMealsListOrdersByEatenAtNotByListPosition() =
        runComposeUiTest {
            // The fixture above builds meals with `mapIndexed`, so `eatenAt` climbs in lockstep
            // with list position and `meals.sortedBy { it.eatenAt }` could be deleted without
            // reddening it. Here the list arrives out of order (08:00, 06:00, 09:00, 07:00) —
            // pinning both which three `takeLast(3)` keeps and the order they render in against
            // the sort being dropped or reversed.
            val shuffled =
                listOf("Приём 3" to 8, "Приём 1" to 6, "Приём 4" to 9, "Приём 2" to 7)
                    .mapIndexed { index, (name, hour) ->
                        Meal(
                            id = MealId("meal-order-$index"),
                            patientId = UserId("patient-order"),
                            eatenAt = Instant.parse("2026-05-31T0$hour:00:00Z"),
                            name = name,
                            source = MealSource.MANUAL,
                            recipeId = null,
                            items = listOf(MealItem("Позиция", 100, MacrosTenths(1000, 100, 100, 100))),
                        )
                    }

            setContent {
                CadenceTheme {
                    TodayScreen(summary = summary(), patientName = "Марина", meals = shuffled, zone = ZONE)
                }
            }

            // Приём 1 (06:00) is chronologically earliest and must be the one `takeLast(3)`
            // drops — not Приём 3, which the shuffled list happens to put first. Scrolled once
            // through the middle row so all three land on screen together for the check below.
            onNodeWithText("Приём 3").performScrollTo()
            onNodeWithText("Приём 2").assertExists()
            onNodeWithText("Приём 3").assertExists()
            onNodeWithText("Приём 4").assertExists()
            assertTrue(onAllNodesWithText("Приём 1").fetchSemanticsNodes().isEmpty())

            // Rendered top-to-bottom in chronological order (07:00, 08:00, 09:00), not shuffled
            // input order. `useUnmergedTree = true` because `TodayMeals`' card is one
            // `pressable`, merging every child row's text into one node otherwise.
            val top2 = onNodeWithText("Приём 2", useUnmergedTree = true).fetchSemanticsNode().boundsInRoot.top
            val top3 = onNodeWithText("Приём 3", useUnmergedTree = true).fetchSemanticsNode().boundsInRoot.top
            val top4 = onNodeWithText("Приём 4", useUnmergedTree = true).fetchSemanticsNode().boundsInRoot.top
            assertTrue(top2 < top3, "Приём 2 (07:00) must render above Приём 3 (08:00)")
            assertTrue(top3 < top4, "Приём 3 (08:00) must render above Приём 4 (09:00)")
        }

    @Test
    fun theGlanceOpensTheBiomarkerSheet() =
        runComposeUiTest {
            // The sheet shipped unreachable: 129 lines and five green tests that nothing but
            // those tests could reach, while the glance's tap went to the trends tab.
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            onNodeWithText("98,4").performScrollTo().performClick()
            waitForIdle()

            onNodeWithText("Открыть детали тренда").assertIsDisplayed()
        }

    @Test
    fun everyControlInTheBodyReportsItself() =
        runComposeUiTest {
            // Six of the screen's fourteen callbacks had no test: swap `onOpenVials` and
            // `onOpenRecipes` at the call sites and «В аптечку» would land in the recipe list, green.
            val seen = mutableListOf<String>()
            setContent {
                CadenceTheme {
                    TodayScreen(
                        summary = summary(),
                        patientName = "Марина",
                        onOpenJournal = { seen += "journal" },
                        onOpenVials = { seen += "vials" },
                        onLogMeal = { seen += "meal" },
                        onOpenRecipes = { seen += "recipes" },
                        onOpenNutrition = { seen += "nutrition" },
                    )
                }
            }

            listOf(
                "Как вы себя чувствуете?" to "journal",
                "Рецепты" to "recipes",
                "Записать приём" to "meal",
                "ПРИЁМЫ СЕГОДНЯ" to "nutrition",
                "Запасного флакона нет" to "vials",
            ).forEach { (label, _) ->
                onNodeWithText(label).performScrollTo().performClick()
            }

            assertEquals(listOf("journal", "recipes", "meal", "nutrition", "vials"), seen)
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
