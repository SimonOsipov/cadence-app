import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { DataProvider, defaultClient } from '../../data/queries'
import { OVERVIEW, PATIENTS, TRIAGE_IDS } from '../../data/fixtures/overview'

const OVERVIEW_TRIAGE = TRIAGE_IDS.map(
  (id) => PATIENTS.find((patient) => patient.id === id)?.name ?? id,
)
import { quantity, whole } from '../../format'
import type { Transport } from '../../data/transport'
import { fixtureTransport } from '../../data/transport'
import type { RosterPage, RosterRow } from '../../api'
import type { ApiClient } from '../../data/api'
import { stubApi } from '../../data/api.stub'
import { OverviewPage } from './overview-page'
import { PatientCard } from './patient-card'

// Two seams now, because the screen reads two: the roster is live and the five sections around it
// are still the fixture transport's. A transport and a client per test — React Query caches across
// renders, and a shared client makes one test's answer the next test's starting point.
const LIVE_PATIENTS: RosterRow[] = [
  { user_id: 'a', full_name: 'Анна Петрова', age: 41, invite_state: 'accepted' },
  { user_id: 'b', full_name: 'Борис Ким', age: 47, invite_state: 'pending' },
]

function liveRoster(page: Partial<RosterPage> = {}): ApiClient {
  return stubApi({ roster: () => Promise.resolve({ patients: LIVE_PATIENTS, ...page }) })
}

function show(transport: Transport = fixtureTransport(), api: ApiClient = liveRoster()) {
  return render(
    <DataProvider transport={transport} api={api} client={defaultClient()}>
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
    // The strip counts the clinic and the roster beside it carries two rows: a strip summing what is
    // on screen would say two, and it says what the seam sent.
    expect(aggregates.patients).toBeGreaterThan(LIVE_PATIENTS.length)
    expect(within(card('Регулярность')).getByText(String(aggregates.averageAdherence))).toBeInTheDocument()
    expect(
      within(card('Дозы сегодня')).getByText(`${aggregates.dosesDone}/${aggregates.dosesToday}`),
    ).toBeInTheDocument()
  })

  // The roster's status tabs counted the same aggregates and are gone with them: the live roster has
  // no status to filter by until M6 answers one, and a tab that filters nothing is the dead control
  // invariant 4 forbids.
})

describe('the roster', () => {
  it('draws the patients the API answered, and not the fixture beside them', async () => {
    show()

    const journal = within(await screen.findByRole('region', { name: 'Журнал протоколов' }))

    expect(await journal.findByText('Анна Петрова')).toBeInTheDocument()
    expect(journal.queryByText(PATIENTS[0]?.name ?? '')).toBeNull()
  })

  it('walks to the next page by the cursor the answer carried', async () => {
    const user = userEvent.setup()
    const asked: (string | undefined)[] = []

    show(fixtureTransport(), stubApi({
      roster: ({ cursor }) => {
        asked.push(cursor)

        return Promise.resolve(
          cursor === undefined
            ? { patients: LIVE_PATIENTS, next_cursor: 'after-boris' }
            : { patients: [{ user_id: 'c', full_name: 'Вера Зорина', age: 45, invite_state: 'unknown' as const }] },
        )
      },
    }))

    const journal = within(await screen.findByRole('region', { name: 'Журнал протоколов' }))
    await journal.findByText('Анна Петрова')

    await user.click(screen.getByRole('button', { name: /Дальше/ }))

    expect(await journal.findByText('Вера Зорина')).toBeInTheDocument()
    expect(asked).toEqual([undefined, 'after-boris'])
  })

  // A clinic that has not created anybody yet, which is the state every new deployment starts in.
  it('says so when the clinic has nobody', async () => {
    show(fixtureTransport(), liveRoster({ patients: [] }))

    expect(await screen.findByText(/Пациентов пока нет/)).toBeInTheDocument()
  })

  // The five fixture sections are not the roster's to take down: what failed is one request.
  it('keeps the rest of the screen when only the roster fails', async () => {
    show(fixtureTransport(), stubApi({ roster: () => Promise.reject(new Error('журнал недоступен')) }))

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('журнал недоступен')

    // The pager survives the failure: a doctor whose journal failed still has to be able to step back
    // to the first page.
    expect(screen.getByRole('button', { name: 'В начало' })).toBeInTheDocument()
    expect(screen.getByRole('region', { name: 'Расписание' })).toBeInTheDocument()
  })

  it('shows its own line while the page is on its way', async () => {
    show(fixtureTransport(), stubApi({ roster: () => new Promise(() => undefined) }))

    expect(await screen.findByText('Загружаем журнал…')).toBeInTheDocument()
  })
})

describe('the schedule', () => {
  it('names the patients it carries, whoever is on the roster page', async () => {
    show()

    const section = within(await screen.findByRole('region', { name: 'Расписание' }))
    // Nobody the roster is showing — it is a live page of other people entirely — which is the case
    // that used to print an id.
    const offPage = OVERVIEW.schedule[0]

    expect(offPage, 'the fixture schedules somebody').toBeDefined()
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
  // Two decimals and not merely fractional: 0,5 мг survives one place, 0,25 мг does not, and it is the
  // second that tells the two settings apart. Dropping the `2` is one token, renders «0,3 мг» for a
  // 0,25 мг protocol and passed every test — format.test.ts pins the function, nothing pinned the
  // caller. The card is the one surface left that writes a dose: the roster's protocol column is
  // M6's, and it left with the fixture the row came from.
  const fractional = PATIENTS.find(
    (patient) => (String(patient.dose.value).split('.')[1]?.length ?? 0) > 1,
  )

  it('is written to the precision the protocol uses', async () => {
    expect(fractional, 'the fixture prescribes a dose needing two decimals').toBeDefined()

    // The card on its own, because the patient prescribed such a dose is not one triage carries and
    // the roster no longer writes a dose at all. What has to stay measured is the call site.
    render(<PatientCard patient={fractional!} onClose={() => undefined} />)

    const written = quantity(fractional!.dose.value, fractional!.dose.unit, 2)
    const card = await screen.findByRole('complementary', { name: `Карточка: ${fractional!.name}` })

    expect(within(card).getByText(new RegExp(written)), 'the card writes it').toBeInTheDocument()
  })
})

describe('the patient card', () => {
  it('opens on a triage tile and carries what the seam worked out', async () => {
    show()
    const user = userEvent.setup()

    // The second tile and not the first: `onOpen(patients[0])` would satisfy a click on tile one, and
    // «the card that opens is the patient that was clicked» is the property.
    const patient = PATIENTS.find((candidate) => candidate.id === TRIAGE_IDS[1])
    const triage = within(await screen.findByRole('region', { name: 'Требуют внимания' }))

    await user.click(await triage.findByText(patient?.name ?? ''))

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

    await user.click(within(card).getByRole('button', { name: 'Закрыть карточку' }))
    // By name: the side menu is a landmark of the same kind, and an unnamed query would answer «still
    // there» about it for ever.
    await waitFor(() =>
      expect(screen.queryByRole('complementary', { name: /^Карточка:/ })).toBeNull(),
    )
  })
})

// The screen is mixed on purpose — one live section and the rest on invented numbers — and the whole
// requirement is that nobody has to guess which is which. See DemoMark for why it is on the screen
// rather than in a comment.
describe('what is still a fixture', () => {
  it('is marked on the screen, section by section, with the release it comes off at', async () => {
    show()

    await screen.findByRole('region', { name: 'Требуют внимания' })

    const marks = screen.getAllByText(/Демо-данные · до M6/)
    expect(marks.length).toBeGreaterThanOrEqual(3)

    for (const section of ['Требуют внимания', 'Расписание']) {
      const region = within(screen.getByRole('region', { name: section }))
      expect(region.getByText(/Демо-данные/), `${section} is not marked`).toBeInTheDocument()
    }
  })

  // The one section that is not a fixture must not carry the mark, or the mark says nothing.
  it('does not mark the roster, which is live', async () => {
    show()

    const journal = within(await screen.findByRole('region', { name: 'Журнал протоколов' }))
    await journal.findByText('Анна Петрова')

    expect(journal.queryByText(/Демо-данные/)).toBeNull()
  })

  it('marks the card a triage tile opens', async () => {
    show()
    const user = userEvent.setup()

    const patient = PATIENTS.find((candidate) => candidate.id === TRIAGE_IDS[0])
    const triage = within(await screen.findByRole('region', { name: 'Требуют внимания' }))

    await user.click(await triage.findByText(patient?.name ?? ''))

    const card = await screen.findByRole('complementary', { name: `Карточка: ${patient?.name ?? ''}` })
    expect(within(card).getByText(/Демо-данные/)).toBeInTheDocument()
  })
})

// The button the prototype draws and this MVP could not honour until the endpoint existed.
describe('taking a patient on', () => {
  const ME = {
    sub: 'the-doctor',
    role: 'doctor',
    expires_at: '2026-08-21T12:00:00Z',
    full_name: 'Ксения Первеева',
  }

  it('is not offered to a screen that does not know who is signed in', async () => {
    show()

    await screen.findByRole('region', { name: 'Журнал протоколов' })
    expect(screen.queryByRole('button', { name: 'Новый пациент' })).toBeNull()
  })

  it('opens the form, and asks the roster again once the patient exists', async () => {
    const user = userEvent.setup()
    let asked = 0

    const api = stubApi({
      roster: () => {
        asked += 1

        return Promise.resolve({ patients: LIVE_PATIENTS })
      },
      staff: () => Promise.resolve({ staff: [] }),
      createPatient: () => Promise.resolve({ user_id: 'the-new-patient' }),
    })

    render(
      <DataProvider transport={fixtureTransport()} api={api} client={defaultClient()}>
        <OverviewPage me={ME} />
      </DataProvider>,
    )

    await user.click(await screen.findByRole('button', { name: 'Новый пациент' }))

    const form = within(await screen.findByRole('region', { name: 'Новый пациент' }))
    await user.type(form.getByLabelText('Имя и фамилия'), 'Марина Волкова')
    await user.type(form.getByLabelText('Почта'), 'marina@clinic.example')

    const before = asked
    await user.click(form.getByRole('button', { name: 'Создать и пригласить' }))

    // The form closes and the roster is read again: the new patient is on a page the server chooses,
    // and a row added by hand here would be this screen deciding where the server puts them.
    await waitFor(() => expect(screen.queryByRole('region', { name: 'Новый пациент' })).toBeNull())
    await waitFor(() => expect(asked).toBeGreaterThan(before))
  })
})
