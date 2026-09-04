package app.cadence.shared.session

import app.cadence.shared.api.apis.IdentityApi
import app.cadence.shared.net.API_BASE
import app.cadence.shared.net.Session
import app.cadence.shared.net.SessionTokens
import app.cadence.shared.net.cadenceHttpClient
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respondOk
import io.ktor.client.request.HttpRequestData
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpMethod
import io.ktor.http.content.TextContent
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals

private const val ACCESS = "the-transport's-access-token"

private object OneSession : SessionTokens {
    override suspend fun current(): Session = Session(access = ACCESS, refresh = "r")

    override suspend fun refreshed(): Session? = null
}

class ZoneReportTransportTest {
    // The generated client carries a bearer helper of its own, left unset because the transport is
    // the token's one owner — the property `scripts/gate/kmp.sh` greps for. Measured here rather
    // than trusted: an unset `HttpBearerAuth` that wrote a header would overwrite the transport's,
    // and the request would go out with a credential nothing refreshes.
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
            assertEquals("/v1/me/session", request.url.encodedPath)
            assertEquals("""{"timezone":"Asia/Tbilisi"}""", (request.body as TextContent).text)
            assertEquals("Bearer $ACCESS", request.headers[HttpHeaders.Authorization])
        }
}
