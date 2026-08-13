package app.cadence.design

import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.onChildren
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import kotlin.test.Test
import kotlin.test.assertEquals

@OptIn(ExperimentalTestApi::class)
class CadenceStepperTest {
    @Test
    fun theStepperClampsAtBothEndsAndStandsStillOnTheBoundary() {
        // Against the mutation «bounded at one end only»: a floor-only stepper
        // passes every assertion about the floor.
        assertEquals(5.0, steppedValue(value = 5.0, delta = -10.0, min = 5.0, max = 600.0, decimals = 0))
        assertEquals(600.0, steppedValue(value = 600.0, delta = 10.0, min = 5.0, max = 600.0, decimals = 0))
        assertEquals(15.0, steppedValue(value = 5.0, delta = 10.0, min = 5.0, max = 600.0, decimals = 0))
    }

    @Test
    fun aStepperWithNoCeilingGrowsWithoutOne() {
        // The grams of a parsed meal item have a floor of 5 and no ceiling
        // (`LogMealScreen.tsx:880`); a shared ceiling would cap it silently.
        assertEquals(10_000.0, steppedValue(value = 9_990.0, delta = 10.0, min = 5.0, max = null, decimals = 0))
    }

    @Test
    fun aTenthOfAStepDoesNotAccumulateBinaryError() {
        // 0.1 + 0.2 is 0.30000000000000004; body fat steps by a tenth and is
        // read as a tenth. Scaled integer arithmetic, like `DOSE_SCALE`.
        var v = 20.0
        repeat(times = 30) { v = steppedValue(v, delta = 0.1, min = 20.0, max = 55.0, decimals = 1) }
        assertEquals(23.0, v)
    }

    @Test
    fun theStepperReportsTheSteppedValueAndNotTheDelta() =
        runComposeUiTest {
            var reported = 0.0
            setContent {
                CadenceTheme {
                    CadenceStepper(
                        value = 100.0,
                        onChange = { reported = it },
                        min = 5.0,
                        max = 600.0,
                        step = 10.0,
                        decimals = 0,
                        unit = "г",
                    )
                }
            }

            onNodeWithTag(CADENCE_STEPPER_PLUS_TAG).performClick()

            assertEquals(110.0, reported, "the stepper reported a delta rather than a value")
        }

    @Test
    fun theMinusButtonDecrementsRatherThanIncrements() =
        runComposeUiTest {
            // No test in this file ever clicks CADENCE_STEPPER_MINUS_TAG, so
            // `steppedValue(value, -step, ...)` at CadenceNumericStepper.kt:92
            // flipped to `steppedValue(value, step, ...)` — a minus button
            // that increments — shipped silently. Same fixture and pattern as
            // theStepperReportsTheSteppedValueAndNotTheDelta, minus button
            // instead of plus.
            var reported = 0.0
            setContent {
                CadenceTheme {
                    CadenceStepper(
                        value = 100.0,
                        onChange = { reported = it },
                        min = 5.0,
                        max = 600.0,
                        step = 10.0,
                        decimals = 0,
                        unit = "г",
                    )
                }
            }

            onNodeWithTag(CADENCE_STEPPER_MINUS_TAG).performClick()

            assertEquals(90.0, reported, "the minus button reported a value that went up, not down")
        }

    @Test
    fun aStepperWithNoUnitDrawsOnlyTheNumber() =
        runComposeUiTest {
            // Against the mutation `unit.orEmpty()`: converting a null unit
            // into "" makes `CadenceNumber`'s `if (unit != null)` true, so a
            // plain count (servings, steps, minutes) would draw a second,
            // empty text run plus its 4.dp `spacedBy` gap that a genuine null
            // skips entirely.
            setContent {
                CadenceTheme {
                    CadenceStepper(
                        value = 3.0,
                        onChange = {},
                        min = 0.0,
                        max = 10.0,
                        step = 1.0,
                        decimals = 0,
                    )
                }
            }

            val textRuns =
                onNodeWithTag(CADENCE_STEPPER_VALUE_TAG, useUnmergedTree = true)
                    .onChildren()
                    .fetchSemanticsNodes()
                    .count { it.config.getOrNull(SemanticsProperties.Text) != null }

            assertEquals(1, textRuns, "a null unit still drew a second text run")
        }
}
