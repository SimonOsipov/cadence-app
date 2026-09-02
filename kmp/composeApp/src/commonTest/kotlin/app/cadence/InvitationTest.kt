package app.cadence

import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.LocalSaveableStateRegistry
import androidx.compose.runtime.saveable.SaveableStateRegistry
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

    // Android recreates the activity for a font-scale or locale change, neither of which
    // `configChanges` names, and a patient can sit on the password form for minutes. Re-running
    // the exchange there answers otp_expired over the session that very link created — «your link
    // is used up» on top of a live session — so the answer is restored rather than asked again.
    @Test
    fun anAnsweredInvitationSurvivesTheScreenBeingRecreated() =
        runComposeUiTest {
            var spent = 0
            val recreation = Recreation()

            setContent {
                recreation.around {
                    App(
                        SessionState.SignedIn,
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
            }
            waitForIdle()
            onNodeWithText(AcceptanceCopy.CHOOSE_PASSWORD).assertIsDisplayed()

            recreation.happen { waitForIdle() }

            assertEquals(1, spent, "the link was spent again when the screen was recreated")
            onNodeWithText(AcceptanceCopy.CHOOSE_PASSWORD).assertIsDisplayed()
        }

    // The other half, and the one that outlives the invitation: recreated after the password is
    // set, the form must not come back — a patient already inside the app would be asked to
    // choose a password for an invitation that is over.
    @Test
    fun aFinishedInvitationDoesNotComeBackWhenTheScreenIsRecreated() =
        runComposeUiTest {
            val recreation = Recreation()

            setContent {
                recreation.around {
                    App(
                        SessionState.SignedIn,
                        rememberInvitation(
                            token = TOKEN,
                            accept = { Acceptance.Accepted },
                            choose = { PasswordSet.Done },
                        ),
                    )
                }
            }

            onNodeWithContentDescription(AcceptanceCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            onNodeWithText(AcceptanceCopy.ENTER).performClick()
            waitForIdle()

            recreation.happen { waitForIdle() }

            assertTrue(
                onAllNodesWithText(AcceptanceCopy.CHOOSE_PASSWORD).fetchSemanticsNodes().isEmpty(),
                "the invitation came back after the password was already set",
            )
        }

    // The refusal's reason travels with it. Restored without the code, a spent link comes back as
    // the sentence that names nothing — the one refusal that promises a patient nothing to do,
    // over a link whose reason is known and actionable.
    @Test
    fun aRefusalKeepsItsReasonWhenTheScreenIsRecreated() =
        runComposeUiTest {
            val recreation = Recreation()

            setContent {
                recreation.around {
                    App(
                        SessionState.SignedOut,
                        rememberInvitation(
                            token = TOKEN,
                            accept = { Acceptance.Refused(AuthErrorCode.OtpExpired) },
                            choose = { PasswordSet.Done },
                        ),
                    )
                }
            }
            waitForIdle()
            onNodeWithText(AcceptanceCopy.SPENT).assertIsDisplayed()

            recreation.happen { waitForIdle() }

            onNodeWithText(AcceptanceCopy.SPENT).assertIsDisplayed()
        }

    // The third answer, and the one nothing else pins: a server that could not be reached is not
    // asked again by a recreation. Asking again is the patient's own tap, which is why the screen
    // offers one here and nowhere else.
    @Test
    fun anUnreachableServerIsNotAskedAgainByARecreation() =
        runComposeUiTest {
            var asked = 0
            val recreation = Recreation()

            setContent {
                recreation.around {
                    App(
                        SessionState.SignedOut,
                        rememberInvitation(
                            token = TOKEN,
                            accept = {
                                asked += 1
                                Acceptance.Unreachable
                            },
                            choose = { PasswordSet.Done },
                        ),
                    )
                }
            }
            waitForIdle()
            onNodeWithText(AcceptanceCopy.OFFLINE).assertIsDisplayed()

            recreation.happen { waitForIdle() }

            assertEquals(1, asked, "a recreation asked the server again on its own")
            onNodeWithText(AcceptanceCopy.OFFLINE).assertIsDisplayed()
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

    // The driver's own guard, asked directly rather than through the screen — measured, removing
    // it leaves every test green because the button is disabled while busy, so the screen was
    // holding this on its own and the guard beneath it was covered rather than measured.
    //
    // Both are kept: the disabled button is what a patient sees, and the guard is what answers a
    // caller that is not this screen.
    @Test
    fun theDriverItselfRefusesASecondAskWhileTheFirstIsInFlight() =
        runComposeUiTest {
            val answer = CompletableDeferred<PasswordSet>()
            var asked = 0
            var invitation: Invitation? = null

            setContent {
                invitation =
                    rememberInvitation(
                        token = TOKEN,
                        accept = { Acceptance.Accepted },
                        choose = {
                            asked += 1
                            answer.await()
                        },
                    )
            }

            waitForIdle()
            requireNotNull(invitation).onPasswordChosen(A_PASSWORD)
            waitForIdle()
            requireNotNull(invitation).onPasswordChosen(A_PASSWORD)
            waitForIdle()

            assertEquals(1, asked, "the driver asked twice while the first ask was in flight")

            answer.complete(PasswordSet.Done)
            waitForIdle()
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

/**
 * What the platform does to a screen it recreates: everything saveable written out, the
 * composition thrown away, and a fresh one handed the saved state back.
 *
 * Driven through the registry rather than `StateRestorationTester`, which answers a
 * `NotImplementedError` outside Android — measured on Compose 1.11.1, the version this module
 * builds against, and composeApp's tests run on the iOS target only. The subtree is disposed and
 * composed again in its own place rather than under a second `setContent`, because
 * `rememberSaveable` keys its state by position: two roots do not agree on one, and the state
 * would come back missing for a reason that is the harness's rather than the code's.
 */
private class Recreation {
    private var registry by mutableStateOf(SaveableStateRegistry(null) { true })
    private var alive by mutableStateOf(true)

    @Composable
    fun around(content: @Composable () -> Unit) =
        CompositionLocalProvider(LocalSaveableStateRegistry provides registry) {
            if (alive) content()
        }

    fun happen(settle: () -> Unit) {
        val kept = registry.performSave()

        alive = false
        settle()

        registry = SaveableStateRegistry(kept) { true }
        alive = true
        settle()
    }
}
