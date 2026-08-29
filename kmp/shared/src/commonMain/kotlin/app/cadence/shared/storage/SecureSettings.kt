package app.cadence.shared.storage

import com.russhwolf.settings.Settings

/**
 * Where the session and the PKCE verifier live.
 *
 * `supabase-kt`'s stock managers keep both in plaintext on both platforms —
 * `SharedPreferences` and `NSUserDefaults` — so this is the [Settings] they are handed
 * instead. Substituting it is a condition of taking that library, not a refinement.
 */
expect fun secureSettings(): Settings
