package app.cadence.design

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.compose.ui.unit.Density
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals

private const val TOLERANCE = 1e-3f

/**
 * A fraction that matches nothing else the body screen draws — not the weight, not
 * the lean mass, not a round quarter of the ring. A sweep computed from the wrong
 * input would have to land on this by coincidence.
 */
private const val FAT_PERCENT = 27.4

@OptIn(ExperimentalTestApi::class)
class BodyAndFeelPrimitivesTest {
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
            setContent {
                CadenceTheme {
                    CadenceStepDots(value = CADENCE_STEP_DOTS, accent = CadenceStepAccent.SLEEP, onChange = {})
                }
            }

            val bar =
                onNodeWithTag(cadenceStepDotTag(1), useUnmergedTree = true)
                    .captureToImage()
                    .toPixelMap()

            assertEquals(CadenceColors.sand700, bar[bar.width / 2, bar.height / 2])
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
        }
}
