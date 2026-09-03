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
import kotlinx.coroutines.CompletableDeferred
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

    // The gap has no sentence of its own, and that is the point: it is enforced against a row only
    // a real patient has, so «письмо уже отправляли» would be a sentence only a real address could
    // provoke. It arrives here as Sent, and the hint has to be true of that ask too — hence the
    // minute and the spam folder.
    @Test
    fun theSentHintCoversTheAskThatWasTooSoon() =
        runComposeUiTest {
            setContent { CadenceTheme { RecoveryScreen(onRecover = {}, onBack = {}, outcome = Recovery.Sent) } }

            onNodeWithText(RecoveryCopy.SENT_HINT).assertIsDisplayed()
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

    // The driver's own guard, asked directly: on screen the disabled button holds it, so removing
    // the guard leaves every test above green while a second tap spends the per-address gap — and
    // the patient is then told to wait a minute for a letter their own second tap delayed.
    @Test
    fun theDriverRefusesASecondAskWhileTheFirstIsInFlight() =
        runComposeUiTest {
            val answer = CompletableDeferred<Recovery>()
            var asked = 0
            var prompt: RecoveryPrompt? = null

            setContent {
                prompt =
                    rememberRecovery {
                        asked += 1
                        answer.await()
                    }
            }

            waitForIdle()
            requireNotNull(prompt).onRecover(AN_ADDRESS)
            waitForIdle()
            requireNotNull(prompt).onRecover(AN_ADDRESS)
            waitForIdle()

            assertEquals(1, asked, "the driver asked twice while the first ask was in flight")

            answer.complete(Recovery.Sent)
            waitForIdle()
        }

    // The one case a patient must retype is the one the sentence hides: a mistyped address is
    // answered «sent» by design, so leaving and coming back has to offer the field again.
    @Test
    fun comingBackToTheFormOffersTheFieldAgain() =
        runComposeUiTest {
            setContent { App(SessionState.SignedOut, recovery = rememberRecovery { Recovery.Sent }) }

            onNodeWithText(RecoveryCopy.FORGOT).performClick()
            waitForIdle()
            onNodeWithContentDescription(RecoveryCopy.ADDRESS_FIELD).performTextInput(AN_ADDRESS)
            onNodeWithText(RecoveryCopy.SEND).performClick()
            waitForIdle()
            onNodeWithText(RecoveryCopy.SENT).assertIsDisplayed()

            onNodeWithText(RecoveryCopy.BACK).performClick()
            waitForIdle()
            onNodeWithText(RecoveryCopy.FORGOT).performClick()
            waitForIdle()

            onNodeWithContentDescription(RecoveryCopy.ADDRESS_FIELD).assertIsDisplayed()
        }

    // A keyboard's trailing space is a 422, and every refusal here is answered «sent» — so an
    // untrimmed address is a letter a patient is told was written and never was.
    @Test
    fun theAddressIsTrimmedOnItsWayOut() =
        runComposeUiTest {
            var asked: String? = null

            setContent { CadenceTheme { RecoveryScreen(onRecover = { asked = it }, onBack = {}) } }

            onNodeWithContentDescription(RecoveryCopy.ADDRESS_FIELD).performTextInput("  $AN_ADDRESS  ")
            onNodeWithText(RecoveryCopy.SEND).performClick()

            assertEquals(AN_ADDRESS, asked, "the address went out with what the keyboard added")
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
