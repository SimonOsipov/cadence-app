package app.cadence.shared.auth

import io.github.jan.supabase.SupabaseClient
import io.github.jan.supabase.auth.OtpType
import io.github.jan.supabase.auth.auth
import io.github.jan.supabase.auth.exception.AuthErrorCode
import io.github.jan.supabase.auth.exception.AuthRestException
import io.github.jan.supabase.exceptions.RestException
import io.ktor.http.Url
import io.ktor.utils.io.errors.IOException
import kotlinx.coroutines.CancellationException

/** Where an invitation lands, registered as an intent-filter and in `CFBundleURLTypes`. */
const val ACCEPT_LINK: String = "cadence://accept"

private const val TOKEN_PARAMETER = "token_hash"

/**
 * The invitation's token, or null when this link is not one.
 *
 * Compared whole rather than by prefix: `cadence://accept/../recover` starts with the accept
 * address and is a different destination.
 */
fun invitationToken(link: String): String? {
    val url = runCatching { Url(link) }.getOrNull() ?: return null

    if ("${url.protocol.name}://${url.host}${url.encodedPath}".trimEnd('/') != ACCEPT_LINK) {
        return null
    }

    return url.parameters[TOKEN_PARAMETER]?.ifEmpty { null }
}

/**
 * What the invitation screen has to tell apart.
 *
 * [Spent] and [Unreachable] are one answer to the transport and two to a patient: a link opened
 * twice is ordinary and is explained, a train in a tunnel is offered another try. Telling them
 * apart wrongly costs a patient their invitation — «already used» over a live link sends them
 * back to the clinic for a new one.
 */
sealed interface Acceptance {
    data object Accepted : Acceptance

    data object Spent : Acceptance

    data object Unreachable : Acceptance
}

/**
 * Exchanges the invitation's token for a session, which the vendor then stores.
 *
 * Measured against v2.194.0 on 2026-09-01: `POST /verify` with an unspent `token_hash` answers the
 * whole session in its body, so nothing is caught out of a URL fragment and no browser is opened;
 * spending it twice answers `403 otp_expired`, and so does a token that never existed. PKCE is not
 * the alternative — the admin route accepts a `code_challenge` and ignores it.
 *
 * **The exception type carries no answer at all**, measured in the 3.7.0 artifact:
 * `parseErrorResponse` builds a `GoTrueErrorResponse` — falling back to the literal
 * `"Unknown error"` when the body will not decode — and hands it to `checkErrorCodes`, which
 * returns an [AuthRestException] whenever that error is non-null. So a spent link, a rate limit
 * and GoTrue restarting all arrive as one type, and only the code inside separates them.
 *
 * [Acceptance.Spent] is therefore given on that code alone and nothing else is guessed into it:
 * being told an invitation is used up sends a patient back to the clinic for one they already
 * have, while «try again» costs them a tap. An unrecognised refusal takes the cheaper mistake.
 *
 * The swallow is the answer, as it is at `SessionTokens.refreshed()`: which failure arrived is the
 * whole of what the screen needs, and carrying it further would log a token's failure.
 */
@Suppress("SwallowedException")
suspend fun SupabaseClient.acceptInvitation(tokenHash: String): Acceptance =
    try {
        auth.verifyEmailOtp(type = OtpType.Email.INVITE, tokenHash = tokenHash)

        Acceptance.Accepted
    } catch (cancelled: CancellationException) {
        throw cancelled
    } catch (refused: RestException) {
        if ((refused as? AuthRestException)?.errorCode == AuthErrorCode.OtpExpired) {
            Acceptance.Spent
        } else {
            Acceptance.Unreachable
        }
    } catch (unreachable: IOException) {
        Acceptance.Unreachable
    }
