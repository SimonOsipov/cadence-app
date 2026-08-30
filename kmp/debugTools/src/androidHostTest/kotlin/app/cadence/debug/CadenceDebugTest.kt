package app.cadence.debug

import app.cadence.shared.storage.PKCE_STORE
import app.cadence.shared.storage.SESSION_STORE
import com.russhwolf.settings.MapSettings
import com.russhwolf.settings.Settings
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.MockRequestHandler
import io.ktor.client.engine.mock.respond
import io.ktor.client.engine.mock.respondError
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import kotlinx.coroutines.test.runTest
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertTrue

private const val API = "http://api.test"
private const val GOTRUE = "http://gotrue.test"

private val json = headersOf(HttpHeaders.ContentType, "application/json")

// A GoTrue token response, so a sign-in in these tests produces a real session — which is what
// puts a Bearer on the API call. Anything less and the credential assertions measure nothing.
private const val TOKEN =
    """{"access_token":"an-access-token","refresh_token":"a-refresh-token",""" +
        """"expires_in":3600,"token_type":"bearer"}"""

private const val ME =
    """{"sub":"9f3c…","role":"patient","expires_at":"2026-09-01T00:00:00Z","full_name":"Марина"}"""

/**
 * The assembly itself, which nothing else measures.
 *
 * Under Robolectric because installing `Auth` starts auto-refresh on the main dispatcher — the
 * same reason the shared module's auth test lives under one. Every seam is a parameter precisely
 * so this can run off a device.
 */
@RunWith(RobolectricTestRunner::class)
class CadenceDebugTest {
    private val asked = mutableListOf<Pair<String, String?>>()

    private fun wiring(
        stores: (String) -> Settings = { MapSettings() },
        answer: MockRequestHandler = { request ->
            respond(if ("token" in request.url.encodedPath) TOKEN else ME, HttpStatusCode.OK, json)
        },
    ) = CadenceDebug(
        api = API,
        gotrue = GOTRUE,
        engine =
            MockEngine { request ->
                asked += request.url.toString() to request.headers[HttpHeaders.Authorization]
                answer(request)
            },
        stores = stores,
    )

    // The two addresses are not interchangeable and nothing said so. Sign-in is in here because
    // it is the only call that carries the GoTrue address: the auth module builds its own client,
    // so until it was handed this engine, pointing it at the API address survived every test and
    // the whole gate — and on the production path that is the password going to the wrong host.
    @Test
    fun eachCallGoesToItsOwnService() =
        runTest {
            val debug = wiring()

            debug.signIn("someone@cadence.local", "a-password")
            assertIs<ProbeState.SignedIn>(debug.me())
            assertTrue(debug.health())

            val hosts = asked.map { (url, _) -> url }
            assertTrue(hosts[0].startsWith(GOTRUE), "sign-in did not go to GoTrue: ${hosts[0]}")
            assertEquals(listOf("$API/v1/me", "$API/healthz"), hosts.drop(1))
        }

    // The step's own acceptance, checked on the client CadenceDebug builds rather than on one a
    // test builds: /healthz is outside the contract so there is nothing to attach a Bearer to,
    // and giving `raw` the auth plugin left every other assertion here green.
    @Test
    fun healthCarriesNoCredentialAndTheApiCallDoes() =
        runTest {
            val debug = wiring()

            debug.signIn("someone@cadence.local", "a-password")
            debug.me()
            debug.health()

            val me = asked.single { (url, _) -> url.endsWith("/v1/me") }
            val health = asked.single { (url, _) -> url.endsWith("/healthz") }

            assertEquals("Bearer an-access-token", me.second)
            assertEquals(null, health.second, "the health probe carried a credential")
        }

    // What reaches the screen on a refusal is the exception's class, pinned by name rather than
    // by shape: an earlier version asserted «contains no space», which killed the mutation only
    // because that fixture's body happened to produce «Unknown error». The refusal GoTrue really
    // gives — invalid_credentials — has no space either, and the mutation would have survived it.
    @Test
    fun aRefusedSignInReportsAClassAndNotAMessage() =
        runTest {
            val debug = wiring(answer = { respondError(HttpStatusCode.BadRequest) })

            val outcome = debug.signIn("someone@cadence.local", "the-wrong-password")

            assertEquals("AuthRestException", assertIs<SignIn.Refused>(outcome).why)
        }

    // The danger itself, asked directly. kotlinx.serialization appends the input it could not
    // decode, and on this path that input is GoTrue's token response — so a refusal carrying a
    // session must not put one on the screen.
    @Test
    fun aRefusalCarryingASessionDoesNotPutItOnTheScreen() =
        runTest {
            val debug =
                wiring(
                    answer = { respond(TOKEN, HttpStatusCode.BadRequest, json) },
                )

            val outcome = debug.signIn("someone@cadence.local", "the-wrong-password")

            val refused = assertIs<SignIn.Refused>(outcome)
            assertTrue(
                "an-access-token" !in refused.why && "a-refresh-token" !in refused.why,
                "a token reached the screen: ${refused.why}",
            )
        }
}
