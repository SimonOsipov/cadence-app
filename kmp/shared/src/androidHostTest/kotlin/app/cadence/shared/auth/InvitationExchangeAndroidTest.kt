package app.cadence.shared.auth

import com.russhwolf.settings.MapSettings
import io.github.jan.supabase.auth.auth
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.client.engine.mock.respondError
import io.ktor.client.request.HttpRequestData
import io.ktor.content.TextContent
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import io.ktor.utils.io.errors.IOException
import kotlinx.coroutines.test.runTest
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import kotlin.test.assertContains
import kotlin.test.assertEquals
import kotlin.test.assertNotNull

private const val GOTRUE = "http://localhost:9999"

private const val TOKEN = "e75b4d4f54a86c915c0afdfc5db3b5cb6eea78ba43c1ccf6bd24c5cb"

// Cut to what the vendor reads out of GoTrue's answer.
private const val A_SESSION = """
    {"access_token":"an-access-token","token_type":"bearer","expires_in":3600,
     "refresh_token":"a-refresh-token"}
"""

private fun clientAnswering(engine: MockEngine) = cadenceAuth(url = GOTRUE, stores = { MapSettings() }, engine = engine)

private fun HttpRequestData.sentBody(): String = (body as TextContent).text

/**
 * The exchange, through a whole client rather than around it.
 *
 * Under Robolectric and on one platform, for the reason [CadenceAuthAndroidTest] states and one
 * more measured here: installing Auth registers a `ProcessLifecycleOwner` observer, and a plain
 * host test answers «Method getMainLooper in android.os.Looper not mocked». Testing the outcome
 * mapping on its own would leave the call site — which failure reaches which arm — unmeasured,
 * which is the trade this file refuses. What the real GoTrue answers is pinned in the **api**
 * harness; these are the arms, and the request that earns them.
 */
@RunWith(RobolectricTestRunner::class)
class InvitationExchangeAndroidTest {
    // The request itself, because everything below is green whatever it says: a recovery type, an
    // empty token or another route all take the happy arm against an engine that answers without
    // looking. On a live GoTrue each of those refuses a fresh invitation, which the mapping would
    // then show as «already used» — the one outcome that costs a patient their invitation.
    @Test
    fun theTokenIsSpentAsAnInvitationAtTheVerifyRoute() =
        runTest {
            val engine =
                MockEngine {
                    respond(A_SESSION, HttpStatusCode.OK, headersOf("Content-Type", "application/json"))
                }

            clientAnswering(engine).acceptInvitation(TOKEN)

            val asked = engine.requestHistory.single()

            assertEquals("/verify", asked.url.encodedPath)
            assertContains(asked.sentBody(), TOKEN)
            assertContains(asked.sentBody(), "invite")
        }

    // «Which the vendor then stores» is what step 3 builds on, and an exchange that answered
    // Accepted while storing nothing would leave the acceptance screen with no session to use.
    @Test
    fun anUnspentTokenBecomesASession() =
        runTest {
            val client =
                clientAnswering(
                    MockEngine {
                        respond(A_SESSION, HttpStatusCode.OK, headersOf("Content-Type", "application/json"))
                    },
                )

            assertEquals(Acceptance.Accepted, client.acceptInvitation(TOKEN))
            assertNotNull(client.auth.currentSessionOrNull(), "nothing was stored to accept with")
        }

    // A link opened twice is the ordinary case — the email read on two devices — and it must not
    // read as «the server is unavailable»: one is explained, the other is offered another try.
    @Test
    fun aSpentTokenIsRefusedRatherThanRetried() =
        runTest {
            val client = clientAnswering(MockEngine { respondError(HttpStatusCode.Forbidden) })

            assertEquals(Acceptance.Spent, client.acceptInvitation(TOKEN))
        }

    // The other direction, and the expensive one: measured in the 3.7.0 artifact, a 500 and a 429
    // are built into the same `RestException` a refusal is, so catching the type told a patient
    // holding a live invitation that it was used up — and sent them back to the clinic for one
    // they did not need.
    @Test
    fun aServerThatAnsweredBadlyIsNotASpentLink() =
        runTest {
            val hiccups = listOf(HttpStatusCode.InternalServerError, HttpStatusCode.TooManyRequests)

            for (hiccup in hiccups) {
                val client = clientAnswering(MockEngine { respondError(hiccup) })

                assertEquals(Acceptance.Unreachable, client.acceptInvitation(TOKEN), "on $hiccup")
            }
        }

    @Test
    fun anUnreachableServerIsNotARefusal() =
        runTest {
            val client = clientAnswering(MockEngine { throw IOException("no route") })

            assertEquals(Acceptance.Unreachable, client.acceptInvitation(TOKEN))
        }
}
