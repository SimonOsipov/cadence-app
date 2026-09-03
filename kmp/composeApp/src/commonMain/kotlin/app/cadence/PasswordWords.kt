package app.cadence

/**
 * The two sentences that differ between the two links a patient can arrive on.
 *
 * Everything else on that screen is the same for both — the exchange, the form, the refusals — and
 * a second screen for two strings would be a second place to keep in step.
 */
data class PasswordWords(
    val choosing: String,
    val spentHint: String,
) {
    companion object {
        val OfAnInvitation =
            PasswordWords(
                choosing = AcceptanceCopy.CHOOSE_PASSWORD,
                spentHint = AcceptanceCopy.SPENT_HINT,
            )

        val OfARecovery =
            PasswordWords(
                choosing = RecoveryCopy.CHOOSE_NEW_PASSWORD,
                spentHint = RecoveryCopy.SPENT_HINT,
            )
    }
}
