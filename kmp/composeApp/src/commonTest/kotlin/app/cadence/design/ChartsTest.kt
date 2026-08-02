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
 * The geometry is asserted directly, not through a Canvas.
 *
 * A NaN coordinate does not crash a draw, it paints nothing, so a test that
 * measured only the box would stay green over a chart that renders empty —
 * which is exactly how the two divide-by-zero guards went unverified in the
 * first version of this file.
 *
 * The degenerate series are not invented edge cases: a patient in their first
 * week has one measurement, and three identical readings are an ordinary week.
 */
@OptIn(ExperimentalTestApi::class)
class ChartsTest {
    private val pad = 2f
    private val w = 120f
    private val h = 36f

    @Test
    fun anOrdinarySeriesSpansTheBoxFromTheLowestValueToTheHighest() {
        val points = sparkPoints(listOf(0f, 0.5f, 1f), w, h, pad)

        assertEquals(2f, points.first().x, "the first point is not at the left padding")
        assertEquals(118f, points.last().x, "the last point is not at the right padding")
        // y is inverted relative to the value — the one thing about this formula
        // that is easy to get backwards.
        assertEquals(2f, points.last().y, "the highest value is not at the top")
        assertEquals(34f, points.first().y, "the lowest value is not at the bottom")
    }

    @Test
    fun aFlatSeriesSitsOnTheCentreLineRatherThanTheCeiling() {
        // Deliberately unlike the prototype, which pins it to the top. With a
        // fill that reads as "at maximum", and «110, 110, 110» draws exactly
        // like «0,5 · 0,5 · 0,5» — the vertical axis stops meaning anything
        // while still looking like it does.
        listOf(listOf(0.5f, 0.5f, 0.5f), listOf(110f, 110f, 110f)).forEach { series ->
            sparkPoints(series, w, h, pad).forEach {
                assertEquals(h / 2f, it.y, "a flat series is not on the centre line")
            }
        }
    }

    @Test
    fun aSingleMeasurementSitsInTheMiddleRatherThanInACorner() {
        val points = sparkPoints(listOf(0.5f), w, h, pad)

        assertEquals(1, points.size)
        // Top-left would read as an old, high reading: the trailing dot marks
        // "now" everywhere else, and the top of the box means the maximum.
        assertEquals(w / 2f, points.single().x, "a single measurement is not centred horizontally")
        assertEquals(h / 2f, points.single().y, "a single measurement is not centred vertically")
    }

    @Test
    fun everyPointOfEveryDegenerateSeriesIsAFiniteCoordinateInsideTheBox() {
        mapOf(
            "flat series" to listOf(0.5f, 0.5f, 0.5f),
            "single point" to listOf(0.5f),
            "two identical points" to listOf(1f, 1f),
            "a NaN among real values" to listOf(1f, Float.NaN, 2f),
            "an infinity among real values" to listOf(1f, Float.POSITIVE_INFINITY, 2f),
        ).forEach { (name, series) ->
            sparkPoints(series, w, h, pad).forEachIndexed { i, p ->
                assertTrue(p.x.isFinite(), "$name gave point $i a non-finite x (${p.x})")
                assertTrue(p.y.isFinite(), "$name gave point $i a non-finite y (${p.y})")
                assertTrue(p.x in 0f..w, "$name put point $i outside the box at x=${p.x}")
                assertTrue(p.y in 0f..h, "$name put point $i outside the box at y=${p.y}")
            }
        }
    }

    @Test
    fun nonFiniteValuesAreDroppedRatherThanPoisoningTheWholeSeries() {
        // One NaN reaching the arithmetic makes max, min and every coordinate
        // NaN, and the chart vanishes entirely. A measurement pipeline that
        // divides can produce one, so the series is filtered rather than
        // trusted.
        assertEquals(2, sparkPoints(listOf(1f, Float.NaN, 2f), w, h, pad).size)
        assertEquals(0, sparkPoints(listOf(Float.NaN, Float.NaN), w, h, pad).size)
    }

    @Test
    fun aSeriesWithNothingInItProducesNoPoints() {
        assertEquals(0, sparkPoints(emptyList(), w, h, pad).size)
    }

    @Test
    fun aBoxSmallerThanItsOwnPaddingStillKeepsItsPointsInside() {
        // Not a crash and not mirrored coordinates: the inner box clamps at
        // zero, so the promise "points are inside the box" holds unconditionally
        // rather than above some size.
        sparkPoints(listOf(0f, 1f), canvasWidth = 3f, canvasHeight = 3f, pad = pad).forEach {
            assertTrue(it.x in 0f..3f, "x escaped a tiny box at ${it.x}")
            assertTrue(it.y in 0f..3f, "y escaped a tiny box at ${it.y}")
        }
    }

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
    fun sparkComposesForEverySeriesIncludingTheOnesThatDrawNoLine() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    Box {
                        CadenceSpark(listOf(0.5f, 0.5f, 0.5f), Modifier.testTag("flat"))
                        CadenceSpark(listOf(0.5f), Modifier.testTag("single"))
                        // Draws a baseline rather than nothing, so "no values"
                        // is distinguishable from "failed to load".
                        CadenceSpark(emptyList(), Modifier.testTag("empty"))
                    }
                }
            }

            listOf("flat", "single", "empty").forEach {
                onNodeWithTag(it).assertWidthIsEqualTo(120.dp)
            }
        }
}
