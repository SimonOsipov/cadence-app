package app.cadence

/**
 * What the recovery screen says.
 *
 * [SENT] is the answer to every address, and that is the screen's security property: told apart, a
 * known address and a stranger's turn the form into a way of asking who is a patient here. So the
 * sentence promises a letter «если такой адрес у нас есть» rather than one that was sent.
 */
internal object RecoveryCopy {
    const val FORGOT = "Забыли пароль?"

    const val TITLE = "Восстановление доступа"
    const val ADDRESS_FIELD = "Почта"
    const val SEND = "Прислать ссылку"
    const val BACK = "Вернуться ко входу"

    const val SENT = "Если такой адрес у нас есть, письмо уже в пути"
    const val SENT_HINT = "Откройте ссылку из письма на этом устройстве"

    // Its own answer, not «нет связи»: the letter is already on its way, and a patient told to
    // check their connection asks again and pushes the gap further out.
    const val TOO_SOON = "Письмо уже отправляли только что"
    const val TOO_SOON_HINT = "Подождите минуту и проверьте почту, включая «Спам»"

    const val OFFLINE = "Сервер сейчас не отвечает"
    const val OFFLINE_HINT = "Попробуйте ещё раз через минуту"

    const val CHOOSE_NEW_PASSWORD = "Придумайте новый пароль"

    // Not «попросите клинику»: unlike an invitation, this letter is one the patient asks for
    // themselves, and sending them to the clinic for it is a detour with a person on the end.
    const val SPENT_HINT = "Запросите новую ссылку на экране входа"
}
