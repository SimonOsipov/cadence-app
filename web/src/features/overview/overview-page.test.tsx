import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { DataProvider, defaultClient } from '../../data/queries'
import { OVERVIEW, PATIENTS } from '../../data/fixtures/overview'
import type { Transport } from '../../data/transport'
import { PAGE_SIZE, fixtureTransport } from '../../data/transport'
import { OverviewPage } from './overview-page'

// A transport per test and a client per test: React Query caches across renders, and a shared client
// makes one test's answer the next test's starting point.
function show(transport: Transport = fixtureTransport()) {
  return render(
    <DataProvider transport={transport} client={defaultClient()}>
      <OverviewPage />
    </DataProvider>,
  )
}

describe('what the doctor sees while it loads, and if it will not', () => {
  it('says it is loading before anything has arrived', () => {
    show(fixtureTransport({ latencyMs: 50 }))

    expect(screen.getByRole('status')).toHaveTextContent('Загружаем дашборд')
  })

  // Reachable only because the seam can be made to fail; a screen whose error state cannot be produced
  // is a screen whose error state has never been looked at.
  it('says what went wrong, and offers to try again', async () => {
    show(fixtureTransport({ failWith: new Error('дашборд недоступен') }))

    const alert = await screen.findByRole('alert')

    expect(alert).toHaveTextContent('Не удалось загрузить данные')
    expect(alert).toHaveTextContent('дашборд недоступен')
    expect(within(alert).getByRole('button', { name: 'Повторить' })).toBeInTheDocument()
  })
})

describe('the side menu', () => {
  it('offers two destinations and not the four the MVP dropped', async () => {
    show()

    const menu = await screen.findByRole('navigation', { name: 'Разделы' })
    const destinations = within(menu)
      .getAllByRole('link')
      .map((link) => link.textContent)

    expect(destinations.map((label) => label?.replace(/\d+$/, ''))).toEqual(['Обзор', 'Сообщения'])

    for (const dropped of ['Пациенты', 'Расписание', 'Аналитика', 'Протоколы']) {
      expect(within(menu).queryByText(dropped)).toBeNull()
    }
  })
})

describe('the numbers on the strip', () => {
  // Not merely present: they are the ones the seam sent. A component that derived them from the rows
  // would agree with the fixture today and disagree the moment the roster is a page of it — which is
  // already true here, since the strip counts 25 and the first page carries eight.
  it('are the aggregates the seam sent, not a sum of the rows on screen', async () => {
    show()

    const { aggregates } = OVERVIEW
    // Scoped to the card that carries it: the same number appears on the roster tab beside it, and a
    // loose query would pass on either.
    const card = (label: string) => screen.getByText(label).closest('article')!

    await screen.findByText('Пациентов')

    expect(within(card('Пациентов')).getByText(String(aggregates.patients))).toBeInTheDocument()
    expect(aggregates.patients).toBeGreaterThan(PAGE_SIZE)
    expect(within(card('Регулярность')).getByText(String(aggregates.averageAdherence))).toBeInTheDocument()
    expect(
      within(card('Дозы сегодня')).getByText(`${aggregates.dosesDone}/${aggregates.dosesToday}`),
    ).toBeInTheDocument()
  })

  it('counts the tabs from the aggregates too', async () => {
    show()

    const tabs = await screen.findAllByRole('tab')
    const counts = tabs.map((tab) => tab.textContent?.replace(/\D+/g, ''))

    expect(counts).toEqual(
      [OVERVIEW.aggregates.byStatus.all, OVERVIEW.aggregates.byStatus.attention, OVERVIEW.aggregates.byStatus.watch, OVERVIEW.aggregates.byStatus.track].map(String),
    )
  })
})

describe('the roster', () => {
  it('shows one page of it and says how many there are', async () => {
    show()

    await screen.findByRole('tab', { name: /Все/ })

    await waitFor(() => {
      expect(screen.getByText(`${PAGE_SIZE} из ${PATIENTS.length}`)).toBeInTheDocument()
    })
  })

  it('walks to the next page', async () => {
    show()
    const user = userEvent.setup()

    // Scoped to the roster: the first patients are also on the triage cards above it, so «gone from the
    // page» is a claim about this section and not about the screen.
    const journal = async () => within(await screen.findByRole('region', { name: 'Журнал протоколов' }))
    const first = PATIENTS[0]?.name ?? ''

    expect(await (await journal()).findByText(first)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Дальше/ }))

    await waitFor(async () => {
      expect((await journal()).queryByText(first)).toBeNull()
    })
    expect((await journal()).getByText(PATIENTS[PAGE_SIZE]?.name ?? '')).toBeInTheDocument()
  })

  // The cursor belongs to the filter it came from. Kept across a tab change it names a patient the new
  // filter has never heard of, and the seam refuses it — so this is the error the screen must not be
  // able to cause.
  it('starts the new filter from its own beginning', async () => {
    show()
    const user = userEvent.setup()

    const journal = async () => within(await screen.findByRole('region', { name: 'Журнал протоколов' }))

    await screen.findByRole('tab', { name: /Все/ })
    await user.click(screen.getByRole('button', { name: /Дальше/ }))
    await waitFor(async () => expect((await journal()).queryByText(PATIENTS[0]?.name ?? '')).toBeNull())

    await user.click(screen.getByRole('tab', { name: /Внимание/ }))

    expect(await screen.findByText(OVERVIEW.aggregates.attention + ' из ' + OVERVIEW.aggregates.attention)).toBeInTheDocument()
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('says so when a page has nobody on it', async () => {
    const empty: Transport = {
      overview: () => fixtureTransport().overview(),
      roster: () => Promise.resolve({ items: [], total: 0, cursor: null }),
    }
    show(empty)

    expect(await screen.findByText(/Никого не нашлось/)).toBeInTheDocument()
  })
})

describe('the patient card', () => {
  it('opens on a row and carries what the seam worked out', async () => {
    show()
    const user = userEvent.setup()

    const patient = PATIENTS[0]
    const journal = within(await screen.findByRole('region', { name: 'Журнал протоколов' }))

    await user.click(await journal.findByText(patient?.name ?? ''))

    const card = await screen.findByRole('complementary', { name: `Карточка: ${patient?.name ?? ''}` })

    // The sentence and not the bare number: `lostKg` is 8 here, and a regex of /8/ matches the cycle
    // week, the adherence and half the biomarkers.
    expect(within(card).getByText(`${patient?.goalProgressPct ?? 0}% пути к цели`)).toBeInTheDocument()
    expect(
      within(card).getByText(
        `↓ ${patient?.lostKg ?? 0} ${patient?.unit ?? ''} с начала · цель ${patient?.goal ?? 0} ${patient?.unit ?? ''}`,
      ),
    ).toBeInTheDocument()

    await user.click(within(card).getByRole('button', { name: 'Закрыть карточку' }))
    // By name: the side menu is a landmark of the same kind, and an unnamed query would answer «still
    // there» about it for ever.
    await waitFor(() =>
      expect(screen.queryByRole('complementary', { name: /^Карточка:/ })).toBeNull(),
    )
  })
})
