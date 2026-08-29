package app.cadence.shared.storage

import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.io.File
import java.io.IOException
import java.security.GeneralSecurityException
import java.security.KeyStore
import java.security.ProviderException
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

private const val KEY_ALIAS = "app.cadence.vault"
private const val PROVIDER = "AndroidKeyStore"
private const val TRANSFORMATION = "AES/GCM/NoPadding"
private const val IV_BYTES = 12
private const val TAG_BITS = 128
private const val AES_BITS = 256

/**
 * AES/GCM over a file, with the key in the Keystore and never in this process.
 *
 * Ours rather than `androidx.security:security-crypto`, which is deprecated with no direct
 * replacement — depending on it would mean depending on something with no upgrade path, on
 * the authentication path of a medical app.
 *
 * The IV comes from the provider and is stored ahead of the ciphertext. GCM under a reused
 * IV and one key is a key recovery, and letting the cipher choose is what guarantees it is
 * not reused. The key is 256 bits explicitly, so the tests and the device agree on a number
 * rather than on the generator's default.
 *
 * [keys] is a parameter because AndroidKeyStore is absent from the test runtime — measured,
 * see AndroidKeyStoreReachabilityTest — so injecting it is what leaves everything decided
 * here measurable. Production callers take the default and never pass one.
 */
class AndroidVault(
    private val file: File,
    private val keys: () -> SecretKey = ::keystoreKey,
) : Vault {
    override fun read(): Stored {
        if (!file.exists()) return Stored.Absent
        val key = key() ?: return Stored.Unavailable("the keystore would not produce the key for $file")

        return stored()?.let { open(it, key) } ?: Stored.Unavailable("$file could not be read")
    }

    @Suppress("SwallowedException")
    private fun stored(): ByteArray? =
        try {
            file.readBytes()
        } catch (expected: IOException) {
            // The file is there and unreadable, which is «not now» rather than «not stored».
            null
        }

    /**
     * Null where the keystore would not hand the key over — a broken or absent provider, or a
     * device that cannot hold a key at all. That is «cannot read now», never «nothing stored».
     */
    @Suppress("SwallowedException")
    private fun key(): SecretKey? =
        try {
            keys()
        } catch (expected: GeneralSecurityException) {
            // Nothing to do with it and nowhere to report it: this module has no logging, which
            // is a named gap in the step. The caller is told «unavailable», which is what it
            // can act on.
            null
        } catch (expected: ProviderException) {
            null
        }

    /**
     * Bytes we hold the key for and still cannot open are ours and corrupt, and corrupt is
     * replaceable — Present, so the store above may write over them.
     *
     * The distinction is the whole point and it is not cosmetic: `File.writeBytes` truncates
     * before it writes, so one kill mid-write leaves a short file. Read as «cannot read now»,
     * that file would make the store unwritable for the life of the installation and the
     * patient would sign in again on every launch, for ever, with no path back. Unavailable is
     * reserved for the case where the key itself could not be had.
     */
    @Suppress("SwallowedException")
    private fun open(
        stored: ByteArray,
        key: SecretKey,
    ): Stored {
        if (stored.size <= IV_BYTES) return Stored.Present(ByteArray(0))

        return try {
            val cipher =
                Cipher.getInstance(TRANSFORMATION).apply {
                    init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(TAG_BITS, stored, 0, IV_BYTES))
                }
            Stored.Present(cipher.doFinal(stored, IV_BYTES, stored.size - IV_BYTES))
        } catch (expected: GeneralSecurityException) {
            // The tag failed under a key we did have: the file is ours and unusable.
            Stored.Present(ByteArray(0))
        }
    }

    /** False rather than thrown: the caller is a Settings, which has no channel for an error. */
    @Suppress("SwallowedException")
    override fun write(bytes: ByteArray): Boolean =
        try {
            val cipher = Cipher.getInstance(TRANSFORMATION).apply { init(Cipher.ENCRYPT_MODE, keys()) }
            file.parentFile?.mkdirs()
            file.writeBytes(cipher.iv + cipher.doFinal(bytes))
            true
        } catch (expected: GeneralSecurityException) {
            false
        } catch (expected: ProviderException) {
            false
        } catch (expected: IOException) {
            false
        }

    override fun wipe(): Boolean = !file.exists() || file.delete()
}

/** The Keystore key for this app's vault, created on first use and never leaving the store. */
private fun keystoreKey(): SecretKey {
    val keyStore = KeyStore.getInstance(PROVIDER).apply { load(null) }
    (keyStore.getKey(KEY_ALIAS, null) as? SecretKey)?.let { return it }

    return KeyGenerator
        .getInstance(KeyProperties.KEY_ALGORITHM_AES, PROVIDER)
        .apply {
            init(
                KeyGenParameterSpec
                    .Builder(KEY_ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                    .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                    .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                    .setKeySize(AES_BITS)
                    .build(),
            )
        }.generateKey()
}
