package app.cadence.debug

/** What a sign-in attempt came back as; «not tried» is a state and not an empty message. */
sealed interface SignIn {
    data object Untried : SignIn

    data object Accepted : SignIn

    /** [why] is the exception's class, never its text — the text can carry the token. */
    data class Refused(
        val why: String,
    ) : SignIn
}
