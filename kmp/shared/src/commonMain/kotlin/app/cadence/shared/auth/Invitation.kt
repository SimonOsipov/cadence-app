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
 * [Refused] and [Unreachable] are one answer to the transport and two to a patient: one is
 * explained and leads to the clinic, the other is offered another try. Getting it wrong is
 * expensive in both directions — «already used» over a live link sends a patient to the clinic
 * for an invitation they are holding, and «try again» over a finished one is a loop with no exit.
 */
sealed interface Acceptance {
    data object Accepted : Acceptance

    /**
     * The invitation cannot be completed, and asking again will not change that.
     *
     * [code] is the vendor's own, and it is what the screen writes its Russian from: measured
     * against v2.194.0, a spent link and a token that never existed both answer `otp_expired`,
     * while a patient the clinic has banned answers `user_banned` — two refusals needing two
     * sentences, which would be one dead end if this carried no reason. Null where none was given.
     */
    data class Refused(
        val code: AuthErrorCode?,
    ) : Acceptance

    data object Unreachable : Acceptance
}

private const val REQUEST_TIMEOUT = 408

// Not a timeout: a refusal to process replayable early data, where retrying is the point.
private const val TOO_EARLY = 425

private const val TOO_MANY_REQUESTS = 429

private const val FIRST_SERVER_ERROR = 500

// Read off the status because no code separates them from a refusal: each of these answers
// differently when asked again, and so does GoTrue once it has restarted. Anything else that
// refuses is about this invitation and will refuse the same way.
private val RETRYABLE = setOf(REQUEST_TIMEOUT, TOO_EARLY, TOO_MANY_REQUESTS)

/**
 * Exchanges the invitation's token for a session, which the vendor then stores.
 *
 * Measured against v2.194.0 on 2026-09-01: `POST /verify` with an unspent `token_hash` answers the
 * whole session in its body, so nothing is caught out of a URL fragment and no browser is opened;
 * spending it twice answers `403 otp_expired`, and so does a token that never existed. PKCE is not
 * the alternative — the admin route accepts a `code_challenge` and ignores it.
 *
 * **The exception type is not the answer**, measured in the 3.7.0 artifact. `parseErrorResponse`
 * decodes the body into a `GoTrueErrorResponse` — whose `error` is read from the `error_code` key
 * alone, and falls back to the literal `"Unknown error"` when the body will not decode at all —
 * and hands it to `checkErrorCodes`, which answers an [AuthRestException] when that error is
 * non-null and **null** when it is not. Three shapes come out of that, not two: a refusal naming a
 * code arrives as [AuthRestException] with the code parsed; a JSON refusal naming none falls
 * through to an `Unauthorized`, `BadRequest` or `UnknownRestException`; and a body that will not
 * decode at all is an [AuthRestException] carrying the literal `"Unknown error"`, which maps to no
 * code. All three are refusals, and only the first can be named.
 *
 * So the retryable answers are picked out by status, everything else is a refusal, and it carries
 * whatever reason the vendor gave rather than a sentence guessed here.
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
        if (refused.statusCode in RETRYABLE || refused.statusCode >= FIRST_SERVER_ERROR) {
            Acceptance.Unreachable
        } else {
            Acceptance.Refused((refused as? AuthRestException)?.errorCode)
        }
    } catch (unreachable: IOException) {
        Acceptance.Unreachable
    }
