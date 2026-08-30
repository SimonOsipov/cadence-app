package app.cadence.debug

import app.cadence.shared.api.apis.IdentityApi
import app.cadence.shared.net.Session
import app.cadence.shared.net.SessionTokens
import app.cadence.shared.net.cadenceHttpClient
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

// The contract's required fields. A body missing them is not a Me, and a test that omitted
// them would be measuring a parse failure while claiming to measure the pipeline.
private fun me(fullName: String?) =
    """{"sub":"9f3c…","role":"patient","expires_at":"2026-09-01T00:00:00Z"""" +
        (fullName?.let { ""","full_name":"$it"""" } ?: "") + "}"

private object Signed : SessionTokens {
    override suspend fun current() = Session(access = "a", refresh = "r")

    override suspend fun refreshed() = current()
}

// Built the way production builds it: the generated class over our own transport. The generated
// `ApiClient(baseUrl, httpClient)` configures nothing on the client it is handed, so what parses
// the body here is the ContentNegotiation cadenceHttpClient installs — measured, not assumed.
private fun identityOver(engine: MockEngine) = IdentityApi(API, cadenceHttpClient(engine, Signed))

private fun answering(
    status: HttpStatusCode,
    body: String = "{}",
) = identityOver(MockEngine { respond(body, status, json) })

private fun refusing() = MockEngine { respondError(HttpStatusCode.ServiceUnavailable) }

private fun unreachable() = MockEngine { throw kotlinx.io.IOException("no route to host") }

class DebugProbeTest {
    @Test
    fun aCallThatWentThroughIsSignedIn() =
        runTest {
            val state = probeMe(answering(HttpStatusCode.OK, me("Марина Волкова")))

            assertIs<ProbeState.SignedIn>(state)
            assertTrue("Марина" in state.body, "the body was not carried back: ${state.body}")
        }

    // The credential is the transport's job and the screen's whole question. Asked here because
    // the generated client is handed a configured client rather than configuring one: a call
    // that reached the server with no header would answer 401 and read as «signed out».
    @Test
    fun theCallCarriesTheSessionAsABearer() =
        runTest {
            var offered: String? = null
            val engine =
                MockEngine { request ->
                    offered = request.headers[HttpHeaders.Authorization]
                    respond(me("Марина Волкова"), HttpStatusCode.OK, json)
                }

            probeMe(identityOver(engine))

            assertEquals("Bearer a", offered)
        }

    // `full_name` is optional in the contract. An account GoTrue holds and the clinic has no
    // profile for is signed in — reporting it as unavailable would send a developer to debug
    // the server over a state the server is describing correctly.
    @Test
    fun anAccountWithNoProfileIsSignedInRatherThanUnavailable() =
        runTest {
            assertIs<ProbeState.SignedIn>(probeMe(answering(HttpStatusCode.OK, me(null))))
        }

    // The one distinction the screen must not draw. The API answers an expired token and a
    // token that was never valid with the same status and an indistinguishable body, on
    // purpose — a screen claiming to tell them apart would be inventing the difference, and
    // the patient-facing app would inherit the invention.
    @Test
    fun anExpiredTokenAndAnUnauthenticatedCallAreOneState() =
        runTest {
            val expired = probeMe(answering(HttpStatusCode.Unauthorized, """{"detail":"unauthorized"}"""))
            val never = probeMe(answering(HttpStatusCode.Unauthorized, ""))

            assertEquals(ProbeState.SignedOut, expired)
            assertEquals(ProbeState.SignedOut, never)
        }

    // A server that answered badly and a server that did not answer are one state too, and it
    // is not «signed out»: telling a developer their session died when the API is down sends
    // them to re-authenticate against something that cannot authenticate them.
    @Test
    fun aServerThatCannotAnswerIsUnavailableRatherThanSignedOut() =
        runTest {
            assertIs<ProbeState.Unavailable>(probeMe(identityOver(refusing())))
            assertIs<ProbeState.Unavailable>(probeMe(identityOver(unreachable())))
        }

    // A body the contract cannot parse is «unavailable» and not a crash: the one surface whose
    // job is to describe failures must survive the failure it describes.
    @Test
    fun aBodyTheContractCannotParseIsUnavailable() =
        runTest {
            assertIs<ProbeState.Unavailable>(probeMe(answering(HttpStatusCode.OK, """{"sub":42}""")))
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
            assertTrue(!probeHealth(HttpClient(unreachable()), API))
        }
}
