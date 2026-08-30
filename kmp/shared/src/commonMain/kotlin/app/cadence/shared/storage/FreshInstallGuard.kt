package app.cadence.shared.storage

import com.russhwolf.settings.Settings

/**
 * Held in storage the platform clears on delete, which is what dates the installation.
 *
 * One marker per store rather than one per installation, and that is not a detail: the stores
 * are guarded lazily, on first use, so a single marker would be put down by whichever secret
 * was asked for first and would retire the guard before the other was ever touched. The
 * verifier would then outlive the installation that wrote it.
 */
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
    persistent: VaultSettings,
    volatileStore: Settings,
    marker: String = INSTALL_MARKER_KEY,
) {
    if (volatileStore.getBoolean(marker, false)) return

    // The marker only goes down on a wipe that answered. Written regardless, one failed
    // delete would retire the guard for the life of the installation, and the store it was
    // supposed to clear would be inherited for good — which is the single thing this
    // function exists to prevent.
    if (!persistent.clearAndConfirm()) return

    volatileStore.putBoolean(marker, true)
}
