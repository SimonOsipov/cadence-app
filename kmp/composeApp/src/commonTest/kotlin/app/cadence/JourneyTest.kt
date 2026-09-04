package app.cadence

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onAllNodesWithContentDescription
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.shared.auth.Acceptance
import app.cadence.shared.auth.PasswordSet
import app.cadence.shared.auth.Recovery
import app.cadence.shared.auth.SessionState
import app.cadence.shared.auth.SignIn
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.emptyFlow
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private const val TOKEN = "e75b4d4f54a86c915c0afdfc5db3b5cb6eea78ba43c1ccf6bd24c5cb"

private const val TODAY_TAB = "Сегодня"

private const val PROFILE_TAB = "Профиль"

private const val A_PASSWORD = "a-long-enough-password"

private const val AN_ADDRESS = "patient@clinic.example"

/**
 * The block walked end to end in one composition: the per-screen files each enter at their own
 * screen, and a journey is the only shape in which one screen's exit is the next one's entry.
 *
 * Individual legs do have files of their own — measured, not assumed: dropping the recovery
 * screen's return fails two `RecoveryScreenTest` tests as well as the journey below. What is new
 * here is the sequence, and this is the suite a ported screen extends with a leg.
 */
@OptIn(ExperimentalTestApi::class)
class JourneyTest {
    // Invitation to inside to out and back in again — the order is the point: an acceptance
    // screen that releases the tree, and a form that still works once it has.
    @Test
    fun aPatientAcceptsAnInvitationSignsOutAndSignsBackIn() =
        runComposeUiTest {
            var session by mutableStateOf<SessionState>(SessionState.SignedIn)
            var signedInWith: Pair<String, String>? = null

            setContent {
                CadenceRoot(
                    session = session,
                    links = MutableStateFlow("cadence://accept?token_hash=$TOKEN"),
                    accept = { Acceptance.Accepted },
                    choose = { PasswordSet.Done },
                    signIn = { address, password ->
                        signedInWith = address to password
                        session = SessionState.SignedIn
                        SignIn.Accepted
                    },
                    signOut = { session = SessionState.SignedOut },
                )
            }
            waitForIdle()

            onNodeWithContentDescription(AcceptanceCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            onNodeWithText(AcceptanceCopy.ENTER).performClick()
            waitForIdle()
            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()

            onNodeWithContentDescription(PROFILE_TAB).performClick()
            waitForIdle()
            onNodeWithText(SignInCopy.SIGN_OUT).performClick()
            waitForIdle()

            assertTrue(
                onAllNodesWithContentDescription(TODAY_TAB).fetchSemanticsNodes().isEmpty(),
                "the area after sign-in survived signing out",
            )

            onNodeWithContentDescription(SignInCopy.ADDRESS_FIELD).performTextInput(AN_ADDRESS)
            onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            onNodeWithText(SignInCopy.ENTER).performClick()
            waitForIdle()

            assertEquals(AN_ADDRESS to A_PASSWORD, signedInWith, "the form's own values never reached the call")
            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
        }

    // The way back for a patient who has lost the password: out to the recovery screen, and back
    // to a form that still signs in. `RecoveryScreenTest` owns the return itself; what it enters
    // directly, this reaches from the form, and it carries on into a sign-in the other file ends
    // before.
    @Test
    fun aPatientAsksForRecoveryReturnsToTheFormAndSignsIn() =
        runComposeUiTest {
            var session by mutableStateOf<SessionState>(SessionState.SignedOut)
            var askedFor: String? = null

            setContent {
                CadenceRoot(
                    session = session,
                    links = emptyFlow(),
                    accept = { Acceptance.Unreachable },
                    choose = { PasswordSet.Done },
                    signIn = { _, _ ->
                        session = SessionState.SignedIn
                        SignIn.Accepted
                    },
                    recover = {
                        askedFor = it
                        Recovery.Sent
                    },
                )
            }
            waitForIdle()

            onNodeWithText(RecoveryCopy.FORGOT).performClick()
            waitForIdle()

            onNodeWithContentDescription(RecoveryCopy.ADDRESS_FIELD).performTextInput(AN_ADDRESS)
            onNodeWithText(RecoveryCopy.SEND).performClick()
            waitForIdle()

            assertEquals(AN_ADDRESS, askedFor, "the address never reached the recovery call")
            onNodeWithText(RecoveryCopy.SENT).assertIsDisplayed()

            onNodeWithText(RecoveryCopy.BACK).performClick()
            waitForIdle()

            onNodeWithContentDescription(SignInCopy.ADDRESS_FIELD).performTextInput(AN_ADDRESS)
            onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            onNodeWithText(SignInCopy.ENTER).performClick()
            waitForIdle()

            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
        }
}
