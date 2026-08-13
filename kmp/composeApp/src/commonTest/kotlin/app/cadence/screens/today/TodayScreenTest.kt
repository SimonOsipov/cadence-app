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
            // 98,8 a week earlier: the delta is computed from the series, not
            // hardcoded the way «↓ 0,6 кг» is in the prototype.
            onNodeWithText("↓ 0,4 кг").assertIsDisplayed()
        }

    @Test
    fun theReorderCardNamesTheCompoundAndTheWeeksLeft() =
        runComposeUiTest {
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            // One dose left at one a week, no sealed spare. The patient has
            // taken three of four Sundays, so the hint fires because they are
            // nearly out rather than because the seed was tuned to fire it.
            onNodeWithText("Семаглутид закончится через ~1 неделю").assertExists()
            onNodeWithText("Запасного флакона нет").assertExists()
        }

    @Test
    fun noReorderMeansNoCard() =
        runComposeUiTest {
            // The seeded day always has a hint — one dose, no spare — so the
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
    fun theProtocolStripDrawsOneRowPerItemOfTheWeek() =
        runComposeUiTest {
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            // The prototype's three rows, now a projection of the same
            // occurrences the calendar reads. The supplement has no phases, so
            // its row has no dose — which is what makes the column optional.
            // assertExists, not assertIsDisplayed: the screen scrolls and these
            // sit below the fold in the test's viewport. What is being asserted
            // is that the strip renders them at all.
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

            // All three items are due today — the weekly injection because it
            // is Sunday, the other two because they are daily — so three rows
            // say «ждёт». Expecting one was the assertion being wrong, not the
            // strip.
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

            // One row, and only one: the seeded history stops the day before,
            // so today's BPC-157 slots are still open and the semaglutide dose
            // just logged is the day's only «записано».
            assertEquals(1, onAllNodesWithText("записано").fetchSemanticsNodes().size)
        }

    @Test
    fun aRowSaysNothingAboutADayItIsNotDueOn() =
        runComposeUiTest {
            // Wednesday: the weekly injection is on the strip — it is the
            // week's protocol — but it is not due, so it carries no «сегодня».
            // On the seeded Sunday all three items are due, which is why a
            // mutation putting a status on every row survived until this test.
            setContent {
                CadenceTheme {
                    TodayScreen(summary = summary("2026-05-20T04:00:00Z"), patientName = "Марина")
                }
            }

            onNodeWithText("Семаглутид").assertExists()
            // One row waits and one is done: the evening supplement is still
            // pending, and BPC-157 is logged because the seed carries what the
            // patient has already done and the 20th is in the past. Semaglutide
            // is on the strip with no status at all — it is not due today.
            assertEquals(1, onAllNodesWithText("ждёт").fetchSemanticsNodes().size)
            assertEquals(1, onAllNodesWithText("записано").fetchSemanticsNodes().size)

            // And the hero says nothing about a dose there is none of. It used
            // to name the compound, print «Недельная инъекция, запланирована на
            // сегодня» and offer «Записать →» — a button whose tap opened the
            // wizard, confirmed, and logged nothing, because the shell's
            // `nextDose?.let` had nothing to let.
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
            // One weigh-in is where a patient starts, and `it[it.size - 2]` on
            // a one-point series throws. The guard is `size >= 2`; loosening it
            // to `>= 1` crashes and every test stayed green, because no fixture
            // ever had exactly one point.
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

            // Three near-identical call lines — the shape that invites a
            // copy-paste swap — and nothing looked at them. The seeded values
            // are all distinct, so a transposed pair fails.
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
            // A day the seed has no meals for (`MockRepositoryTest
            // .anotherDayHasItsOwnMealsAndNotTheSeededDays`) — `suggestNextMeal`'s
            // `meals.length == 0` branch.
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
            // The default fixture's seeded Sunday: two meals, `suggestNextMeal`'s
            // `meals.length == 2` branch. Against the mutation «the hint is
            // constant» — a fixed eyebrow/title would pass one of this test and
            // `theMealHintOnAZeroMealDayIsTheFirstOfFourStates`, never both.
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
            // 2026-05-25, Monday — the seed's own one-meal day
            // (`MockSeed.kt:588`, «under target: one meal»).
            // `suggestNextMeal`'s `meals.length == 1` branch has no other
            // witness anywhere in `commonTest`.
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
            // 2026-05-27, Wednesday — the seed's own three-meal day
            // (`MockSeed.kt:617`, «three meals»). `suggestNextMeal`'s `else`
            // branch has no other witness anywhere in `commonTest`.
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
            // 06:30 UTC is 09:30 in Moscow — the seeded breakfast's one item
            // and its own 320 kcal, not the lunch's 520 or either's protein.
            onNodeWithText("09:30 · 1 позиция").assertExists()
            onNodeWithText("320 ккал").assertExists()
            // 10:00 UTC is 13:00 in Moscow — the seeded lunch, so the row
            // reads its own meal's tenths rather than the breakfast's again.
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
            // The prototype's own cap: `meals.slice(-3)` (`TodayScreen.tsx:857`).
            // No seeded day reaches four meals, so the cap itself needs a
            // hand-built fixture — without this, `RECENT_MEALS_LIMIT = 3`
            // could read `100` and nothing would notice.
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
            // The fixture above builds its four meals with `mapIndexed`, so
            // `eatenAt` climbs in lockstep with list position — list order and
            // time order never disagree there, and `meals.sortedBy { it.eatenAt }`
            // could be deleted outright without reddening it. Here the list
            // arrives out of order (08:00, 06:00, 09:00, 07:00) while the
            // fixture's own name still names each meal's true hour, so both the
            // *set* TodayMeals keeps (dropping the sort changes which three
            // survive `takeLast(3)`, since it would then trim off the input
            // list's own first entry rather than its earliest one) and the
            // *order* it renders them in are pinned against the sort being
            // dropped or reversed.
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

            // Chronologically, Приём 1 (06:00) is the earliest of the four and
            // must be the one `takeLast(3)` drops — not Приём 3, which the
            // shuffled list happens to put first. Scrolled into view once,
            // through the middle row, so all three land on screen together
            // for the position check below rather than each scroll bumping
            // the other two back off it.
            onNodeWithText("Приём 3").performScrollTo()
            onNodeWithText("Приём 2").assertExists()
            onNodeWithText("Приём 3").assertExists()
            onNodeWithText("Приём 4").assertExists()
            assertTrue(onAllNodesWithText("Приём 1").fetchSemanticsNodes().isEmpty())

            // And rendered top-to-bottom in chronological order (07:00, 08:00,
            // 09:00) rather than the shuffled input order (08:00, 09:00, 07:00
            // once Приём 1 is trimmed off). `useUnmergedTree = true` because
            // `TodayMeals`' own card is one `pressable`, merging every child
            // row's text into a single semantics node otherwise — every match
            // would then report that one card's bounds, not its own row's.
            val top2 = onNodeWithText("Приём 2", useUnmergedTree = true).fetchSemanticsNode().boundsInRoot.top
            val top3 = onNodeWithText("Приём 3", useUnmergedTree = true).fetchSemanticsNode().boundsInRoot.top
            val top4 = onNodeWithText("Приём 4", useUnmergedTree = true).fetchSemanticsNode().boundsInRoot.top
            assertTrue(top2 < top3, "Приём 2 (07:00) must render above Приём 3 (08:00)")
            assertTrue(top3 < top4, "Приём 3 (08:00) must render above Приём 4 (09:00)")
        }

    @Test
    fun theGlanceOpensTheBiomarkerSheet() =
        runComposeUiTest {
            // The sheet shipped unreachable: 129 lines and five green tests
            // that nothing but those tests could reach, while the glance's tap
            // went to the trends tab. In the prototype the sheet *is* the
            // glance's behaviour.
            setContent { CadenceTheme { TodayScreen(summary = summary(), patientName = "Марина") } }

            onNodeWithText("98,4").performScrollTo().performClick()
            waitForIdle()

            onNodeWithText("Открыть детали тренда").assertIsDisplayed()
        }

    @Test
    fun everyControlInTheBodyReportsItself() =
        runComposeUiTest {
            // Six of the screen's fourteen callbacks had no test. Swap
            // `onOpenVials` and `onOpenRecipes` at the call sites and «В
            // аптечку» on a low-stock warning lands in the recipe list, green.
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
