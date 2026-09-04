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

// A second address, so the sign-in leg cannot pass on text the recovery screen left behind:
// performTextInput appends, and one address for both legs would read the same either way.
private const val THE_ADDRESS_TYPED_AFTERWARDS = "patient.again@clinic.example"

/**
 * The two longest sequences in the suite, and the only thing here that is new is the length: the
 * per-screen files already cross single boundaries — `RecoveryScreenTest` enters at the sign-in
 * form and walks to recovery and back, `SignOutTest` walks out of the signed-in area.
 *
 * Measured rather than assumed: dropping `recovering = false` from `App`'s recovery `onBack` fails
 * `recoveryLeadsBackToTheSignInForm` and `comingBackToTheFormOffersTheFieldAgain` as well as the
 * second journey below. This is the suite a ported screen extends with a leg.
 */
@OptIn(ExperimentalTestApi::class)
class JourneyTest {
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

    @Test
    fun aPatientAsksForRecoveryReturnsToTheFormAndSignsIn() =
        runComposeUiTest {
            var session by mutableStateOf<SessionState>(SessionState.SignedOut)
            var askedFor: String? = null
            var signedInWith: Pair<String, String>? = null

            setContent {
                CadenceRoot(
                    session = session,
                    links = emptyFlow(),
                    accept = { Acceptance.Unreachable },
                    choose = { PasswordSet.Done },
                    signIn = { address, password ->
                        signedInWith = address to password
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

            onNodeWithContentDescription(SignInCopy.ADDRESS_FIELD)
                .performTextInput(THE_ADDRESS_TYPED_AFTERWARDS)
            onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            onNodeWithText(SignInCopy.ENTER).performClick()
            waitForIdle()

            assertEquals(
                THE_ADDRESS_TYPED_AFTERWARDS to A_PASSWORD,
                signedInWith,
                "the form carried something other than what was typed into it",
            )
            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
        }
}
