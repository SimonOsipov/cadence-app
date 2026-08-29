package app.cadence.shared.storage

import com.russhwolf.settings.Settings

/** The session `supabase-kt` keeps, and the store its `SettingsSessionManager` is handed. */
const val SESSION_STORE: String = "session"

/** The PKCE verifier, which sits on the invite-acceptance path and is a separate secret. */
const val PKCE_STORE: String = "pkce"

/**
 * Where a named secret lives.
 *
 * `supabase-kt`'s stock managers keep both of the stores above in plaintext on both platforms
 * — `SharedPreferences` and `NSUserDefaults` — so this is the [Settings] they are handed
 * instead. Substituting it is a condition of taking that library, not a refinement.
 *
 * Named, because there is more than one and the whole store is written back on every change:
 * two consumers sharing one would each rewrite the other's keys out of it, and the verifier
 * would vanish in the middle of accepting an invite. One instance per name, so two callers
 * asking for the same secret share the map rather than racing two copies of it.
 */
expect fun secureSettings(name: String): Settings
