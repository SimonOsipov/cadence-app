package app.cadence.shared.storage

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** A vault holding bytes in memory, standing in for Keychain and Keystore alike. */
private class FakeVault(
    var bytes: ByteArray? = null,
    private val readFails: Boolean = false,
) : Vault {
    override fun read(): ByteArray? = if (readFails) error("unreadable") else bytes

    override fun write(bytes: ByteArray) {
        this.bytes = bytes
    }

    override fun wipe() {
        bytes = null
    }
}

class VaultSettingsTest {
    @Test
    fun aValueSurvivesANewInstanceOverTheSameVault() {
        val vault = FakeVault()
        VaultSettings(vault).putString("refresh_token", "rt-1")

        assertEquals("rt-1", VaultSettings(vault).getStringOrNull("refresh_token"))
    }

    // The whole reason this type exists rather than a plain map: the session is what is
    // stored, and a blob no key here can open must read as «not signed in» rather than take
    // the app down on launch.
    @Test
    fun unreadableStorageIsNoSessionRatherThanACrash() {
        val settings = VaultSettings(FakeVault(readFails = true))

        assertNull(settings.getStringOrNull("refresh_token"))
        assertEquals(0, settings.size)
    }

    @Test
    fun bytesThatAreNotAStoreAreAlsoNoSession() {
        val settings = VaultSettings(FakeVault(bytes = byteArrayOf(7, 7, 7)))

        assertNull(settings.getStringOrNull("refresh_token"))
    }

    // Every type supabase-kt's SettingsSessionManager reaches for, because a store that
    // round-trips strings and loses the expiry is a session that never looks expired.
    @Test
    fun everyTypeTheSessionManagerStoresSurvivesTheRoundTrip() {
        val vault = FakeVault()
        val written = VaultSettings(vault)
        written.putString("s", "с кириллицей")
        written.putLong("expires_at", 1_924_000_000L)
        written.putBoolean("b", true)

        val read = VaultSettings(vault)
        assertEquals("с кириллицей", read.getStringOrNull("s"))
        assertEquals(1_924_000_000L, read.getLongOrNull("expires_at"))
        assertEquals(true, read.getBooleanOrNull("b"))
    }

    // A value carrying whatever the blob is framed with. Without this case the framing can
    // be a character a token is allowed to contain, and one such token empties the store.
    @Test
    fun aValueCarryingTheFramingIsStoredWhole() {
        val awkward = "abc:12:d"
        val vault = FakeVault()
        VaultSettings(vault).putString("token", awkward)

        assertEquals(awkward, VaultSettings(vault).getStringOrNull("token"))
    }

    @Test
    fun removingAndClearingReachTheVault() {
        val vault = FakeVault()
        val settings = VaultSettings(vault)
        settings.putString("a", "1")
        settings.putString("b", "2")

        settings.remove("a")
        assertFalse(settings.hasKey("a"))
        assertTrue(settings.hasKey("b"))

        settings.clear()
        assertEquals(0, VaultSettings(vault).size)
    }
}
