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
                "the inside was composed before the store answered",
            )
            assertTrue(
                onAllNodesWithText(SIGN_IN_MARKER).fetchSemanticsNodes().isEmpty(),
                "the sign-in area flashed before the store answered",
            )
        }

    // The value the roots see before the stream speaks, which is the one literal in this design
    // nothing else can measure: written SignedOut it flashes the sign-in screen on every launch
    // of a signed-in app, and both roots stay green because neither is composed by any test.
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
