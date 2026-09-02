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
import app.cadence.shared.auth.SignIn
import kotlinx.coroutines.CompletableDeferred
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

private const val AN_ADDRESS = "patient@clinic.example"

private const val A_PASSWORD = "a-long-enough-password"

@OptIn(ExperimentalTestApi::class)
class SignInScreenTest {
    @Test
    fun theFormHandsOverWhatWasTyped() =
        runComposeUiTest {
            var given: Pair<String, String>? = null

            setContent {
                CadenceTheme {
                    SignInScreen(
                        onSignIn = { address, password -> given = address to password },
                    )
                }
            }

            onNodeWithContentDescription(SignInCopy.ADDRESS_FIELD).performTextInput(AN_ADDRESS)
            onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            onNodeWithText(SignInCopy.ENTER).performClick()

            assertEquals(AN_ADDRESS to A_PASSWORD, given)
        }

    // Both halves: the sentence, and the form still standing under it with what was typed. A
    // refusal that clears the screen makes a mistyped password look like a broken app.
    @Test
    fun aRefusalIsExplainedWithoutLosingTheForm() =
        runComposeUiTest {
            setContent { CadenceTheme { SignInScreen(onSignIn = { _, _ -> }, problem = SignIn.Refused) } }

            onNodeWithText(SignInCopy.REFUSED).assertIsDisplayed()
            onNodeWithText(SignInCopy.REFUSED_HINT).assertIsDisplayed()
            onNodeWithContentDescription(SignInCopy.ADDRESS_FIELD).assertIsDisplayed()
        }

    // The other direction of the same mistake as on the acceptance screen: «check your address and
    // password» over a server that is down sends a patient to change a password that was right.
    @Test
    fun anUnreachableServerIsNotToldAsARefusal() =
        runComposeUiTest {
            setContent { CadenceTheme { SignInScreen(onSignIn = { _, _ -> }, problem = SignIn.Unreachable) } }

            onNodeWithText(SignInCopy.OFFLINE).assertIsDisplayed()
            assertTrue(
                onAllNodesWithText(SignInCopy.REFUSED).fetchSemanticsNodes().isEmpty(),
                "a server that could not be reached was told as a refused sign-in",
            )
        }

    @Test
    fun theButtonIsOffWhileAnAskIsInFlight() =
        runComposeUiTest {
            setContent { CadenceTheme { SignInScreen(onSignIn = { _, _ -> }, busy = true) } }

            onNodeWithContentDescription(SignInCopy.ADDRESS_FIELD).performTextInput(AN_ADDRESS)
            onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)

            onNodeWithText(SignInCopy.ENTER).assertIsNotEnabled()
        }

    // The driver's own guard, asked directly rather than through the screen: on screen the
    // disabled button holds it, so removing the guard leaves every test above green while a
    // caller that is not this screen spends the rate limit on the same credentials twice.
    @Test
    fun theDriverRefusesASecondAskWhileTheFirstIsInFlight() =
        runComposeUiTest {
            val answer = CompletableDeferred<SignIn>()
            var asked = 0
            var prompt: SignInPrompt? = null

            setContent {
                prompt =
                    rememberSignIn { _, _ ->
                        asked += 1
                        answer.await()
                    }
            }

            waitForIdle()
            requireNotNull(prompt).onSignIn(AN_ADDRESS, A_PASSWORD)
            waitForIdle()
            requireNotNull(prompt).onSignIn(AN_ADDRESS, A_PASSWORD)
            waitForIdle()

            assertEquals(1, asked, "the driver asked twice while the first ask was in flight")

            answer.complete(SignIn.Accepted)
            waitForIdle()
        }

    // An accepted sign-in leaves no refusal behind it: the session carries the patient inside, and
    // a sentence left under the form would be the last thing they saw of it.
    @Test
    fun anAcceptedSignInLeavesNoRefusalBehind() =
        runComposeUiTest {
            var answer: SignIn = SignIn.Refused
            var prompt: SignInPrompt? = null

            setContent { prompt = rememberSignIn { _, _ -> answer } }

            waitForIdle()
            requireNotNull(prompt).onSignIn(AN_ADDRESS, A_PASSWORD)
            waitForIdle()
            assertEquals(SignIn.Refused, requireNotNull(prompt).problem)

            answer = SignIn.Accepted
            requireNotNull(prompt).onSignIn(AN_ADDRESS, A_PASSWORD)
            waitForIdle()

            assertNull(requireNotNull(prompt).problem, "a refusal outlived the sign-in that succeeded")
        }

    // An empty form is not a sign-in attempt: the server would refuse it, and the refusal reads to
    // a patient as «your password is wrong» when they have not typed one.
    @Test
    fun anEmptyFormIsNotOfferedAsAnAttempt() =
        runComposeUiTest {
            setContent { CadenceTheme { SignInScreen(onSignIn = { _, _ -> }) } }

            onNodeWithText(SignInCopy.ENTER).assertIsNotEnabled()
        }
}
