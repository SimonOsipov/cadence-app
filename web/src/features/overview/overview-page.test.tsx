import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { DataProvider, defaultClient } from '../../data/queries'
import { OVERVIEW, PATIENTS, TRIAGE_IDS } from '../../data/fixtures/overview'

const OVERVIEW_TRIAGE = TRIAGE_IDS.map(
  (id) => PATIENTS.find((patient) => patient.id === id)?.name ?? id,
)
const TRIAGE_LAST_ID = PATIENTS.filter((patient) => patient.status === 'attention').at(-1)?.id ?? null
import { quantity, whole } from '../../format'
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
  // The button is wired, not merely present: a transport that fails once and then answers proves the
  // click does something.
  it('retries when asked to', async () => {
    let asked = 0
    const flaky: Transport = {
      overview: () => {
        asked += 1

        return asked === 1
          ? Promise.reject(new Error('дашборд недоступен'))
          : fixtureTransport().overview()
      },
      roster: (query) => fixtureTransport().roster(query),
    }
    show(flaky)
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Повторить' }))

    expect(await screen.findByText('Пациентов')).toBeInTheDocument()
  })

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
    const destinations = [...menu.querySelectorAll('a, span[aria-disabled]')].map((el) => el.textContent)

    expect(destinations.map((label) => label?.replace(/\d+$/, ''))).toEqual(['Обзор', 'Сообщения'])

    // Only the screen that exists is a link. «Сообщения» would otherwise reload the page and serve the
    // Overview back — a control promising a destination this block does not have.
    expect(within(menu).getAllByRole('link')).toHaveLength(1)
    expect(within(menu).getByText('Сообщения').closest('[aria-disabled]')).not.toBeNull()

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

  // Pairs and not a list of digits: «Внимание» and «Наблюдение» are both 4 in this fixture, so a list
  // passes with the two swapped — the label is half of what is being asserted.
  it.each([
    ['Все', OVERVIEW.aggregates.byStatus.all],
    ['Внимание', OVERVIEW.aggregates.byStatus.attention],
    ['Наблюдение', OVERVIEW.aggregates.byStatus.watch],
    ['В норме', OVERVIEW.aggregates.byStatus.track],
  ] as const)('counts %s from the aggregates', async (label, count) => {
    show()

    const tab = await screen.findByRole('tab', { name: new RegExp(`^${label}`) })

    expect(tab.textContent?.replace(/\D+/g, '')).toBe(String(count))
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
    await screen.findByText(`${PAGE_SIZE} из ${PATIENTS.length}`)

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

    // The pager is disabled until the first page lands, and a click on a disabled button is not
    // delivered. Waited for rather than assumed: measured, a transport answering the roster 30ms after
    // the overview turns this green test red, and today it passes only because both timers fire in one
    // tick.
    await screen.findByText(`${PAGE_SIZE} из ${PATIENTS.length}`)
    await user.click(screen.getByRole('button', { name: /Дальше/ }))
    await waitFor(async () => expect((await journal()).queryByText(PATIENTS[0]?.name ?? '')).toBeNull())

    await user.click(screen.getByRole('tab', { name: /Внимание/ }))

    expect(await screen.findByText(OVERVIEW.aggregates.attention + ' из ' + OVERVIEW.aggregates.attention)).toBeInTheDocument()
    expect(screen.queryByRole('alert')).toBeNull()
  })

  // Through the real seam and not a stub. The fixture has patients of every status and the seam offers
  // no page after the last one with rows, so the browser cannot reach this state today — which is
  // exactly why the empty screen would otherwise be drawn against nothing. The seam is asked the one
  // question that does produce it.
  it('says so when a page has nobody on it', async () => {
    const attention = OVERVIEW.aggregates.byStatus.attention
    const past: Transport = {
      overview: () => fixtureTransport().overview(),
      // The one question the seam answers with an empty page: the filter's last patient as the cursor.
      roster: () => fixtureTransport().roster({ filter: 'attention', cursor: TRIAGE_LAST_ID }),
    }
    show(past)

    expect(await screen.findByText(/Никого не нашлось/)).toBeInTheDocument()
    expect(attention).toBeGreaterThan(0)
  })

  // Both branches were unreachable in tests: `failWith` fails the overview too, so the page
  // short-circuits before the roster is ever rendered.
  it('keeps the filters usable when only the roster fails', async () => {
    const rosterDown: Transport = {
      overview: () => fixtureTransport().overview(),
      roster: () => Promise.reject(new Error('журнал недоступен')),
    }
    show(rosterDown)

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('журнал недоступен')
    // The tabs and the pager survive the failure: replacing the section wholesale leaves a doctor with
    // no way to change the filter or step back to the first page.
    expect(screen.getAllByRole('tab')).toHaveLength(4)
    expect(screen.getByRole('button', { name: 'В начало' })).toBeInTheDocument()
  })

  it('shows its own line while the page is on its way', async () => {
    const slow: Transport = {
      overview: () => fixtureTransport().overview(),
      roster: (query) => fixtureTransport({ latencyMs: 60 }).roster(query),
    }
    show(slow)

    expect(await screen.findByText('Загружаем журнал…')).toBeInTheDocument()
  })
})

describe('the schedule', () => {
  it('names the patients it carries, whoever is on the roster page', async () => {
    show()

    const section = within(await screen.findByRole('region', { name: 'Расписание' }))
    const offPage = OVERVIEW.schedule.find(
      (entry) => !PATIENTS.slice(0, PAGE_SIZE).some((patient) => patient.id === entry.patientId),
    )

    // Deliberately one the roster is not showing: that is the case that used to print an id.
    expect(offPage, 'the fixture schedules somebody off the first page').toBeDefined()
    expect(section.getByText(offPage!.patientName)).toBeInTheDocument()
    expect(section.queryByText(offPage!.patientId)).toBeNull()
  })

  it('tells the states apart', async () => {
    show()

    const section = within(await screen.findByRole('region', { name: 'Расписание' }))

    expect(section.getAllByText('выполнено').length).toBe(
      OVERVIEW.schedule.filter((entry) => entry.state === 'done').length,
    )
    expect(section.getAllByText('предстоит').length).toBe(
      OVERVIEW.schedule.filter((entry) => entry.state === 'due').length,
    )
  })
})

describe('the triage row', () => {
  it('draws the patients the seam chose', async () => {
    show()

    const section = within(await screen.findByRole('region', { name: 'Требуют внимания' }))

    for (const patient of OVERVIEW_TRIAGE) {
      expect(section.getByText(patient)).toBeInTheDocument()
    }
  })

  it('opens the card from a triage tile', async () => {
    show()
    const user = userEvent.setup()

    const section = within(await screen.findByRole('region', { name: 'Требуют внимания' }))
    const first = OVERVIEW_TRIAGE[0]!

    await user.click(section.getByRole('button', { name: first }))

    expect(await screen.findByRole('complementary', { name: `Карточка: ${first}` })).toBeInTheDocument()
  })
})

describe('the dose', () => {
  // Two decimal places, asserted at the call site. Dropping the `2` is one token and renders «0,3 мг»
  // for a protocol that says 0,25 мг — a different dose, on both surfaces of the screen — and it passed
  // every test, because format.test.ts pins the function and nothing pinned the caller. This is the
  // shape the weight fix closed, reopened by the fix for it.
  // Two decimals and not merely fractional: 0,5 мг survives one place, 0,25 мг does not, and it is the
  // second that tells the two settings apart.
  const fractional = PATIENTS.find(
    (patient) => (String(patient.dose.value).split('.')[1]?.length ?? 0) > 1,
  )

  it('is written to the precision the protocol uses', async () => {
    expect(fractional, 'the fixture prescribes a dose needing two decimals').toBeDefined()

    show()
    const user = userEvent.setup()

    const written = quantity(fractional!.dose.value, fractional!.dose.unit, 2)
    const journal = within(await screen.findByRole('region', { name: 'Журнал протоколов' }))

    await user.click(await journal.findByText(fractional!.name))
    const card = await screen.findByRole('complementary', { name: `Карточка: ${fractional!.name}` })

    expect(within(card).getByText(new RegExp(written))).toBeInTheDocument()
  })
})

describe('the patient card', () => {
  it('opens on a row and carries what the seam worked out', async () => {
    show()
    const user = userEvent.setup()

    // The second row and not the first: `onOpen(items[0])` would satisfy a click on row one, and
    // «the card that opens is the patient that was clicked» is the property.
    const patient = PATIENTS[1]
    const journalRegion = await screen.findByRole('region', { name: 'Журнал протоколов' })
    const journal = within(journalRegion)

    await user.click(await journal.findByText(patient?.name ?? ''))

    const card = await screen.findByRole('complementary', { name: `Карточка: ${patient?.name ?? ''}` })

    // The sentence and not the bare number: a regex of the figure alone matches the cycle week, the
    // adherence and half the biomarkers. Built through the formatter, because interpolating the raw
    // value is what made this assertion pass with the formatter removed.
    expect(within(card).getByText(`${whole(patient!.goalProgressPct)}% пути к цели`)).toBeInTheDocument()
    expect(
      within(card).getByText(
        `↓ ${quantity(patient!.lostKg, patient!.unit)} с начала · цель ${quantity(patient!.goal, patient!.unit)}`,
      ),
    ).toBeInTheDocument()

    // The formatter at the call site, which is the half no test covered: measured, taking `quantity()`
    // back out of the roster and the card left all 285 tests green, because the only patient any
    // assertion reached had whole numbers.
    expect(patient?.weight).not.toBeCloseTo(Math.round(patient?.weight ?? 0), 5)
    expect(within(card).getByText(new RegExp(`${quantity(patient!.weight, patient!.unit)}`))).toBeInTheDocument()
    expect(
      within(journalRegion).getByText(new RegExp(quantity(patient!.weight, patient!.unit))),
    ).toBeInTheDocument()

    await user.click(within(card).getByRole('button', { name: 'Закрыть карточку' }))
    // By name: the side menu is a landmark of the same kind, and an unnamed query would answer «still
    // there» about it for ever.
    await waitFor(() =>
      expect(screen.queryByRole('complementary', { name: /^Карточка:/ })).toBeNull(),
    )
  })
})
