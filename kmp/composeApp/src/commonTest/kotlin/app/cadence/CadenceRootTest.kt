package app.cadence

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.shared.auth.Acceptance
import app.cadence.shared.auth.PasswordSet
import app.cadence.shared.auth.SessionState
import kotlin.test.Test
import kotlin.test.assertEquals

private const val TOKEN = "e75b4d4f54a86c915c0afdfc5db3b5cb6eea78ba43c1ccf6bd24c5cb"

private const val ANOTHER_TOKEN = "1a5f0c3e9d8b7a6c5e4d3f2a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e"

private const val TODAY_TAB = "Сегодня"

@OptIn(ExperimentalTestApi::class)
class CadenceRootTest {
    // The cold start the whole block is for: what the platform hands over is a link, and until it
    // is read the exchange written in step 2 has no caller — every test beneath it passes a token
    // no patient could have produced.
    @Test
    fun aColdStartFromALinkOpensOnTheAcceptanceScreen() =
        runComposeUiTest {
            var handed: String? = null

            setContent {
                CadenceRoot(
                    session = SessionState.SignedOut,
                    link = "cadence://accept?token_hash=$TOKEN",
                    accept = {
                        handed = it
                        Acceptance.Accepted
                    },
                    choose = { PasswordSet.Done },
                )
            }
            waitForIdle()

            assertEquals(TOKEN, handed, "the link's token never reached the exchange")
            onNodeWithText(AcceptanceCopy.CHOOSE_PASSWORD).assertIsDisplayed()
        }

    // The roots are handed whatever the system decides to open the app with, and the launcher
    // hands over nothing at all. Answering a link that is not an invitation would send a
    // stranger's string to /verify on an ordinary launch.
    @Test
    fun aLinkThatIsNotAnInvitationIsNotAnswered() =
        runComposeUiTest {
            var asked = 0

            setContent {
                CadenceRoot(
                    session = SessionState.SignedOut,
                    link = "cadence://accept/../recover?token_hash=$TOKEN",
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

    // What `onNewIntent` on Android and `onOpenURL` on Apple exist for, measured where it can be:
    // the tree is composed once and outlives every link, so a link arriving into a live app has
    // to reach it through the value rather than through a new composition root.
    @Test
    fun aLinkArrivingWhileTheAppIsAliveIsAnswered() =
        runComposeUiTest {
            var link by mutableStateOf<String?>(null)

            setContent {
                CadenceRoot(
                    session = SessionState.SignedIn,
                    link = link,
                    accept = { Acceptance.Accepted },
                    choose = { PasswordSet.Done },
                )
            }

            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()

            link = "cadence://accept?token_hash=$TOKEN"
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
            var link by mutableStateOf("cadence://accept?token_hash=$TOKEN")

            setContent {
                CadenceRoot(
                    session = SessionState.SignedOut,
                    link = link,
                    accept = {
                        spent += it
                        Acceptance.Accepted
                    },
                    choose = { PasswordSet.Done },
                )
            }
            waitForIdle()

            link = "cadence://accept?token_hash=$ANOTHER_TOKEN"
            waitForIdle()

            assertEquals(listOf(TOKEN, ANOTHER_TOKEN), spent, "the second link was not answered as its own")
        }
}
