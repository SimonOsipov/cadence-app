package app.cadence.shared.storage

import com.russhwolf.settings.MapSettings
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class FreshInstallGuardTest {
    // The case the guard exists for: Apple's keychain outlives the app, so the next
    // installation — a different person on a shared device, or the same one after a restore
    // — would be handed a session it never signed into.
    @Test
    fun aStoreOutlivingItsInstallationIsWiped() {
        val persistent = MapSettings("refresh_token" to "rt-of-whoever-had-this-phone")
        val volatileStore = MapSettings()

        guardFreshInstall(persistent, volatileStore)

        assertNull(persistent.getStringOrNull("refresh_token"))
        assertTrue(volatileStore.getBoolean(INSTALL_MARKER_KEY, false))
    }

    @Test
    fun anOrdinaryLaunchKeepsTheSession() {
        val persistent = MapSettings("refresh_token" to "rt-1")
        val volatileStore = MapSettings(INSTALL_MARKER_KEY to true)

        guardFreshInstall(persistent, volatileStore)

        assertEquals("rt-1", persistent.getStringOrNull("refresh_token"))
    }

    // Idempotent, because it runs on every launch and the second run must not be a sign-out:
    // the marker the first run wrote is what the second one reads.
    @Test
    fun runningTwiceSignsNobodyOut() {
        val persistent = MapSettings()
        val volatileStore = MapSettings()
        guardFreshInstall(persistent, volatileStore)
        persistent.putString("refresh_token", "rt-after-sign-in")

        guardFreshInstall(persistent, volatileStore)

        assertEquals("rt-after-sign-in", persistent.getStringOrNull("refresh_token"))
    }
}
