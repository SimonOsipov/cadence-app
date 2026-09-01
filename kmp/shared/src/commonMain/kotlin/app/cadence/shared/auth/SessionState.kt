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
     * Three paths end it, measured in the 3.7.0 artifact because the names name none of them.
     * `AuthImpl.init` runs `loadFromStorage` before `setupPlatform`, and a session read from the
     * store leaves through `importSession` — which sets [SessionStatus.Authenticated] outright
     * only while more than `SESSION_REFRESH_THRESHOLD` (0.2 of `expiresIn`, so 720s of GoTrue's
     * 3600s) remains. Under that, and **any** cold start later than 48 minutes after the token
     * was issued is under it, `importSession` waits on a refresh instead and the network's
     * outcome sets the status — so a patient holding a session watches this state for a round
     * trip. Where **nothing** is read no status is written at all, and that path waits for
     * `UtilsKt.initDone`, the only other place [SessionStatus.NotAuthenticated] is built beside
     * `AuthImpl.clearSession`. On Apple `setupPlatform` calls `initDone` itself; on Android only
     * when `enableLifecycleCallbacks` is off, otherwise from a `ProcessLifecycleOwner` ON_START
     * callback that itself returns early unless `alwaysAutoRefresh` is on and auto-refresh is
     * not already running — both defaults, and neither is set here. It arrives out of
     * `androidx.lifecycle:lifecycle-process`, which `supabase-kt-android` declares at 2.10.0.
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
