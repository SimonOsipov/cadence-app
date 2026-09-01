package app.cadence.shared.auth

import com.russhwolf.settings.MapSettings
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.client.engine.mock.respondError
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import io.ktor.utils.io.errors.IOException
import kotlinx.coroutines.test.runTest
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import kotlin.test.assertEquals

private const val GOTRUE = "http://localhost:9999"

private const val TOKEN = "e75b4d4f54a86c915c0afdfc5db3b5cb6eea78ba43c1ccf6bd24c5cb"

// Cut to what the vendor reads out of GoTrue's answer.
private const val A_SESSION = """
    {"access_token":"an-access-token","token_type":"bearer","expires_in":3600,
     "refresh_token":"a-refresh-token"}
"""

private fun clientAnswering(engine: MockEngine) = cadenceAuth(url = GOTRUE, stores = { MapSettings() }, engine = engine)

/**
 * The exchange, through a whole client rather than around it.
 *
 * Under Robolectric and on one platform, for the reason [CadenceAuthAndroidTest] states and one
 * more measured here: installing Auth registers a `ProcessLifecycleOwner` observer, and a plain
 * host test answers «Method getMainLooper in android.os.Looper not mocked». Testing the outcome
 * mapping on its own would leave the call site — which failure reaches which arm — unmeasured,
 * which is the trade this file refuses. What the real GoTrue answers is pinned in the **api**
 * harness; these are the arms, not the answers.
 */
@RunWith(RobolectricTestRunner::class)
class InvitationExchangeAndroidTest {
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
        }

    // A link opened twice is the ordinary case — the email read on two devices — and it must not
    // read as «the server is unavailable»: one is explained, the other is offered another try.
    @Test
    fun aSpentTokenIsRefusedRatherThanRetried() =
        runTest {
            val client = clientAnswering(MockEngine { respondError(HttpStatusCode.Forbidden) })

            assertEquals(Acceptance.Spent, client.acceptInvitation(TOKEN))
        }

    @Test
    fun anUnreachableServerIsNotARefusal() =
        runTest {
            val client = clientAnswering(MockEngine { throw IOException("no route") })

            assertEquals(Acceptance.Unreachable, client.acceptInvitation(TOKEN))
        }
}
