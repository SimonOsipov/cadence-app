package app.cadence.screens.inventory

import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CadenceTheme
import app.cadence.shared.domain.VialDraft
import app.cadence.shared.mock.MockSeed
import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

private val TODAY = LocalDate(2026, 5, 31)

@OptIn(ExperimentalTestApi::class)
class AddVialScreenTest {
    private fun ComposeUiTest.field(name: String) = onNodeWithTag(addVialFieldTag(name), useUnmergedTree = true)

    private fun ComposeUiTest.save() = onNodeWithTag(ADD_VIAL_SAVE_TAG, useUnmergedTree = true)

    @Test
    fun saveIsDeadUntilTheThreeFactsAVialCannotExistWithoutAreGiven() =
        runComposeUiTest {
            setContent { CadenceTheme { AddVialScreen(compounds = MockSeed.compounds, today = TODAY) } }

            // By tag: a placeholder is not a handle — «12» matches the dose
            // count, the date and the lot, and the labels are set in small caps.
            save().assertIsNotEnabled()

            onNodeWithText("Семаглутид").performScrollTo().performClick()
            waitForIdle()
            save().assertIsNotEnabled()

            field("total").performTextReplacement("12")
            waitForIdle()
            save().assertIsNotEnabled()

            field("expires").performTextReplacement("2026-12-01")
            waitForIdle()

            save().assertIsEnabled()
        }

    @Test
    fun everythingElseFilledInIsStillNotAVialWithoutACompound() =
        runComposeUiTest {
            // The mock refuses this too, but its own null-check catches it — so
            // the rule in `canSave` was measured nowhere and the button would
            // have gone live on a draft the write then rejected.
            setContent { CadenceTheme { AddVialScreen(compounds = MockSeed.compounds, today = TODAY) } }

            field("total").performTextReplacement("12")
            field("expires").performTextReplacement("2026-12-01")
            waitForIdle()

            save().assertIsNotEnabled()

            onNodeWithText("Семаглутид").performScrollTo().performClick()
            waitForIdle()

            save().assertIsEnabled()
        }

    @Test
    fun savingHandsOverADraftAndNotARenderedString() =
        runComposeUiTest {
            var saved: VialDraft? = null
            setContent {
                CadenceTheme {
                    AddVialScreen(compounds = MockSeed.compounds, today = TODAY, onSave = { saved = it })
                }
            }

            onNodeWithText("Семаглутид").performScrollTo().performClick()
            waitForIdle()
            field("total").performTextReplacement("12")
            field("expires").performTextReplacement("2026-09-14")
            waitForIdle()
            save().performClick()

            assertEquals(MockSeed.semaglutide.id, saved?.compoundId)
            assertEquals(12, saved?.totalDoses)
            // A date, not «14 сен». The prototype's field holds the label.
            assertEquals(LocalDate(2026, 9, 14), saved?.expiresOn)
        }

    @Test
    fun anExpiryAlreadyPastIsRefusedWithAReason() =
        runComposeUiTest {
            var saved: VialDraft? = null
            setContent {
                CadenceTheme {
                    AddVialScreen(compounds = MockSeed.compounds, today = TODAY, onSave = { saved = it })
                }
            }

            onNodeWithText("Семаглутид").performScrollTo().performClick()
            waitForIdle()
            field("total").performTextReplacement("12")
            field("expires").performTextReplacement("2026-05-30")
            waitForIdle()

            onNodeWithText("Срок уже истёк").assertExists()
            save().assertIsNotEnabled()
            assertNull(saved)
        }

    @Test
    fun theFormAsksForNoDoseBecauseTheProtocolDecidesIt() =
        runComposeUiTest {
            // The prototype asks «Дозировка · 0,25 мг» and stores it on the
            // vial — the same number the phase already decides, which is a
            // derived value stored. What the glass carries is a concentration.
            setContent { CadenceTheme { AddVialScreen(compounds = MockSeed.compounds, today = TODAY) } }

            // Uppercased on screen: the field labels are eyebrows.
            onNodeWithText("Дозировка", ignoreCase = true).assertDoesNotExist()
            onNodeWithText("Концентрация", ignoreCase = true).assertExists()
        }
}
