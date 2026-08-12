package app.cadence.design

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals

@OptIn(ExperimentalTestApi::class)
class CadenceRingsTest {
    @Test
    fun theTwoRingsMeasureTwoDifferentThings() {
        // The fixture is deliberately lopsided: 900 of 1800 kcal is a half,
        // 35 of 140 g of protein is a quarter. On a fixture where both are the
        // same, «both rings read calories» passes ([[a-passing-assert-may-pass-for-another-reason]]).
        assertEquals(0.5f, ringSweep(value = 900.0, goal = 1800.0))
        assertEquals(0.25f, ringSweep(value = 35.0, goal = 140.0))
    }

    @Test
    fun aRingIsClampedAndSurvivesAZeroGoal() {
        assertEquals(1f, ringSweep(value = 2400.0, goal = 1800.0))
        assertEquals(0f, ringSweep(value = 900.0, goal = 0.0))
    }

    @Test
    fun theCentreReadsTheCalorieRingAndNotTheProteinOne() =
        runComposeUiTest {
            // With this lopsided fixture, wiring the centre text to the protein
            // ring instead of the calorie one reads «25» rather than «50» —
            // the mutation this test exists to fail against. `%` is a unit,
            // asserted as its own node — same split as
            // aStepperWithNoUnitDrawsOnlyTheNumber in CadenceStepperTest.
            setContent {
                CadenceTheme {
                    CadenceRings(kcal = 900.0, kcalGoal = 1800.0, proteinG = 35.0, proteinGoal = 140.0)
                }
            }

            onNodeWithText("50").assertExists()
            onNodeWithText("%").assertExists()
        }

    @Test
    fun theRingsNameBothOfTheirOwnFractions() =
        runComposeUiTest {
            // The centre text names only the calorie ring, and a `Canvas`
            // reads nothing back — the protein ring's own sweep is otherwise
            // unreachable by any test. Mutating the protein ring to read
            // kcal/kcalGoal instead of proteinG/proteinGoal (both rings
            // drawing the same metric) leaves this description's protein
            // clause at «50%» instead of «25%», which is the mutation this
            // test exists to fail against.
            setContent {
                CadenceTheme {
                    CadenceRings(kcal = 900.0, kcalGoal = 1800.0, proteinG = 35.0, proteinGoal = 140.0)
                }
            }

            val described =
                onNodeWithTag(CADENCE_RINGS_TAG, useUnmergedTree = true)
                    .fetchSemanticsNode()
                    .config
                    .getOrNull(SemanticsProperties.ContentDescription)
                    ?.firstOrNull()

            assertEquals("Калории 900 из 1800 ккал · 50% · Белок 35 из 140 г · 25%", described)
        }

    @Test
    fun theProteinRingSweepsOnlyItsOwnFractionOfTheCircle() =
        runComposeUiTest {
            // Nothing on the canvas itself is asserted anywhere in this file
            // — contentDescription (theRingsNameBothOfTheirOwnFractions)
            // proves the numbers were computed, not that they were drawn —
            // so `sweepFraction = proteinSweep` collapsing to a hardcoded
            // `1f` (the inner ring always drawing a full circle) survived.
            // With this fixture's proteinSweep = 25%, the inner ring's arc
            // (START_ANGLE_DEGREES = -90, i.e. 12 o'clock) only ever reaches
            // as far as 3 o'clock — 9 o'clock, on the opposite side of the
            // same radius, stays the track's own resting colour under
            // correct code and turns forest700 under the mutation. The probe
            // point is derived from CadenceRings.kt's own geometry: centre
            // at RING_SIZE/2 = 60dp; innerRadius = outerRadius - (outerStroke
            // + gap) = (120-10)/2 - (10+4) = 41dp; 9 o'clock sits
            // 60 - 41 = 19dp left of centre, at the same y as centre.
            var sunkColor = Color.Unspecified
            lateinit var density: Density
            setContent {
                CadenceTheme {
                    density = LocalDensity.current
                    sunkColor = Cadence.palette.sunk
                    CadenceRings(kcal = 900.0, kcalGoal = 1800.0, proteinG = 35.0, proteinGoal = 140.0)
                }
            }

            val rings = onNodeWithTag(CADENCE_RINGS_TAG, useUnmergedTree = true).captureToImage()
            val pixels = rings.toPixelMap()
            val nineOClockX = with(density) { RINGS_INNER_NINE_OCLOCK_X.roundToPx() }
            val centerY = with(density) { RINGS_CENTER.roundToPx() }

            assertEquals(
                sunkColor,
                pixels[nineOClockX, centerY],
                "9 o'clock on the inner ring is not the track colour — the protein ring swept past its own fraction",
            )
        }
}

/**
 * [CadenceRings]'s own centre, `RING_SIZE / 2` — 120dp / 2. Reused for both axes of the
 * 9-o'clock probe: it is the arc's centre x and the y of every point level with it.
 */
private val RINGS_CENTER = 60.dp

/**
 * 9 o'clock on [CadenceRings]'s inner ring: `RING_SIZE/2 - innerRadius`, where
 * `innerRadius = outerRadius - (OUTER_STROKE + RING_GAP)` and
 * `outerRadius = (RING_SIZE - OUTER_STROKE) / 2` — (120-10)/2 - (10+4) = 41, so
 * 60 - 41 = 19. Opposite the inner ring's own sweep, which never passes 3 o'clock at
 * this file's fixtures, so this point stays the track colour under correct code.
 */
private val RINGS_INNER_NINE_OCLOCK_X = 19.dp
