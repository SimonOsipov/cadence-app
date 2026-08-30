package app.cadence.shared.auth

import com.russhwolf.settings.MapSettings
import kotlinx.coroutines.test.runTest
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import kotlin.test.assertNull

private const val GOTRUE = "http://localhost:9999"

/**
 * What the transport is handed before anybody signs in.
 *
 * Under Robolectric rather than in commonTest, and measured: installing Auth starts its own
 * auto-refresh on the main dispatcher, and a plain host test answers «Method getMainLooper in
 * android.os.Looper not mocked». Switching auto-refresh off for the test was the alternative and
 * is worse — it is the whole reason this module owns the token, so a test without it measures
 * another client. **The Apple side of this has no equivalent**: the iOS test target runs no such
 * runtime, and what stands there is step 3's XCTest work.
 */
@RunWith(RobolectricTestRunner::class)
class CadenceAuthAndroidTest {
    // The contract the transport was written against, and the one the module does not keep:
    // refreshCurrentSession throws rather than answering. Left to propagate, the first request
    // of a signed-out app throws instead of routing to sign-in, and «signed out» stops being a
    // state the screen can reach — every refusal arrives as «the server is unavailable».
    @Test
    fun refreshingWithNoSessionAnswersNothingRatherThanThrowing() =
        runTest {
            val tokens = cadenceAuth(url = GOTRUE) { MapSettings() }.sessionTokens()

            assertNull(tokens.refreshed())
        }

    // «No session», not an empty one that reads as signed in: the transport asks this before
    // every request, and an empty answer would send one with no token and read the 401 as an
    // expiry — refreshing a session that never existed.
    @Test
    fun aFreshInstallHandsTheTransportNothing() =
        runTest {
            val tokens = cadenceAuth(url = GOTRUE) { MapSettings() }.sessionTokens()

            assertNull(tokens.current())
        }
}
