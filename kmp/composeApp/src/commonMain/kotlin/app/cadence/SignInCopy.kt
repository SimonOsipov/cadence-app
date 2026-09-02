package app.cadence

/**
 * What the sign-in screen says.
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

    const val OFFLINE = "Нет связи с сервером"
    const val OFFLINE_HINT = "Проверьте подключение и попробуйте ещё раз"

    const val SIGN_OUT = "Выйти из аккаунта"
}
