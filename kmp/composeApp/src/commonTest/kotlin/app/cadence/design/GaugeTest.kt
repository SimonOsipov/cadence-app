package app.cadence.design

import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.v2.runComposeUiTest
import kotlin.math.abs
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

@OptIn(ExperimentalTestApi::class)
class GaugeTest {
    @Test
    fun theFractionIsTheRatioAndIsClampedAtBothEnds() {
        assertEquals(0.5f, gaugeFraction(left = 6, total = 12))
        assertEquals(1f, gaugeFraction(left = 20, total = 12), "not clamped at the top")
        assertEquals(0f, gaugeFraction(left = -3, total = 12), "not clamped at the bottom")
    }

    @Test
    fun aGaugeWithNoTotalDrawsNothingRatherThanDividing() {
        assertEquals(0f, gaugeFraction(left = 6, total = 0))
    }

    @Test
    fun theFillIsAsWideAsItsFraction() =
        runComposeUiTest {
            // Measured, not described. A meter painted full regardless of its
            // input satisfies every assertion about that input, and the syringe
            // bar shipped exactly that defect until its fill became a laid-out
            // node with bounds.
            setContent { CadenceTheme { CadenceGauge(left = 3, total = 12) } }

            val track = onNodeWithTag(CADENCE_GAUGE_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            val fill = onNodeWithTag(CADENCE_GAUGE_FILL_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot

            assertTrue(
                abs(fill.width / track.width - 0.25f) < TOLERANCE,
                "the fill is ${fill.width / track.width} of the track, not 0.25",
            )
        }

    @Test
    fun theGaugeNamesItsOwnNumbers() =
        runComposeUiTest {
            // A meter with no words is a meter a screen reader cannot read, and
            // «сколько осталось» is the one number this card exists to answer.
            setContent { CadenceTheme { CadenceGauge(left = 8, total = 12) } }

            val described =
                onNodeWithTag(CADENCE_GAUGE_TAG, useUnmergedTree = true)
                    .fetchSemanticsNode()
                    .config
                    .getOrNull(SemanticsProperties.ContentDescription)
                    ?.firstOrNull()

            assertEquals("8 из 12 доз · 67 %", described)
        }

    @Test
    fun theUsageRowScalesEveryBarAgainstItsTallestWeek() =
        runComposeUiTest {
            // The prototype draws weekly usage as bars whose heights are
            // relative to the busiest week, not to a fixed maximum — otherwise
            // a patient on one dose a week sees a row of slivers.
            setContent { CadenceTheme { CadenceUsageRow(dosesPerWeek = listOf(2, 8, 4)) } }

            val row = onNodeWithTag(CADENCE_USAGE_ROW_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            val bars =
                (0..2).map {
                    onNodeWithTag(
                        usageBarTag(it),
                        useUnmergedTree = true,
                    ).fetchSemanticsNode().boundsInRoot
                }

            // Against the row, not against itself: `bars[1] / bars[1]` is one
            // for any implementation, including one that draws every bar the
            // same height.
            assertTrue(abs(bars[1].height / row.height - 1f) < TOLERANCE, "the tallest week does not fill the row")
            assertTrue(abs(bars[0].height / row.height - 0.25f) < TOLERANCE, "two of eight is not a quarter")
            assertTrue(abs(bars[2].height / row.height - 0.5f) < TOLERANCE, "four of eight is not a half")
        }

    @Test
    fun aWeekWithNoDosesStillDrawsSomething() =
        runComposeUiTest {
            // A zero-height bar is an invisible bar, and a gap in a row of bars
            // reads as missing data rather than as a week off.
            setContent { CadenceTheme { CadenceUsageRow(dosesPerWeek = listOf(0, 4)) } }

            val empty = onNodeWithTag(usageBarTag(0), useUnmergedTree = true).fetchSemanticsNode().boundsInRoot

            assertTrue(empty.height > 0f, "an empty week is not drawn at all")
        }

    @Test
    fun aRowWithNoWeeksDrawsNothingRatherThanDividingByItsTallest() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceUsageRow(dosesPerWeek = emptyList()) } }

            assertEquals(0, onAllNodesWithTag(usageBarTag(0), useUnmergedTree = true).fetchSemanticsNodes().size)
            // And the row itself is not drawn: a vial with no history would
            // otherwise reserve thirty-two points of empty space under it,
            // which reads as a chart that failed to load.
            assertEquals(
                0,
                onAllNodesWithTag(CADENCE_USAGE_ROW_TAG, useUnmergedTree = true).fetchSemanticsNodes().size,
                "an empty row still takes up its height",
            )
        }
}

/** Half a bar's rounding, in fractions. */
private const val TOLERANCE = 0.03f
