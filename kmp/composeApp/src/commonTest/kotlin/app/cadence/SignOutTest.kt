package app.cadence

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onAllNodesWithContentDescription
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.shared.auth.SessionState
import app.cadence.shared.auth.SignIn
import kotlin.test.Test
import kotlin.test.assertTrue

private const val TODAY_TAB = "Сегодня"

@OptIn(ExperimentalTestApi::class)
class SignOutTest {
    // The whole of «signing out works»: a control a patient can reach, and an area that is gone
    // once they do. With a password now mandatory on acceptance, this has stopped being a
    // voluntary lockout — which is why the button can exist at all.
    @Test
    fun signingOutLeavesTheProtectedAreaUnreachable() =
        runComposeUiTest {
            var session by mutableStateOf<SessionState>(SessionState.SignedIn)

            setContent { App(session, onSignOut = { session = SessionState.SignedOut }) }

            onNodeWithContentDescription("Профиль").performClick()
            waitForIdle()

            onNodeWithText(SignInCopy.SIGN_OUT).performClick()
            waitForIdle()

            assertTrue(
                onAllNodesWithContentDescription(TODAY_TAB).fetchSemanticsNodes().isEmpty(),
                "the area after sign-in survived signing out",
            )
            // The form itself and not its title: the title is what the placeholder said too, so
            // asserting on it would pass over a screen with nothing to type into.
            onNodeWithContentDescription(SignInCopy.ADDRESS_FIELD).assertIsDisplayed()
        }

    // The refusal has to travel the last hop too: the screen draws what it is handed, and handed
    // nothing it draws a form that looks as if the tap did not register.
    @Test
    fun aRefusalReachesTheFormThroughTheApp() =
        runComposeUiTest {
            setContent { App(SessionState.SignedOut, signIn = SignInPrompt(problem = SignIn.Refused)) }

            onNodeWithText(SignInCopy.REFUSED).assertIsDisplayed()
        }

    // Without a session the pre-sign-in area is the form, not a word: step 1 left a marker here
    // deliberately, and this is the step that owes it a screen.
    @Test
    fun withoutASessionTheFormIsWhatIsShown() =
        runComposeUiTest {
            setContent { App(SessionState.SignedOut) }

            onNodeWithContentDescription(SignInCopy.ADDRESS_FIELD).assertIsDisplayed()
            onNodeWithContentDescription(SignInCopy.PASSWORD_FIELD).assertIsDisplayed()
        }
}
