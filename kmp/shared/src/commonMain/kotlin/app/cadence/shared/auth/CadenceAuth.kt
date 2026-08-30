package app.cadence.shared.auth

import app.cadence.shared.net.Session
import app.cadence.shared.net.SessionTokens
import app.cadence.shared.storage.PKCE_STORE
import app.cadence.shared.storage.SESSION_STORE
import app.cadence.shared.storage.secureSettings
import com.russhwolf.settings.Settings
import io.github.jan.supabase.SupabaseClient
import io.github.jan.supabase.auth.Auth
import io.github.jan.supabase.auth.SettingsCodeVerifierCache
import io.github.jan.supabase.auth.SettingsSessionManager
import io.github.jan.supabase.auth.auth
import io.github.jan.supabase.createSupabaseClient

/**
 * The Auth module, pointed at our own GoTrue and holding its secrets where we put them.
 *
 * `customUrl` is what makes the vendor's client talk to a self-hosted server: measured in the
 * 3.7.0 artifact, `MainPlugin.resolveUrl` appends `auth/v1` only where it is null, and ours
 * answers on its own root — every `/auth/v1` path is a 404 there, measured against the local
 * contour. ADR-008 took the vendor away and left the mechanism.
 *
 * Both storage seams are substituted, and that is the condition the library was taken on rather
 * than a refinement: `SettingsSessionManager` and `SettingsCodeVerifierCache` default to
 * plaintext on both platforms — `SharedPreferences` and `NSUserDefaults`. They are handed two
 * different secure stores, because the store is written whole and one shared between them would
 * lose the verifier to the session's next write, in the middle of accepting an invite.
 *
 * [stores] is a parameter so the seam can be measured without a device; production passes
 * [secureSettings].
 */
fun cadenceAuth(
    url: String,
    stores: (String) -> Settings = ::secureSettings,
): SupabaseClient =
    createSupabaseClient(supabaseUrl = url, supabaseKey = "") {
        install(Auth) {
            customUrl = url
            sessionManager = SettingsSessionManager(stores(SESSION_STORE))
            codeVerifierCache = SettingsCodeVerifierCache(stores(PKCE_STORE))
        }
    }

/**
 * The transport's view of this session, and the whole of the coupling between them.
 *
 * Refresh belongs to the Auth module and the transport does not do it — `refreshCurrentSession`
 * is the owner, and Ktor's plugin is what serialises the callers. Two mechanisms rotating one
 * refresh token is a revoked session.
 */
fun SupabaseClient.sessionTokens(): SessionTokens =
    object : SessionTokens {
        override suspend fun current(): Session? = auth.currentSessionOrNull()?.asSession()

        override suspend fun refreshed(): Session? {
            auth.refreshCurrentSession()

            return auth.currentSessionOrNull()?.asSession()
        }
    }

private fun io.github.jan.supabase.auth.user.UserSession.asSession() =
    Session(access = accessToken, refresh = refreshToken)
