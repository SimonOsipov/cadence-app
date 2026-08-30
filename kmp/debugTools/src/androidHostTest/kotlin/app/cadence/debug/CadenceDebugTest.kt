package app.cadence.debug

import app.cadence.shared.storage.PKCE_STORE
import app.cadence.shared.storage.SESSION_STORE
import com.russhwolf.settings.MapSettings
import com.russhwolf.settings.Settings
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
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
    private val asked = mutableListOf<String>()

    private fun wiring(stores: (String) -> Settings = { MapSettings() }) =
        CadenceDebug(
            api = API,
            gotrue = GOTRUE,
            engine =
                MockEngine { request ->
                    asked += request.url.toString()
                    respond(ME, HttpStatusCode.OK, json)
                },
            stores = stores,
        )

    // The two addresses are not interchangeable and nothing said so: swapped in the body, the
    // eight probe tests and the whole gate stayed green, because the artifact greps measure
    // presence and not wiring.
    @Test
    fun eachCallGoesToItsOwnService() =
        runTest {
            val debug = wiring()

            assertIs<ProbeState.SignedIn>(debug.me())
            assertTrue(debug.health())

            assertEquals(listOf("$API/v1/me", "$API/healthz"), asked)
        }

    // Two stores and not one, checked where they are actually handed out. The blob is written
    // whole, so a shared store would drop the PKCE verifier at the session's next write.
    @Test
    fun theSessionAndTheVerifierGetDifferentStores() {
        val handed = mutableMapOf<String, Settings>()

        wiring { name -> handed.getOrPut(name) { MapSettings() } }

        assertEquals(setOf(SESSION_STORE, PKCE_STORE), handed.keys)
        assertTrue(handed[SESSION_STORE] !== handed[PKCE_STORE])
    }
}
