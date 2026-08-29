package app.cadence.shared.storage

import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.io.File
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

private const val KEY_ALIAS = "app.cadence.vault"
private const val PROVIDER = "AndroidKeyStore"
private const val TRANSFORMATION = "AES/GCM/NoPadding"
private const val IV_BYTES = 12
private const val TAG_BITS = 128

/**
 * AES/GCM over a file, with the key in the Keystore and never in this process.
 *
 * Ours rather than `androidx.security:security-crypto`, which is deprecated with no direct
 * replacement — depending on it would mean depending on something with no upgrade path, on
 * the authentication path of a medical app.
 *
 * The IV comes from the provider and is stored ahead of the ciphertext. GCM under a reused
 * IV and one key is a key recovery, and letting the cipher choose is what guarantees it is
 * not reused.
 *
 * [keys] is a parameter because AndroidKeyStore is absent from the test runtime — measured,
 * see AndroidKeyStoreReachabilityTest — so injecting it is what leaves everything decided
 * here measurable. Production callers take the default and never pass one.
 */
class AndroidVault(
    private val file: File,
    private val keys: () -> SecretKey = ::keystoreKey,
) : Vault {
    override fun read(): ByteArray? {
        if (!file.exists()) return null

        return try {
            val stored = file.readBytes()
            if (stored.size <= IV_BYTES) return null
            val cipher =
                Cipher.getInstance(TRANSFORMATION).apply {
                    init(Cipher.DECRYPT_MODE, keys(), GCMParameterSpec(TAG_BITS, stored, 0, IV_BYTES))
                }
            cipher.doFinal(stored, IV_BYTES, stored.size - IV_BYTES)
        } catch (_: Exception) {
            // A changed lock screen invalidates the key, a restore brings a file no key here
            // can open, and a half-written one fails the tag. All three are «no session»,
            // which is the only thing the caller can act on.
            null
        }
    }

    override fun write(bytes: ByteArray) {
        val cipher = Cipher.getInstance(TRANSFORMATION).apply { init(Cipher.ENCRYPT_MODE, keys()) }
        file.parentFile?.mkdirs()
        file.writeBytes(cipher.iv + cipher.doFinal(bytes))
    }

    override fun wipe() {
        file.delete()
    }
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
                    .build(),
            )
        }.generateKey()
}
