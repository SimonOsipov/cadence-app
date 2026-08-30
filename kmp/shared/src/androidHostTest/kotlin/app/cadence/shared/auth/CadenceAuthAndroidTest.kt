package app.cadence.shared.auth

import app.cadence.shared.storage.SESSION_STORE
import app.cadence.shared.storage.installSecureStorage
import app.cadence.shared.storage.secureSettings
import com.russhwolf.settings.MapSettings
import io.github.jan.supabase.auth.auth
import io.github.jan.supabase.auth.user.UserSession
import kotlinx.coroutines.test.runTest
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

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
    // Every other test hands cadenceAuth a MapSettings, so ::secureSettings — the production
    // default — was exercised by nothing on either platform. Asked through the seam rather than
    // by inspection: a default pointed anywhere else, the vendor's plaintext SharedPreferences
    // among them, leaves that store empty while every other assertion in this file passes.
    @Test
    fun theProductionDefaultIsTheSecureStore() =
        runTest {
            installSecureStorage(RuntimeEnvironment.getApplication())
            val manager = assertNotNull(cadenceAuth(url = GOTRUE).auth.config.sessionManager)

            manager.saveSession(
                UserSession(
                    accessToken = "an-access-token",
                    refreshToken = "a-refresh-token",
                    expiresIn = 3600,
                    tokenType = "bearer",
                ),
            )

            // Read back through secureSettings, which is the claim: the manager and the store
            // under the session's name are one. What this does **not** show is the bytes
            // crossing the vault — measured, Robolectric has no AndroidKeyStore, so the key is
            // never issued, `write` refuses, and VaultSettings keeps the value in memory with
            // `writable = false`. Asserting a file here fails on a correct implementation. The
            // crossing is AndroidVaultTest's, which supplies its own key through the vault seam.
            assertTrue(
                secureSettings(SESSION_STORE).keys.isNotEmpty(),
                "the manager and the session store are not the same store",
            )
            assertNotNull(manager.loadSession())
        }

    // The contract the transport was written against, and the one the module does not keep:
    // refreshCurrentSession throws rather than answering. Left to propagate, the first request
    // of a signed-out app throws instead of routing to sign-in, and «signed out» stops being a
    // state the screen can reach — every refusal arrives as «the server is unavailable».
    @Test
    fun refreshingWithNoSessionAnswersNothingRatherThanThrowing() =
        runTest {
            val tokens = cadenceAuth(url = GOTRUE, stores = { MapSettings() }).sessionTokens()

            assertNull(tokens.refreshed())
        }

    // «No session», not an empty one that reads as signed in: the transport asks this before
    // every request, and an empty answer would send one with no token and read the 401 as an
    // expiry — refreshing a session that never existed.
    @Test
    fun aFreshInstallHandsTheTransportNothing() =
        runTest {
            val tokens = cadenceAuth(url = GOTRUE, stores = { MapSettings() }).sessionTokens()

            assertNull(tokens.current())
        }
}
