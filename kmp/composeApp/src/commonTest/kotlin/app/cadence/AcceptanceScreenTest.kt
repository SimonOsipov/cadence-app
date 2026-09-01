package app.cadence

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CadenceTheme
import app.cadence.shared.auth.Acceptance
import io.github.jan.supabase.auth.exception.AuthErrorCode
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

@OptIn(ExperimentalTestApi::class)
class AcceptanceScreenTest {
    // Each refusal gets its own sentence, and the pairs are what makes that measurable: asserting
    // only that something is shown passes on three identical screens, which is the state this
    // exists to prevent — «already used» to a banned patient sends them for an invitation that
    // will refuse the same way.
    @Test
    fun eachRefusalSaysItsOwnThing() =
        runComposeUiTest {
            val sentences =
                listOf(
                    Acceptance.Refused(AuthErrorCode.OtpExpired) to AcceptanceCopy.SPENT,
                    Acceptance.Refused(AuthErrorCode.UserBanned) to AcceptanceCopy.BANNED,
                    Acceptance.Refused(null) to AcceptanceCopy.UNNAMED,
                )

            for ((outcome, said) in sentences) {
                setContent {
                    CadenceTheme { AcceptanceScreen(outcome, onPasswordChosen = {}, onRetry = {}) }
                }

                onNodeWithText(said).assertIsDisplayed()

                for ((_, other) in sentences) {
                    if (other == said) continue

                    assertTrue(
                        onAllNodesWithText(other).fetchSemanticsNodes().isEmpty(),
                        "«$said» and «$other» were shown together, so the two are one screen",
                    )
                }
            }
        }

    // Offered a retry, and only here: a refusal is not something to ask again for, and a button
    // saying so would walk a banned patient into the rate limit.
    @Test
    fun onlyAnUnreachableServerIsOfferedAnotherTry() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    AcceptanceScreen(Acceptance.Unreachable, onPasswordChosen = {}, onRetry = {})
                }
            }

            onNodeWithText(AcceptanceCopy.RETRY).assertIsDisplayed()
        }

    @Test
    fun aRefusalIsNotOfferedAnotherTry() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    AcceptanceScreen(
                        Acceptance.Refused(AuthErrorCode.OtpExpired),
                        onPasswordChosen = {},
                        onRetry = {},
                    )
                }
            }

            assertTrue(
                onAllNodesWithText(AcceptanceCopy.RETRY).fetchSemanticsNodes().isEmpty(),
                "a spent link offered a retry, which asks the patient to walk into a rate limit",
            )
        }

    // Mandatory, and drawn as such rather than accepted and refused later: without a password a
    // lost session means waiting for email, which is what the spec was rewritten to close.
    @Test
    fun anEmptyPasswordCannotBeSubmitted() =
        runComposeUiTest {
            var chosen: String? = null

            setContent {
                CadenceTheme {
                    AcceptanceScreen(
                        Acceptance.Accepted,
                        onPasswordChosen = { chosen = it },
                        onRetry = {},
                    )
                }
            }

            onNodeWithText(AcceptanceCopy.ENTER).assertIsNotEnabled()
            onNodeWithText(AcceptanceCopy.ENTER).performClick()

            assertNull(chosen, "acceptance completed without a password")
        }

    @Test
    fun aPasswordTypedIsThePasswordHandedOver() =
        runComposeUiTest {
            var chosen: String? = null

            setContent {
                CadenceTheme {
                    AcceptanceScreen(
                        Acceptance.Accepted,
                        onPasswordChosen = { chosen = it },
                        onRetry = {},
                    )
                }
            }

            onNodeWithContentDescription(AcceptanceCopy.PASSWORD_FIELD)
                .performTextInput("a-chosen-password")
            onNodeWithText(AcceptanceCopy.ENTER).performClick()

            assertEquals("a-chosen-password", chosen)
        }
}
