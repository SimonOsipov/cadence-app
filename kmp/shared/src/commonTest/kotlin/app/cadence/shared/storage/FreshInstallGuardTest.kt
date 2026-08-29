package app.cadence.shared.storage

import com.russhwolf.settings.MapSettings
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class FreshInstallGuardTest {
    // The case the guard exists for: Apple's keychain outlives the app, so the next
    // installation — a different person on a shared device, or the same one after a restore
    // — would be handed a session it never signed into.
    @Test
    fun aStoreOutlivingItsInstallationIsWiped() {
        val vault = storeHolding("rt-of-whoever-had-this-phone")
        val volatileStore = MapSettings()

        guardFreshInstall(VaultSettings(vault), volatileStore)

        assertNull(VaultSettings(vault).getStringOrNull("refresh_token"))
        assertTrue(volatileStore.getBoolean(INSTALL_MARKER_KEY, false))
    }

    @Test
    fun anOrdinaryLaunchKeepsTheSession() {
        val vault = storeHolding("rt-1")
        val volatileStore = MapSettings(INSTALL_MARKER_KEY to true)

        guardFreshInstall(VaultSettings(vault), volatileStore)

        assertEquals("rt-1", VaultSettings(vault).getStringOrNull("refresh_token"))
    }

    // Idempotent, because it runs on every launch and the second run must not be a sign-out:
    // the marker the first run wrote is what the second one reads.
    @Test
    fun runningTwiceSignsNobodyOut() {
        val vault = FakeVault()
        val volatileStore = MapSettings()
        guardFreshInstall(VaultSettings(vault), volatileStore)
        VaultSettings(vault).putString("refresh_token", "rt-after-sign-in")

        guardFreshInstall(VaultSettings(vault), volatileStore)

        assertEquals("rt-after-sign-in", VaultSettings(vault).getStringOrNull("refresh_token"))
    }

    // One marker per store: guarded lazily on first use, a single marker would be put down by
    // whichever secret was asked for first and would retire the guard before the second was
    // touched at all — leaving the verifier alive across a reinstall.
    @Test
    fun eachStoreIsGuardedOnItsOwnMarker() {
        val session = storeHolding("rt-of-whoever-had-this-phone")
        val verifier = storeHolding("verifier-of-whoever-had-this-phone")
        val volatileStore = MapSettings()

        guardFreshInstall(VaultSettings(session), volatileStore, "$INSTALL_MARKER_KEY.session")
        guardFreshInstall(VaultSettings(verifier), volatileStore, "$INSTALL_MARKER_KEY.pkce")

        assertNull(VaultSettings(session).getStringOrNull("refresh_token"))
        assertNull(VaultSettings(verifier).getStringOrNull("refresh_token"))
    }

    // A wipe that did not happen must not retire the guard: marked anyway, it never runs
    // again for this installation, and the store it was meant to clear is inherited for good
    // — the single outcome this function exists to prevent, reached by declaring success.
    @Test
    fun aWipeThatFailedLeavesTheGuardArmedForTheNextLaunch() {
        val vault = storeHolding("rt-of-whoever-had-this-phone").also { it.bytes }
        val stubborn = FakeVault(bytes = vault.bytes, wipes = false)
        val volatileStore = MapSettings()

        guardFreshInstall(VaultSettings(stubborn), volatileStore)

        assertFalse(volatileStore.getBoolean(INSTALL_MARKER_KEY, false))
    }
}
