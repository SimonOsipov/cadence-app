package app.cadence.android

import android.app.Application
import app.cadence.shared.storage.installSecureStorage

/**
 * Hands the shared module the app's own storage, which is the one thing only a Context can
 * answer. Here rather than in the activity because a background token refresh can outlive
 * any activity, and asking then must not depend on a window having been created.
 */
class CadenceApplication : Application() {
    override fun onCreate() {
        super.onCreate()
        installSecureStorage(this)
    }
}
