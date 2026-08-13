package app.cadence.design

import androidx.compose.foundation.layout.width
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performTouchInput
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.DoseBand
import app.cadence.shared.domain.DoseUnit
import app.cadence.shared.domain.Measurement
import app.cadence.shared.domain.MeasurementId
import app.cadence.shared.domain.MeasurementSource
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.MetricAccent
import app.cadence.shared.domain.ProtocolMark
import app.cadence.shared.domain.ProtocolMarkKind
import app.cadence.shared.domain.TrendRange
import app.cadence.shared.domain.TrendSeries
import app.cadence.shared.domain.UserId
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalDateTime
import kotlinx.datetime.LocalTime
import kotlinx.datetime.TimeZone
import kotlinx.datetime.toInstant
import kotlin.math.abs
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

private const val TOLERANCE = 1e-3f

/** A pixel or two of rounding is not a defect; a whole point of drift is. */
private const val PIXEL_TOLERANCE = 2f

private val INSET = ChartInset(left = 10f, right = 10f, top = 20f, bottom = 20f)

private val WINDOW = TrendRange(LocalDate(2026, 5, 28), LocalDate(2026, 5, 31))

private fun reading(
    day: Int,
    value: Double,
): Measurement =
    Measurement(
        id = MeasurementId("m-$day"),
        patientId = UserId("p"),
        metric = Metric.WEIGHT,
        value = value,
        unit = "kg",
        measuredAt = LocalDateTime(LocalDate(2026, 5, day), LocalTime(6, 0)).toInstant(TimeZone.UTC),
        source = MeasurementSource.MANUAL,
        externalId = null,
        note = null,
    )

private fun series(vararg values: Double): TrendSeries =
    TrendSeries(
        metric = Metric.WEIGHT,
        range = WINDOW,
        points = values.mapIndexed { index, value -> reading(28 + index, value) },
        zone = TimeZone.UTC,
    )

/** Readings on named days of the window, for series that are not one-per-day. */
private fun sparseSeries(vararg readings: Pair<Int, Double>): TrendSeries =
    TrendSeries(
        metric = Metric.WEIGHT,
        range = TrendRange(LocalDate(2026, 5, 1), LocalDate(2026, 5, 31)),
        points = readings.map { (day, value) -> reading(day, value) },
        zone = TimeZone.UTC,
    )

private fun readingsOf(vararg pairs: Pair<Float, Float>): List<ChartReading> =
    pairs.mapIndexed { index, (fraction, value) -> ChartReading(index, fraction, value) }

class ScrubPointsTest {
    @Test
    fun theReadingsSpreadEvenlyAcrossTheWidthInsideTheBorders() {
        val points =
            scrubPoints(
                readingsOf(0f to 1f, 0.5f to 2f, 1f to 3f),
                canvasWidth = 210f,
                canvasHeight = 100f,
                inset = INSET,
            )

        assertEquals(listOf(10f, 105f, 200f), points.map { it.x })
    }

    @Test
    fun aHigherReadingSitsHigherOnTheCanvas() {
        // Screen coordinates run downwards, so «higher value» must mean
        // «smaller y». A chart that got this backwards would draw every weight
        // loss as a climb.
        val points = scrubPoints(readingsOf(0f to 1f, 1f to 3f), canvasWidth = 210f, canvasHeight = 100f, inset = INSET)

        assertTrue(points[1].y < points[0].y, "3 should sit above 1")
    }

    @Test
    fun neitherThePeakNorTheTroughTouchesTheBorder() {
        // Twelve percent of headroom at both ends, as in the prototype. Without
        // it a peak kisses the top border and reads as clipped — as though the
        // chart had more to show and ran out of room.
        val points = scrubPoints(readingsOf(0f to 1f, 1f to 3f), canvasWidth = 210f, canvasHeight = 100f, inset = INSET)
        val plotTop = INSET.top
        val plotBottom = 100f - INSET.bottom

        assertTrue(points.minOf { it.y } > plotTop, "the peak is off the top border")
        assertTrue(points.maxOf { it.y } < plotBottom, "the trough is off the bottom border")
        // Exactly the headroom, not merely some: 0,12 / (1 + 2 × 0,12) of the
        // plot's height at each end.
        val expected = plotTop + (plotBottom - plotTop) * (0.12f / 1.24f)
        assertTrue(abs(points.minOf { it.y } - expected) < TOLERANCE, "headroom is ${points.minOf { it.y }}")
    }

    @Test
    fun aFlatSeriesRunsThroughTheMiddleRatherThanAlongAnEdge() {
        // `sparkPoints`' answer, for its reason: pinned to the top with a fill,
        // «98,4 · 98,4 · 98,4» draws identically to a series that climbed.
        val points =
            scrubPoints(
                readingsOf(0f to 98.4f, 0.5f to 98.4f, 1f to 98.4f),
                canvasWidth = 210f,
                canvasHeight = 100f,
                inset = INSET,
            )

        assertEquals(listOf(50f, 50f, 50f), points.map { it.y })
    }

    @Test
    fun oneReadingSitsInTheMiddleOfBothAxes() {
        val points = scrubPoints(readingsOf(0.5f to 98.4f), canvasWidth = 210f, canvasHeight = 100f, inset = INSET)

        assertEquals(listOf(Offset(105f, 50f)), points)
    }

    @Test
    fun nothingToDrawIsAnEmptyListAndNotACrash() {
        assertEquals(emptyList(), scrubPoints(emptyList(), 210f, 100f, INSET))
    }

    @Test
    fun aBoxSmallerThanItsOwnBordersHasNoInsideRatherThanANegativeOne() {
        val points = scrubPoints(readingsOf(0f to 1f, 1f to 2f), canvasWidth = 4f, canvasHeight = 4f, inset = INSET)

        // Both sides: an unclamped inner width mirrors the points past the
        // left border instead of collapsing them onto it.
        assertEquals(listOf(INSET.left, INSET.left), points.map { it.x })
    }
}

class ChartReadingsTest {
    @Test
    fun aReadingIsPlacedOnTheDayItWasTakenAndNotOnItsTurnInTheList() {
        // The defect this exists to prevent: weight is weekly and the tape is
        // measured on Sundays, so on a month-long window four readings spread
        // evenly across the plot while the dose bands and the titration
        // hairline below stand at their true dates — putting a dose change
        // under the wrong dot.
        val sparse = sparseSeries(1 to 100.0, 8 to 99.0, 15 to 98.0, 31 to 97.0)

        val placed = chartReadings(sparse).map { it.fraction }

        // 31 days: day 1 owns [0, 1/31], day 8 owns [7/31, 8/31], and each mark
        // stands in the middle of its own day.
        assertEquals(0.5f / 31f, placed[0], TOLERANCE)
        assertEquals(7.5f / 31f, placed[1], TOLERANCE)
        assertEquals(14.5f / 31f, placed[2], TOLERANCE)
        assertEquals(30.5f / 31f, placed[3], TOLERANCE)
        // Ordinal spacing would have said 0, ⅓, ⅔, 1 — and does not.
        assertTrue(placed[1] < 0.34f, "the second reading sits where its date is, not a third of the way along")
    }

    @Test
    fun aReadingThatCannotBeDrawnIsDroppedButItsNeighboursKeepTheirNumbers() {
        // A measurement pipeline that divides can produce a NaN, and one of
        // them makes every coordinate NaN — a chart that vanishes without an
        // error. Dropping it renumbers what is left unless the original index
        // travels along, and then a scrub reports the reading beside the one
        // the cursor stands on.
        val withHole =
            TrendSeries(
                metric = Metric.WEIGHT,
                range = WINDOW,
                points = listOf(reading(28, Double.NaN), reading(29, 99.0), reading(30, 98.0)),
                zone = TimeZone.UTC,
            )

        assertEquals(listOf(1, 2), chartReadings(withHole).map { it.index })
    }
}

class PlotXTest {
    @Test
    fun aFractionLandsOnTheSameAxisTheReadingsDo() {
        // The marks and the line share one mapping. Measured against
        // `scrubPoints` rather than against a literal: a mark laid out over the
        // full width while the readings are inset leans right on every chart,
        // and an assertion about `middleOf` alone would never see it.
        val points =
            scrubPoints(
                readingsOf(0f to 1f, 0.5f to 2f, 1f to 3f),
                canvasWidth = 210f,
                canvasHeight = 100f,
                inset = INSET,
            )

        assertEquals(points.first().x, plotX(0f, 210f, INSET))
        assertEquals(points.last().x, plotX(1f, 210f, INSET))
        assertEquals(points[1].x, plotX(0.5f, 210f, INSET))
    }

    @Test
    fun aBoxNarrowerThanItsOwnBordersDoesNotRunBackwards() {
        assertEquals(INSET.left, plotX(1f, canvasWidth = 4f, inset = INSET))
    }
}

class AccentTest {
    @Test
    fun theTwoFamiliesAreTwoDifferentColours() {
        // The colour reaches a canvas and a border, and neither lands in
        // semantics — so an inverted branch is invisible to a UI test and has
        // to be named to be measured.
        assertEquals(CadenceColors.forest700, accentStroke(MetricAccent.FOREST))
        assertEquals(CadenceColors.sand700, accentStroke(MetricAccent.SAND))
    }
}

class ChartInsetTest {
    @Test
    fun theBordersAreTheSameOnesTheGestureAndTheCursorRead() {
        // The single place four `Dp` constants become pixels. Swapping `top`
        // for `bottom` here survives every other test in this file.
        val inset = Density(2f).chartInset()

        assertEquals(ChartInset(left = 16f, right = 16f, top = 40f, bottom = 32f), inset)
    }
}

class NearestIndexTest {
    @Test
    fun theNearestReadingWinsAndNotTheOneToTheLeft() {
        val points = listOf(Offset(0f, 0f), Offset(100f, 0f), Offset(200f, 0f))

        assertEquals(0, nearestIndex(40f, points))
        assertEquals(1, nearestIndex(60f, points), "past the midpoint the next reading is nearer")
        assertEquals(2, nearestIndex(199f, points), "the last reading is reachable at the right edge")
        // Exactly between two, the earlier one wins — arbitrary, but fixed, so
        // a finger held on a midpoint does not flicker between readings.
        assertEquals(0, nearestIndex(50f, points))
    }

    @Test
    fun aTouchOutsideTheReadingsLandsOnTheNearestEnd() {
        val points = listOf(Offset(10f, 0f), Offset(200f, 0f))

        assertEquals(0, nearestIndex(-50f, points))
        assertEquals(1, nearestIndex(9_000f, points))
    }

    @Test
    fun nothingToPointAtIsNullRatherThanZero() {
        // Zero would index into an empty list at the call site.
        assertNull(nearestIndex(10f, emptyList()))
    }
}

@OptIn(ExperimentalTestApi::class)
private fun ComposeUiTest.boundsOf(tag: String): Rect =
    onNodeWithTag(tag, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot

/**
 * The chart is laid out in `Dp` and measured in pixels, and the tests run at
 * whatever density the simulator has. These turn the production constants into
 * the units an assertion reads, rather than restating them as literals.
 */
private const val CHART_INSET_X_DP = 8f
private const val BAND_INSET_DP = 2f

/** Density is 1 in the Compose test environment unless a test sets one. */
private fun insetPx(): Float = CHART_INSET_X_DP

private fun bandInsetPx(): Float = BAND_INSET_DP

private fun plotWidth(chart: Rect): Float = chart.width - insetPx() * 2

/**
 * Where a reading of [series] should land, in root coordinates.
 *
 * Derived from the same placement the component uses rather than from hand
 * arithmetic — the point of these assertions is that the cursor is *applied*
 * where `chartReadings` says, and hand-computed literals drift from the fixture
 * the moment a test adds a reading.
 */
private fun expectedX(
    chart: Rect,
    series: TrendSeries,
    index: Int,
): Float = chart.left + insetPx() + chartReadings(series)[index].fraction * plotWidth(chart)

private fun assertClose(
    expected: Float,
    actual: Float,
    message: String,
) = assertTrue(abs(expected - actual) < PIXEL_TOLERANCE, "$message: expected $expected, was $actual")

@OptIn(ExperimentalTestApi::class)
class ScrubChartTest {
    @Test
    fun theCursorStandsOnTheReadingItWasAskedFor() =
        runComposeUiTest {
            // Measured, not described — and to the pixel rather than to a
            // quarter of the width: a quarter stops telling one reading from
            // the next as soon as a series has six of them, and it hides the
            // half-cursor the offset takes back to centre it.
            val data = series(100.0, 99.0, 98.0)
            setContent {
                CadenceTheme {
                    CadenceScrubChart(data, Modifier.width(300.dp), scrubIndex = 0)
                }
            }

            val chart = boundsOf(CADENCE_SCRUB_CHART_TAG)
            val cursor = boundsOf(CADENCE_SCRUB_CURSOR_TAG)
            assertClose(expectedX(chart, data, 0), cursor.center.x, "the cursor sits on the first reading")
            assertTrue(cursor.center.y > chart.top, "and inside the plot rather than at its corner")
            assertTrue(cursor.center.y < chart.bottom)
        }

    @Test
    fun theCursorMovesToWhicheverReadingItIsGiven() {
        // One render, three indices, compared against each other. The
        // previous shape of this test rendered one configuration and
        // asserted an absolute position, so «moves» existed only as the
        // difference between two test bodies.
        val positions = mutableListOf<Float>()

        listOf(0, 1, 2).forEach { index ->
            runComposeUiTest {
                setContent {
                    CadenceTheme {
                        CadenceScrubChart(series(100.0, 99.0, 98.0), Modifier.width(300.dp), scrubIndex = index)
                    }
                }
                val chart = boundsOf(CADENCE_SCRUB_CHART_TAG)
                positions += boundsOf(CADENCE_SCRUB_CURSOR_TAG).center.x - chart.left
            }
        }

        assertEquals(positions.sorted(), positions, "later readings sit further right")
        assertEquals(3, positions.toSet().size, "three readings, three places")
    }

    @Test
    fun theCursorAlsoMovesUpAndDownWithTheReading() {
        // The y is the other half of «the cursor stands on the reading».
        // Asserted by ordering rather than by a literal, so the headroom
        // constant stays free to change: a falling series draws downwards.
        val heights = mutableListOf<Float>()

        listOf(0, 1, 2).forEach { index ->
            runComposeUiTest {
                setContent {
                    CadenceTheme {
                        CadenceScrubChart(series(100.0, 99.0, 98.0), Modifier.width(300.dp), scrubIndex = index)
                    }
                }
                val chart = boundsOf(CADENCE_SCRUB_CHART_TAG)
                heights += boundsOf(CADENCE_SCRUB_CURSOR_TAG).center.y - chart.top
            }
        }

        assertEquals(heights.sorted(), heights, "100 → 99 → 98 falls, and the cursor falls with it")
        assertEquals(3, heights.toSet().size)
    }

    @Test
    fun aChartWithNoIndexOpensOnTheLatestReading() =
        runComposeUiTest {
            val data = series(100.0, 99.0, 98.0)
            setContent {
                CadenceTheme {
                    CadenceScrubChart(data, Modifier.width(300.dp))
                }
            }

            val chart = boundsOf(CADENCE_SCRUB_CHART_TAG)
            val cursor = boundsOf(CADENCE_SCRUB_CURSOR_TAG)

            assertClose(expectedX(chart, data, 2), cursor.center.x, "«now» is where a patient opens a chart")
        }

    @Test
    fun anIndexLeftOverFromALongerWindowFallsBackToTheLatestReading() =
        runComposeUiTest {
            // The window is hoisted into the shell and shared with the detail
            // screen, so an index scrubbed on «весь цикл» outlives a switch to
            // «7 дней». The prototype clamps for this reason; a chart that
            // answered a stale index with no cursor at all would be worse.
            val data = series(100.0, 99.0)
            setContent {
                CadenceTheme {
                    CadenceScrubChart(data, Modifier.width(300.dp), scrubIndex = 30)
                }
            }

            val chart = boundsOf(CADENCE_SCRUB_CHART_TAG)
            val cursor = boundsOf(CADENCE_SCRUB_CURSOR_TAG)

            assertClose(expectedX(chart, data, 1), cursor.center.x, "a stale index falls back to the last reading")
        }

    @Test
    fun aSingleReadingIsDrawnWithACursorOnIt() =
        runComposeUiTest {
            // Every patient's first measurement. There is no line and no area
            // to draw, and a chart that required two readings would show a
            // patient nothing on the day they started.
            setContent {
                CadenceTheme {
                    CadenceScrubChart(series(100.0), Modifier.width(300.dp))
                }
            }

            assertNotNull(onNodeWithTag(CADENCE_SCRUB_CURSOR_TAG, useUnmergedTree = true).fetchSemanticsNode())
        }

    @Test
    fun touchingTheChartReportsTheIndexAndTheReadingUnderTheFinger() =
        runComposeUiTest {
            val scrubbed = mutableListOf<Pair<Int, Double>>()

            setContent {
                CadenceTheme {
                    CadenceScrubChart(
                        series(100.0, 99.0, 98.0),
                        Modifier.width(300.dp),
                        onScrub = { index, point -> scrubbed += index to point.value },
                    )
                }
            }

            onNodeWithTag(CADENCE_SCRUB_CHART_TAG).performTouchInput {
                down(Offset(left + 2f, centerY))
                up()
            }

            // Both halves of «скраб сообщает индекс и значение», and the value
            // is the reading rather than the index looked up a second time.
            assertEquals(0 to 100.0, scrubbed.firstOrNull())
        }

    @Test
    fun draggingAcrossTheChartReportsEveryReadingItPassesOver() =
        runComposeUiTest {
            val scrubbed = mutableListOf<Int>()

            setContent {
                CadenceTheme {
                    CadenceScrubChart(
                        series(100.0, 99.0, 98.0),
                        Modifier.width(300.dp),
                        onScrub = { index, _ -> scrubbed += index },
                    )
                }
            }

            onNodeWithTag(CADENCE_SCRUB_CHART_TAG).performTouchInput {
                val from = left + 2f
                val to = right - 2f
                down(Offset(from, centerY))
                // Stepped by hand rather than with one `moveTo`: a single move
                // event sends the finger straight from one edge to the other,
                // and a handler that reported only the press and the final
                // position would pass while never naming the middle reading.
                (1..8).forEach { step ->
                    moveTo(Offset(from + (to - from) * step / 8f, centerY))
                }
                up()
            }

            assertEquals(listOf(0, 1, 2), scrubbed.distinct(), "the finger passed over all three")
        }

    @Test
    fun theReadingAScrubReportsIsTheOneTheCursorThenStandsOn() =
        runComposeUiTest {
            // The loop the two halves of this component form. The gesture reads
            // the laid-out size, the cursor reads `BoxWithConstraints` — this
            // is the only test where the two have to agree.
            val data = series(100.0, 99.0, 98.0)
            setContent {
                CadenceTheme {
                    var index by remember { mutableStateOf<Int?>(null) }
                    CadenceScrubChart(
                        data,
                        Modifier.width(300.dp),
                        scrubIndex = index,
                        onScrub = { at, _ -> index = at },
                    )
                }
            }

            val chart = boundsOf(CADENCE_SCRUB_CHART_TAG)
            // Aimed at the middle reading rather than at the middle of the box:
            // the readings sit on their own days, and the box's centre falls
            // exactly between two of them, where the tie-break decides.
            val target = expectedX(chart, data, 1)
            onNodeWithTag(CADENCE_SCRUB_CHART_TAG).performTouchInput {
                down(Offset(target - chart.left, centerY))
                up()
            }

            val cursor = boundsOf(CADENCE_SCRUB_CURSOR_TAG)
            assertClose(target, cursor.center.x, "the cursor followed the finger to the middle reading")
        }

    @Test
    fun aMetricWithNoReadingsDrawsNoCursorAndReportsNothing() =
        runComposeUiTest {
            val scrubbed = mutableListOf<Int>()

            setContent {
                CadenceTheme {
                    CadenceScrubChart(
                        TrendSeries(Metric.CHEST, WINDOW, emptyList(), TimeZone.UTC),
                        Modifier.width(300.dp),
                        onScrub = { index, _ -> scrubbed += index },
                    )
                }
            }

            onNodeWithTag(CADENCE_SCRUB_CHART_TAG).performTouchInput {
                down(Offset(centerX, centerY))
                up()
            }

            onNodeWithTag(CADENCE_SCRUB_CURSOR_TAG, useUnmergedTree = true).assertDoesNotExist()
            assertEquals(emptyList(), scrubbed, "there is no reading to report")
        }

    @Test
    fun aProtocolMarkStandsOverTheDayItHappenedOn() =
        runComposeUiTest {
            // The dashed line is painted, because a canvas is the only way to
            // dash — but a node stands at the same x, so its place is measured
            // rather than described. Two marks, three readings: the mark on the
            // last day must land on the last reading's cursor.
            val marks =
                listOf(
                    ProtocolMark(ProtocolMarkKind.START, LocalDate(2026, 5, 28), null, Dose(0.25, DoseUnit.MG)),
                    ProtocolMark(
                        ProtocolMarkKind.TITRATION,
                        LocalDate(2026, 5, 31),
                        Dose(0.25, DoseUnit.MG),
                        Dose(0.5, DoseUnit.MG),
                    ),
                )

            setContent {
                CadenceTheme {
                    CadenceScrubChart(series(100.0, 99.0, 98.0, 97.0), Modifier.width(300.dp), marks = marks)
                }
            }

            val cursor = boundsOf(CADENCE_SCRUB_CURSOR_TAG)
            val first = boundsOf(cadenceScrubMarkTag(0))
            val last = boundsOf(cadenceScrubMarkTag(1))

            assertTrue(first.left < last.left, "the earlier mark stands to the left")
            // The cursor opens on the last reading, taken on the same day the
            // titration happened — so the two share an axis or they do not.
            assertClose(cursor.center.x, last.center.x, "the mark and the reading of that day share one x")
        }

    @Test
    fun eachDoseBandTakesTheShareOfTheRowItsDaysAreWorth() =
        runComposeUiTest {
            // Four days, two bands of two. Measured against the row, because a
            // band drawn full-width regardless of its dates satisfies every
            // assertion about those dates.
            val bands =
                listOf(
                    DoseBand(Dose(0.25, DoseUnit.MG), TrendRange(LocalDate(2026, 5, 28), LocalDate(2026, 5, 29))),
                    DoseBand(Dose(0.5, DoseUnit.MG), TrendRange(LocalDate(2026, 5, 30), LocalDate(2026, 5, 31))),
                )

            setContent {
                CadenceTheme {
                    CadenceScrubChart(series(100.0, 99.0, 98.0, 97.0), Modifier.width(300.dp), bands = bands)
                }
            }

            val row = boundsOf(CADENCE_SCRUB_DOSE_ROW_TAG)
            val first = boundsOf(cadenceScrubBandTag(0))
            val second = boundsOf(cadenceScrubBandTag(1))

            assertClose(row.left, first.left - bandInsetPx(), "the first band opens at the row's left")
            assertClose(row.center.x, second.left - bandInsetPx(), "the second band opens halfway")
            assertTrue(
                abs(first.width - second.width) < PIXEL_TOLERANCE,
                "two bands of two days each are the same width: ${first.width} vs ${second.width}",
            )
            // And each carries its own dose, not the first one's twice.
            onNodeWithText("0,25 мг").assertExists()
            onNodeWithText("0,5 мг").assertExists()
        }

    @Test
    fun aBandSitsUnderTheReadingsOfItsOwnDays() =
        runComposeUiTest {
            // The strip and the plot are two layouts, and only an assertion
            // that crosses them holds them on one axis. A band covering
            // exactly the last day is centred on that day — which is where the
            // cursor of the reading taken that day stands.
            val data = series(100.0, 99.0, 98.0, 97.0)
            val lastDayOnly =
                listOf(DoseBand(Dose(0.5, DoseUnit.MG), TrendRange(LocalDate(2026, 5, 31), LocalDate(2026, 5, 31))))

            setContent {
                CadenceTheme {
                    CadenceScrubChart(data, Modifier.width(300.dp), bands = lastDayOnly)
                }
            }

            val cursor = boundsOf(CADENCE_SCRUB_CURSOR_TAG)
            val band = boundsOf(cadenceScrubBandTag(0))

            assertClose(cursor.center.x, band.center.x, "the band and the reading of that day share one axis")
        }

    @Test
    fun theChartDrawsInsideWhateverHeightItWasGiven() =
        runComposeUiTest {
            // The plot's height is a parameter, and every y in the file is a
            // fraction of it. A height baked into the geometry instead would
            // put the line and the cursor off the bottom of a shorter chart.
            setContent {
                CadenceTheme {
                    // The *lowest* reading, which is the one a geometry built
                    // for a taller chart pushes off the bottom.
                    CadenceScrubChart(series(100.0, 99.0, 98.0), Modifier.width(300.dp), scrubIndex = 2, height = 90.dp)
                }
            }

            val chart = boundsOf(CADENCE_SCRUB_CHART_TAG)
            val cursor = boundsOf(CADENCE_SCRUB_CURSOR_TAG)

            assertTrue(cursor.center.y > chart.top, "the cursor is inside a ${chart.height}px chart")
            assertTrue(
                cursor.center.y < chart.bottom,
                "the cursor is at ${cursor.center.y}, chart ends at ${chart.bottom}",
            )
        }

    @Test
    fun aBandCoveringTheWholeWindowFillsTheRow() =
        runComposeUiTest {
            val whole = listOf(DoseBand(Dose(0.25, DoseUnit.MG), WINDOW))

            setContent {
                CadenceTheme {
                    CadenceScrubChart(series(100.0, 99.0, 98.0, 97.0), Modifier.width(300.dp), bands = whole)
                }
            }

            val row = boundsOf(CADENCE_SCRUB_DOSE_ROW_TAG)
            val band = boundsOf(cadenceScrubBandTag(0))

            // Not «most of it»: a window whose last day was treated as a point
            // rather than a span leaves a quarter of this row empty.
            assertTrue(
                band.width > row.width * 0.9f,
                "the band covers ${band.width} of a ${row.width} row",
            )
        }

    @Test
    fun aChartWithNoBandsHasNoDoseRowAtAll() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceScrubChart(series(100.0, 99.0), Modifier.width(300.dp))
                }
            }

            // An empty strip under the axis reads as a prescription that failed
            // to load rather than one that does not exist.
            onNodeWithTag(CADENCE_SCRUB_DOSE_ROW_TAG, useUnmergedTree = true).assertDoesNotExist()
            assertNotNull(onNodeWithTag(CADENCE_SCRUB_CHART_TAG, useUnmergedTree = true).fetchSemanticsNode())
        }
}
