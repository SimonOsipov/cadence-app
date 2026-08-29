package app.cadence.shared.storage

import com.russhwolf.settings.Settings

/** Held in storage the platform clears on delete, which is what dates the installation. */
const val INSTALL_MARKER_KEY: String = "app.cadence.installed"

/**
 * Wipes the persistent store when the installation that filled it is gone.
 *
 * Apple's keychain survives app deletion by design, so without this the next installation
 * inherits the previous one's session. [volatileStore] is storage the platform does clear —
 * `NSUserDefaults`, `SharedPreferences` — and its marker is the only thing separating a fresh
 * install from an ordinary launch.
 */
fun guardFreshInstall(
    persistent: Settings,
    volatileStore: Settings,
) {
    if (volatileStore.getBoolean(INSTALL_MARKER_KEY, false)) return

    persistent.clear()
    volatileStore.putBoolean(INSTALL_MARKER_KEY, true)
}
