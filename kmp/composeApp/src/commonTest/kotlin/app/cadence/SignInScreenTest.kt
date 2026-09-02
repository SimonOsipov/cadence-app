package app.cadence

import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextClearance
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CadenceTheme
import app.cadence.shared.auth.SignIn
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CompletableDeferred
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotEquals
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

    // One field filled is the sequence a patient actually produces — type the address, tap. Sent,
    // an empty password comes back as «проверьте почту и пароль» for a password never typed.
    @Test
    fun oneFieldFilledIsNotAnAttemptEither() =
        runComposeUiTest {
            setContent { CadenceTheme { SignInScreen(onSignIn = { _, _ -> }) } }

            onNodeWithContentDescription(SignInCopy.ADDRESS_FIELD).performTextInput(AN_ADDRESS)
            onNodeWithText(SignInCopy.ENTER).assertIsNotEnabled()

            onNodeWithContentDescription(SignInCopy.ADDRESS_FIELD).performTextClearance()
            onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            onNodeWithText(SignInCopy.ENTER).assertIsNotEnabled()
        }

    // The two halves of the guard are deliberately different, and nothing else measures the
    // difference: an address of spaces is no address, and a password of spaces is a password —
    // ours to send and the server's to judge.
    @Test
    fun spacesAreAPasswordButNotAnAddress() =
        runComposeUiTest {
            setContent { CadenceTheme { SignInScreen(onSignIn = { _, _ -> }) } }

            onNodeWithContentDescription(SignInCopy.ADDRESS_FIELD).performTextInput("   ")
            onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            onNodeWithText(SignInCopy.ENTER).assertIsNotEnabled()

            onNodeWithContentDescription(SignInCopy.ADDRESS_FIELD).performTextClearance()
            onNodeWithContentDescription(SignInCopy.ADDRESS_FIELD).performTextInput(AN_ADDRESS)
            onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD).performTextClearance()
            onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD).performTextInput("   ")
            onNodeWithText(SignInCopy.ENTER).assertIsEnabled()
        }

    // Both fields are plain `remember`, and the password deliberately so: `rememberSaveable` on it
    // would write the patient's password in clear text into Android's saved-instance bundle. This
    // is the guard against somebody «fixing» the address by making the pair saveable.
    @Test
    fun aRecreationDoesNotBringThePasswordBack() =
        runComposeUiTest {
            val recreation = Recreation()

            setContent { recreation.around { CadenceTheme { SignInScreen(onSignIn = { _, _ -> }) } } }

            onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            waitForIdle()

            recreation.happen { waitForIdle() }

            assertNotEquals(
                A_PASSWORD,
                onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD)
                    .fetchSemanticsNode()
                    .config
                    .getOrNull(SemanticsProperties.InputText)
                    ?.text,
                "the password came back after a recreation, which puts it in the saved-instance bundle",
            )
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

    // The one escape the `finally` is for. A seam that gives up cancellation-shaped finishes this
    // coroutine without touching the composition, so without the finally the form stays busy for
    // ever: a dead button with nothing written under it. An ordinary throw is not this case — it
    // takes the whole tree with it, and there is no frame left to be dead in.
    @Test
    fun anAskThatGivesUpLeavesTheFormUsableAgain() =
        runComposeUiTest {
            var prompt: SignInPrompt? = null

            setContent { prompt = rememberSignIn { _, _ -> throw CancellationException("the seam gave up") } }

            waitForIdle()
            requireNotNull(prompt).onSignIn(AN_ADDRESS, A_PASSWORD)
            waitForIdle()

            assertFalse(requireNotNull(prompt).busy, "the form stayed busy after the ask was cancelled")
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

    // The refusal goes while the next ask is in flight. Left standing under a form the patient is
    // waiting on, it reads as the answer to the tap they have just made.
    @Test
    fun aRefusalIsGoneWhileTheNextAskIsInFlight() =
        runComposeUiTest {
            val answer = CompletableDeferred<SignIn>()
            var first = true
            var prompt: SignInPrompt? = null

            setContent {
                prompt =
                    rememberSignIn { _, _ ->
                        if (first) {
                            first = false
                            SignIn.Refused
                        } else {
                            answer.await()
                        }
                    }
            }

            waitForIdle()
            requireNotNull(prompt).onSignIn(AN_ADDRESS, A_PASSWORD)
            waitForIdle()
            assertEquals(SignIn.Refused, requireNotNull(prompt).problem)

            requireNotNull(prompt).onSignIn(AN_ADDRESS, A_PASSWORD)
            waitForIdle()

            assertNull(requireNotNull(prompt).problem, "the last refusal stood under a form mid-ask")

            answer.complete(SignIn.Accepted)
            waitForIdle()
        }

    // The screen's own use of it, because the component test measures the component: dropped here,
    // the password is drawn in clear text with every test in design/ still green.
    @Test
    fun thePasswordFieldIsMasked() =
        runComposeUiTest {
            setContent { CadenceTheme { SignInScreen(onSignIn = { _, _ -> }) } }

            onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            waitForIdle()

            val field = onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD).fetchSemanticsNode()

            assertNotEquals(
                A_PASSWORD,
                field.config.getOrNull(SemanticsProperties.EditableText)?.text,
                "the password was drawn in clear text",
            )
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
