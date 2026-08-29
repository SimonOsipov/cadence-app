package app.cadence.shared.storage

import android.content.Context
import com.russhwolf.settings.Settings
import java.io.File

private const val DIRECTORY = "secure"
private const val STORE = "session.bin"

// internal so the suite can put it back: the tests share a process, and one that installed
// would otherwise decide whether the next one sees an install at all.
internal var storageRoot: File? = null

/**
 * Handed the app's own storage once, from the Android application.
 *
 * A held reference rather than a parameter on [secureSettings]: the common declaration has
 * nowhere to carry a `Context`, and threading an Android type through it would put a
 * platform concept in the shared signature. The cost is that forgetting this call is a
 * runtime failure — so it is a loud one, named below.
 */
fun installSecureStorage(context: Context) {
    storageRoot = File(context.filesDir, DIRECTORY)
}

/**
 * No fresh-install guard here, and that is a platform difference rather than an omission:
 * the file lives in the app's own storage, which Android removes on uninstall. Apple's
 * keychain does not, which is what [guardFreshInstall] is for.
 */
actual fun secureSettings(): Settings {
    val root =
        checkNotNull(storageRoot) {
            "installSecureStorage(context) has not run — the Android application must call it " +
                "before anything asks for a session"
        }

    return VaultSettings(AndroidVault(File(root, STORE)))
}
