package app.cadence.shared.storage

import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import java.security.KeyStore
import java.security.KeyStoreException
import kotlin.test.assertContains
import kotlin.test.assertFailsWith

@RunWith(RobolectricTestRunner::class)
class AndroidKeyStoreReachabilityTest {
    // Measured 2026-08-29, and it is why AndroidVault takes its key as a parameter:
    // Robolectric ships no AndroidKeyStore provider, so `KeyStore.getInstance` answers
    // «AndroidKeyStore not found» here while working on any device. Asserted as the
    // refusal rather than deleted, because the day Robolectric grows the provider this
    // goes red and the seam beside it can be simplified.
    //
    // The alternative — androidDeviceTest with an emulator — was refused: this gate
    // promises to run on any machine, Linux included, and an emulator withdraws that.
    @Test
    fun theAndroidKeyStoreProviderIsAbsentFromThisRuntime() {
        val refusal =
            assertFailsWith<KeyStoreException> {
                KeyStore.getInstance("AndroidKeyStore")
            }

        assertContains(refusal.message.orEmpty(), "AndroidKeyStore")
    }
}
