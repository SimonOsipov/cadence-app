package app.cadence.shared.net

import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.client.plugins.auth.Auth
import io.ktor.client.plugins.contentnegotiation.ContentNegotiation
import io.ktor.client.plugins.pluginOrNull
import io.ktor.client.request.get
import io.ktor.client.statement.HttpResponse
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNotSame
import kotlin.test.assertSame
import kotlin.test.assertTrue

private const val STALE = "stale-access-token"
private const val FRESH = "fresh-access-token"

private const val AN_ADDRESS = "https://api.cadence.example"

private var engineCalls = 0

// Completed once the engine has refused every request in flight. The refresh waits on it, so
// «they all met the expiry together» is arranged rather than hoped for — see the test.
private var allRefused = CompletableDeferred<Unit>()

private fun oneExpiryThenFresh(expected: Int): MockEngine {
    var refusals = 0

    return MockEngine { request ->
        engineCalls++
        val offered = request.headers[HttpHeaders.Authorization]
        if (offered == "Bearer $FRESH") {
            respond("{}", HttpStatusCode.OK, headersOf(HttpHeaders.ContentType, "application/json"))
        } else {
            refusals++
            if (refusals == expected) allRefused.complete(Unit)
            respond("", HttpStatusCode.Unauthorized)
        }
    }
}

private class CountingSession(
    private val until: CompletableDeferred<Unit>? = null,
) : SessionTokens {
    var refreshes = 0

    override suspend fun current(): Session = Session(access = STALE, refresh = "r")

    override suspend fun refreshed(): Session? {
        until?.await()
        refreshes++

        return Session(access = FRESH, refresh = "r")
    }
}

class CadenceHttpClientTest {
    // The property the whole transport is arranged around, and the one no single request can
    // show: several requests meeting an expired token at once must produce one refresh, not one
    // each. Under refresh-token rotation the extra ones spend a token already spent, and the
    // session is revoked — the patient is signed out by their own app being busy.
    @Test
    fun concurrentExpiriesProduceOneRefreshAndEveryRequestSucceeds() =
        runTest {
            engineCalls = 0
            allRefused = CompletableDeferred()
            val session = CountingSession(until = allRefused)
            val client = cadenceHttpClient(oneExpiryThenFresh(expected = 5), session)

            val answers =
                (1..5)
                    .map { async { client.get("$API_BASE/v1/me") } }
                    .awaitAll()

            assertEquals(1, session.refreshes, "each request refreshed on its own")
            answers.forEach { assertEquals(HttpStatusCode.OK, it.status) }

            // The witness that the five were actually in flight together, and without it the
            // assertion above passes for the wrong reason: one refresh is also what taking
            // turns produces. Ten is five refusals and five retries.
            //
            // The refresh waits until the engine has refused all five, so the interleaving is
            // arranged and not observed. It was observed once — this line asserted ten because
            // ten is what a macOS run produced — and the CI runner scheduled it differently and
            // saw fewer. A count that depends on which coroutine wakes first is a property of
            // the scheduler, not of the transport.
            assertEquals(10, engineCalls, "the requests did not meet the expiry together")
        }

    // The header and never the query string: a token in a URL is written to every access log
    // and proxy along the way, and it is the same token that opens the patient's record.
    @Test
    fun theTokenTravelsAsAHeader() =
        runTest {
            var seenUrl = ""
            var seenHeader: String? = null
            val engine =
                MockEngine { request ->
                    seenUrl = request.url.toString()
                    seenHeader = request.headers[HttpHeaders.Authorization]
                    respond("{}", HttpStatusCode.OK, headersOf(HttpHeaders.ContentType, "application/json"))
                }

            cadenceHttpClient(engine, CountingSession()).get("$API_BASE/v1/me")

            assertEquals("Bearer $STALE", seenHeader)
            assertTrue(STALE !in seenUrl, "the token reached the url: $seenUrl")
        }

    // A session printed into a log or a crash report must not carry the token with it. Ktor's
    // BearerTokens holds both in fields — measured with javap on the 3.5.1 artifact, it has no
    // toString of its own — and this pins the type the app's own code holds.
    @Test
    fun aSessionDoesNotPrintItsTokens() {
        val printed = Session(access = STALE, refresh = "refresh-token").toString()

        assertTrue(STALE !in printed, "the access token was printed: $printed")
        assertTrue("refresh-token" !in printed, "the refresh token was printed: $printed")
    }

    // The overload the app builds; every other test here hands in an engine. Dropped to a bare
    // `HttpClient()`, production would send every request with no Authorization header at all.
    @Test
    fun theClientTheAppBuildsCarriesTheSameConfiguration() {
        val built = cadenceHttpClientFor(AN_ADDRESS, CountingSession())

        assertNotNull(built.pluginOrNull(Auth), "the app's client has no bearer plugin")
        assertNotNull(built.pluginOrNull(ContentNegotiation), "the app's client has no JSON negotiation")
    }

    // One address, one transport — see `theTransport`.
    @Test
    fun theTransportForOneAddressIsBuiltOnce() {
        assertSame(
            cadenceHttpClientFor(AN_ADDRESS, CountingSession()),
            cadenceHttpClientFor(AN_ADDRESS, CountingSession()),
        )
    }

    // The url is the key and not decoration: collapsed to one field, the map hands the first
    // caller's transport — with the first caller's session — to an address that is not theirs.
    @Test
    fun aSecondAddressGetsATransportOfItsOwn() {
        assertNotSame(
            cadenceHttpClientFor(AN_ADDRESS, CountingSession()),
            cadenceHttpClientFor("https://auth.cadence.example", CountingSession()),
        )
    }
}

private suspend fun io.ktor.client.HttpClient.get(url: String): HttpResponse = get(url) {}
