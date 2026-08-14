package app.cadence.design

import androidx.compose.foundation.layout.width
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.compose.ui.unit.dp
import app.cadence.shared.domain.MoodLevel
import app.cadence.shared.domain.TrendRange
import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.plus
import kotlin.math.abs
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** The prototype's own seed opens on a Sunday — the case that costs its grid six days. */
private val START = LocalDate(2026, 5, 24)

private val COURSE = TrendRange(START, START.plus(DatePeriod(days = COURSE_LAST_DAY)))

private fun day(index: Int): LocalDate = START.plus(DatePeriod(days = index))

private val TITRATIONS = listOf(day(28), day(56))

/** Uneven on purpose: equal gaps would look the same laid out by date and by list position. */
private val UNEVEN_MARKS = listOf(day(7), day(28), day(56))

@OptIn(ExperimentalTestApi::class)
class JournalPrimitivesTest {
    @Test
    fun theHeatmapDrawsTheDayTheProtoypesTwelveRowsRanOutBefore() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceHeatmap(
                        course = COURSE,
                        today = day(40),
                        moodByDate = emptyMap(),
                        titrations = emptySet(),
                        modifier = Modifier.width(343.dp),
                    )
                }
            }

            onNodeWithTag(cadenceHeatmapCellTag(day(COURSE_LAST_DAY))).assertIsDisplayed()
        }

    @Test
    fun everyHeatmapCellStaysInsideTheWidthItWasGiven() =
        runComposeUiTest {
            val width = 343.dp

            setContent {
                CadenceTheme {
                    CadenceHeatmap(
                        course = COURSE,
                        today = day(40),
                        moodByDate = emptyMap(),
                        titrations = TITRATIONS.toSet(),
                        modifier = Modifier.width(width),
                    )
                }
            }

            // positionInRoot, not boundsInRoot: bounds are clipped by the parent,
            // so a cell pushed past the edge would report itself inside it.
            val grid = onNodeWithTag(CADENCE_HEATMAP_TAG).fetchSemanticsNode()
            val gridRight = grid.positionInRoot.x + grid.size.width

            listOf(day(0), day(6), day(COURSE_LAST_DAY)).forEach { date ->
                val cell = onNodeWithTag(cadenceHeatmapCellTag(date)).fetchSemanticsNode()
                val right = cell.positionInRoot.x + cell.size.width

                assertTrue(
                    cell.positionInRoot.x >= grid.positionInRoot.x - 1f && right <= gridRight + 1f,
                    "the cell for $date runs from ${cell.positionInRoot.x} to $right, outside $gridRight",
                )
            }
        }

    @Test
    fun aTitrationDayCarriesItsDotAndAnOrdinaryDayDoesNot() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceHeatmap(
                        course = COURSE,
                        today = day(60),
                        moodByDate = emptyMap(),
                        titrations = TITRATIONS.toSet(),
                        modifier = Modifier.width(343.dp),
                    )
                }
            }

            TITRATIONS.forEach { onNodeWithTag(cadenceHeatmapTitrationTag(it), useUnmergedTree = true).assertExists() }
            onNodeWithTag(cadenceHeatmapTitrationTag(day(29)), useUnmergedTree = true).assertDoesNotExist()
        }

    @Test
    fun theMoodChartStandsEachTitrationOnItsOwnDay() =
        runComposeUiTest {
            val width = 343.dp

            setContent {
                CadenceTheme {
                    CadenceMoodChart(
                        readings = emptyList(),
                        course = COURSE,
                        today = day(40),
                        titrations = UNEVEN_MARKS,
                        modifier = Modifier.width(width),
                    )
                }
            }

            val marks =
                UNEVEN_MARKS.map { date ->
                    onNodeWithTag(cadenceMoodMarkTag(date), useUnmergedTree = true)
                        .fetchSemanticsNode()
                        .positionInRoot.x
                }

            // Days 7, 28 and 56: gaps of 21 and 28 days, so the second gap is
            // four thirds of the first. Laid out by position in the list the two
            // gaps would be equal. Gaps rather than absolute offsets, so the
            // plot's own left inset cancels instead of being asserted twice.
            val first = marks[1] - marks[0]
            val second = marks[2] - marks[1]

            assertTrue(first > 0f, "the marks run left to right, got $first")
            assertEquals(28f / 21f, second / first, 0.02f)
        }

    @Test
    fun theMoodChartLabelsTheFourWeeksTheProtoypeLabels() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceMoodChart(
                        readings =
                            listOf(
                                MoodReading(day(0), MoodLevel.EVEN, dosed = false),
                                MoodReading(day(3), MoodLevel.BRIGHT, dosed = true),
                            ),
                        course = COURSE,
                        today = day(40),
                        titrations = TITRATIONS,
                        modifier = Modifier.width(343.dp),
                    )
                }
            }

            listOf(1, 4, 8, 12).forEach { week ->
                onNodeWithText("нед $week").assertExists()
            }
        }

    @Test
    fun theLastWeekLabelStaysInsideTheChart() =
        runComposeUiTest {
            val width = 343.dp

            setContent {
                CadenceTheme {
                    CadenceMoodChart(
                        readings = emptyList(),
                        course = COURSE,
                        today = day(40),
                        titrations = TITRATIONS,
                        modifier = Modifier.width(width),
                    )
                }
            }

            val chart = onNodeWithTag(CADENCE_MOOD_CHART_TAG).fetchSemanticsNode()
            val label = onNodeWithText("нед 12").fetchSemanticsNode()

            val overhang = (label.positionInRoot.x + label.size.width) - (chart.positionInRoot.x + chart.size.width)

            assertTrue(overhang <= 1f, "«нед 12» hangs ${abs(overhang)}px past the right edge")
        }
}
