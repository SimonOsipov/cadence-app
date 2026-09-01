package app.cadence.shared.auth

import io.github.jan.supabase.SupabaseClient
import io.github.jan.supabase.auth.auth
import io.github.jan.supabase.auth.status.RefreshFailureCause
import io.github.jan.supabase.auth.status.SessionStatus
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map

/** Which of the app's two areas the patient belongs in, and «not yet» before the vendor decides. */
sealed interface SessionState {
    /**
     * Nothing has been decided yet. Rendered as neither area: mapped to [SignedOut] the sign-in
     * screen flashes on every launch, which reads to a patient as having been signed out.
     *
     * What ends it is **not** the store answering, measured in the 3.7.0 artifact:
     * `loadFromStorage` never touches `sessionStatus`, and [SessionStatus.NotAuthenticated] is
     * constructed in exactly two places — `AuthImpl.clearSession` and `UtilsKt.initDone`. On
     * Apple `setupPlatform` calls `initDone` itself; on Android it calls it only when
     * `enableLifecycleCallbacks` is off, and otherwise from a `ProcessLifecycleOwner` ON_START
     * callback — so an empty or unreadable store leaves this state on a lifecycle event, out of
     * `androidx.lifecycle:lifecycle-process` (2.10.0 on the runtime classpath, reached
     * transitively; `auth-kt-android` declares no lifecycle dependency of its own).
     */
    data object Deciding : SessionState

    data object SignedOut : SessionState

    data object SignedIn : SessionState
}

/**
 * The vendor's four statuses as the two the shell navigates on.
 *
 * **A `RefreshFailure` of either cause keeps the patient inside**, and that is the whole of the
 * decision. Measured in the 3.7.0 artifact rather than reasoned from the names, because the
 * names invite the opposite reading and the first version of this file took it: `AuthImpl`
 * branches on `NETWORK_ERROR_CODES`, which its static initialiser builds as
 * `[500, 502, 503, 504, 520, 521, 522, 523, 524, 530]`. A status **in** that list logs
 * «Couldn't refresh session due to an internal server error. Retrying in …» and constructs
 * `InternalServerError` — the session is kept and a retry scheduled. **Any** status outside that
 * list calls `clearSession()` instead and arrives here as [SessionStatus.NotAuthenticated];
 * there is no second list on this path. (`SIGN_OUT_IGNORE_CODES`, the `[401, 403, 404]` beside
 * it in the same file, is read by `signOut` and never by the refresh.) Measured against our own
 * GoTrue on 2026-08-31: a revoked refresh token answers `400 refresh_token_not_found`, so it is
 * the outside-the-list branch that carries the case this mapping cares about.
 *
 * So `InternalServerError` is the retryable five-hundred and not a refusal; mapping it to
 * «signed out» throws every patient to the sign-in screen while GoTrue restarts and snaps them
 * back when it recovers. `NotAuthenticated` is the only refusal there is, and it is what the
 * criterion «expiry in the background routes to sign-in» is served by.
 *
 * This is the other side of [kmp-app]'s invariant 4: at the `SessionTokens.refreshed()` seam a
 * refused token and a lost signal genuinely arrive alike and nothing is cleared. Here neither
 * is a sign-out either — the vendor has already decided, and both causes mean «still trying».
 */
fun SessionStatus.asSessionState(): SessionState =
    when (this) {
        is SessionStatus.Initializing -> {
            SessionState.Deciding
        }

        is SessionStatus.Authenticated -> {
            SessionState.SignedIn
        }

        is SessionStatus.NotAuthenticated -> {
            SessionState.SignedOut
        }

        // Both causes, and deliberately not split: see above. A cause the vendor adds later
        // arrives here as a compile error rather than as a silent sign-out.
        is SessionStatus.RefreshFailure -> {
            when (cause) {
                is RefreshFailureCause.NetworkError -> SessionState.SignedIn
                is RefreshFailureCause.InternalServerError -> SessionState.SignedIn
            }
        }
    }

/**
 * Derived and de-duplicated, which is what keeps ten requests meeting one expiry from moving
 * the patient ten times: the transport already collapses them into one refresh, and a shell
 * reacting per request would undo that.
 */
fun Flow<SessionStatus>.asSessionStates(): Flow<SessionState> = map { it.asSessionState() }.distinctUntilChanged()

/** The stream a platform root hands the shell. */
fun SupabaseClient.sessionStates(): Flow<SessionState> = auth.sessionStatus.asSessionStates()
