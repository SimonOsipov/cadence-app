package app.cadence.design

import androidx.compose.foundation.layout.Box
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertHeightIsEqualTo
import androidx.compose.ui.test.assertWidthIsEqualTo
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A drawn shape has no text to query, so what is asserted here is layout and
 * survival. Whether the line is in the right place is step 11's side-by-side
 * against the running prototype.
 *
 * The degenerate series are not edge cases invented for the test: a patient
 * with one measurement, or three identical ones, is an ordinary first week, and
 * both divide by zero in the prototype's formula.
 */
@OptIn(ExperimentalTestApi::class)
class ChartsTest {
    @Test
    fun sparkOccupiesTheSizeItWasGiven() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceSpark(
                        data = listOf(0.3f, 0.5f, 0.4f, 0.6f, 0.55f, 0.7f, 0.8f),
                        modifier = Modifier.testTag("spark"),
                        width = 120.dp,
                        height = 36.dp,
                    )
                }
            }

            onNodeWithTag("spark").assertWidthIsEqualTo(120.dp).assertHeightIsEqualTo(36.dp)
        }

    @Test
    fun everyPointOfADegenerateSeriesIsStillAFiniteCoordinate() {
        // Asserted on the geometry rather than through a Canvas on purpose: a
        // NaN coordinate does not crash a draw, it paints nothing. A test that
        // only measured the box would stay green over an empty chart, which is
        // exactly how the two guards below went unverified at first.
        mapOf(
            "flat series" to listOf(0.5f, 0.5f, 0.5f),
            "single point" to listOf(0.5f),
            "two identical points" to listOf(1f, 1f),
        ).forEach { (name, series) ->
            val points = sparkPoints(series, canvasWidth = 120f, canvasHeight = 36f)

            assertEquals(series.size, points.size, "$name lost points")
            points.forEachIndexed { i, p ->
                assertTrue(p.x.isFinite(), "$name gave point $i a non-finite x (${p.x})")
                assertTrue(p.y.isFinite(), "$name gave point $i a non-finite y (${p.y})")
                assertTrue(p.x in 0f..120f, "$name put point $i outside the box at x=${p.x}")
                assertTrue(p.y in 0f..36f, "$name put point $i outside the box at y=${p.y}")
            }
        }
    }

    @Test
    fun anOrdinarySeriesSpansTheBoxFromTheLowestValueToTheHighest() {
        val points = sparkPoints(listOf(0f, 0.5f, 1f), canvasWidth = 120f, canvasHeight = 36f)

        // 2px padding on every side, so the drawable box is 116 × 32.
        assertEquals(2f, points.first().x, "the first point is not at the left padding")
        assertEquals(118f, points.last().x, "the last point is not at the right padding")
        // The highest value sits at the top, the lowest at the bottom: y is
        // inverted relative to the value, which is the one thing about this
        // formula that is easy to get backwards.
        assertEquals(2f, points.last().y, "the highest value is not at the top")
        assertEquals(34f, points.first().y, "the lowest value is not at the bottom")
    }

    @Test
    fun sparkSurvivesTheSeriesThatDivideByZero() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    Box {
                        // max == min: the vertical span is zero.
                        CadenceSpark(listOf(0.5f, 0.5f, 0.5f), Modifier.testTag("flat"))
                        // n - 1 == 0: the horizontal step is zero.
                        CadenceSpark(listOf(0.5f), Modifier.testTag("single"))
                        // Nothing to draw at all.
                        CadenceSpark(emptyList(), Modifier.testTag("empty"))
                    }
                }
            }

            listOf("flat", "single", "empty").forEach {
                onNodeWithTag(it).assertWidthIsEqualTo(120.dp)
            }
        }
}
