package app.cadence.shared.auth

import io.github.jan.supabase.auth.status.RefreshFailureCause
import io.github.jan.supabase.auth.status.SessionStatus
import io.github.jan.supabase.auth.user.UserSession
import io.github.jan.supabase.exceptions.UnknownRestException
import io.ktor.client.HttpClient
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.client.request.get
import io.ktor.http.HttpStatusCode
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.yield
import kotlinx.io.IOException
import kotlin.test.Test
import kotlin.test.assertEquals

private fun aSession() =
    UserSession(
        accessToken = "an-access-token",
        refreshToken = "a-refresh-token",
        expiresIn = 3600,
        tokenType = "bearer",
    )

// Built from a 502, because that is the only shape the vendor puts inside InternalServerError:
// AuthImpl constructs it exactly when the status is in NETWORK_ERROR_CODES, and 401/403/404 take
// clearSession() instead. A fixture built from a 401 certifies a value that never occurs — which
// is what the first version of this file did, and what let an inverted mapping pass the gate.
private suspend fun aRetryableFiveHundred(): UnknownRestException {
    val answer = HttpClient(MockEngine { respond("", HttpStatusCode.BadGateway) }).get("http://gotrue.test/token")

    return UnknownRestException("bad gateway", answer)
}

class SessionStateTest {
    // Launching with a session must open the app inside. While the store is still being read
    // the answer is «not yet» — mapped to SignedOut it would flash the sign-in screen, which
    // reads to a patient as having been signed out.
    @Test
    fun beforeTheStoreHasAnsweredNobodyIsSignedOut() {
        assertEquals(SessionState.Deciding, SessionStatus.Initializing.asSessionState())
    }

    @Test
    fun aHeldSessionIsSignedIn() {
        assertEquals(SessionState.SignedIn, SessionStatus.Authenticated(aSession()).asSessionState())
    }

    @Test
    fun noSessionIsSignedOut() {
        assertEquals(SessionState.SignedOut, SessionStatus.NotAuthenticated(false).asSessionState())
    }

    // The distinction the transport cannot make and this seam can. `SessionTokens.refreshed()`
    // answers null for both, deliberately — a refused token and a lost signal arrive alike
    // there, so it clears nothing. Here the vendor hands us the cause, and a patient whose
    // train entered a tunnel must not be put back on the sign-in screen.
    @Test
    fun aRefreshThatFoundNoNetworkKeepsThemSignedIn() {
        val lost = SessionStatus.RefreshFailure(RefreshFailureCause.NetworkError(IOException("no route")))

        assertEquals(SessionState.SignedIn, lost.asSessionState())
    }

    // The other failure cause, and it is not a refusal at all. InternalServerError means the
    // vendor got a retryable five-hundred, kept the session and scheduled a retry — so a
    // patient must stay inside while GoTrue restarts, not be thrown out and snapped back.
    @Test
    fun aRetryableServerErrorKeepsThemSignedIn() =
        runTest {
            val hiccup = SessionStatus.RefreshFailure(RefreshFailureCause.InternalServerError(aRetryableFiveHundred()))

            assertEquals(SessionState.SignedIn, hiccup.asSessionState())
        }

    // Signing out and being signed out are one state. The vendor's flag distinguishes a
    // deliberate sign-out from a session it cleared after a refusal, and the shell must not:
    // both put the patient in the same area, and a difference here would be one the product
    // does not have.
    @Test
    fun aDeliberateSignOutAndAClearedSessionAreOneState() {
        assertEquals(
            SessionStatus.NotAuthenticated(isSignOut = true).asSessionState(),
            SessionStatus.NotAuthenticated(isSignOut = false).asSessionState(),
        )
    }

    // The axis distinctUntilChanged actually lives on, and the one a StateFlow does not cover
    // for us: a successful refresh replaces the session, so the status changes while the state
    // does not. Without the operator the shell is handed «signed in» twice and re-enters the
    // area it is already in; MutableStateFlow's own conflation cannot see it, because the two
    // statuses are not equal.
    @Test
    fun aRenewedSessionIsNotASecondArrival() =
        runTest {
            val statuses = MutableStateFlow<SessionStatus>(SessionStatus.Authenticated(aSession()))
            val seen = mutableListOf<SessionState>()

            // The yield before the change is the arrangement, not politeness: without it the
            // reader has not subscribed when the value moves, sees only the last one, and the
            // assertion passes with the operator removed.
            val reader = launch { statuses.asSessionStates().toList(seen) }
            yield()
            statuses.value = SessionStatus.Authenticated(aSession().copy(accessToken = "a-renewed-token"))
            yield()
            reader.cancel()

            assertEquals(listOf<SessionState>(SessionState.SignedIn), seen)
        }

    // Ten requests meeting an expired token produce one refresh — the transport's property —
    // and the shell must not turn that into ten navigations.
    //
    // Driven through flowOf and not a StateFlow, which is the whole difference between this
    // test and a green one that measures nothing: MutableStateFlow drops an assignment equal to
    // the value it holds, and conflates the rest when nothing suspends between them, so ten
    // writes reach the operator as one whatever the operator does — flowOf emits all ten.
    @Test
    fun manyWaysOfSayingSignedOutAreOneTransition() =
        runTest {
            val expiries = (1..10).map { SessionStatus.NotAuthenticated(isSignOut = it % 2 == 0) }

            val seen = flowOf(*expiries.toTypedArray()).asSessionStates().toList()

            assertEquals(listOf<SessionState>(SessionState.SignedOut), seen)
        }
}
