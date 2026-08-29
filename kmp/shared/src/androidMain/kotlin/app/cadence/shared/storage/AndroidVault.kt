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

        val stored =
            try {
                file.readBytes()
            } catch (expected: IOException) {
                return Stored.Unavailable("reading $file: $expected")
            }

        return open(stored)
    }

    /** A file too short to carry an IV is not a truncated store; it is not one of ours. */
    private fun open(stored: ByteArray): Stored {
        if (stored.size <= IV_BYTES) return Stored.Unavailable("$file holds ${stored.size} bytes")

        return try {
            val cipher =
                Cipher.getInstance(TRANSFORMATION).apply {
                    init(Cipher.DECRYPT_MODE, keys(), GCMParameterSpec(TAG_BITS, stored, 0, IV_BYTES))
                }
            Stored.Present(cipher.doFinal(stored, IV_BYTES, stored.size - IV_BYTES))
        } catch (expected: GeneralSecurityException) {
            // A restore brings a file no key here can open, and a half-written one fails the
            // GCM tag. ProviderException is the other shape: a device whose keystore is
            // broken refuses to hand the key over at all. None of the three says the patient
            // is signed out — only that this store cannot be read now, which is why the
            // answer is Unavailable and the caller may not write over it.
            Stored.Unavailable("opening $file: $expected")
        } catch (expected: ProviderException) {
            Stored.Unavailable("the key for $file: $expected")
        }
    }

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
