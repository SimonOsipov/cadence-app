package app.cadence.debug

/**
 * What the debug screen found, in the three states the spec asks to be distinguishable.
 *
 * Three and not four: an expired token and a call that was never authenticated are one state
 * here because they are one answer from the API — same status, indistinguishable body, on
 * purpose. A screen that drew a fourth would be inventing a difference the product does not
 * have, and the patient-facing app would inherit the invention.
 */
sealed interface ProbeState {
    /** The API did not answer, or answered that it could not. Says nothing about the token. */
    data class Unavailable(
        val why: String,
    ) : ProbeState

    /** The call went through — after the transport's refresh, or without needing one. */
    data class SignedIn(
        val body: String,
    ) : ProbeState

    /** The token was refused and the transport could not renew it. */
    data object SignedOut : ProbeState
}
