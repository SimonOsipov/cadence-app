package app.cadence.debug

import io.ktor.client.HttpClient
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.client.engine.mock.respondError
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertTrue

private const val API = "http://localhost:8080"

private val json = headersOf(HttpHeaders.ContentType, "application/json")

private fun answering(
    status: HttpStatusCode,
    body: String = "{}",
) = HttpClient(MockEngine { respond(body, status, json) })

private fun refusing() = HttpClient(MockEngine { respondError(HttpStatusCode.ServiceUnavailable) })

private fun unreachable() = HttpClient(MockEngine { throw kotlinx.io.IOException("no route to host") })

class DebugProbeTest {
    @Test
    fun aCallThatWentThroughIsSignedIn() =
        runTest {
            val state = probeMe(answering(HttpStatusCode.OK, """{"full_name":"Марина Волкова"}"""), API)

            assertIs<ProbeState.SignedIn>(state)
            assertTrue("Марина" in state.body, "the body was not carried back: ${state.body}")
        }

    // The one distinction the screen must not draw. The API answers an expired token and a
    // token that was never valid with the same status and an indistinguishable body, on
    // purpose — a screen claiming to tell them apart would be inventing the difference, and
    // the patient-facing app would inherit the invention.
    @Test
    fun anExpiredTokenAndAnUnauthenticatedCallAreOneState() =
        runTest {
            val expired = probeMe(answering(HttpStatusCode.Unauthorized, """{"detail":"unauthorized"}"""), API)
            val never = probeMe(answering(HttpStatusCode.Unauthorized, ""), API)

            assertEquals(ProbeState.SignedOut, expired)
            assertEquals(ProbeState.SignedOut, never)
        }

    // A server that answered badly and a server that did not answer are one state too, and it
    // is not «signed out»: telling a developer their session died when the API is down sends
    // them to re-authenticate against something that cannot authenticate them.
    @Test
    fun aServerThatCannotAnswerIsUnavailableRatherThanSignedOut() =
        runTest {
            assertIs<ProbeState.Unavailable>(probeMe(refusing(), API))
            assertIs<ProbeState.Unavailable>(probeMe(unreachable(), API))
        }

    // /healthz is outside the contract deliberately, so it is absent from the generated client
    // and there is nothing to attach a Bearer to. Asked on a client with no auth plugin, which
    // is what makes «the API is up» separable from «my token works».
    @Test
    fun healthIsAskedWithoutACredential() =
        runTest {
            var offered: String? = null
            val raw =
                HttpClient(
                    MockEngine { request ->
                        offered = request.headers[HttpHeaders.Authorization]
                        respond("ok", HttpStatusCode.OK)
                    },
                )

            assertTrue(probeHealth(raw, API))
            assertEquals(null, offered, "the health probe carried a credential")
        }

    @Test
    fun healthIsFalseWhereTheApiDoesNotAnswer() =
        runTest {
            assertTrue(!probeHealth(unreachable(), API))
        }
}
