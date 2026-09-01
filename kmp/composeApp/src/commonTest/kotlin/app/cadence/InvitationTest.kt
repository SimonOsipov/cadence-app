package app.cadence

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.shared.auth.Acceptance
import app.cadence.shared.auth.PasswordSet
import app.cadence.shared.auth.SessionState
import io.github.jan.supabase.auth.exception.AuthErrorCode
import kotlinx.coroutines.CompletableDeferred
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private const val TOKEN = "e75b4d4f54a86c915c0afdfc5db3b5cb6eea78ba43c1ccf6bd24c5cb"

private const val A_PASSWORD = "a-long-enough-password"

@OptIn(ExperimentalTestApi::class)
class InvitationTest {
    // The whole path in one: link, exchange, password, inside. Each half alone leaves a patient
    // somewhere — an exchange with no password is an account nobody can sign into again, and a
    // password on a screen that never leaves is a form completed in front of a locked door.
    @Test
    fun anInvitationEndsInsideTheApp() =
        runComposeUiTest {
            var setWith: String? = null

            setContent {
                App(
                    session = SessionState.SignedIn,
                    invitation =
                        rememberInvitation(
                            token = TOKEN,
                            accept = { Acceptance.Accepted },
                            choose = {
                                setWith = it
                                PasswordSet.Done
                            },
                        ),
                )
            }

            onNodeWithContentDescription(AcceptanceCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            onNodeWithText(AcceptanceCopy.ENTER).performClick()
            waitForIdle()

            assertEquals(A_PASSWORD, setWith)
            assertTrue(
                onAllNodesWithText(AcceptanceCopy.CHOOSE_PASSWORD).fetchSemanticsNodes().isEmpty(),
                "the invitation stayed on screen after the password was set",
            )
        }

    // The token is spent once. Anything that recomposes the tree — a session arriving, a rotation
    // — would otherwise run the exchange again, and the second spend answers otp_expired: the
    // screen would tell a patient mid-acceptance that their link was used up.
    @Test
    fun theTokenIsSpentOncePerLink() =
        runComposeUiTest {
            var spent = 0
            var session by mutableStateOf<SessionState>(SessionState.Deciding)

            setContent {
                App(
                    session,
                    rememberInvitation(
                        token = TOKEN,
                        accept = {
                            spent += 1
                            Acceptance.Accepted
                        },
                        choose = { PasswordSet.Done },
                    ),
                )
            }

            session = SessionState.SignedOut
            waitForIdle()
            session = SessionState.SignedIn
            waitForIdle()

            assertEquals(1, spent, "the link was followed more than once")
        }

    // A refusal the patient can act on stays under the form with what they typed still there.
    @Test
    fun aWeakPasswordIsExplainedWithoutLosingTheForm() =
        runComposeUiTest {
            setContent {
                App(
                    SessionState.SignedOut,
                    rememberInvitation(
                        token = TOKEN,
                        accept = { Acceptance.Accepted },
                        choose = { PasswordSet.Refused(AuthErrorCode.WeakPassword) },
                    ),
                )
            }

            onNodeWithContentDescription(AcceptanceCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            onNodeWithText(AcceptanceCopy.ENTER).performClick()
            waitForIdle()

            onNodeWithText(AcceptanceCopy.TOO_WEAK).assertIsDisplayed()
            onNodeWithText(AcceptanceCopy.CHOOSE_PASSWORD).assertIsDisplayed()
        }

    // A second tap while the first is in flight sets the password twice and leaves the patient
    // watching a form that looks stuck.
    @Test
    fun aSecondTapWhileTheFirstIsInFlightIsIgnored() =
        runComposeUiTest {
            val answer = CompletableDeferred<PasswordSet>()
            var asked = 0

            setContent {
                App(
                    SessionState.SignedOut,
                    rememberInvitation(
                        token = TOKEN,
                        accept = { Acceptance.Accepted },
                        choose = {
                            asked += 1
                            answer.await()
                        },
                    ),
                )
            }

            onNodeWithContentDescription(AcceptanceCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            onNodeWithText(AcceptanceCopy.ENTER).performClick()
            waitForIdle()

            onNodeWithText(AcceptanceCopy.ENTER).assertIsNotEnabled()

            answer.complete(PasswordSet.Done)
            waitForIdle()

            assertEquals(1, asked, "the password was set more than once")
        }

    // «Try again» has to mean it: an unreachable server that answers on the second ask must move
    // the patient on, not redraw the same message.
    @Test
    fun retryingAsksAgain() =
        runComposeUiTest {
            var asked = 0

            setContent {
                App(
                    SessionState.SignedOut,
                    rememberInvitation(
                        token = TOKEN,
                        accept = { if (++asked == 1) Acceptance.Unreachable else Acceptance.Accepted },
                        choose = { PasswordSet.Done },
                    ),
                )
            }

            onNodeWithText(AcceptanceCopy.RETRY).performClick()
            waitForIdle()

            assertEquals(2, asked)
            onNodeWithText(AcceptanceCopy.CHOOSE_PASSWORD).assertIsDisplayed()
        }
}
