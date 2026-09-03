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
import app.cadence.shared.auth.Recovery
import app.cadence.shared.auth.SessionState
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private const val AN_ADDRESS = "patient@clinic.example"

@OptIn(ExperimentalTestApi::class)
class RecoveryScreenTest {
    @Test
    fun theFormHandsOverTheAddress() =
        runComposeUiTest {
            var asked: String? = null

            setContent { CadenceTheme { RecoveryScreen(onRecover = { asked = it }, onBack = {}) } }

            onNodeWithContentDescription(RecoveryCopy.ADDRESS_FIELD).performTextInput(AN_ADDRESS)
            onNodeWithText(RecoveryCopy.SEND).performClick()

            assertEquals(AN_ADDRESS, asked)
        }

    @Test
    fun anEmptyAddressIsNotAnAttempt() =
        runComposeUiTest {
            setContent { CadenceTheme { RecoveryScreen(onRecover = {}, onBack = {}) } }

            onNodeWithText(RecoveryCopy.SEND).assertIsNotEnabled()
        }

    // The sentence promises a letter conditionally, and the form is gone: told «sent» over a form
    // still standing, a patient types the address again and spends the gap.
    @Test
    fun aSentLetterIsSaidWithoutConfirmingTheAddress() =
        runComposeUiTest {
            setContent { CadenceTheme { RecoveryScreen(onRecover = {}, onBack = {}, outcome = Recovery.Sent) } }

            onNodeWithText(RecoveryCopy.SENT).assertIsDisplayed()
            assertTrue(
                onAllNodesWithText(RecoveryCopy.SEND).fetchSemanticsNodes().isEmpty(),
                "the form stayed open under a letter that is already on its way",
            )
        }

    // Two refusals that ask for opposite things, and the expensive mistake is telling the second as
    // the first: «check your connection» over a gap makes a patient retry and push it further out.
    @Test
    fun theGapAndAnUnreachableServerAreSaidApart() =
        runComposeUiTest {
            setContent { CadenceTheme { RecoveryScreen(onRecover = {}, onBack = {}, outcome = Recovery.TooSoon) } }

            onNodeWithText(RecoveryCopy.TOO_SOON).assertIsDisplayed()
            assertTrue(
                onAllNodesWithText(RecoveryCopy.OFFLINE).fetchSemanticsNodes().isEmpty(),
                "the per-address gap was told as a server that could not be reached",
            )
        }

    // The other direction: the letter never left, so the form has to stay under the sentence.
    @Test
    fun anUnreachableServerLeavesTheFormToTryAgain() =
        runComposeUiTest {
            setContent { CadenceTheme { RecoveryScreen(onRecover = {}, onBack = {}, outcome = Recovery.Unreachable) } }

            onNodeWithText(RecoveryCopy.OFFLINE).assertIsDisplayed()
            onNodeWithText(RecoveryCopy.SEND).assertIsDisplayed()
        }

    @Test
    fun theWayBackIsOffered() =
        runComposeUiTest {
            var back = false

            setContent { CadenceTheme { RecoveryScreen(onRecover = {}, onBack = { back = true }) } }

            onNodeWithText(RecoveryCopy.BACK).performClick()

            assertTrue(back, "the recovery screen is a dead end")
        }

    // The way in, from the one screen a patient who cannot sign in is looking at. Without it the
    // recovery screen exists and nothing reaches it.
    @Test
    fun theSignInFormLeadsToRecovery() =
        runComposeUiTest {
            setContent { App(SessionState.SignedOut) }

            onNodeWithText(RecoveryCopy.FORGOT).performClick()
            waitForIdle()

            onNodeWithContentDescription(RecoveryCopy.ADDRESS_FIELD).assertIsDisplayed()
        }

    // And back, because a patient who tapped it by mistake is otherwise stuck on a form that asks
    // for the one thing they came here already knowing.
    @Test
    fun recoveryLeadsBackToTheSignInForm() =
        runComposeUiTest {
            setContent { App(SessionState.SignedOut) }

            onNodeWithText(RecoveryCopy.FORGOT).performClick()
            waitForIdle()
            onNodeWithText(RecoveryCopy.BACK).performClick()
            waitForIdle()

            onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD).assertIsDisplayed()
        }

    // The address reaches the client through the whole area, not only through the screen.
    @Test
    fun theAddressReachesTheClientThroughTheApp() =
        runComposeUiTest {
            var asked: String? = null

            setContent {
                App(
                    SessionState.SignedOut,
                    recovery =
                        rememberRecovery {
                            asked = it
                            Recovery.Sent
                        },
                )
            }

            onNodeWithText(RecoveryCopy.FORGOT).performClick()
            waitForIdle()
            onNodeWithContentDescription(RecoveryCopy.ADDRESS_FIELD).performTextInput(AN_ADDRESS)
            onNodeWithText(RecoveryCopy.SEND).performClick()
            waitForIdle()

            assertEquals(AN_ADDRESS, asked)
            onNodeWithText(RecoveryCopy.SENT).assertIsDisplayed()
        }
}
