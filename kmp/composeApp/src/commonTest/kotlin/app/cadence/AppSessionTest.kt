package app.cadence

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onAllNodesWithContentDescription
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.shared.auth.SessionState
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.emptyFlow
import kotlin.test.Test
import kotlin.test.assertTrue

private const val TODAY_TAB = "Сегодня"

@OptIn(ExperimentalTestApi::class)
class AppSessionTest {
    // Launching with a session opens the app inside. An intermediate sign-in screen reads to a
    // patient as having been signed out, so «inside» has to be the first thing composed.
    @Test
    fun aHeldSessionOpensInsideRatherThanOnSignIn() =
        runComposeUiTest {
            setContent { App(SessionState.SignedIn) }

            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
        }

    // The three leak paths at once, and closed by construction rather than guarded: the area
    // after sign-in is not composed at all, so there is no destination to navigate to, no back
    // stack entry to pop into, and nothing for a deep link to resolve against.
    @Test
    fun withoutASessionTheInsideIsNotInTheTree() =
        runComposeUiTest {
            setContent { App(SessionState.SignedOut) }

            assertTrue(
                onAllNodesWithContentDescription(TODAY_TAB).fetchSemanticsNodes().isEmpty(),
                "the area after sign-in was composed for a patient without a session",
            )
        }

    // The leak path the construction argument is actually about, and the one a cold SignedOut
    // cannot measure: from a cold start there was never a back stack to return into, so the
    // assertion is vacuous. Here the shell is entered, navigated inside, and only then does the
    // session go — which is what a background expiry does to a patient mid-screen.
    @Test
    fun anExpiryMidScreenLeavesNothingToComeBackTo() =
        runComposeUiTest {
            var session by mutableStateOf<SessionState>(SessionState.SignedIn)
            setContent { App(session) }

            // Somewhere other than the first destination, so the shell's own back stack is not
            // empty when the session goes.
            onNodeWithContentDescription("Расписание").performClick()
            waitForIdle()

            // The probe has to be a node this destination draws — ScheduleScreen's own band.
            // Keyed on the tab bar it would be blind: this destination deliberately draws none,
            // so a «no tab bar» assertion is true before the session goes and cannot fail.
            onNodeWithText("ТЕКУЩИЙ КУРС").assertIsDisplayed()

            session = SessionState.SignedOut
            waitForIdle()

            assertTrue(
                onAllNodesWithText("ТЕКУЩИЙ КУРС").fetchSemanticsNodes().isEmpty(),
                "the screen the patient was on survived the expiry",
            )
            assertTrue(
                onAllNodesWithContentDescription(TODAY_TAB).fetchSemanticsNodes().isEmpty(),
                "the area after sign-in survived the expiry",
            )
            assertTrue(
                onAllNodesWithText(SIGN_IN_MARKER).fetchSemanticsNodes().isNotEmpty(),
                "the expiry led nowhere",
            )
        }

    // Neither area while nothing is decided yet — and it is not the store that decides, see
    // SessionState.Deciding. Rendered as signed out this flashes the sign-in screen on every
    // launch of a signed-in app.
    @Test
    fun whileNothingIsDecidedNeitherAreaIsShown() =
        runComposeUiTest {
            setContent { App(SessionState.Deciding) }

            assertTrue(
                onAllNodesWithContentDescription(TODAY_TAB).fetchSemanticsNodes().isEmpty(),
                "the inside was composed before anything was decided",
            )
            assertTrue(
                onAllNodesWithText(SIGN_IN_MARKER).fetchSemanticsNodes().isEmpty(),
                "the sign-in area flashed before anything was decided",
            )
            // And it is not blank either: this is what a patient with a session looks at for a
            // network round trip on an ordinary launch.
            onNodeWithText(SPLASH_MARKER).assertIsDisplayed()
        }

    // The value the roots see before the stream speaks, which is the one literal in this design
    // nothing else can measure: written SignedOut it flashes the sign-in screen on every launch
    // of a signed-in app, and both roots stay green because neither is composed by any test.
    //
    // Driven from an empty flow and not a StateFlow, which is the whole of why it measures
    // anything: a StateFlow hands over its held value as soon as collection starts, so the
    // literal is overwritten before the first assertion and the mutation survives.
    @Test
    fun beforeTheStreamSpeaksNeitherAreaIsShown() =
        runComposeUiTest {
            setContent { App(emptyFlow<SessionState>().collectAsSessionState()) }

            assertTrue(
                onAllNodesWithContentDescription(TODAY_TAB).fetchSemanticsNodes().isEmpty(),
                "the inside was composed before the stream said anything",
            )
            assertTrue(
                onAllNodesWithText(SIGN_IN_MARKER).fetchSemanticsNodes().isEmpty(),
                "the sign-in area flashed before the stream said anything",
            )
        }

    // The other half of the same function, and the half the test above cannot see: that what the
    // stream says afterwards arrives at all. Stubbed to answer Deciding for ever — a screen blank
    // for the whole life of the app — every other test here stays green.
    @Test
    fun whatTheStreamSaysAfterwardsIsWhatIsShown() =
        runComposeUiTest {
            val sessions = MutableStateFlow<SessionState>(SessionState.Deciding)
            setContent { App(sessions.collectAsSessionState()) }

            sessions.value = SessionState.SignedIn
            waitForIdle()

            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
        }

    // An invitation outranks whatever the session says, in both directions: a patient with a live
    // session who followed a link is answering the link, and one without a session must not be
    // shown a sign-in screen for an account the link is about to create.
    @Test
    fun anInvitationIsAnsweredWhateverTheSessionSays() =
        runComposeUiTest {
            val sessions = listOf(SessionState.SignedIn, SessionState.SignedOut, SessionState.Deciding)

            for (session in sessions) {
                setContent { App(session, invitation = Invitation.InFlight) }

                onNodeWithText(AcceptanceCopy.CHECKING).assertIsDisplayed()

                assertTrue(
                    onAllNodesWithContentDescription(TODAY_TAB).fetchSemanticsNodes().isEmpty(),
                    "the area after sign-in was composed over an invitation, on $session",
                )
                assertTrue(
                    onAllNodesWithText(SIGN_IN_MARKER).fetchSemanticsNodes().isEmpty(),
                    "the sign-in area was composed over an invitation, on $session",
                )
            }
        }

    // Absence is what returns the app to its two areas: an invitation that stayed on screen after
    // it was answered would strand a patient who has just set a password.
    @Test
    fun withoutAnInvitationTheAreasAreWhatTheSessionSays() =
        runComposeUiTest {
            setContent { App(SessionState.SignedIn, invitation = null) }

            onNodeWithContentDescription(TODAY_TAB).assertIsSelected()
        }

    // The pre-sign-in area exists and is what a patient without a session reaches. Steps 3-5
    // fill it; step 1 owns only that it is the one composed.
    @Test
    fun withoutASessionThePreSignInAreaIsWhatIsShown() =
        runComposeUiTest {
            setContent { App(SessionState.SignedOut) }

            assertTrue(
                onAllNodesWithText(SIGN_IN_MARKER).fetchSemanticsNodes().isNotEmpty(),
                "a patient without a session reached neither area",
            )
        }
}
