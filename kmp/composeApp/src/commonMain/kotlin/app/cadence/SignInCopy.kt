package app.cadence

/**
 * What the sign-in screen says, and the one word the way out of the app says — [SIGN_OUT] is
 * drawn by the profile route, and moves with that screen when it is ported.
 *
 * The refusal is one sentence for every cause the server can have, which is the screen's whole
 * security property: told apart, «no such address» and «wrong password» turn the form into a way
 * of asking which of the clinic's patients has an account.
 */
internal object SignInCopy {
    const val TITLE = "Вход в Cadence"

    const val ADDRESS_FIELD = "Почта"
    const val PASSWORD_FIELD = "Пароль"
    const val ENTER = "Войти"

    const val REFUSED = "Не удалось войти"
    const val REFUSED_HINT = "Проверьте почту и пароль"

    // Not «check your connection»: this answer also covers a rate limit and a restarting server,
    // and telling a patient on a working network to reconnect and retry immediately is how the
    // rate-limit window gets extended.
    const val OFFLINE = "Сервер сейчас не отвечает"
    const val OFFLINE_HINT = "Попробуйте ещё раз через минуту"

    const val SIGN_OUT = "Выйти из аккаунта"
}
