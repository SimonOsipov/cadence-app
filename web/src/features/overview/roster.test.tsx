import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import type { RosterPage, RosterRow } from '../../api'
import { Roster } from './roster'

function row(over: Partial<RosterRow> = {}): RosterRow {
  return { user_id: '3f2a', full_name: 'Марина Волкова', age: 38, invite_state: 'accepted', ...over }
}

function show(page: RosterPage | undefined, over: Partial<Parameters<typeof Roster>[0]> = {}) {
  return render(<Roster page={page} loading={false} onPage={() => undefined} {...over} />)
}

describe('the roster the API answers', () => {
  it('draws a patient by the name and the age the server worked out', () => {
    show({ patients: [row()] })

    const patient = screen.getByRole('row', { name: /Марина Волкова/ })
    expect(within(patient).getByText('38 лет')).toBeInTheDocument()
  })

  // The API answers null for a patient whose date of birth the clinic never entered, and «0 лет» is
  // a number the screen would be making up.
  it('says nothing about the age of a patient who has no date of birth', () => {
    show({ patients: [row({ age: null })] })

    expect(screen.getByRole('row', { name: /Марина Волкова/ })).not.toHaveTextContent('лет')
  })

  // The state of the invitation is what this screen was extended for: it tells a doctor who is in the
  // app and who has not opened the letter.
  it('names each state of an invitation in the language the clinic reads', () => {
    show({
      patients: [
        row({ user_id: 'a', full_name: 'Принявшая', invite_state: 'accepted' }),
        row({ user_id: 'b', full_name: 'Ожидающий', invite_state: 'pending' }),
        row({ user_id: 'c', full_name: 'Истёкшая', invite_state: 'expired' }),
        row({ user_id: 'd', full_name: 'Неизвестный', invite_state: 'unknown' }),
      ],
    })

    for (const [name, state] of [
      ['Принявшая', 'В приложении'],
      ['Ожидающий', 'Приглашение отправлено'],
      ['Истёкшая', 'Приглашение истекло'],
      ['Неизвестный', 'Статус неизвестен'],
    ]) {
      expect(within(screen.getByRole('row', { name: new RegExp(name ?? '') })).getByText(state ?? '')).toBeInTheDocument()
    }
  })

  it('says the clinic has nobody rather than drawing an empty table', () => {
    show({ patients: [] })

    expect(screen.getByText(/Пациентов пока нет/)).toBeInTheDocument()
  })

  it('says it is loading before the first page arrives', () => {
    show(undefined)

    expect(screen.getByText(/Загружаем журнал/)).toBeInTheDocument()
  })

  // The reason is shown rather than swallowed: the API writes its refusals for the person reading them.
  it('shows the refusal and lets the doctor ask again', async () => {
    const user = userEvent.setup()
    const retry = vi.fn()

    show(undefined, { error: new Error('Страница не найдена. Откройте реестр заново.'), onRetry: retry })

    expect(screen.getByRole('alert')).toHaveTextContent('Страница не найдена')

    await user.click(screen.getByRole('button', { name: 'Повторить' }))
    expect(retry).toHaveBeenCalledOnce()
  })
})

describe('paging', () => {
  it('asks for the page after the cursor the answer carried', async () => {
    const user = userEvent.setup()
    const asked = vi.fn()

    show({ patients: [row()], next_cursor: 'after-marina' }, { onPage: asked })

    await user.click(screen.getByRole('button', { name: /Дальше/ }))
    expect(asked).toHaveBeenCalledWith('after-marina')
  })

  // Keyset paging: the last page carries no cursor, and a «Дальше» that can be pressed there asks the
  // API for a page after nobody.
  it('offers no next page when the answer carries no cursor', () => {
    show({ patients: [row()] })

    expect(screen.getByRole('button', { name: /Дальше/ })).toBeDisabled()
  })

  it('goes back to the first page by asking for no cursor at all', async () => {
    const user = userEvent.setup()
    const asked = vi.fn()

    show({ patients: [row()], next_cursor: 'after-marina' }, { onPage: asked })

    await user.click(screen.getByRole('button', { name: 'В начало' }))
    expect(asked).toHaveBeenCalledWith(null)
  })
})
