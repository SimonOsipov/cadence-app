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

// Refusals in the shape GoTrue actually sends them, because the shape is the measurement: a body
// that will not decode falls back to «Unknown error» inside the vendor and carries no code at all,
// so a fixture built from `respondError` alone certifies an answer the server never gives.
private fun aRefusal(code: String) = """{"code":403,"error_code":"$code","msg":"a refusal"}"""

private fun jsonError(
    status: HttpStatusCode,
    code: String,
) = MockEngine { respond(aRefusal(code), status, headersOf("Content-Type", "application/json")) }

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

            // Field and value together: the neighbouring overload puts the token in `token` and
            // adds an `email`, which leaves the path, the word «invite» and the token itself all
            // where a looser assertion would still find them — and a live GoTrue refusing.
            assertEquals("/verify", asked.url.encodedPath)
            assertContains(asked.sentBody(), """"token_hash":"$TOKEN"""")
            assertContains(asked.sentBody(), """"type":"invite"""")
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
            val client = clientAnswering(jsonError(HttpStatusCode.Forbidden, "otp_expired"))

            assertEquals(Acceptance.Spent, client.acceptInvitation(TOKEN))
        }

    // The other direction, and the expensive one: measured in the 3.7.0 artifact, every refusal
    // arrives as one exception type, so reading the type told a patient holding a live invitation
    // that it was used up and sent them back to the clinic for one they did not need. A refusal
    // with no code at all is in the list because it is what an undecodable body becomes.
    @Test
    fun onlyASpentLinkReadsAsSpent() =
        runTest {
            val others =
                mapOf(
                    "a rate limit" to jsonError(HttpStatusCode.TooManyRequests, "over_request_rate_limit"),
                    "a restarting server" to jsonError(HttpStatusCode.InternalServerError, "unexpected_failure"),
                    "another refusal" to jsonError(HttpStatusCode.Forbidden, "user_banned"),
                    "a body that will not decode" to MockEngine { respondError(HttpStatusCode.Forbidden) },
                )

            for ((what, engine) in others) {
                val client = clientAnswering(engine)

                assertEquals(Acceptance.Unreachable, client.acceptInvitation(TOKEN), "on $what")
            }
        }

    @Test
    fun anUnreachableServerIsNotARefusal() =
        runTest {
            val client = clientAnswering(MockEngine { throw IOException("no route") })

            assertEquals(Acceptance.Unreachable, client.acceptInvitation(TOKEN))
        }
}
