package app.cadence.shared.storage

import com.russhwolf.settings.NSUserDefaultsSettings
import com.russhwolf.settings.Settings
import platform.Foundation.NSUserDefaults

private const val SERVICE = "app.cadence.session"

/**
 * The guard runs before the store is handed out, because the keychain outlives the app: the
 * marker sits in `NSUserDefaults`, which deletion does clear.
 */
actual fun secureSettings(): Settings {
    val persistent = VaultSettings(KeychainVault(SERVICE))
    guardFreshInstall(persistent, NSUserDefaultsSettings(NSUserDefaults.standardUserDefaults))

    return persistent
}
