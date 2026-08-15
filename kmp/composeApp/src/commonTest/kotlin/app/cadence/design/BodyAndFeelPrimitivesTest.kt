package app.cadence.design

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals

private const val TOLERANCE = 1e-3f

/**
 * Two fractions that fall on opposite sides of three o'clock, which is what makes the
 * pair of them pin the sweep rather than bound it: at 27.4% the arc has passed three
 * o'clock (98.64°), at 15.0% it has not (54°). No constant sweep can be sand at one and
 * forest at the other, so a hardcoded angle dies on the pair where it survives either
 * alone. Neither matches anything else the body screen draws.
 */
private const val FAT_PERCENT = 27.4
private const val FAT_PERCENT_BEFORE_THREE = 15.0

/**
 * The ring's geometry, derived here rather than read from the component: read from it,
 * a mutation to RING_SIZE or RING_STROKE would move the drawing and the probe together
 * and the test would follow it down. 150dp across with a 14dp stroke puts the centre at
 * 75dp and the middle of the track at (150 - 14) / 2 = 68dp from it.
 */
private val RING_CENTRE = 75.dp
private val RING_TRACK_RADIUS = 68.dp

@OptIn(ExperimentalTestApi::class)
class BodyAndFeelPrimitivesTest {
    private fun ComposeUiTest.centrePixelOf(tag: String): Color {
        val bar = onNodeWithTag(tag, useUnmergedTree = true).captureToImage().toPixelMap()
        return bar[bar.width / 2, bar.height / 2]
    }

    @Test
    fun theArcIsTheFatShareOfTheWholeCircle() {
        assertEquals(98.64f, compositionSweep(FAT_PERCENT), TOLERANCE)
    }

    @Test
    fun aShareOutsideTheScaleIsClampedRatherThanWoundRoundTwice() {
        // A bad reading is a drawing problem, not an arithmetic one: 140% would
        // otherwise draw a full ring plus a second arc over its own start.
        assertEquals(0f, compositionSweep(-3.0), TOLERANCE)
        assertEquals(FULL_TURN, compositionSweep(140.0), TOLERANCE)
        // coerceIn hands NaN straight back: every comparison against it is false.
        assertEquals(0f, compositionSweep(Double.NaN), TOLERANCE)
    }

    @Test
    fun theRingCarriesTheWeightAndItsUnit() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceCompositionRing(weightKg = 84.3, fatPercent = FAT_PERCENT)
                }
            }

            onNodeWithText("84,3").assertExists()
            onNodeWithText("кг").assertExists()
        }

    @Test
    fun tappingTheThirdBarFillsThreeAndNotOnlyTheThird() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    var value by remember { mutableStateOf(1) }

                    CadenceStepDots(
                        value = value,
                        accent = CadenceStepAccent.ENERGY,
                        onChange = { value = it },
                    )
                }
            }

            onNodeWithTag(cadenceStepDotTag(1)).assertIsSelected()
            onNodeWithTag(cadenceStepDotTag(2)).assertIsNotSelected()

            onNodeWithTag(cadenceStepDotTag(3)).performClick()

            // Cumulative, not a single highlight: one, two and three are all filled.
            (1..3).forEach { onNodeWithTag(cadenceStepDotTag(it)).assertIsSelected() }
            (4..5).forEach { onNodeWithTag(cadenceStepDotTag(it)).assertIsNotSelected() }

            // A second tap somewhere other than the middle: three is the scale's own
            // fixed point, so a handler nailed to `onChange(3)` — or one reading the
            // scale backwards — answers correctly there and nowhere else.
            onNodeWithTag(cadenceStepDotTag(2)).performClick()

            (1..2).forEach { onNodeWithTag(cadenceStepDotTag(it)).assertIsSelected() }
            (3..5).forEach { onNodeWithTag(cadenceStepDotTag(it)).assertIsNotSelected() }
        }

    @Test
    fun theTwoAccentsAreNotTheSameColour() {
        val energy = stepAccentColor(CadenceStepAccent.ENERGY)
        val sleep = stepAccentColor(CadenceStepAccent.SLEEP)

        assertEquals(CadenceColors.forest700, energy)
        assertEquals(CadenceColors.sand700, sleep)
        assertNotEquals(energy, sleep)
    }

    @Test
    fun aLitBarIsPaintedTheAccentAndAnUnlitOneTheGround() {
        val palette = CadenceLightPalette

        // The paint, not only the semantics: drawn from a second branch of its own,
        // an inverted fill left every «selected» assertion green.
        assertEquals(CadenceColors.forest700, stepDotFill(2, value = 3, CadenceStepAccent.ENERGY, palette))
        assertEquals(palette.sunk, stepDotFill(4, value = 3, CadenceStepAccent.ENERGY, palette))
        assertEquals(CadenceColors.sand700, stepDotFill(1, value = 1, CadenceStepAccent.SLEEP, palette))
    }

    @Test
    fun theSleepRowIsPaintedItsOwnAccentAndNotTheEnergyOne() =
        runComposeUiTest {
            // The helper maps both accents; that the composable passes on the accent it
            // was given is a separate claim, and replacing the argument with a literal
            // ENERGY left every other test here green.
            var sunk = Color.Unspecified

            setContent {
                CadenceTheme {
                    sunk = Cadence.palette.sunk
                    CadenceStepDots(value = 1, accent = CadenceStepAccent.SLEEP, onChange = {})
                }
            }

            assertEquals(CadenceColors.sand700, centrePixelOf(cadenceStepDotTag(1)))
            // The unlit bar too: painted from the accent alone, with the predicate
            // dropped at the call site, every bar lights and the answer stops reading.
            assertEquals(sunk, centrePixelOf(cadenceStepDotTag(CADENCE_STEP_DOTS)))
        }

    @Test
    fun theScaleIsFiveBarsLong() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceStepDots(value = 0, accent = CadenceStepAccent.ENERGY, onChange = {})
                }
            }

            // Counted, not sampled: a loop opening at zero draws a sixth bar that no
            // assertion about bars one to five would ever ask about.
            assertEquals(
                CADENCE_STEP_DOTS,
                onNodeWithTag(CADENCE_STEP_DOTS_ROW_TAG, useUnmergedTree = true)
                    .fetchSemanticsNode()
                    .children
                    .size,
            )
        }

    @Test
    fun nothingIsFilledBeforeTheFirstAnswer() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceStepDots(value = 0, accent = CadenceStepAccent.SLEEP, onChange = {})
                }
            }

            // Zero is «not answered yet», and it has to look unlike «the worst there is».
            (1..5).forEach { onNodeWithTag(cadenceStepDotTag(it)).assertIsNotSelected() }
        }

    /**
     * Nothing above reaches the canvas: `compositionSweep` is pinned as arithmetic, and
     * arithmetic proves the number was computed, not that it was drawn. Three one-token
     * mutations survived every other test here — the sweep collapsing to `FULL_TURN`,
     * the sweep collapsing to `0f`, and the two colours swapped — and under the last of
     * them step-12's legend would name sand «% жира» while the ring painted lean in sand.
     *
     * The probes are the ring's own geometry, read from [RING_CENTRE] and
     * [RING_TRACK_RADIUS] rather than recomputed here. The arc opens at twelve o'clock,
     * so at 27.4% it sweeps 98.64° and covers three o'clock but not nine.
     */
    @Test
    fun theFatArcCoversItsOwnShareAndTheLeanRestIsWhatShowsBeneath() =
        runComposeUiTest {
            lateinit var density: Density

            setContent {
                CadenceTheme {
                    density = LocalDensity.current
                    CadenceCompositionRing(weightKg = 84.3, fatPercent = FAT_PERCENT)
                }
            }

            val pixels =
                onNodeWithTag(CADENCE_COMPOSITION_RING_TAG, useUnmergedTree = true)
                    .captureToImage()
                    .toPixelMap()

            val centre = with(density) { RING_CENTRE.roundToPx() }
            val radius = with(density) { RING_TRACK_RADIUS.roundToPx() }

            assertEquals(
                CadenceColors.sand500,
                pixels[centre + radius, centre],
                "three o'clock is inside the fat share and is not painted sand",
            )
            assertEquals(
                CadenceColors.forest700,
                pixels[centre - radius, centre],
                "nine o'clock is past the fat share and is not the lean the arc leaves showing",
            )
            // Six o'clock is 180° from the opening. Under a correct arc it is outside
            // the share; opened at three o'clock instead of twelve, it falls inside.
            assertEquals(
                CadenceColors.forest700,
                pixels[centre, centre + radius],
                "six o'clock is inside the fat share — the arc does not open at twelve",
            )
        }

    @Test
    fun aSmallerFatShareStopsBeforeThreeOClock() =
        runComposeUiTest {
            // The companion to the probe above, and the reason there are two fixtures:
            // a sweep hardcoded to any angle at all is sand at one share or forest at
            // the other, never both.
            lateinit var density: Density

            setContent {
                CadenceTheme {
                    density = LocalDensity.current
                    CadenceCompositionRing(weightKg = 84.3, fatPercent = FAT_PERCENT_BEFORE_THREE)
                }
            }

            val pixels =
                onNodeWithTag(CADENCE_COMPOSITION_RING_TAG, useUnmergedTree = true)
                    .captureToImage()
                    .toPixelMap()

            val centre = with(density) { RING_CENTRE.roundToPx() }
            val radius = with(density) { RING_TRACK_RADIUS.roundToPx() }

            assertEquals(
                CadenceColors.forest700,
                pixels[centre + radius, centre],
                "at 15% the arc has not reached three o'clock, yet three o'clock is sand",
            )
        }
}
