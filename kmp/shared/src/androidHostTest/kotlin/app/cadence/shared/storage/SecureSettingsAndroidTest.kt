package app.cadence.shared.storage

import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import kotlin.test.AfterTest
import kotlin.test.assertContains
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

/**
 * The seam wired end to end, which nothing below it can show: that [installSecureStorage]
 * reaches [secureSettings], and that forgetting it says so.
 *
 * A write is not asserted here and cannot be: the default key comes from the Keystore, which
 * this runtime has none of — see [AndroidKeyStoreReachabilityTest]. What the vault does with
 * a key it has is [AndroidVaultTest]'s.
 */
@RunWith(RobolectricTestRunner::class)
class SecureSettingsAndroidTest {
    @AfterTest
    fun forget() {
        storageRoot = null
    }

    // A missing install is a programmer error on the launch path, so it is loud rather than
    // an empty store: an empty store reads as «signed out», and a patient holding a valid
    // session would be sent back to the sign-in screen on every launch instead.
    @Test
    fun askingBeforeTheApplicationInstalledItFailsByName() {
        storageRoot = null

        val refusal = assertFailsWith<IllegalStateException> { secureSettings() }

        assertContains(refusal.message.orEmpty(), "installSecureStorage")
    }

    // And with it installed the same call answers a store — empty, because a fresh
    // installation has written nothing, and reachable, because the file is not there to read.
    @Test
    fun installedItAnswersAnEmptyStoreRatherThanRefusing() {
        installSecureStorage(RuntimeEnvironment.getApplication())

        assertEquals(0, secureSettings().size)
    }
}
