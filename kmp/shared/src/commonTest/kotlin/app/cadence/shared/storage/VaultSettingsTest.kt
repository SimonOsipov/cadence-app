package app.cadence.shared.storage

import kotlin.test.Test
import kotlin.test.assertContentEquals
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** A vault holding bytes in memory, standing in for Keychain and Keystore alike. */
class VaultSettingsTest {
    @Test
    fun aValueSurvivesANewInstanceOverTheSameVault() {
        val vault = FakeVault()
        VaultSettings(vault).putString("refresh_token", "rt-1")

        assertEquals("rt-1", VaultSettings(vault).getStringOrNull("refresh_token"))
    }

    // The whole reason this type exists rather than a plain map: the session is what is
    // stored, and a store no key here can open must read as «not signed in» rather than take
    // the app down on launch.
    @Test
    fun unreadableStorageIsNoSessionRatherThanACrash() {
        val settings = VaultSettings(FakeVault(unavailable = true))

        assertNull(settings.getStringOrNull("refresh_token"))
        assertEquals(0, settings.size)
    }

    // And the other half of that sentence, which the assertion above cannot make: reading
    // nothing must not become writing nothing. The blob goes back whole, so one write after
    // one failed read would replace a live session with an empty store — «unreadable is no
    // session» is about reading, and this is what stops it becoming a claim about erasing.
    @Test
    fun anUnreadableStoreIsNotOverwrittenByTheNextWrite() {
        val session = "8:rt-token4:rt-1".encodeToByteArray()
        val vault = FakeVault(bytes = session, unavailable = true)
        val settings = VaultSettings(vault)

        settings.putString("refresh_token", "rt-2")

        assertContentEquals(session, vault.bytes)
        assertFalse(settings.isWritable)
    }

    // The running session still answers, so a device that cannot reach its store is usable
    // until it is restarted rather than unusable now.
    @Test
    fun anUnreadableStoreStillAnswersWithinTheSession() {
        val settings = VaultSettings(FakeVault(unavailable = true))

        settings.putString("refresh_token", "rt-2")

        assertEquals("rt-2", settings.getStringOrNull("refresh_token"))
    }

    @Test
    fun bytesThatAreNotAStoreAreAlsoNoSession() {
        val settings = VaultSettings(FakeVault(bytes = byteArrayOf(7, 7, 7)))

        assertNull(settings.getStringOrNull("refresh_token"))
    }

    // Corruption of our own format is ours to replace: bytes that reached us through the
    // platform's integrity check and still will not parse are a defect here, and keeping them
    // would leave the store unusable for the life of the installation.
    @Test
    fun aStoreWeCannotParseIsStillWritable() {
        val vault = FakeVault(bytes = byteArrayOf(7, 7, 7))
        val settings = VaultSettings(vault)

        settings.putString("refresh_token", "rt-1")

        assertTrue(settings.isWritable)
        assertEquals("rt-1", VaultSettings(vault).getStringOrNull("refresh_token"))
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

    // Read back through a new instance, because in memory it passes with flush() deleted from
    // remove() — and remove(key) is a sign-out path, so the token would stay in the keychain
    // while the app believed it was gone.
    @Test
    fun removingAndClearingReachTheVault() {
        val vault = FakeVault()
        val settings = VaultSettings(vault)
        settings.putString("a", "1")
        settings.putString("b", "2")

        settings.remove("a")

        val reread = VaultSettings(vault)
        assertFalse(reread.hasKey("a"))
        assertTrue(reread.hasKey("b"))

        settings.clear()
        assertEquals(0, VaultSettings(vault).size)
    }

    // The one branch throwOnInvalidSequence exists for. Bytes that passed the platform's
    // integrity check and are still not text are this module's own corruption, and the
    // previous fixture — 7, 7, 7 — is valid UTF-8 that never reached it.
    @Test
    fun bytesThatAreNotTextAreAlsoNoSession() {
        val settings = VaultSettings(FakeVault(bytes = byteArrayOf(0xC3.toByte())))

        assertEquals(0, settings.size)
    }

    // A vault that refuses the write must not leave the store looking written: the Apple one
    // deletes before it adds, so a refused add empties the keychain while this map still
    // answers with the value — and the caller reads back its own copy and believes it kept.
    @Test
    fun aRefusedWriteIsNoticedRatherThanDiscarded() {
        val settings = VaultSettings(FakeVault(writes = false))

        settings.putString("refresh_token", "rt-1")

        assertFalse(settings.isWritable)
    }

    // Sign-out reaches the store even where it cannot be rewritten. Doing nothing at rest is
    // the one outcome worse than erasing too much: the patient asked for the session to go,
    // and the next launch would read it back.
    @Test
    fun signingOutOfAnUnreadableStoreStillWipesIt() {
        val vault = FakeVault(bytes = "8:rt-token4:rt-1".encodeToByteArray(), unavailable = true)
        val settings = VaultSettings(vault)

        settings.remove("refresh_token")

        assertNull(vault.bytes)
    }

    @Test
    fun clearingAnUnreadableStoreStillWipesIt() {
        val vault = FakeVault(bytes = "8:rt-token4:rt-1".encodeToByteArray(), unavailable = true)

        VaultSettings(vault).clear()

        assertNull(vault.bytes)
    }

    // Sign-out has to be able to tell, which a Settings.clear() returning Unit cannot: a
    // wipe taken for done leaves the next person on the device holding this session.
    @Test
    fun clearingAnswersWhetherTheStoreIsActuallyGone() {
        val refuses = FakeVault(bytes = "x".encodeToByteArray(), wipes = false)

        assertFalse(VaultSettings(refuses).clearAndConfirm())
        assertTrue(VaultSettings(FakeVault()).clearAndConfirm())
    }
}
