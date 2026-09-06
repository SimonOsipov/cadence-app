package app.cadence.shared.auth

import com.russhwolf.settings.MapSettings
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
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

private const val GOTRUE = "http://localhost:9999"

private const val A_PATIENT = "patient@clinic.example"

private const val A_STRANGER = "nobody@elsewhere.example"

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

private fun accepted() =
    MockEngine {
        respond("{}", HttpStatusCode.OK, headersOf("Content-Type", "application/json"))
    }

private fun clientAnswering(engine: MockEngine) = cadenceAuth(url = GOTRUE, stores = { MapSettings() }, engine = engine)

private fun HttpRequestData.sentBody(): String = (body as TextContent).text

/**
 * Asking for a recovery mail, through a whole client rather than around it — the reasons are
 * [InvitationExchangeAndroidTest]'s, and so is the Robolectric runtime.
 */
@RunWith(RobolectricTestRunner::class)
class RecoveryAndroidTest {
    // The request, because everything below is green whatever it says: an engine answering without
    // looking takes the happy arm for a magic link or a sign-up just as readily.
    @Test
    fun theAddressIsSentToTheRecoverRoute() =
        runTest {
            val engine = accepted()

            clientAnswering(engine).recover(A_PATIENT)

            val asked = engine.requestHistory.single()

            assertEquals("/recover", asked.url.encodedPath)
            assertContains(asked.sentBody(), """"email":"$A_PATIENT"""")
        }

    // The whole security property of this screen. A patient's address and a stranger's have to come
    // back as one sentence, or the form answers «is this person a patient here?» to anyone who
    // types an address. Compared to each other rather than each to a constant: an answer that later
    // grows a reason fails here.
    @Test
    fun aKnownAndAnUnknownAddressAnswerAlike() =
        runTest {
            val known = clientAnswering(accepted())
            // What the provider answers for an address it has never seen is its business and can
            // change; what cannot change is that this screen says the same thing either way.
            val unknown = clientAnswering(jsonError(HttpStatusCode.UnprocessableEntity, "validation_failed"))

            assertEquals(
                known.recover(A_PATIENT),
                unknown.recover(A_STRANGER),
                "the recovery form tells a patient's address apart from a stranger's",
            )
            assertEquals(Recovery.Sent, known.recover(A_PATIENT))
        }

    // The second half of the same property, and the one a first-ask test cannot reach: the
    // per-address gap is enforced against a row only a real patient has, so a sentence of its own
    // here would be one only a real patient could provoke. Two asks and sixty seconds were the
    // oracle; folded into «sent», they are not.
    @Test
    fun askingAgainTooSoonIsAnsweredLikeAStrangersAddress() =
        runTest {
            val patient = clientAnswering(jsonError(HttpStatusCode.TooManyRequests, "over_email_send_rate_limit"))
            val stranger = clientAnswering(jsonError(HttpStatusCode.UnprocessableEntity, "validation_failed"))

            assertEquals(
                stranger.recover(A_STRANGER),
                patient.recover(A_PATIENT),
                "the second ask inside the gap tells a patient's address apart from a stranger's",
            )
        }

    @Test
    fun aServerThatWillAnswerLaterIsWorthAnotherTry() =
        runTest {
            val later =
                mapOf(
                    "a restarting server" to jsonError(HttpStatusCode.InternalServerError, "unexpected_failure"),
                    "a gateway with nothing behind it" to jsonError(HttpStatusCode.BadGateway),
                    "a timeout" to jsonError(HttpStatusCode.RequestTimeout),
                    "no connection at all" to MockEngine { throw IOException("no route to host") },
                )

            for ((what, engine) in later) {
                assertEquals(Recovery.Unreachable, clientAnswering(engine).recover(A_PATIENT), "on $what")
            }
        }
}
