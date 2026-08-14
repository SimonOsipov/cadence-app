package app.cadence.design

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.DoseUnit
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

@OptIn(ExperimentalTestApi::class)
class StepperAndSliderTest {
    @Test
    fun theStepperReportsTheDoseItLandsOnAndNotTheDelta() =
        runComposeUiTest {
            // The caller gets a `Dose`, not a nudge. Everything downstream of
            // this control stores `{value, unit}`, and a stepper that reported
            // «+1» would put the arithmetic back in the screen.
            var dose by mutableStateOf(Dose(0.25, DoseUnit.MG))

            setContent { CadenceTheme { CadenceDoseStepper(dose = dose, onChange = { dose = it }) } }

            onNodeWithContentDescription("Увеличить дозу").performClick()
            assertEquals(Dose(0.30, DoseUnit.MG), dose)

            onNodeWithContentDescription("Уменьшить дозу").performClick()
            assertEquals(Dose(0.25, DoseUnit.MG), dose)
        }

    @Test
    fun theStepInMicrogramsIsAWholeNumberAndInMilligramsIsAFraction() =
        runComposeUiTest {
            // The prototype's own two rates: `step={state.unit === 'мкг' ? 25 : 0.05}`.
            var dose by mutableStateOf(Dose(250.0, DoseUnit.MCG))

            setContent { CadenceTheme { CadenceDoseStepper(dose = dose, onChange = { dose = it }) } }

            onNodeWithContentDescription("Увеличить дозу").performClick()

            assertEquals(Dose(275.0, DoseUnit.MCG), dose)
        }

    @Test
    fun theStepperRoundsRatherThanTruncating() =
        runComposeUiTest {
            // `0.35 + 0.05` is 0.39999999999999997 in binary floating point.
            // Truncating to three places records 0,399 while `formatDose`
            // rounds it back to «0,4 мг» on screen — the number the patient
            // reads and the number written to `dose_events` stop being one.
            var dose by mutableStateOf(Dose(0.35, DoseUnit.MG))

            setContent { CadenceTheme { CadenceDoseStepper(dose = dose, onChange = { dose = it }) } }

            onNodeWithContentDescription("Увеличить дозу").performClick()
            waitForIdle()

            assertEquals(0.4, dose.value)

            onNodeWithContentDescription("Уменьшить дозу").performClick()
            waitForIdle()

            assertEquals(0.35, dose.value)
        }

    @Test
    fun aCallerCanOverrideTheStepWithoutTouchingTheDesignSystem() =
        runComposeUiTest {
            // The rate is a fact about the protocol, not about the control.
            var dose by mutableStateOf(Dose(1.0, DoseUnit.MG))

            setContent {
                CadenceTheme { CadenceDoseStepper(dose = dose, onChange = { dose = it }, step = 0.5) }
            }

            onNodeWithContentDescription("Увеличить дозу").performClick()

            assertEquals(Dose(1.5, DoseUnit.MG), dose)
        }

    @Test
    fun theStepperNeverGoesBelowZero() =
        runComposeUiTest {
            var dose by mutableStateOf(Dose(0.05, DoseUnit.MG))

            setContent { CadenceTheme { CadenceDoseStepper(dose = dose, onChange = { dose = it }) } }

            repeat(3) {
                onNodeWithContentDescription("Уменьшить дозу").performClick()
                waitForIdle()
            }

            assertEquals(0.0, dose.value)
            assertEquals(DoseUnit.MG, dose.unit, "clamping lost the unit")
        }

    @Test
    fun theStepperRendersTheDoseThroughTheFormatterAndNotAsALiteral() =
        runComposeUiTest {
            setContent {
                CadenceTheme { CadenceDoseStepper(dose = Dose(0.25, DoseUnit.MG), onChange = { }) }
            }

            // «0,25», with the comma the RU formatter puts there, and the unit
            // beside it rather than inside the number.
            onNodeWithText("0,25").assertIsDisplayed()
            onNodeWithText("мг").assertIsDisplayed()
        }

    @Test
    fun theSyringeFillIsAFractionOfItsMaxAndIsClamped() {
        // A `Canvas` has no text, so the arithmetic is a function and the
        // drawing reads it. Clamped at both ends: the prototype clamps the top
        // and floors at 1, and a fill wider than the barrel draws outside it.
        assertEquals(0.25f, syringeFillFraction(units = 25f, max = 100f))
        assertEquals(1f, syringeFillFraction(units = 250f, max = 100f), "not clamped at the top")
        assertEquals(0f, syringeFillFraction(units = -4f, max = 100f), "not clamped at the bottom")
    }

    @Test
    fun aSyringeWithNoBarrelDrawsNothingRatherThanDividingByZero() {
        assertEquals(0f, syringeFillFraction(units = 25f, max = 0f))
    }

    @Test
    fun theSyringeDrawsAndCarriesItsFillForATestToSee() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceSyringeBar(units = 40f, max = 100f) } }

            val node = onNodeWithTag(CADENCE_SYRINGE_TAG).fetchSemanticsNode()
            val described = node.config.getOrNull(SemanticsProperties.ContentDescription)?.firstOrNull()

            // The percentage, not only the two inputs: a description built
            // from the arguments alone leaves the wire between
            // `syringeFillFraction` and the drawing untested, and a barrel
            // painted full regardless of the dose would pass.
            assertEquals("40 из 100 ед. · 40%", described)
        }

    @Test
    fun theSyringeFillIsAsWideAsItsFraction() =
        runComposeUiTest {
            // The wire between `syringeFillFraction` and what is drawn. Asserted
            // by measuring, because the description is computed independently:
            // a barrel painted full regardless of the dose satisfies every
            // assertion about its inputs.
            setContent { CadenceTheme { CadenceSyringeBar(units = 40f, max = 100f) } }

            val barrel = onNodeWithTag(CADENCE_SYRINGE_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            val fill = onNodeWithTag(CADENCE_SYRINGE_FILL_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot

            assertTrue(
                kotlin.math.abs(fill.width / barrel.width - 0.4f) < 0.01f,
                "the fill is ${fill.width / barrel.width} of the barrel, not 0.4",
            )
        }

    @Test
    fun theSyringeReportsTheFractionItDraws() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceSyringeBar(units = 250f, max = 100f) } }

            assertEquals(
                "250 из 100 ед. · 100%",
                onNodeWithTag(CADENCE_SYRINGE_TAG)
                    .fetchSemanticsNode()
                    .config
                    .getOrNull(SemanticsProperties.ContentDescription)
                    ?.firstOrNull(),
            )
        }

    @Test
    fun aMoodOutsideTheScaleShowsNoWordRatherThanCrashing() =
        runComposeUiTest {
            // The parameter admits any Int and the invariant 1..5 is not the
            // type's. A composition that threw would take the whole step down.
            setContent { CadenceTheme { CadenceMoodSlider(value = 7, onChange = { }) } }

            CADENCE_MOOD_LABELS.drop(1).dropLast(1).forEach {
                assertEquals(0, onAllNodesWithText(it).fetchSemanticsNodes().size)
            }
        }

    @Test
    fun theMoodSliderReportsTheValueItLandsOn() =
        runComposeUiTest {
            var mood by mutableStateOf<Int?>(null)

            setContent { CadenceTheme { CadenceMoodSlider(value = mood, onChange = { mood = it }) } }

            // Each of the five, separately: a row built in a loop that captured
            // the wrong index reports the last one for every tap.
            (1..CADENCE_MOOD_MAX).forEach { n ->
                mood = null
                onNodeWithContentDescription("Самочувствие $n из $CADENCE_MOOD_MAX").performClick()
                waitForIdle()

                assertEquals(n, mood, "tapping $n reported something else")
            }
        }

    @Test
    fun theMoodSliderNamesTheChosenPointAndMarksItSelected() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceMoodSlider(value = 4, onChange = { }) } }

            // «Хорошо» — `labels[value - 1]` in the prototype, and the reason
            // the off-by-one is worth a test.
            onNodeWithText("Хорошо").assertIsDisplayed()

            val node = onNodeWithContentDescription("Самочувствие 4 из $CADENCE_MOOD_MAX").fetchSemanticsNode()
            assertEquals(true, node.config.getOrNull(SemanticsProperties.Selected))
        }

    @Test
    fun theMoodSliderShowsNoWordUntilOneIsChosen() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceMoodSlider(value = null, onChange = { }) } }

            // Step 4 is «всё по желанию». The prototype seeds `mood: 3` and so
            // always shows «Ровно» — a word the patient did not say.
            //
            // The two ends are captions and stay: the scale is labelled «Никак»
            // … «Светло» whether or not anything is chosen. What must be absent
            // is the word for a chosen point.
            CADENCE_MOOD_LABELS.drop(1).dropLast(1).forEach {
                assertEquals(
                    0,
                    onAllNodesWithText(it).fetchSemanticsNodes().size,
                    "«$it» is claimed before anything was chosen",
                )
            }
            listOf(CADENCE_MOOD_LABELS.first(), CADENCE_MOOD_LABELS.last()).forEach {
                assertEquals(1, onAllNodesWithText(it).fetchSemanticsNodes().size, "«$it» is not the end of the scale")
            }
            assertTrue(CADENCE_MOOD_LABELS.size == CADENCE_MOOD_MAX)
        }

    /**
     * The scale is the product's, not this file's. Every assertion above compares what was
     * drawn against `CADENCE_MOOD_LABELS` — the same list that produced it — so renaming a
     * level passes all of them; `:247` only counted. Literals here, and the set rather than
     * its size ([[assert-the-set-not-the-size]] in the register of hard-won lessons).
     */
    @Test
    fun theSliderNamesTheFiveLevelsTheJournalNames() =
        runComposeUiTest {
            // Level 2, not 1 or 5: the two ends are also drawn as captions, so a chosen end
            // renders its word twice and «displayed» stops being a single node.
            setContent { CadenceTheme { CadenceMoodSlider(value = 2, onChange = { }) } }

            // The journal's wording (`journal/data.ts:46-53`), not the dose wizard's own
            // «Никак / Слабо» — one number cannot have two words.
            onNodeWithText("Так себе").assertIsDisplayed()
            assertEquals(1, onAllNodesWithText("Тяжело").fetchSemanticsNodes().size, "the scale starts at «Тяжело»")
            assertEquals(1, onAllNodesWithText("Светло").fetchSemanticsNodes().size, "the scale ends at «Светло»")
            assertEquals(0, onAllNodesWithText("Никак").fetchSemanticsNodes().size)
            assertEquals(0, onAllNodesWithText("Слабо").fetchSemanticsNodes().size)
        }
}
