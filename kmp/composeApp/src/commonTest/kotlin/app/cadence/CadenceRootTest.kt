package app.cadence

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.shared.auth.Acceptance
import app.cadence.shared.auth.PasswordSet
import app.cadence.shared.auth.SessionState
import app.cadence.shared.auth.SignIn
import io.github.jan.supabase.auth.exception.AuthErrorCode
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.flow.MutableStateFlow
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

// Deliberately not token-shaped. Neither parser constrains the alphabet or the length, so a
// realistic hex fixture bought nothing and cost a secret-scanner incident on every PR.
private const val TOKEN = "a-token-a-test-made-up"

private const val ANOTHER_TOKEN = "another-token-a-test-made-up"

private const val TODAY_TAB = "Сегодня"

private const val A_PASSWORD = "a-long-enough-password"

private const val AN_ADDRESS = "patient@clinic.example"

private fun accept(token: String) = "cadence://accept?token_hash=$token"

@OptIn(ExperimentalTestApi::class)
class CadenceRootTest {
    // The cold start this block is for: what the platform hands over is a link, and until it is
    // read `invitationToken` has no caller in the app — InvitationTest hands over a bare token,
    // which is the one hand-over no patient makes. A link carried to the password is pinned here
    // and nowhere else.
    @Test
    fun aColdStartFromALinkOpensOnTheAcceptanceScreen() =
        runComposeUiTest {
            var handed: String? = null
            var setWith: String? = null

            setContent {
                CadenceRoot(
                    // What the exchange leaves behind, and the screen outranks it until the
                    // password is set.
                    session = SessionState.SignedIn,
                    links = MutableStateFlow(accept(TOKEN)),
                    accept = {
                        handed = it
                        Acceptance.Accepted
                    },
                    choose = {
                        setWith = it
                        PasswordSet.Done
                    },
                )
            }
            waitForIdle()

            assertEquals(TOKEN, handed, "the link's token never reached the exchange")
            onNodeWithText(AcceptanceCopy.CHOOSE_PASSWORD).assertIsDisplayed()

            onNodeWithContentDescription(AcceptanceCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            onNodeWithText(AcceptanceCopy.ENTER).performClick()
            waitForIdle()

            assertEquals(A_PASSWORD, setWith, "the password never reached the write behind the form")
            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
        }

    // The roots are handed whatever the system opens the app with. Answering an address that is
    // not an invitation would send a stranger's string to /verify.
    @Test
    fun aLinkThatIsNotAnInvitationIsNotAnswered() =
        runComposeUiTest {
            var asked = 0

            setContent {
                CadenceRoot(
                    session = SessionState.SignedOut,
                    links = MutableStateFlow("cadence://accept/../recover?token_hash=$TOKEN"),
                    accept = {
                        asked += 1
                        Acceptance.Accepted
                    },
                    choose = { PasswordSet.Done },
                    // Both exchanges, because this address is one both parsers refuse and only one
                    // of them was ever counted.
                    acceptRecovery = {
                        asked += 1
                        Acceptance.Accepted
                    },
                )
            }
            waitForIdle()

            assertEquals(0, asked, "a link that is not an invitation was sent to an exchange")
            onNodeWithContentDescription(SignInCopy.ADDRESS_FIELD).assertIsDisplayed()
        }

    // What `onNewIntent` on Android and `onOpenURL` on Apple exist for: the tree is composed once
    // and outlives every link, so one arriving into a live app reaches it through the value.
    @Test
    fun aLinkArrivingWhileTheAppIsAliveIsAnswered() =
        runComposeUiTest {
            val links = MutableStateFlow<String?>(null)

            setContent {
                CadenceRoot(
                    session = SessionState.SignedIn,
                    links = links,
                    accept = { Acceptance.Accepted },
                    choose = { PasswordSet.Done },
                )
            }

            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()

            links.value = accept(TOKEN)
            waitForIdle()

            onNodeWithText(AcceptanceCopy.CHOOSE_PASSWORD).assertIsDisplayed()
        }

    // Two links in one process, each with its own token. Keyed on anything coarser than the token
    // — a flag saying «a link arrived» — the second invitation is answered with the first token,
    // which is spent, and the patient is told their live link is used up.
    @Test
    fun aSecondLinkIsAnsweredWithItsOwnToken() =
        runComposeUiTest {
            val spent = mutableListOf<String>()
            val links = MutableStateFlow(accept(TOKEN))

            setContent {
                CadenceRoot(
                    session = SessionState.SignedOut,
                    links = links,
                    accept = {
                        spent += it
                        Acceptance.Accepted
                    },
                    choose = { PasswordSet.Done },
                )
            }
            waitForIdle()

            links.value = accept(ANOTHER_TOKEN)
            waitForIdle()

            assertEquals(listOf(TOKEN, ANOTHER_TOKEN), spent, "the second link was not answered as its own")
        }

    // The other direction of the same key, and the one that costs a patient their account: the
    // same token delivered twice — a link re-opened, or the same intent replayed — is one
    // invitation, and spending it again answers otp_expired over the session it just created.
    @Test
    fun theSameTokenArrivingTwiceIsOneInvitation() =
        runComposeUiTest {
            var spent = 0
            val links = MutableStateFlow(accept(TOKEN))

            setContent {
                CadenceRoot(
                    session = SessionState.SignedOut,
                    links = links,
                    accept = {
                        spent += 1
                        Acceptance.Accepted
                    },
                    choose = { PasswordSet.Done },
                )
            }
            waitForIdle()

            links.value = "${accept(TOKEN)}&from=recents"
            waitForIdle()

            assertEquals(1, spent, "one token was spent twice")
        }

    // The form's two fields reach the client as they were typed. Wired to one seam and not the
    // other — address and password swapped — every screen test on this page stays green.
    @Test
    fun theFormReachesTheClientWithWhatWasTyped() =
        runComposeUiTest {
            var given: Pair<String, String>? = null

            setContent {
                CadenceRoot(
                    session = SessionState.SignedOut,
                    links = MutableStateFlow(null),
                    accept = { Acceptance.Accepted },
                    choose = { PasswordSet.Done },
                    signIn = { address, password ->
                        given = address to password
                        SignIn.Accepted
                    },
                )
            }

            onNodeWithContentDescription(SignInCopy.ADDRESS_FIELD).performTextInput(AN_ADDRESS)
            onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            onNodeWithText(SignInCopy.ENTER).performClick()
            waitForIdle()

            assertEquals(AN_ADDRESS to A_PASSWORD, given, "the form's fields did not reach the client")
        }

    // Signing out is a suspend call behind a button, and the button is two composables away from
    // the client: the screen calls the shell's action, the shell calls what the root gave it.
    @Test
    fun signingOutReachesTheClient() =
        runComposeUiTest {
            var signedOut = false

            setContent {
                CadenceRoot(
                    session = SessionState.SignedIn,
                    links = MutableStateFlow(null),
                    accept = { Acceptance.Accepted },
                    choose = { PasswordSet.Done },
                    signOut = { signedOut = true },
                )
            }

            onNodeWithContentDescription("Профиль").performClick()
            waitForIdle()
            onNodeWithText(SignInCopy.SIGN_OUT).performClick()
            waitForIdle()

            assertTrue(signedOut, "the sign-out button never reached the client")
        }

    // The recreation InvitationTest cannot reach: it hands `rememberInvitation` a token from the
    // first frame, and the platforms never do — the link arrives through a flow, so frame one has
    // none.
    @Test
    fun anInvitationSurvivesARecreationOnThePathThePlatformsTake() =
        runComposeUiTest {
            var spent = 0
            val recreation = Recreation()

            setContent {
                recreation.around {
                    CadenceRoot(
                        session = SessionState.SignedIn,
                        links = MutableStateFlow(accept(TOKEN)),
                        accept = {
                            spent += 1
                            Acceptance.Accepted
                        },
                        choose = { PasswordSet.Done },
                    )
                }
            }
            waitForIdle()
            onNodeWithText(AcceptanceCopy.CHOOSE_PASSWORD).assertIsDisplayed()

            recreation.happen { waitForIdle() }

            assertEquals(1, spent, "the link was spent again when the screen was recreated")
            onNodeWithText(AcceptanceCopy.CHOOSE_PASSWORD).assertIsDisplayed()
        }

    // The recovery link's own landing. Sent to the invitation's exchange it would be refused, and
    // the screen would tell a patient holding a live link that it was already used.
    @Test
    fun aRecoveryLinkLandsOnANewPassword() =
        runComposeUiTest {
            var spentAsRecovery: String? = null
            var spentAsInvitation: String? = null

            setContent {
                CadenceRoot(
                    session = SessionState.SignedOut,
                    links = MutableStateFlow("cadence://recover?token_hash=$TOKEN"),
                    accept = {
                        spentAsInvitation = it
                        Acceptance.Accepted
                    },
                    choose = { PasswordSet.Done },
                    acceptRecovery = {
                        spentAsRecovery = it
                        Acceptance.Accepted
                    },
                )
            }
            waitForIdle()

            assertEquals(TOKEN, spentAsRecovery, "the recovery token never reached the recovery exchange")
            assertNull(spentAsInvitation, "the recovery token was spent as an invitation")
            onNodeWithText(RecoveryCopy.CHOOSE_NEW_PASSWORD).assertIsDisplayed()
        }

    // What a patient reads while the recovery exchange is in flight, and it is the one sentence
    // the first version of PasswordWords missed: «Проверяем приглашение» to somebody who tapped
    // «Восстановить доступ», on the normal path, for the whole round trip.
    @Test
    fun aRecoveryInFlightDoesNotMentionAnInvitation() =
        runComposeUiTest {
            val answer = CompletableDeferred<Acceptance>()

            setContent {
                CadenceRoot(
                    session = SessionState.SignedOut,
                    links = MutableStateFlow("cadence://recover?token_hash=$TOKEN"),
                    accept = { Acceptance.Accepted },
                    choose = { PasswordSet.Done },
                    acceptRecovery = { answer.await() },
                )
            }
            waitForIdle()

            onNodeWithText(RecoveryCopy.CHECKING).assertIsDisplayed()
            assertTrue(
                onAllNodesWithText(AcceptanceCopy.CHECKING).fetchSemanticsNodes().isEmpty(),
                "a patient recovering a password was told an invitation was being checked",
            )

            answer.complete(Acceptance.Accepted)
            waitForIdle()
        }

    // The recovery landing has to survive a recreation for the reason the invitation's does: its
    // token is single-use, and a rotation on the new-password screen would otherwise send the
    // patient back to sign-in holding a link already spent.
    @Test
    fun aRecoveryLandingSurvivesARecreation() =
        runComposeUiTest {
            var spent = 0
            val recreation = Recreation()

            setContent {
                recreation.around {
                    CadenceRoot(
                        session = SessionState.SignedIn,
                        links = MutableStateFlow("cadence://recover?token_hash=$TOKEN"),
                        accept = { Acceptance.Accepted },
                        choose = { PasswordSet.Done },
                        acceptRecovery = {
                            spent += 1
                            Acceptance.Accepted
                        },
                    )
                }
            }
            waitForIdle()
            onNodeWithText(RecoveryCopy.CHOOSE_NEW_PASSWORD).assertIsDisplayed()

            recreation.happen { waitForIdle() }

            assertEquals(1, spent, "the recovery link was spent again when the screen was recreated")
            onNodeWithText(RecoveryCopy.CHOOSE_NEW_PASSWORD).assertIsDisplayed()
        }

    // Recovery all the way through, and from a live session: a patient who forgot their password
    // may still be signed in on this device, and the landing has to outrank that the way the
    // invitation's does — then end inside the app rather than on a form with nowhere to go.
    @Test
    fun aRecoveryEndsInsideTheApp() =
        runComposeUiTest {
            var setWith: String? = null

            setContent {
                CadenceRoot(
                    session = SessionState.SignedIn,
                    links = MutableStateFlow("cadence://recover?token_hash=$TOKEN"),
                    accept = { Acceptance.Accepted },
                    choose = {
                        setWith = it
                        PasswordSet.Done
                    },
                    acceptRecovery = { Acceptance.Accepted },
                )
            }
            waitForIdle()

            onNodeWithContentDescription(AcceptanceCopy.PASSWORD_FIELD).performTextInput(A_PASSWORD)
            onNodeWithText(AcceptanceCopy.ENTER).performClick()
            waitForIdle()

            assertEquals(A_PASSWORD, setWith, "the new password never reached the write behind the form")
            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
        }

    // The sentence a spent recovery link gets is not the invitation's: this letter is one the
    // patient asks for themselves, and sending them to the clinic is a detour with a person on it.
    @Test
    fun aSpentRecoveryLinkPointsBackAtTheFormThatSendsIt() =
        runComposeUiTest {
            setContent {
                CadenceRoot(
                    session = SessionState.SignedOut,
                    links = MutableStateFlow("cadence://recover?token_hash=$TOKEN"),
                    accept = { Acceptance.Accepted },
                    choose = { PasswordSet.Done },
                    acceptRecovery = { Acceptance.Refused(AuthErrorCode.OtpExpired) },
                )
            }
            waitForIdle()

            onNodeWithText(RecoveryCopy.SPENT_HINT).assertIsDisplayed()
            assertTrue(
                onAllNodesWithText(AcceptanceCopy.SPENT_HINT).fetchSemanticsNodes().isEmpty(),
                "a spent recovery link sent the patient to the clinic for a letter they can ask for",
            )
        }

    // A recovery link arriving mid-acceptance does take the screen, and that is the choice: the
    // patient tapped it just now, the invitation's token is already spent, and the new landing ends
    // in the same place — a password. What it must not do is answer both, which is what two slots
    // did: the one not on screen spent its single-use token with nothing ever drawn for it.
    @Test
    fun theLinkTappedLastIsTheOneAnswered() =
        runComposeUiTest {
            var invitations = 0
            var recoveries = 0
            val links = MutableStateFlow<String?>(accept(TOKEN))

            setContent {
                CadenceRoot(
                    session = SessionState.SignedIn,
                    links = links,
                    accept = {
                        invitations += 1
                        Acceptance.Accepted
                    },
                    choose = { PasswordSet.Done },
                    acceptRecovery = {
                        recoveries += 1
                        Acceptance.Accepted
                    },
                )
            }
            waitForIdle()
            assertEquals(0, recoveries, "the recovery exchange ran for a link nobody followed")

            links.value = "cadence://recover?token_hash=$ANOTHER_TOKEN"
            waitForIdle()

            assertEquals(1, invitations, "the invitation was answered more than once")
            assertEquals(1, recoveries, "the link the patient just tapped was not answered")
            onNodeWithText(RecoveryCopy.CHOOSE_NEW_PASSWORD).assertIsDisplayed()
        }

    @Test
    fun anotherAddressDoesNotTakeAwayTheInvitationBeingAnswered() =
        runComposeUiTest {
            val links = MutableStateFlow<String?>(accept(TOKEN))

            setContent {
                CadenceRoot(
                    session = SessionState.SignedIn,
                    links = links,
                    accept = { Acceptance.Accepted },
                    choose = { PasswordSet.Done },
                )
            }
            waitForIdle()
            onNodeWithText(AcceptanceCopy.CHOOSE_PASSWORD).assertIsDisplayed()

            links.value = "cadence://settings?token_hash=$ANOTHER_TOKEN"
            waitForIdle()

            assertTrue(
                onAllNodesWithText(AcceptanceCopy.CHOOSE_PASSWORD).fetchSemanticsNodes().isNotEmpty(),
                "an address that is not an invitation took the acceptance screen away",
            )
        }
}
