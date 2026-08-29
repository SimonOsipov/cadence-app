package app.cadence.shared.storage

import com.russhwolf.settings.NSUserDefaultsSettings
import com.russhwolf.settings.Settings
import platform.Foundation.NSUserDefaults

private const val SERVICE_PREFIX = "app.cadence."

private val stores = mutableMapOf<String, Settings>()

/**
 * The guard runs once per store, before it is handed out, because the keychain outlives the
 * app: the marker sits in `NSUserDefaults`, which deletion does clear.
 */
actual fun secureSettings(name: String): Settings =
    stores.getOrPut(name) {
        val persistent = VaultSettings(KeychainVault(SERVICE_PREFIX + name))
        guardFreshInstall(
            persistent,
            NSUserDefaultsSettings(NSUserDefaults.standardUserDefaults),
            "$INSTALL_MARKER_KEY.$name",
        )

        persistent
    }
