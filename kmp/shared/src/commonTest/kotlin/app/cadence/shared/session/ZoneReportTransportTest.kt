package app.cadence.shared.session

import app.cadence.shared.api.apis.IdentityApi
import app.cadence.shared.net.API_BASE
import app.cadence.shared.net.Session
import app.cadence.shared.net.SessionTokens
import app.cadence.shared.net.cadenceHttpClient
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.client.engine.mock.respondOk
import io.ktor.client.request.HttpRequestData
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpMethod
import io.ktor.http.HttpStatusCode
import io.ktor.http.Url
import io.ktor.http.content.TextContent
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

private const val ACCESS = "the-transport's-access-token"

private object OneSession : SessionTokens {
    override suspend fun current(): Session = Session(access = ACCESS, refresh = "r")

    override suspend fun refreshed(): Session? = null
}

class ZoneReportTransportTest {
    // The address is pinned host **and port**, because `API_BASE` is handed in as an argument and
    // the two bases differ only by port — measured: with the host alone, `baseUrl = AUTH_BASE` sent
    // the zone to the identity server and every assertion here stayed green. The bearer helper
    // `ApiClient` builds stays unset, so the header is the transport's.
    @Test
    fun theZoneGoesToTheEndpointUnderTheTransportsToken() =
        runTest {
            var seen: HttpRequestData? = null
            val engine =
                MockEngine { request ->
                    seen = request
                    respondOk()
                }

            IdentityApi(baseUrl = API_BASE, httpClient = cadenceHttpClient(engine, OneSession))
                .zoneReporter()("Asia/Tbilisi")

            val request = checkNotNull(seen) { "the reporter sent nothing" }
            assertEquals(HttpMethod.Post, request.method)
            assertEquals(Url(API_BASE).host, request.url.host)
            assertEquals(Url(API_BASE).port, request.url.port)
            assertEquals("/v1/me/session", request.url.encodedPath)
            assertEquals("""{"timezone":"Asia/Tbilisi"}""", (request.body as TextContent).text)
            assertEquals("Bearer $ACCESS", request.headers[HttpHeaders.Authorization])
        }

    // A refusal answers as normally as a 200 — `expectSuccess` is unset and `wrap()` never fails on
    // the status — so without the reporter's own check the whole refusal class is invisible.
    //
    // The status is asserted, not just the type: `check` and the collector's cancellation both
    // arrive as `IllegalStateException`, and so does a plugin precondition or a decode failure.
    @Test
    fun aRefusedZoneIsRaisedRatherThanReturned() =
        runTest {
            val engine = MockEngine { respond("", HttpStatusCode.BadRequest) }

            val raised =
                assertFailsWith<IllegalStateException> {
                    IdentityApi(baseUrl = API_BASE, httpClient = cadenceHttpClient(engine, OneSession))
                        .zoneReporter()("Mars/Olympus")
                }

            assertTrue("400" in raised.message.orEmpty(), "raised without the status: ${raised.message}")
        }
}
