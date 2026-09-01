package app.cadence

/**
 * What the invitation screen says, and why each sentence is its own.
 *
 * The three refusals are one answer to the transport and three to a patient, measured against
 * v2.194.0: a link opened twice and a token that never existed both answer `otp_expired`; a
 * patient the clinic has banned answers `user_banned` on a link nobody spent; and a refusal can
 * arrive naming nothing at all. Telling the second one «already used» sends them to ask for an
 * invitation that will refuse the same way.
 */
internal object AcceptanceCopy {
    const val CHECKING = "Проверяем приглашение"
    const val CHOOSE_PASSWORD = "Придумайте пароль"

    // Its own word rather than the title again: two nodes carrying one string is a screen a test
    // cannot point at, and a placeholder repeating the heading tells the patient nothing twice.
    const val PASSWORD_FIELD = "Пароль"
    const val PASSWORD_HINT = "Дальше вы будете входить по нему"
    const val ENTER = "Войти"

    const val SPENT = "Эта ссылка уже использована"
    const val SPENT_HINT = "Попросите клинику прислать новую"

    const val BANNED = "Доступ приостановлен"
    const val BANNED_HINT = "Обратитесь в клинику"

    const val UNNAMED = "Не удалось принять приглашение"
    const val UNNAMED_HINT = "Обратитесь в клинику"

    const val OFFLINE = "Нет связи с сервером"
    const val OFFLINE_HINT = "Проверьте подключение"
    const val RETRY = "Попробовать ещё раз"
}
