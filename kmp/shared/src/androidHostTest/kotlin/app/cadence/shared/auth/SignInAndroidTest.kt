package app.cadence.shared.auth

import com.russhwolf.settings.MapSettings
import io.github.jan.supabase.auth.auth
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.client.request.HttpRequestData
import io.ktor.content.TextContent
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import io.ktor.utils.io.errors.IOException
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runTest
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import kotlin.test.assertContains
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull

private const val GOTRUE = "http://localhost:9999"

private const val AN_ADDRESS = "patient@clinic.example"

private const val A_PASSWORD = "a-long-enough-password"

private const val A_SESSION = """
    {"access_token":"an-access-token","token_type":"bearer","expires_in":3600,
     "refresh_token":"a-refresh-token"}
"""

private fun aRefusal(
    status: HttpStatusCode,
    code: String?,
) = if (code == null) {
    """{"code":${status.value},"msg":"a refusal"}"""
} else {
    """{"code":${status.value},"error_code":"$code","msg":"a refusal"}"""
}

private fun jsonError(
    status: HttpStatusCode,
    code: String? = null,
) = MockEngine {
    respond(aRefusal(status, code), status, headersOf("Content-Type", "application/json"))
}

private fun sessions() =
    MockEngine {
        respond(A_SESSION, HttpStatusCode.OK, headersOf("Content-Type", "application/json"))
    }

private fun clientAnswering(engine: MockEngine) = cadenceAuth(url = GOTRUE, stores = { MapSettings() }, engine = engine)

private fun HttpRequestData.sentBody(): String = (body as TextContent).text

/**
 * Signing in and out, through a whole client rather than around it — the reasons are
 * [InvitationExchangeAndroidTest]'s, and so is the Robolectric runtime.
 */
@RunWith(RobolectricTestRunner::class)
class SignInAndroidTest {
    // The request, because everything below is green whatever it says: an engine that answers
    // without looking takes the happy arm for a sign-up, a magic link or an empty password alike.
    @Test
    fun theAddressAndPasswordAreSentToTheTokenRoute() =
        runTest {
            val engine = sessions()

            clientAnswering(engine).signIn(AN_ADDRESS, A_PASSWORD)

            val asked = engine.requestHistory.single()

            assertEquals("/token", asked.url.encodedPath)
            assertEquals("password", asked.url.parameters["grant_type"])
            assertContains(asked.sentBody(), """"email":"$AN_ADDRESS"""")
            assertContains(asked.sentBody(), """"password":"$A_PASSWORD"""")
        }

    @Test
    fun correctCredentialsBecomeASession() =
        runTest {
            val client = clientAnswering(sessions())

            assertEquals(SignIn.Accepted, client.signIn(AN_ADDRESS, A_PASSWORD))
            assertNotNull(client.auth.currentSessionOrNull(), "nothing was stored to be signed in with")
            // The store and the stream are two observables, and the app navigates on the second:
            // asserting only the first would pass over a sign-in the shell never notices.
            assertEquals(SessionState.SignedIn, client.sessionStates().first())
        }

    // The form must not answer «no such address» to one and «wrong password» to the other: told
    // apart, it tells anyone who asks which of the clinic's patients has an account here. Compared
    // to each other rather than to a constant — a refusal that later grows a reason fails here.
    @Test
    fun anUnknownAddressAndAWrongPasswordAreRefusedAlike() =
        runTest {
            val unknownAddress = clientAnswering(jsonError(HttpStatusCode.BadRequest, "user_not_found"))
            val wrongPassword = clientAnswering(jsonError(HttpStatusCode.BadRequest, "invalid_credentials"))

            assertEquals(
                unknownAddress.signIn(AN_ADDRESS, A_PASSWORD),
                wrongPassword.signIn(AN_ADDRESS, A_PASSWORD),
                "the sign-in form tells an unknown address apart from a wrong password",
            )
            assertEquals(SignIn.Refused, wrongPassword.signIn(AN_ADDRESS, A_PASSWORD))
        }

    // The other direction: «check the address and the password» over a server that is simply down
    // sends a patient to change a password that was right.
    @Test
    fun aServerThatWillAnswerLaterIsWorthAnotherTry() =
        runTest {
            val later =
                mapOf(
                    "a rate limit" to jsonError(HttpStatusCode.TooManyRequests, "over_request_rate_limit"),
                    "a restarting server" to jsonError(HttpStatusCode.InternalServerError, "unexpected_failure"),
                    "a gateway with nothing behind it" to jsonError(HttpStatusCode.BadGateway),
                    "a timeout" to jsonError(HttpStatusCode.RequestTimeout),
                    "no connection at all" to MockEngine { throw IOException("no route to host") },
                )

            for ((what, engine) in later) {
                assertEquals(SignIn.Unreachable, clientAnswering(engine).signIn(AN_ADDRESS, A_PASSWORD), "on $what")
            }
        }

    @Test
    fun signingOutLeavesNoSession() =
        runTest {
            val client = clientAnswering(sessions())
            client.signIn(AN_ADDRESS, A_PASSWORD)

            client.signOut()

            assertNull(client.auth.currentSessionOrNull(), "the session survived signing out")
            assertEquals(SessionState.SignedOut, client.sessionStates().first())
        }

    // Signing out is the patient's decision, and a server that cannot be reached does not get to
    // veto it: left signed in on a device they have just handed to somebody, the refusal is the
    // whole harm the button exists to prevent.
    @Test
    fun signingOutWithoutAServerStillLeavesNoSession() =
        runTest {
            var answered = 0
            val client =
                clientAnswering(
                    MockEngine {
                        if (answered++ == 0) {
                            respond(A_SESSION, HttpStatusCode.OK, headersOf("Content-Type", "application/json"))
                        } else {
                            throw IOException("no route to host")
                        }
                    },
                )
            client.signIn(AN_ADDRESS, A_PASSWORD)

            client.signOut()

            assertNull(client.auth.currentSessionOrNull(), "an unreachable server kept the patient signed in")
            // The one the store alone cannot answer: `clearSession` is a different entry point
            // from `signOut`, and a wiped store behind a stream still saying «inside» would leave
            // the patient in the protected area with this test green.
            assertEquals(SessionState.SignedOut, client.sessionStates().first())
        }

    // The ordinary refusal on this route, not an exotic one: a refresh token already expired or
    // revoked answers with a status, not a dropped connection. Nothing else feeds that arm, and
    // without it signOut throws out of the coroutine the button launched.
    @Test
    fun aRefusedSignOutIsStillASignOut() =
        runTest {
            var answered = 0
            val client =
                clientAnswering(
                    MockEngine { request ->
                        if (answered++ == 0) {
                            respond(A_SESSION, HttpStatusCode.OK, headersOf("Content-Type", "application/json"))
                        } else {
                            respond(
                                aRefusal(HttpStatusCode.Unauthorized, "session_not_found"),
                                HttpStatusCode.Unauthorized,
                                headersOf("Content-Type", "application/json"),
                            )
                        }
                    },
                )
            client.signIn(AN_ADDRESS, A_PASSWORD)

            client.signOut()

            assertEquals(SessionState.SignedOut, client.sessionStates().first())
        }
}
