package app.cadence.screens.nutrition

import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CADENCE_RINGS_TAG
import app.cadence.design.CADENCE_WEEK_ROW_TAG
import app.cadence.design.CadenceTheme
import app.cadence.design.weekBarFraction
import app.cadence.design.weekBarTag
import app.cadence.design.weekScale
import app.cadence.shared.domain.Macros
import app.cadence.shared.domain.NutritionTargets
import app.cadence.shared.domain.UserId
import app.cadence.shared.repository.NutritionDay
import app.cadence.shared.repository.NutritionWeek
import app.cadence.shared.repository.NutritionWeekDay
import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.minus
import kotlin.math.abs
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** `MockSeed.targets` — the same 1800/140/200/60 the spec puts in exactly one place. */
private val TARGETS =
    NutritionTargets(patientId = UserId("patient-1"), macros = Macros(1800, 140, 200, 60), waterMl = null)
private val TODAY = LocalDate(2026, 5, 31)

private fun dayOf(
    totals: Macros,
    date: LocalDate = TODAY,
) = NutritionDay(date = date, meals = emptyList(), totals = totals, targets = TARGETS)

/** Seven days, oldest first, ending on [endDate] — the same walk `NutritionRepository.week()` does. */
private fun weekEnding(
    endDate: LocalDate,
    kcal: List<Int> = List(NUTRITION_WEEK_DAYS) { 900 },
    protein: List<Int> = List(NUTRITION_WEEK_DAYS) { 90 },
): NutritionWeek {
    val dates = (NUTRITION_WEEK_DAYS - 1 downTo 0).map { daysAgo -> endDate.minus(DatePeriod(days = daysAgo)) }
    return NutritionWeek(
        days =
            dates.mapIndexed { index, date ->
                NutritionWeekDay(date = date, kcal = kcal[index], proteinG = protein[index])
            },
    )
}

private const val NUTRITION_WEEK_DAYS = 7
private const val BAR_FRACTION_TOLERANCE = 0.01f

@OptIn(ExperimentalTestApi::class)
class NutritionScreenTest {
    /**
     * "Пустой день рисует **оба** приглашения, и нажатие на ссылку в ленте
     * открывает запись приёма" (spec step-6). Two separate invitations —
     * the hero's italic line and the feed's empty card — so a mutation that
     * drops either one on its own still reddens this test; and the
     * emphasised link is its own pressable node, proven by actually clicking
     * it rather than only asserting it exists.
     */
    @Test
    fun anEmptyDayDrawsBothInvitationsAndTheLinkOpensMealLogging() =
        runComposeUiTest {
            var opened = false
            setContent {
                CadenceTheme {
                    NutritionScreen(
                        day = dayOf(Macros(0, 0, 0, 0)),
                        week = weekEnding(TODAY),
                        onLogMeal = { opened = true },
                    )
                }
            }

            // The hero's own invitation — `NutritionHero`'s empty branch.
            onNodeWithText("Пока ничего — начнём, когда будете готовы.").assertExists()
            // The feed's own invitation — `EmptyFeedCard`.
            onNodeWithText("Сегодня пока ничего.").assertExists()

            onNodeWithText("Запишите первый приём").performClick()
            waitForIdle()

            assertTrue(opened, "the emphasised link in the empty feed did not open meal logging")
        }

    /**
     * The wiring test for [NutritionRingsCard]'s call into `CadenceRings`:
     * against the same lopsided fixture `CadenceRingsTest.kt` uses at the
     * primitive level (900/1800 kcal = 50%, 35/140 g protein = 25%), this
     * proves the *screen* threads `day.totals`/`day.targets.macros` into the
     * right slots rather than, say, passing kcal into both. Swapping the
     * protein arguments for the kcal ones would read «Белок 900 из 1800 г ·
     * 50%» here — the mutation this test exists to fail against.
     */
    @Test
    fun theProteinRingDrawsADifferentArcThanTheCalorieRing() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    NutritionScreen(
                        day = dayOf(Macros(kcal = 900, proteinG = 35, carbsG = 0, fatG = 0)),
                        week = weekEnding(TODAY),
                    )
                }
            }

            val described =
                onNodeWithTag(CADENCE_RINGS_TAG, useUnmergedTree = true)
                    .fetchSemanticsNode()
                    .config
                    .getOrNull(SemanticsProperties.ContentDescription)
                    ?.firstOrNull()

            assertEquals("Калории 900 из 1800 ккал · 50% · Белок 35 из 140 г · 25%", described)
        }

    /**
     * The step's own most important test. `MockSeed.DEMO_NOW` — the day every
     * other fixture in this suite is dated on — is a Sunday, and on a Sunday
     * the derived weekday order (Пн…Сб, oldest to youngest) happens to read
     * identically to the prototype's own literal `['Пн','Вт','Ср','Чт','Пт','Сб']`.
     * A mutation that swaps [weekDayLabels] for that literal list would pass
     * silently against any Sunday-ending fixture — so this fixture ends on a
     * Wednesday instead, where the two orders disagree: the derived labels
     * read Чт, Пт, Сб, Вс, Пн, Вт (today, Wednesday, becomes «Сег»), and
     * critically include «Вс» and omit «Ср» — neither of which the hardcoded
     * literal can ever produce.
     */
    @Test
    fun theWeekDayLabelsAreDerivedFromTheWeeksOwnDatesNotHardcodedLiterals() =
        runComposeUiTest {
            val wednesday = LocalDate(2026, 6, 3)
            setContent {
                CadenceTheme {
                    NutritionScreen(day = dayOf(Macros(0, 0, 0, 0), date = wednesday), week = weekEnding(wednesday))
                }
            }

            listOf("Чт", "Пт", "Сб", "Вс", "Пн", "Вт").forEach { label ->
                onNodeWithText(label, useUnmergedTree = true).assertExists("expected the derived label \"$label\"")
            }
            onNodeWithText("Сег", useUnmergedTree = true).assertExists()
            // The literal's own Wednesday entry — present in the hardcoded list,
            // absent from a week whose today already claimed that date as «Сег».
            onNodeWithText("Ср", useUnmergedTree = true).assertDoesNotExist()
        }

    /**
     * The counterpart to [theWeekDayLabelsAreDerivedFromTheWeeksOwnDatesNotHardcodedLiterals]:
     * on `DEMO_NOW`'s own Sunday, the derived labels coincide with the
     * prototype's literal Monday-first list — proving *why* the test above
     * had to wind the week off a Wednesday rather than trust this shape
     * alone. A "labels are hardcoded" mutation passes this fixture too; it is
     * not evidence of correct derivation by itself.
     */
    @Test
    fun theWeekDayLabelsOnTheDemoDayCoincideWithThePrototypesLiteralOrder() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    NutritionScreen(day = dayOf(Macros(0, 0, 0, 0)), week = weekEnding(TODAY))
                }
            }

            listOf("Пн", "Вт", "Ср", "Чт", "Пт", "Сб").forEach { label ->
                onNodeWithText(label, useUnmergedTree = true).assertExists("expected the derived label \"$label\"")
            }
            onNodeWithText("Сег", useUnmergedTree = true).assertExists()
        }

    /**
     * "Столбец «Сег» равен итогу дня" (spec step-6). Against a mutation that
     * points the last column at the wrong index, or at a value not sourced
     * from [NutritionWeek] at all: the fixture's seven kcal totals
     * (`MockSeed`'s own week, §11) are pairwise distinct, so only the exact
     * value at the last index produces this fraction of the row's height.
     */
    @Test
    fun theTodayColumnOfTheWeekChartEqualsTheDaysTotal() =
        runComposeUiTest {
            val kcalByDay = listOf(415, 1000, 1400, 2100, 1550, 1720, 840)
            setContent {
                CadenceTheme {
                    NutritionScreen(
                        day = dayOf(Macros(kcal = 840, proteinG = 60, carbsG = 90, fatG = 20)),
                        week = weekEnding(TODAY, kcal = kcalByDay),
                    )
                }
            }

            val row = onNodeWithTag(CADENCE_WEEK_ROW_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            val today =
                onNodeWithTag(weekBarTag(NUTRITION_WEEK_DAYS - 1), useUnmergedTree = true)
                    .fetchSemanticsNode()
                    .boundsInRoot

            val scale = weekScale(kcalByDay.map { it.toDouble() }, TARGETS.macros.kcal.toDouble())
            val expected = weekBarFraction(840.0, scale)
            val actual = today.height / row.height

            assertTrue(
                abs(actual - expected) < BAR_FRACTION_TOLERANCE,
                "today's column filled to $actual, not $expected — the day's own total",
            )
        }

    /**
     * «Белок · средн.» reads the week's own protein column, not its kcal one.
     * The two averages are chosen far apart (110 g vs. ~1289 kcal) so a
     * mutation that threads the wrong list into [weekProteinAverage] shows a
     * visibly wrong number rather than a coincidentally matching one.
     */
    @Test
    fun theWeekProteinAverageIsComputedFromTheWeeksOwnProteinNotKcal() =
        runComposeUiTest {
            val week =
                weekEnding(
                    TODAY,
                    kcal = listOf(415, 1000, 1400, 2100, 1550, 1720, 840),
                    protein = listOf(80, 90, 100, 110, 120, 130, 140),
                )
            setContent {
                CadenceTheme {
                    NutritionScreen(day = dayOf(Macros(0, 0, 0, 0)), week = week)
                }
            }

            onNodeWithText("110").assertExists()
        }

    /**
     * The transition card into the recipe library. Pressed by its own tag
     * rather than its copy, the same reason `CADENCE_TRENDS_JOURNAL_TAG` is
     * in `TrendsScreenTest.kt` — a card's text can move without the wiring
     * this test actually cares about changing.
     */
    @Test
    fun theRecipesLinkCardOpensTheRecipeLibrary() =
        runComposeUiTest {
            var opened = false
            setContent {
                CadenceTheme {
                    NutritionScreen(
                        day = dayOf(Macros(0, 0, 0, 0)),
                        week = weekEnding(TODAY),
                        onOpenRecipes = { opened = true },
                    )
                }
            }

            onNodeWithText("Рецепты и конструктор").assertExists()
            onNodeWithText("Белковые блюда · соберите своё").assertExists()
            onNodeWithTag(CADENCE_NUTRITION_RECIPES_LINK_TAG).performScrollTo().performClick()
            waitForIdle()

            assertTrue(opened, "the recipes card did not open the recipe library")
        }
}
