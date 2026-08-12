package app.cadence.shared.mock

import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.MealDraft
import app.cadence.shared.domain.MealItem
import app.cadence.shared.domain.MealSource
import app.cadence.shared.repository.MealLogResult
import kotlinx.coroutines.test.runTest
import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs

private val MOSCOW = TimeZone.of("Europe/Moscow")
private const val DEMO_NOW = "2026-05-31T09:00:00Z"
private val DEMO_DATE = LocalDate(2026, 5, 31)

/** The mock wound to [DEMO_NOW], in [zone]. */
private fun mocks(zone: TimeZone = MOSCOW) = CadenceMocks(clock = FixedCadenceClock.at(DEMO_NOW), zone = zone)

class NutritionRepositoryTest {
    @Test
    fun aMealLoggedThroughNutritionShowsUpOnToday() =
        runTest {
            val cadence = mocks()
            val before = cadence.today.today().mealMacros

            val draft =
                MealDraft(
                    name = "Перекус",
                    source = MealSource.AI_TEXT,
                    items =
                        listOf(
                            MealItem(
                                name = "Яблоко",
                                grams = 150,
                                macros =
                                    MacrosTenths(
                                        kcalTenths = 800,
                                        proteinGTenths = 4,
                                        carbsGTenths = 200,
                                        fatGTenths = 2,
                                    ),
                            ),
                        ),
                )

            val result = cadence.nutrition.log(draft)
            assertIs<MealLogResult.Written>(result)

            val after = cadence.today.today().mealMacros
            // 84 kcal from the new item's own rounding, folded exactly with
            // the two seeded meals rather than as a second, separate figure —
            // the mutation this kills is a nutrition repository that keeps its
            // own list, which `today()` would never see.
            assertEquals(before.kcal + 80, after.kcal)
            assertEquals(before.proteinG, after.proteinG)
        }

    @Test
    fun aRejectedDraftLeavesTodayUntouched() =
        runTest {
            val cadence = mocks()
            val before = cadence.today.today().mealMacros

            val result = cadence.nutrition.log(MealDraft(name = "", source = MealSource.AI_TEXT))
            assertIs<MealLogResult.Rejected>(result)

            assertEquals(before, cadence.today.today().mealMacros)
        }

    @Test
    fun theWeekEndingOnDemoNowHasSevenNamedDaysPairwiseDistinct() =
        runTest {
            val week = mocks().nutrition.week(endingOn = DEMO_DATE)

            assertEquals(7, week.days.size)

            assertEquals(LocalDate(2026, 5, 25), week.days[0].date)
            assertEquals(415, week.days[0].kcal)

            assertEquals(LocalDate(2026, 5, 26), week.days[1].date)
            assertEquals(1000, week.days[1].kcal)

            assertEquals(LocalDate(2026, 5, 27), week.days[2].date)
            assertEquals(1400, week.days[2].kcal)

            assertEquals(LocalDate(2026, 5, 28), week.days[3].date)
            assertEquals(2100, week.days[3].kcal)

            assertEquals(LocalDate(2026, 5, 29), week.days[4].date)
            assertEquals(1550, week.days[4].kcal)

            assertEquals(LocalDate(2026, 5, 30), week.days[5].date)
            assertEquals(1720, week.days[5].kcal)

            assertEquals(DEMO_DATE, week.days[6].date)
            assertEquals(840, week.days[6].kcal)

            val kcals = week.days.map { it.kcal }
            assertEquals(kcals.distinct().size, kcals.size, "the seven days must be pairwise distinct")

            val day = mocks().nutrition.day(DEMO_DATE)
            assertEquals(day.totals.kcal, week.days[6].kcal, "the week's last column is the day's own total")
        }

    @Test
    fun dailyTotalsAgreeInUtcAndMoscow() =
        runTest {
            val utcDay = mocks(zone = TimeZone.UTC).nutrition.day(DEMO_DATE)
            val moscowDay = mocks(zone = MOSCOW).nutrition.day(DEMO_DATE)

            assertEquals(840, utcDay.totals.kcal)
            assertEquals(moscowDay.totals, utcDay.totals)
        }

    @Test
    fun theDayServesTargetsFromTheSeed() =
        runTest {
            val day = mocks().nutrition.day(DEMO_DATE)

            assertEquals(1800, day.targets.macros.kcal)
            assertEquals(140, day.targets.macros.proteinG)
            assertEquals(200, day.targets.macros.carbsG)
            assertEquals(60, day.targets.macros.fatG)
            assertEquals(MockSeed.patientId, day.targets.patientId)
        }
}
