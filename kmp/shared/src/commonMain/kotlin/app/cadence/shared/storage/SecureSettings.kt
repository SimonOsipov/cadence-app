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
 *
 * **Named gap.** That last sentence holds for callers arriving one at a time, which is all
 * there are today: the map behind it is an ordinary one and `getOrPut` is not atomic. Two
 * first calls in flight at once would build two instances over one vault, each holding a
 * stale map and writing the blob back whole — the very thing the naming prevents. It becomes
 * reachable when step 2 puts the session manager on a background dispatcher, and the
 * primitive to close it with is that step's decision rather than this one's.
 */
expect fun secureSettings(name: String): Settings
