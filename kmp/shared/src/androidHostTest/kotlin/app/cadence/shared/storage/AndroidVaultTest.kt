package app.cadence.shared.storage

import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import java.io.File
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import kotlin.test.assertContentEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertNull

private const val AES_BITS = 256

// Fewer bytes than the IV every stored blob begins with.
private const val TOO_SHORT_FOR_AN_IV = 3

/**
 * The key is a plain AES one rather than the Keystore's, and that is what
 * [AndroidKeyStoreReachabilityTest] measured the need for: Robolectric ships no
 * AndroidKeyStore provider, so injecting the key is what leaves every decision this file
 * makes — the format, where the IV sits, what a tampered file answers — measurable here.
 * What stays unmeasured is the key acquisition itself, which is platform API and not ours.
 */
@RunWith(RobolectricTestRunner::class)
class AndroidVaultTest {
    private val key: SecretKey = KeyGenerator.getInstance("AES").apply { init(AES_BITS) }.generateKey()

    private fun aFile(): File = File.createTempFile("vault", ".bin").apply { delete() }

    private fun vaultOver(file: File) = AndroidVault(file) { key }

    @Test
    fun whatIsWrittenComesBack() {
        val file = aFile()
        vaultOver(file).write("rt-1".encodeToByteArray())

        assertContentEquals("rt-1".encodeToByteArray(), vaultOver(file).read())
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
    fun nothingWrittenReadsAsNothing() {
        assertNull(vaultOver(aFile()).read())
    }

    @Test
    fun aTamperedFileReadsAsNothingRatherThanThrowing() {
        val file = aFile()
        vaultOver(file).write("rt-1".encodeToByteArray())
        val bytes = file.readBytes()
        bytes[bytes.size - 1] = (bytes[bytes.size - 1] + 1).toByte()
        file.writeBytes(bytes)

        assertNull(vaultOver(file).read())
    }

    // A file too short to hold an IV is not a truncated store, it is not a store at all —
    // and the difference matters because reading it as one would hand the cipher garbage.
    @Test
    fun aFileShorterThanItsIvReadsAsNothing() {
        val file = aFile()
        file.writeBytes(ByteArray(TOO_SHORT_FOR_AN_IV))

        assertNull(vaultOver(file).read())
    }

    @Test
    fun wipingLeavesNothingBehind() {
        val file = aFile()
        val vault = vaultOver(file)
        vault.write("rt-1".encodeToByteArray())

        vault.wipe()

        assertNull(vault.read())
    }
}
