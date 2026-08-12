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

            // 840 seeded, plus 80 from the drafted item — `kcalTenths = 800`
            // is already a whole number of kcal, so no rounding is involved
            // here, only the exact fold. Absolute values, not `before + 80`:
            // the field this pins is `MealLogResult.Written.dayTotals`, which
            // has to carry the sum taken *after* the write, not a snapshot
            // hoisted above it — a pre-write snapshot would report the same
            // 840 and this assertion would catch it where `before`/`after` on
            // `Today` cannot, because those are read fresh either way.
            assertEquals(920, result.dayTotals.kcal)
            assertEquals(120, result.dayTotals.carbsG)

            val after = cadence.today.today().mealMacros
            assertEquals(840, before.kcal)
            assertEquals(920, after.kcal)
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
            // Protein, not just kcal — a mutation that zeroes `proteinG` while
            // leaving `kcal` alone would return seven empty protein columns
            // and this is the only pair of assertions that would notice.
            assertEquals(88, week.days[2].proteinG)

            assertEquals(LocalDate(2026, 5, 28), week.days[3].date)
            assertEquals(2100, week.days[3].kcal)
            assertEquals(86, week.days[3].proteinG)

            assertEquals(LocalDate(2026, 5, 29), week.days[4].date)
            assertEquals(1550, week.days[4].kcal)

            assertEquals(LocalDate(2026, 5, 30), week.days[5].date)
            assertEquals(1720, week.days[5].kcal)

            assertEquals(DEMO_DATE, week.days[6].date)
            assertEquals(840, week.days[6].kcal)

            val kcals = week.days.map { it.kcal }
            assertEquals(kcals.size, kcals.distinct().size, "the seven days must be pairwise distinct")

            val day = mocks().nutrition.day(DEMO_DATE)
            assertEquals(day.totals.kcal, week.days[6].kcal, "the week's last column is the day's own total")
        }

    @Test
    fun dailyTotalsAgreeInUtcAndMoscow() =
        runTest {
            // All seven seeded dates, not just DEMO_DATE — that one day is
            // untouched from before this step, and passing on it alone would
            // not exercise the six days this step added. At UTC+3 it is the
            // upper bound that bites: a meal at or after 21:00Z is already the
            // next day in Moscow while still today in UTC, so only a date that
            // actually carries such a meal shows the disagreement. The band's
            // lower bound guards zones west of UTC, which this test does not
            // exercise — it is a seed rule, not something measured here.
            val expectedKcal =
                listOf(
                    LocalDate(2026, 5, 25) to 415,
                    LocalDate(2026, 5, 26) to 1000,
                    LocalDate(2026, 5, 27) to 1400,
                    LocalDate(2026, 5, 28) to 2100,
                    LocalDate(2026, 5, 29) to 1550,
                    LocalDate(2026, 5, 30) to 1720,
                    DEMO_DATE to 840,
                )
            val utc = mocks(zone = TimeZone.UTC)
            val moscow = mocks(zone = MOSCOW)

            for ((date, kcal) in expectedKcal) {
                val utcDay = utc.nutrition.day(date)
                val moscowDay = moscow.nutrition.day(date)

                assertEquals(kcal, utcDay.totals.kcal, "UTC total for $date")
                assertEquals(moscowDay.totals, utcDay.totals, "UTC and Moscow disagree on $date")
            }
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
