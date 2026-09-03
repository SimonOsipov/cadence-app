package app.cadence

/**
 * Every sentence on the password screen that names which link the patient arrived on.
 *
 * The screen itself is one for both — the exchange, the form, the four refusals — and a twin of it
 * for a handful of strings would be a second place to keep in step. The set grew after review: it
 * started at two and missed [checking] and [unnamed], so a patient who tapped «Восстановить
 * доступ» was told the app was checking an invitation they never received, on the normal path.
 */
data class PasswordWords(
    val checking: String,
    val choosing: String,
    val spentHint: String,
    val unnamed: String,
    val unnamedHint: String,
) {
    companion object {
        val OfAnInvitation =
            PasswordWords(
                checking = AcceptanceCopy.CHECKING,
                choosing = AcceptanceCopy.CHOOSE_PASSWORD,
                spentHint = AcceptanceCopy.SPENT_HINT,
                unnamed = AcceptanceCopy.UNNAMED,
                unnamedHint = AcceptanceCopy.UNNAMED_HINT,
            )

        val OfARecovery =
            PasswordWords(
                checking = RecoveryCopy.CHECKING,
                choosing = RecoveryCopy.CHOOSE_NEW_PASSWORD,
                spentHint = RecoveryCopy.SPENT_HINT,
                unnamed = RecoveryCopy.UNNAMED,
                unnamedHint = RecoveryCopy.UNNAMED_HINT,
            )
    }
}
