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
import kotlinx.coroutines.flow.MutableStateFlow
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private const val TOKEN = "e75b4d4f54a86c915c0afdfc5db3b5cb6eea78ba43c1ccf6bd24c5cb"

private const val ANOTHER_TOKEN = "1a5f0c3e9d8b7a6c5e4d3f2a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e"

private const val TODAY_TAB = "Сегодня"

private const val A_PASSWORD = "a-long-enough-password"

private fun accept(token: String) = "cadence://accept?token_hash=$token"

@OptIn(ExperimentalTestApi::class)
class CadenceRootTest {
    // The cold start this block is for: what the platform hands over is a link, and until it is
    // read `invitationToken` in :shared has no caller — the tests in InvitationTest hand over a
    // token no patient could have produced. Carried to the password, which nothing else pins.
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
                )
            }
            waitForIdle()

            assertEquals(0, asked, "a link that is not an invitation was sent to the exchange")
            onNodeWithText(SIGN_IN_MARKER).assertIsDisplayed()
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

    // The recreation the other tests cannot reach: they hand `rememberInvitation` a token from the
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

    // The address arrives after the screen is up, which is the ordering the KDoc's rule is about.
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

            links.value = "cadence://recover?token_hash=$ANOTHER_TOKEN"
            waitForIdle()

            assertTrue(
                onAllNodesWithText(AcceptanceCopy.CHOOSE_PASSWORD).fetchSemanticsNodes().isNotEmpty(),
                "an address that is not an invitation took the acceptance screen away",
            )
        }
}
