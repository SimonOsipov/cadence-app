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
    // The condition of taking supabase-kt at all: its stock managers keep the session and the
    // PKCE verifier in plaintext on both platforms — SharedPreferences and NSUserDefaults — so
    // both are handed the secure store instead. Substituting them is what the choice was made
    // conditional on, and nothing else in the tree would notice if they were left as they came.
    @Test
    fun bothStoresAreSubstituted() {
        val stores = RecordingStores()

        val auth = cadenceAuth(url = GOTRUE, stores = stores::named).auth

        assertIs<SettingsSessionManager>(auth.config.sessionManager)
        assertIs<SettingsCodeVerifierCache>(auth.config.codeVerifierCache)
    }

    // Two stores and not one: the blob is written whole, so a shared store would have the
    // session's next write drop the verifier out of it — silently, in the middle of accepting
    // an invite, which is the one moment both are in use at once.
    @Test
    fun theSessionAndTheVerifierAreHeldApart() {
        val stores = RecordingStores()

        cadenceAuth(url = GOTRUE, stores = stores::named)

        assertEquals(setOf(SESSION_STORE, PKCE_STORE), stores.handed.keys)
        assertTrue(stores.handed[SESSION_STORE] !== stores.handed[PKCE_STORE])
    }

    // The address is ours, not Supabase's. Measured in the 3.7.0 artifact: MainPlugin.resolveUrl
    // appends `auth/v1` only where customUrl is null, and our GoTrue answers on its own root —
    // 404 on every /auth/v1 path, measured against the local contour.
    @Test
    fun theAuthModuleTalksToOurGoTrueRoot() {
        val auth = cadenceAuth(url = GOTRUE, stores = RecordingStores()::named).auth

        assertEquals(GOTRUE, auth.config.customUrl)
        assertEquals("$GOTRUE/token", auth.resolveUrl("token"))
    }
}
