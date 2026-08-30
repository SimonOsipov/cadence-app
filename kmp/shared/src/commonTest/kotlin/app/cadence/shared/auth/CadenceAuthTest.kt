package app.cadence.shared.auth

import app.cadence.shared.storage.PKCE_STORE
import app.cadence.shared.storage.SESSION_STORE
import com.russhwolf.settings.MapSettings
import com.russhwolf.settings.Settings
import io.github.jan.supabase.auth.SettingsCodeVerifierCache
import io.github.jan.supabase.auth.SettingsSessionManager
import io.github.jan.supabase.auth.auth
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertTrue

private const val GOTRUE = "http://localhost:9999"

private class RecordingStores {
    val handed = mutableMapOf<String, Settings>()

    fun named(name: String): Settings = handed.getOrPut(name) { MapSettings() }
}

class CadenceAuthTest {
    // Nothing else in the tree would notice if the stock managers were left as they came.
    @Test
    fun bothStoresAreSubstituted() {
        val stores = RecordingStores()

        val auth = cadenceAuth(url = GOTRUE, stores = stores::named).auth

        assertIs<SettingsSessionManager>(auth.config.sessionManager)
        assertIs<SettingsCodeVerifierCache>(auth.config.codeVerifierCache)
    }

    @Test
    fun theSessionAndTheVerifierAreHeldApart() {
        val stores = RecordingStores()

        cadenceAuth(url = GOTRUE, stores = stores::named)

        assertEquals(setOf(SESSION_STORE, PKCE_STORE), stores.handed.keys)
        assertTrue(stores.handed[SESSION_STORE] !== stores.handed[PKCE_STORE])
    }

    // The address is ours, not Supabase's — see cadenceAuth for the measurement.
    @Test
    fun theAuthModuleTalksToOurGoTrueRoot() {
        val auth = cadenceAuth(url = GOTRUE, stores = RecordingStores()::named).auth

        assertEquals(GOTRUE, auth.config.customUrl)
        assertEquals("$GOTRUE/token", auth.resolveUrl("token"))
    }
}
