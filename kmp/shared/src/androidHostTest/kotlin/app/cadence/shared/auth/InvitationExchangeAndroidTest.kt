package app.cadence.shared.auth

import com.russhwolf.settings.MapSettings
import io.github.jan.supabase.auth.auth
import io.github.jan.supabase.auth.exception.AuthErrorCode
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
// so a fixture built from `respondError` alone certifies an answer the server never gives. The
// body's own `code` mirrors the status on the live server, so it is derived rather than spelled —
// a fixture pairing 403 with a 429 is another shape nobody sends.
private fun aRefusal(
    status: HttpStatusCode,
    code: String?,
) = if (code == null) {
    """{"code":${status.value},"msg":"a refusal"}"""
} else {
    """{"code":${status.value},"error_code":"$code","msg":"a refusal"}"""
}

private fun jsonError(
    status: HttpStatusCode,
    code: String? = null,
) = MockEngine {
    respond(aRefusal(status, code), status, headersOf("Content-Type", "application/json"))
}

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

            assertEquals(Acceptance.Refused(AuthErrorCode.OtpExpired), client.acceptInvitation(TOKEN))
        }

    // What the patient is offered another try at, and nothing else: the expensive mistake in this
    // direction is telling somebody holding a live invitation that it is used up.
    @Test
    fun aServerThatWillAnswerLaterIsWorthAnotherTry() =
        runTest {
            val later =
                mapOf(
                    "a rate limit" to jsonError(HttpStatusCode.TooManyRequests, "over_request_rate_limit"),
                    "a restarting server" to jsonError(HttpStatusCode.InternalServerError, "unexpected_failure"),
                    "a gateway with nothing behind it" to jsonError(HttpStatusCode.BadGateway),
                    "a timeout" to jsonError(HttpStatusCode.RequestTimeout),
                    "early data refused" to jsonError(HttpStatusCode.TooEarly),
                )

            for ((what, engine) in later) {
                val client = clientAnswering(engine)

                assertEquals(Acceptance.Unreachable, client.acceptInvitation(TOKEN), "on $what")
            }
        }

    // The mistake in the other direction, and it has a live path: a banned patient asking again
    // for ever. The reason travels so step 3 can write two sentences rather than one.
    //
    // The last two rows are the two unnameable shapes, and they arrive by different roads —
    // measured, and the comment here was wrong about it once. A JSON refusal naming no code is not
    // an AuthRestException at all, which is the arm the `as?` exists for; a body that will not
    // decode **is** one, carrying the vendor's literal «Unknown error», which maps to no code.
    // Named rather than keyed on the expectation, so a regression in one is not read as the other.
    @Test
    fun aRefusalCarriesTheReasonItGave() =
        runTest {
            val refusals =
                listOf(
                    Triple("banned", jsonError(HttpStatusCode.Forbidden, "user_banned"), AuthErrorCode.UserBanned),
                    Triple("spent", jsonError(HttpStatusCode.Forbidden, "otp_expired"), AuthErrorCode.OtpExpired),
                    Triple("naming no code", jsonError(HttpStatusCode.BadRequest), null),
                    Triple("not decodable", MockEngine { respondError(HttpStatusCode.Forbidden) }, null),
                )

            for ((what, engine, code) in refusals) {
                val client = clientAnswering(engine)

                assertEquals(Acceptance.Refused(code), client.acceptInvitation(TOKEN), "on $what")
            }
        }

    @Test
    fun anUnreachableServerIsNotARefusal() =
        runTest {
            val client = clientAnswering(MockEngine { throw IOException("no route") })

            assertEquals(Acceptance.Unreachable, client.acceptInvitation(TOKEN))
        }
}
