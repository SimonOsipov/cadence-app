package app.cadence.shared.storage

import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import java.io.File
import java.security.ProviderException
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import kotlin.test.assertContentEquals
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

private const val AES_BITS = 256

// Fewer bytes than the IV every stored blob begins with.
private const val TOO_SHORT_FOR_AN_IV = 3

/**
 * The key is a plain AES one rather than the Keystore's, and that is what
 * [AndroidKeyStoreReachabilityTest] measured the need for: Robolectric ships no
 * AndroidKeyStore provider, so injecting the key is what leaves every decision this file
 * makes — the format, where the IV sits, what a tampered file answers — measurable here.
 * What stays unmeasured is the key acquisition itself, which is platform API and not ours.
 *
 * The function count detekt objects to is the number of ways bytes at rest go wrong, and each
 * one of them is a case below.
 */
@Suppress("TooManyFunctions")
@RunWith(RobolectricTestRunner::class)
class AndroidVaultTest {
    private val key: SecretKey = KeyGenerator.getInstance("AES").apply { init(AES_BITS) }.generateKey()

    private fun aFile(): File = File.createTempFile("vault", ".bin").apply { delete() }

    private fun vaultOver(file: File) = AndroidVault(file) { key }

    @Test
    fun whatIsWrittenComesBack() {
        val file = aFile()
        vaultOver(file).write("rt-1".encodeToByteArray())

        assertContentEquals("rt-1".encodeToByteArray(), vaultOver(file).presentBytes())
    }

    // The point of the whole class: the bytes on disk are not the bytes handed in. Without
    // this the file could be a plaintext store and every other test here would still pass.
    @Test
    fun whatIsOnDiskIsNotWhatWasHandedIn() {
        val file = aFile()
        val secret = "refresh-token-in-the-clear".encodeToByteArray()

        vaultOver(file).write(secret)

        assertNotEquals(secret.toList(), file.readBytes().toList())
    }

    // Two writes of one value under one key must not produce one ciphertext: GCM under a
    // reused IV is a key recovery, and letting the provider choose the IV is what prevents
    // it. Equal blobs here would mean the IV is fixed somewhere.
    @Test
    fun theSameValueWrittenTwiceIsNotTheSameBytes() {
        val first = aFile()
        val second = aFile()
        vaultOver(first).write("rt-1".encodeToByteArray())
        vaultOver(second).write("rt-1".encodeToByteArray())

        assertNotEquals(first.readBytes().toList(), second.readBytes().toList())
    }

    @Test
    fun nothingWrittenIsAbsentRatherThanUnavailable() {
        assertIs<Stored.Absent>(vaultOver(aFile()).read())
    }

    // Present and empty rather than Unavailable, and the difference decides whether the
    // installation ever writes again: bytes we hold the key for and cannot open are our own
    // corruption, and corruption is replaceable. Read as «cannot read now», one kill
    // mid-write would leave the store unwritable for good.
    @Test
    fun aTamperedFileIsCorruptAndReplaceable() {
        val file = aFile()
        vaultOver(file).write("rt-1".encodeToByteArray())
        val bytes = file.readBytes()
        bytes[bytes.size - 1] = (bytes[bytes.size - 1] + 1).toByte()
        file.writeBytes(bytes)

        // Empty, not merely Present: handing the unopenable ciphertext back as content would
        // satisfy an `assertIs` and put bytes no key produced into the store.
        assertContentEquals(ByteArray(0), (vaultOver(file).read() as Stored.Present).bytes)
    }

    // A half-written file is the same case: replaceable, because writeBytes truncates before
    // it writes and a kill in that window is how one is produced.
    @Test
    fun aFileShorterThanItsIvIsCorruptAndReplaceable() {
        val file = aFile()
        file.writeBytes(ByteArray(TOO_SHORT_FOR_AN_IV))

        assertIs<Stored.Present>(vaultOver(file).read())
        assertTrue(VaultSettings(vaultOver(file)).isWritable)
    }

    // A key this vault cannot get is «cannot read now», never «nothing stored» — a device
    // whose keystore is broken would otherwise look to every caller like a patient who has
    // not signed in, and the next write would take the session away for real.
    @Test
    fun aKeyThatCannotBeProducedIsUnavailableAndRefusesToWrite() {
        val file = aFile()
        vaultOver(file).write("rt-1".encodeToByteArray())
        val broken = AndroidVault(file) { throw ProviderException("this device has no keystore") }

        assertIs<Stored.Unavailable>(broken.read())
        assertFalse(broken.write("rt-2".encodeToByteArray()))
        assertContentEquals("rt-1".encodeToByteArray(), vaultOver(file).presentBytes())
    }

    @Test
    fun wipingLeavesNothingBehindAndSaysSo() {
        val file = aFile()
        val vault = vaultOver(file)
        vault.write("rt-1".encodeToByteArray())

        assertTrue(vault.wipe())
        assertIs<Stored.Absent>(vault.read())
    }

    // Nothing to delete is a wipe that happened: sign-out on a device that never stored a
    // session must not report failure, or the guard above it would refuse to arm.
    @Test
    fun wipingWhatWasNeverThereSucceeds() {
        assertTrue(vaultOver(aFile()).wipe())
    }

    private fun AndroidVault.presentBytes(): ByteArray? = (read() as? Stored.Present)?.bytes
}
