package app.cadence.shared.net

import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.client.request.get
import io.ktor.client.statement.HttpResponse
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private const val STALE = "stale-access-token"
private const val FRESH = "fresh-access-token"

/** Answers 401 to anything carrying the stale token and 200 to anything carrying the fresh one. */
private var engineCalls = 0

private fun oneExpiryThenFresh(): MockEngine =
    MockEngine { request ->
        engineCalls++
        val offered = request.headers[HttpHeaders.Authorization]
        if (offered == "Bearer $FRESH") {
            respond("{}", HttpStatusCode.OK, headersOf(HttpHeaders.ContentType, "application/json"))
        } else {
            respond("", HttpStatusCode.Unauthorized)
        }
    }

private class CountingSession : SessionTokens {
    var refreshes = 0

    override suspend fun current(): Session = Session(access = STALE, refresh = "r")

    override suspend fun refreshed(): Session? {
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
            val session = CountingSession()
            val client = cadenceHttpClient(oneExpiryThenFresh(), session)

            val answers =
                (1..5)
                    .map { async { client.get("$API_BASE/v1/me") } }
                    .awaitAll()

            assertEquals(1, session.refreshes, "each request refreshed on its own")
            answers.forEach { assertEquals(HttpStatusCode.OK, it.status) }

            // The witness that the five were actually in flight together, and without it the
            // assertion above passes for the wrong reason: runTest is single-threaded, and one
            // refresh is also what taking turns produces. Sequentially the engine sees six —
            // one refusal, one retry, then four already carrying the fresh token, the refresh
            // itself never reaching it. Ten is five refusals and five retries. Both numbers
            // measured by running each shape; the first figure written here said seven and was
            // arithmetic rather than measurement.
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
    // own BearerTokens is a data class and would; ours is the type that crosses this seam.
    @Test
    fun aSessionDoesNotPrintItsTokens() {
        val printed = Session(access = STALE, refresh = "refresh-token").toString()

        assertTrue(STALE !in printed, "the access token was printed: $printed")
        assertTrue("refresh-token" !in printed, "the refresh token was printed: $printed")
    }
}

private suspend fun io.ktor.client.HttpClient.get(url: String): HttpResponse = get(url) {}
