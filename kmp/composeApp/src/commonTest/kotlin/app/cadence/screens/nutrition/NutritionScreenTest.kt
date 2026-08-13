package app.cadence.screens.nutrition

import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CADENCE_RINGS_TAG
import app.cadence.design.CADENCE_WEEK_ROW_TAG
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceTheme
import app.cadence.design.macroFillTag
import app.cadence.design.macroFraction
import app.cadence.design.macroTrackTag
import app.cadence.design.weekBarFraction
import app.cadence.design.weekBarTag
import app.cadence.design.weekScale
import app.cadence.format.formatInteger
import app.cadence.shared.domain.Macros
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.Meal
import app.cadence.shared.domain.MealId
import app.cadence.shared.domain.MealItem
import app.cadence.shared.domain.MealSource
import app.cadence.shared.domain.NutritionTargets
import app.cadence.shared.domain.UserId
import app.cadence.shared.repository.NutritionDay
import app.cadence.shared.repository.NutritionWeek
import app.cadence.shared.repository.NutritionWeekDay
import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlinx.datetime.minus
import kotlin.math.abs
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue
import kotlin.time.Instant

/** `MockSeed.targets` — the same 1800/140/200/60 the spec puts in exactly one place. */
private val TARGETS =
    NutritionTargets(patientId = UserId("patient-1"), macros = Macros(1800, 140, 200, 60), waterMl = null)
private val TODAY = LocalDate(2026, 5, 31)

private fun dayOf(
    totals: Macros,
    date: LocalDate = TODAY,
    meals: List<Meal> = emptyList(),
) = NutritionDay(date = date, meals = meals, totals = totals, targets = TARGETS)

/**
 * Two positions, one landing kcal and protein on a `.5` tenths value — the shape a stubbed or
 * half-to-even `tenthsLabel` rounds differently from this codebase's half-up rule. The chicken
 * item reuses `NutritionTest.kt`'s `chicken-bowl`-at-150-g fixture rather than inventing a new one.
 */
private fun twoItemMeal(): Meal =
    Meal(
        id = MealId("meal-1"),
        patientId = UserId("patient-1"),
        eatenAt = Instant.parse("2026-05-31T09:15:00Z"),
        name = "Обед",
        source = MealSource.MANUAL,
        recipeId = null,
        items =
            listOf(
                // 150 g chicken: 247,5 kcal / 46,5 g protein, both round up (248, 47) under
                // half-up, not half-to-even's 246/46.
                MealItem("Куриная грудка", 150, MacrosTenths(2475, 465, 0, 54)),
                MealItem("Бурый рис", 120, MacrosTenths(1476, 32, 300, 12)),
            ),
    )

/**
 * A second, single-item meal, so a day built with [twoItemMeal] has `meals.size == 2`, not 1 —
 * a one-meal fixture can't tell `formatInteger(meals.size)` or `pluralMeals(mealCount)` apart
 * from a hardcoded literal, the one-shape-hides-an-axis defect this branch keeps reintroducing.
 * Its own item count also matters: [twoItemMeal] alone (always two items) can't tell
 * `pluralItems(meal.items.size)` apart from a hardcoded «2 позиции» — this meal's «1 позиция»
 * header is the one count a wrong declension changes. Never expanded, so only its collapsed
 * header and kcal readout need to stay distinct from [twoItemMeal]'s.
 */
private fun singleItemMeal(): Meal =
    Meal(
        id = MealId("meal-2"),
        patientId = UserId("patient-1"),
        eatenAt = Instant.parse("2026-05-31T07:00:00Z"),
        name = "Завтрак",
        source = MealSource.MANUAL,
        recipeId = null,
        items = listOf(MealItem("Йогурт", 150, MacrosTenths(800, 30, 100, 20))),
    )

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
     * "Пустой день рисует **оба** приглашения, и нажатие на ссылку в ленте открывает запись
     * приёма" (spec step-6): two separate invitations (hero's line, feed's empty card) so a
     * mutation dropping either one alone still reddens this, and the link is proven pressable
     * by actually clicking it rather than only asserting it exists.
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

            // `NutritionHero`'s empty branch.
            onNodeWithText("Пока ничего — начнём, когда будете готовы.").assertExists()
            // `EmptyFeedCard`'s own invitation.
            onNodeWithText("Сегодня пока ничего.").assertExists()

            onNodeWithText("Запишите первый приём").performClick()
            waitForIdle()

            assertTrue(opened, "the emphasised link in the empty feed did not open meal logging")
        }

    /**
     * "Позиции показываются округлёнными — точность живёт в модели, не на экране" (spec step-6):
     * unasserted by every other test here, since [dayOf] hardcoded `meals = emptyList()`, making
     * `MealFeedCard`/`MealAvatar`/`MealFeedItemRow`, expand/collapse, the meal counter and
     * `pluralMeals` unreachable. [twoItemMeal]'s chicken lands both kcal and protein on a `.5`
     * tenths value, so a stubbed or half-to-even `tenthsLabel` both read wrong here. Two meals,
     * not one, so `formatInteger(meals.size)`/`pluralMeals(1)` can't hide behind a hardcoded
     * literal — see [singleItemMeal]'s own KDoc for why its header count matters too.
     */
    @Test
    fun aLoggedMealCardShowsItsHeaderThenItsRoundedItemsOnExpand() =
        runComposeUiTest {
            val meal = twoItemMeal()
            setContent {
                CadenceTheme {
                    NutritionScreen(
                        day =
                            dayOf(
                                Macros(kcal = 900, proteinG = 65, carbsG = 80, fatG = 15),
                                meals = listOf(meal, singleItemMeal()),
                            ),
                        week = weekEnding(TODAY),
                        zone = TimeZone.UTC,
                    )
                }
            }

            // The hero's own non-empty branch and its `pluralMeals` declension.
            onNodeWithText("2 приёма.").assertExists("the hero's non-empty branch")
            // The feed's own bare meal counter.
            onNodeWithText("2").assertExists("the feed's own meal counter")

            // Collapsed header: time, position count, meal's total protein.
            onNodeWithText("09:15 · 2 позиции · 50 г белка").assertExists()
            onNodeWithText("Обед").assertExists()
            // The second meal's «1 позиция» is the one item-count declension on screen a
            // hardcoded «2 позиции» would get wrong.
            onNodeWithText("07:00 · 1 позиция · 3 г белка").assertExists("the second meal's own item-count declension")
            // The exact fold rounded once (395), not the sum of each item's already-rounded
            // kcal (248 + 148 = 396) — unlike the protein line above (497 tenths rounds to 50
            // either way), this field actually distinguishes the two roundings.
            onNodeWithText("395").assertExists("the meal card's own kcal readout")

            onNodeWithText("Обед").performClick()
            waitForIdle()

            // Expanded: half-up rounding (248, 47), not half-to-even's 246/46, not a stub's 0.
            onNodeWithText("Белок 47г · Углеводы 0г · Жиры 5г").assertExists()
            onNodeWithText("150 г").assertExists()
            onNodeWithText("248").assertExists("the chicken item's own kcal readout")
        }

    /**
     * The wiring test for [NutritionRingsCard]'s call into `CadenceRings`: against the same
     * lopsided fixture `CadenceRingsTest.kt` uses at the primitive level (900/1800 kcal = 50%,
     * 35/140 g protein = 25%), this proves the *screen* threads `day.totals`/`day.targets.macros`
     * into the right slots. Swapping the protein args for the kcal ones would read «Белок 900 из
     * 1800 г · 50%» — the mutation this exists to fail against.
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
     * Closes the step-1 `[!question]`'s id/label transposition risk now that
     * `NutritionRingsMacros` gives `CadenceMacroBar` its first three real call sites (spec:
     * «Закрыть, когда экран питания даст этим компонентам реальные места вызова»).
     * Protein/carbs/fat are pairwise-distinct fractions of their own goals, so a value/goal
     * swap between any two bars shows a wrong fraction on that bar's own tag. The carbs fill
     * is also probed for `sand600` — the token this branch added for it — so a colour-only
     * swap (fractions unchanged) still reddens.
     */
    @Test
    fun theThreeMacroBarsEachDrawTheirOwnMacroNotAnotherOnes() =
        runComposeUiTest {
            val eaten = Macros(kcal = 900, proteinG = 35, carbsG = 90, fatG = 20)
            setContent {
                CadenceTheme {
                    NutritionScreen(day = dayOf(eaten), week = weekEnding(TODAY))
                }
            }

            fun fractionOf(id: String): Float {
                val track = onNodeWithTag(macroTrackTag(id), useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
                val fill = onNodeWithTag(macroFillTag(id), useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
                return fill.width / track.width
            }

            val expectedFractions =
                mapOf(
                    "protein" to macroFraction(eaten.proteinG.toDouble(), TARGETS.macros.proteinG.toDouble()),
                    "carbs" to macroFraction(eaten.carbsG.toDouble(), TARGETS.macros.carbsG.toDouble()),
                    "fat" to macroFraction(eaten.fatG.toDouble(), TARGETS.macros.fatG.toDouble()),
                )
            expectedFractions.forEach { (id, expected) ->
                val actual = fractionOf(id)
                assertTrue(
                    abs(actual - expected) < BAR_FRACTION_TOLERANCE,
                    "\"$id\" bar filled to $actual, not $expected",
                )
            }

            val carbsFill = onNodeWithTag(macroFillTag("carbs"), useUnmergedTree = true).captureToImage()
            val carbsPixel = carbsFill.toPixelMap()[carbsFill.width / 2, carbsFill.height / 2]
            assertEquals(CadenceColors.sand600, carbsPixel, "the carbs fill is not painted sand600")
        }

    /**
     * `MockSeed.DEMO_NOW` (every other fixture's date) is a Sunday, where the derived weekday
     * order happens to read identically to the prototype's literal
     * `['Пн','Вт','Ср','Чт','Пт','Сб','Сег']` — so a mutation swapping [weekDayLabels] for that
     * literal would pass silently there. This fixture ends on a Wednesday instead, where the
     * two disagree: derived labels include «Вс» and omit «Ср», which the literal can never produce.
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
            // The literal's Wednesday entry, absent here since today already claims «Сег».
            onNodeWithText("Ср", useUnmergedTree = true).assertDoesNotExist()
        }

    /**
     * The counterpart to [theWeekDayLabelsAreDerivedFromTheWeeksOwnDatesNotHardcodedLiterals]:
     * on `DEMO_NOW`'s own Sunday, derived labels coincide with the prototype's literal
     * Monday-first list, proving *why* that test had to wind off a Wednesday instead — a
     * "labels are hardcoded" mutation passes this fixture too.
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
     * "Столбец «Сег» равен итогу дня" (spec step-6). Against a mutation pointing the last
     * column at the wrong index, or a value not sourced from [NutritionWeek]: the seven kcal
     * totals are pairwise distinct, so only the exact last-index value produces this height
     * fraction. Also the only witness for `todayIndex` and `goalLabel` themselves: the height
     * check alone can't tell `todayIndex = week.days.lastIndex` apart from a mutated `0` (only
     * `selected` says which bar), and this fixture's max (2100) beats the 1800 goal, so even
     * `goal = 0.0` wouldn't move the scale — `goalLabel` needs its own text assertion.
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

            onNodeWithTag(weekBarTag(NUTRITION_WEEK_DAYS - 1), useUnmergedTree = true).assertIsSelected()
            onNodeWithTag(weekBarTag(NUTRITION_WEEK_DAYS - 2), useUnmergedTree = true).assertIsNotSelected()

            onNodeWithText(
                "цель ${formatInteger(TARGETS.macros.kcal)}",
                useUnmergedTree = true,
            ).assertExists("the goal caption did not read the day's own kcal goal")
        }

    /**
     * «Белок · средн.» reads the week's protein column, not kcal: the two averages are chosen
     * far apart (110 g vs. ~1289 kcal) so threading the wrong list into [weekProteinAverage]
     * shows a visibly wrong number.
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
     * The transition card into the recipe library, pressed by its own tag rather than its
     * copy — same reason as `CADENCE_TRENDS_JOURNAL_TAG` in `TrendsScreenTest.kt` — since a
     * card's text can move without the wiring this test cares about changing.
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
