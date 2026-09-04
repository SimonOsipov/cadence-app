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

private const val ACCESS = "the-transport's-access-token"

private object OneSession : SessionTokens {
    override suspend fun current(): Session = Session(access = ACCESS, refresh = "r")

    override suspend fun refreshed(): Session? = null
}

class ZoneReportTransportTest {
    // The host is pinned because `API_BASE` is handed in as an argument: without it the test only
    // restates what it just passed, and sending the zone to the identity server instead would read
    // the same. The bearer helper `ApiClient` builds stays unset, so the header is the transport's.
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
            assertEquals("/v1/me/session", request.url.encodedPath)
            assertEquals("""{"timezone":"Asia/Tbilisi"}""", (request.body as TextContent).text)
            assertEquals("Bearer $ACCESS", request.headers[HttpHeaders.Authorization])
        }

    // A refusal answers as normally as a 200 — `expectSuccess` is not set and the generated `wrap()`
    // never reads the status — so without the reporter's own check the whole refusal class is
    // invisible. This is the test that fails against dropping it.
    @Test
    fun aRefusedZoneIsRaisedRatherThanReturned() =
        runTest {
            val engine = MockEngine { respond("", HttpStatusCode.BadRequest) }

            assertFailsWith<IllegalStateException> {
                IdentityApi(baseUrl = API_BASE, httpClient = cadenceHttpClient(engine, OneSession))
                    .zoneReporter()("Mars/Olympus")
            }
        }
}
