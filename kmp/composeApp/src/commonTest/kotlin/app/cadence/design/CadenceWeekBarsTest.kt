package app.cadence.design

import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import kotlin.math.abs
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

@OptIn(ExperimentalTestApi::class)
class CadenceWeekBarsTest {
    @Test
    fun theScaleGrowsWhenAValueBeatsTheGoal() {
        // Against the mutation «scale by the goal»: a day over the goal would
        // otherwise draw past the top of the chart.
        assertEquals(1890.0, weekScale(values = listOf(1500.0, 1700.0), goal = 1800.0), absoluteTolerance = 0.001)
        assertEquals(2100.0, weekScale(values = listOf(1500.0, 2000.0), goal = 1800.0), absoluteTolerance = 0.001)
    }

    @Test
    fun theWeekBarFractionIsClampedAndSurvivesAZeroScale() {
        assertEquals(0.5f, weekBarFraction(value = 945.0, scale = 1890.0))
        assertEquals(1f, weekBarFraction(value = 2000.0, scale = 1890.0), "past the scale is not a bar overflowing")
        assertEquals(0f, weekBarFraction(value = -1.0, scale = 1890.0))
        assertEquals(0f, weekBarFraction(value = 900.0, scale = 0.0), "a zero scale divides")
    }

    @Test
    fun theWeekGoalFractionIsClampedAndSurvivesAZeroScale() {
        assertEquals(0.5f, weekGoalFraction(goal = 945.0, scale = 1890.0))
        assertEquals(1f, weekGoalFraction(goal = 2000.0, scale = 1890.0), "past the scale still draws inside the chart")
        assertEquals(0f, weekGoalFraction(goal = -1.0, scale = 1890.0))
        assertEquals(0f, weekGoalFraction(goal = 900.0, scale = 0.0), "a zero scale divides")
    }

    @Test
    fun theWeekBarsEachFillToTheirOwnFractionOfTheScale() =
        runComposeUiTest {
            // Today (day 0), the week's maximum (day 1) and a plain day (day 2)
            // are three different indices, and none of the three values equals
            // the goal — so "by index", "by max" and "by today" cannot agree by
            // accident. scale = max(600, 1800, 1200, goal 900) * 1.05 = 1890.
            // Against the mutation «every bar reads day 0's value»: day 0 alone
            // cannot catch it (it would still read 600/1890), which is exactly
            // why day 1 is measured too.
            setContent { CadenceTheme { ShortWeekFixture() } }

            val row = onNodeWithTag(CADENCE_WEEK_ROW_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            val day0 = onNodeWithTag(weekBarTag(0), useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            val day1 = onNodeWithTag(weekBarTag(1), useUnmergedTree = true).fetchSemanticsNode().boundsInRoot

            val ratio0 = day0.height / row.height
            val ratio1 = day1.height / row.height
            assertTrue(
                abs(ratio0 - 600f / 1890f) < BAR_FRACTION_TOLERANCE,
                "day 0 filled to $ratio0, not 600/1890 of the row",
            )
            assertTrue(
                abs(ratio1 - 1800f / 1890f) < BAR_FRACTION_TOLERANCE,
                "day 1 filled to $ratio1, not 1800/1890 of the row",
            )
        }

    @Test
    fun theTodayColumnIsSelectedAndItsNeighbourIsNot() =
        runComposeUiTest {
            // Same separated fixture as theWeekBarsEachFillToTheirOwnFractionOfTheScale:
            // today (day 0) is neither the maximum nor equal to the goal, so this
            // checks selection alone rather than any of those other axes.
            // Against the mutation «selected = false» (or any constant): every
            // column would read alike, and this is the only signal a test can
            // reach for the highlight — a colour painted on a Box is invisible
            // to a query, the same reasoning CadenceTabBar's Destination is
            // built on.
            setContent { CadenceTheme { ShortWeekFixture() } }

            onNodeWithTag(weekBarTag(0), useUnmergedTree = true).assertIsSelected()
            onNodeWithTag(weekBarTag(1), useUnmergedTree = true).assertIsNotSelected()
        }

    @Test
    fun theTodayColumnPaintsForestAndItsNeighbourPaintsSomethingElse() =
        runComposeUiTest {
            // theTodayColumnIsSelectedAndItsNeighbourIsNot reaches only
            // `selected` semantics — this component's own comment concedes a
            // colour painted on a Box is invisible to a query — so
            // `if (isToday) forest700 else sand500.copy(0.7f)` collapsing to
            // always the resting colour leaves `selected` wired correctly
            // while today's own visual distinction is gone, and nothing else
            // in this file catches it.
            setContent { CadenceTheme { ShortWeekFixture() } }

            val today = onNodeWithTag(weekBarTag(0), useUnmergedTree = true).captureToImage()
            val other = onNodeWithTag(weekBarTag(1), useUnmergedTree = true).captureToImage()
            val todayPixel = today.toPixelMap()[today.width / 2, today.height / 2]
            val otherPixel = other.toPixelMap()[other.width / 2, other.height / 2]

            assertEquals(CadenceColors.forest700, todayPixel, "today's own bar is not painted forest700")
            assertNotEquals(todayPixel, otherPixel, "today's bar and its resting neighbour paint the same colour")
        }

    @Test
    fun theGoalLineCoincidesWithABarThatEqualsTheGoal() =
        runComposeUiTest {
            // Day 0's value equals the goal on purpose: its bar top and the
            // dashed goal line must land at the same y. Against the mutation
            // that centred the line inside the caption's Row instead of the
            // full chart height: the caption's ~13dp intrinsic text height ate
            // space the two weighted spacers should have divided, and the
            // line landed rowHeight × (goalFraction − 0.5) off — ~6dp, on an
            // 84dp chart.
            setContent { CadenceTheme { GoalAtMaxFixture() } }

            val row = onNodeWithTag(CADENCE_WEEK_ROW_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            val bar = onNodeWithTag(weekBarTag(0), useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            val line =
                onNodeWithTag(CADENCE_WEEK_GOAL_LINE_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot

            val gap = abs(bar.top - line.center.y) / row.height
            assertTrue(
                gap < BAR_FRACTION_TOLERANCE,
                "the goal line sits $gap of the row's height from a bar at the goal",
            )
        }

    @Test
    fun theGoalCaptionOffsetPrefersAboveTheLineAndFallsBackBelowIt() {
        // Room above (a tall chart, a line well clear of the top): the caption
        // sits GOAL_CAPTION_GAP above it. Against the mutation that always
        // returns the above-the-line candidate regardless of room: this case
        // alone can't tell the two apart, which is why the next assertion
        // exists.
        assertEquals(20.dp, weekGoalCaptionOffset(lineY = 40.dp, chartHeight = 84.dp, captionHeight = 16.dp))

        // No room above (lineY = 4dp, less than gap + caption height): falls
        // back to just below the line instead of drawing off the top of the
        // chart. Against the mutation above: this returns a negative offset
        // (4 - 4 - 16 = -16.dp) instead of the below-the-line fallback (9.dp).
        assertEquals(9.dp, weekGoalCaptionOffset(lineY = 4.dp, chartHeight = 84.dp, captionHeight = 16.dp))

        // A chart too short for even the fallback to fit: clamped to the
        // floor rather than drawn past the bottom. Against the mutation that
        // drops the final coerceIn: this returns 7.dp (the raw fallback)
        // instead of 0.dp (the clamped floor, since chartHeight - captionHeight
        // is negative here and coerced up to zero).
        assertEquals(0.dp, weekGoalCaptionOffset(lineY = 2.dp, chartHeight = 10.dp, captionHeight = 16.dp))

        // Same lineY/chartHeight as the first case, a taller captionHeight:
        // the result moves with it (6.dp, not 20.dp). Against the mutation
        // that hardcodes a captionHeight inside the function instead of using
        // the parameter: this would still return 20.dp, proving the function
        // genuinely takes the caller's measured height rather than assuming
        // one internally — the exact defect this parameter was added to fix.
        assertEquals(6.dp, weekGoalCaptionOffset(lineY = 40.dp, chartHeight = 84.dp, captionHeight = 30.dp))
    }

    @Test
    fun theGoalCaptionStaysInsideTheChart() =
        runComposeUiTest {
            // goal (1800) is the week's own maximum, so goalFraction = 1/1.05
            // ≈ 0.952 — the common case where the line sits near the very top
            // of the chart. Against the mutation that offsets the caption by
            // a fixed nudge above the line (lineY - 8.dp) instead of
            // weekGoalCaptionOffset: with lineY ≈ 4dp that nudge goes
            // negative, and a Box does not clip its children, so the caption
            // drew above the chart into whatever a screen places over it.
            setContent { CadenceTheme { GoalAtMaxFixture() } }

            val row = onNodeWithTag(CADENCE_WEEK_ROW_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            val caption =
                onNodeWithTag(CADENCE_WEEK_GOAL_CAPTION_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot

            assertTrue(
                caption.top >= row.top - 1f,
                "the caption's top (${caption.top}) is above the chart (${row.top})",
            )
            assertTrue(
                caption.bottom <= row.bottom + 1f,
                "the caption's bottom (${caption.bottom}) is below the chart (${row.bottom})",
            )
        }

    @Test
    fun theGoalCaptionStaysInsideTheChartAtALargeFontScale() =
        runComposeUiTest {
            // A low goal (100 against a scale of 1890) puts the line near the
            // bottom of the chart — lineY ≈ 79.6dp of the 84dp band — so there
            // is room above at any caption height this test produces, and the
            // above-branch's own top is what has to stay honest. Against the
            // mutation that computes the offset from an assumed caption
            // height while the real, rendered box is taller (round 2's
            // GOAL_CAPTION_HEIGHT = 16.dp, reintroduced as a hardcoded
            // argument to weekGoalCaptionOffset instead of the measured
            // captionPlaceable.height): a default-scale fixture cannot catch
            // this, because the real caption there (~14dp) is smaller than
            // the old assumption and the bug never triggers. 2.5x font scale
            // makes the real caption taller than 16dp, so the assumed-height
            // top computes a box whose real bottom lands past the chart.
            setContent {
                val scaled = Density(LocalDensity.current.density, fontScale = 2.5f)
                CompositionLocalProvider(LocalDensity provides scaled) {
                    CadenceTheme {
                        CadenceWeekBars(
                            values = listOf(1800.0),
                            labels = listOf("Пн"),
                            goal = 100.0,
                            goalLabel = "цель 100",
                            todayIndex = 0,
                        )
                    }
                }
            }

            val row = onNodeWithTag(CADENCE_WEEK_ROW_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            val caption =
                onNodeWithTag(CADENCE_WEEK_GOAL_CAPTION_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot

            assertTrue(
                caption.top >= row.top - 1f,
                "the caption's top (${caption.top}) is above the chart (${row.top})",
            )
            assertTrue(
                caption.bottom <= row.bottom + 1f,
                "the caption's bottom (${caption.bottom}) is below the chart (${row.bottom})",
            )
        }

    @Test
    fun everyLabelCentresOnItsOwnBarAtAFullWeek() =
        runComposeUiTest {
            // The bar row spaces its columns with Arrangement.spacedBy(xxs);
            // the label row below it did not, so the two rows' column
            // boundaries disagreed and a label drifted away from its own bar
            // — worst at seven columns, where the gaps compound. All seven
            // days share one value so every bar is the same width, isolating
            // the alignment axis from the fill-fraction one.
            val labels = listOf("Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Сег")
            setContent {
                CadenceTheme {
                    CadenceWeekBars(
                        values = List(labels.size) { 900.0 },
                        labels = labels,
                        goal = 900.0,
                        goalLabel = "цель 900",
                        todayIndex = labels.lastIndex,
                    )
                }
            }

            labels.forEachIndexed { index, label ->
                val bar = onNodeWithTag(weekBarTag(index), useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
                val text = onNodeWithText(label, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot

                val drift = abs(bar.center.x - text.center.x)
                assertTrue(drift < LABEL_DRIFT_TOLERANCE_PX, "label \"$label\" drifted ${drift}px from its own bar")
            }
        }
}

/**
 * Today (day 0), the week's maximum (day 1) and a plain day (day 2) at three different
 * indices, none equal to the goal. Shared by every test that only needs "a week", not a
 * goal at its own maximum — see [GoalAtMaxFixture] for that shape.
 */
@Composable
private fun ShortWeekFixture() {
    CadenceWeekBars(
        values = listOf(600.0, 1800.0, 1200.0),
        labels = listOf("Пн", "Вт", "Ср"),
        goal = 900.0,
        goalLabel = "цель 900",
        todayIndex = 0,
    )
}

/**
 * A two-day week whose goal equals day 0's own value — the fixture the goal-line and
 * goal-caption tests need, where the target sits at the week's own maximum.
 */
@Composable
private fun GoalAtMaxFixture() {
    CadenceWeekBars(
        values = listOf(1800.0, 900.0),
        labels = listOf("Пн", "Вт"),
        goal = 1800.0,
        goalLabel = "цель 1800",
        todayIndex = 0,
    )
}

/** Sub-pixel rounding slack for [everyLabelCentresOnItsOwnBarAtAFullWeek] — not the ~1.7dp drift it exists to catch. */
private const val LABEL_DRIFT_TOLERANCE_PX = 2f
