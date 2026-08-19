import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import type { NewPatientBody, StaffPage } from '../../api'
import { ApiRefusal, type ApiClient } from '../../data/api'
import { stubApi } from '../../data/api.stub'
import { defaultClient } from '../../data/queries'
import { NewPatientForm } from './new-patient-form'

const ME = { sub: 'the-doctor', role: 'doctor', expires_at: '2026-08-20T12:00:00Z', full_name: 'Ксения Первеева' }

const STAFF: StaffPage = {
  staff: [
    { user_id: 'the-doctor', full_name: 'Ксения Первеева', role: 'doctor', title_ru: 'Эндокринолог' },
    { user_id: 'the-dietitian', full_name: 'Мария Светова', role: 'doctor', title_ru: 'Диетолог' },
  ],
}

function show(api: ApiClient, onCreated = () => undefined) {
  return render(
    <QueryClientProvider client={defaultClient()}>
      <NewPatientForm api={api} me={ME} onCreated={onCreated} onClose={() => undefined} />
    </QueryClientProvider>,
  )
}

async function fillIn(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText('Имя и фамилия'), 'Марина Волкова')
  await user.type(screen.getByLabelText('Почта'), 'marina@clinic.example')
}

describe('creating a patient', () => {
  it('sends everything the endpoint accepts', async () => {
    const user = userEvent.setup()
    const created = vi.fn((patient: NewPatientBody) => Promise.resolve({ user_id: `created-${patient.email}` }))

    show(stubApi({ staff: () => Promise.resolve(STAFF), createPatient: created }))

    await fillIn(user)
    await user.type(screen.getByLabelText('Дата рождения'), '1988-03-14')
    await user.selectOptions(screen.getByLabelText('Пол'), 'female')
    await user.type(screen.getByLabelText('Рост, см'), '188')
    await user.type(screen.getByLabelText('Целевой вес, кг'), '102')

    await user.click(screen.getByRole('button', { name: 'Создать и пригласить' }))

    await waitFor(() => expect(created).toHaveBeenCalledOnce())

    const sent = created.mock.calls[0]?.[0]
    expect(sent?.full_name).toBe('Марина Волкова')
    expect(sent?.email).toBe('marina@clinic.example')
    expect(sent?.date_of_birth).toBe('1988-03-14')
    expect(sent?.sex).toBe('female')
    expect(sent?.height_cm).toBe(188)
    expect(sent?.target_weight_kg).toBe(102)
  })

  // The API refuses a doctor who is not on the care team they wrote, so the form puts them there —
  // and their care role is theirs to state, because a doctor is not always the endocrinologist.
  it('puts the doctor creating the patient on the care team, as the specialist they say they are', async () => {
    const user = userEvent.setup()
    const created = vi.fn((patient: NewPatientBody) => Promise.resolve({ user_id: `created-${patient.email}` }))

    show(stubApi({ staff: () => Promise.resolve(STAFF), createPatient: created }))

    await fillIn(user)
    await user.click(screen.getByRole('button', { name: 'Создать и пригласить' }))

    await waitFor(() => expect(created).toHaveBeenCalledOnce())

    const sent = created.mock.calls[0]?.[0]
    expect(sent?.specialists).toEqual([{ provider_id: 'the-doctor', care_role: 'endo', primary: true }])
  })

  it('adds a second specialist from the clinic, with a care role of their own', async () => {
    const user = userEvent.setup()
    const created = vi.fn((patient: NewPatientBody) => Promise.resolve({ user_id: `created-${patient.email}` }))

    show(stubApi({ staff: () => Promise.resolve(STAFF), createPatient: created }))

    await fillIn(user)
    await user.selectOptions(await screen.findByLabelText('Ещё специалист'), 'the-dietitian')
    await user.selectOptions(screen.getByLabelText('Роль второго специалиста'), 'dietitian')

    await user.click(screen.getByRole('button', { name: 'Создать и пригласить' }))

    await waitFor(() => expect(created).toHaveBeenCalledOnce())

    const sent = created.mock.calls[0]?.[0]
    expect(sent?.specialists).toEqual([
      { provider_id: 'the-doctor', care_role: 'endo', primary: true },
      { provider_id: 'the-dietitian', care_role: 'dietitian', primary: false },
    ])
  })

  // Exactly one primary is the schema's rule and the database's; the form cannot offer a way to
  // break it, so the second specialist is never primary.
  it('leaves the demographics out when the clinic did not state them', async () => {
    const user = userEvent.setup()
    const created = vi.fn((patient: NewPatientBody) => Promise.resolve({ user_id: `created-${patient.email}` }))

    show(stubApi({ staff: () => Promise.resolve(STAFF), createPatient: created }))

    await fillIn(user)
    await user.click(screen.getByRole('button', { name: 'Создать и пригласить' }))

    await waitFor(() => expect(created).toHaveBeenCalledOnce())

    const sent = created.mock.calls[0]?.[0]
    for (const absent of ['date_of_birth', 'sex', 'height_cm', 'target_weight_kg'] as const) {
      expect(sent?.[absent], `${absent} was sent anyway`).toBeUndefined()
    }
  })
})

describe('what the form does with a refusal', () => {
  it('says in Russian that the address is already somebody', async () => {
    const user = userEvent.setup()

    show(
      stubApi({
        staff: () => Promise.resolve(STAFF),
        createPatient: () => Promise.reject(new ApiRefusal(409, 'Адрес занят.')),
      }),
    )

    await fillIn(user)
    await user.click(screen.getByRole('button', { name: 'Создать и пригласить' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Этот адрес уже принадлежит')
  })

  it('says in Russian that the invitation could not be sent, and that the patient was not created', async () => {
    const user = userEvent.setup()

    show(
      stubApi({
        staff: () => Promise.resolve(STAFF),
        createPatient: () => Promise.reject(new ApiRefusal(503, 'Сервис временно недоступен.')),
      }),
    )

    await fillIn(user)
    await user.click(screen.getByRole('button', { name: 'Создать и пригласить' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Приглашение не отправлено')
  })

  // A refusal the form has no sentence for still has to say something a person can act on.
  it('falls back to what the API wrote', async () => {
    const user = userEvent.setup()

    show(
      stubApi({
        staff: () => Promise.resolve(STAFF),
        createPatient: () => Promise.reject(new Error('Электронный адрес не распознан.')),
      }),
    )

    await fillIn(user)
    await user.click(screen.getByRole('button', { name: 'Создать и пригласить' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Электронный адрес не распознан.')
  })
})

// An invitation cannot be unsent. A second click while the first is in flight is a second patient,
// a second mailbox, and an address the clinic then cannot invite again.
describe('a second click', () => {
  it('does not send a second invitation', async () => {
    const user = userEvent.setup()
    let inFlight: () => void = () => undefined
    const created = vi.fn(
      () =>
        new Promise<{ user_id: string }>((resolve) => {
          inFlight = () => resolve({ user_id: 'the-new-patient' })
        }),
    )

    show(stubApi({ staff: () => Promise.resolve(STAFF), createPatient: created }))

    await fillIn(user)

    const submit = screen.getByRole('button', { name: 'Создать и пригласить' })
    await user.click(submit)
    await waitFor(() => expect(submit).toBeDisabled())
    await user.click(submit)

    expect(created).toHaveBeenCalledOnce()

    // Settled at the end so the promise does not outlive the test and warn about a state update
    // after it finished.
    inFlight()
  })
})

describe('when the patient is created', () => {
  it('tells the screen above, so the roster can show them', async () => {
    const user = userEvent.setup()
    const onCreated = vi.fn()

    show(
      stubApi({ staff: () => Promise.resolve(STAFF), createPatient: () => Promise.resolve({ user_id: 'new' }) }),
      onCreated,
    )

    await fillIn(user)
    await user.click(screen.getByRole('button', { name: 'Создать и пригласить' }))

    await waitFor(() => expect(onCreated).toHaveBeenCalledOnce())
  })
})

// An administrator may create a patient without being on their care team — and may not be on one at
// all: a specialist has to be a doctor, which the API reads off the profile and refuses. So the form
// asks them who leads instead of putting them there.
describe('an administrator filling the form in', () => {
  const ADMIN = { sub: 'the-admin', role: 'admin', expires_at: '2026-08-21T12:00:00Z', full_name: 'Пётр Аверин' }

  function showAsAdmin(api: ApiClient) {
    return render(
      <QueryClientProvider client={defaultClient()}>
        <NewPatientForm api={api} me={ADMIN} onCreated={() => undefined} onClose={() => undefined} />
      </QueryClientProvider>,
    )
  }

  it('names a doctor as the leading specialist, and is not on the team themselves', async () => {
    const user = userEvent.setup()
    const created = vi.fn((patient: NewPatientBody) => Promise.resolve({ user_id: `created-${patient.email}` }))

    showAsAdmin(
      stubApi({
        staff: () =>
          Promise.resolve({
            staff: [...STAFF.staff, { user_id: 'the-admin', full_name: 'Пётр Аверин', role: 'admin', title_ru: null }],
          }),
        createPatient: created,
      }),
    )

    await user.type(screen.getByLabelText('Имя и фамилия'), 'Марина Волкова')
    await user.type(screen.getByLabelText('Почта'), 'marina@clinic.example')
    await user.selectOptions(await screen.findByLabelText('Ведущий специалист'), 'the-doctor')

    await user.click(screen.getByRole('button', { name: 'Создать и пригласить' }))

    await waitFor(() => expect(created).toHaveBeenCalledOnce())

    const sent = created.mock.calls[0]?.[0]
    expect(sent?.specialists).toEqual([{ provider_id: 'the-doctor', care_role: 'endo', primary: true }])
  })

  // The administrator is staff and the list carries them; a picker of specialists must not.
  it('is not offered themselves, or any other administrator, as a specialist', async () => {
    showAsAdmin(
      stubApi({
        staff: () =>
          Promise.resolve({
            staff: [...STAFF.staff, { user_id: 'the-admin', full_name: 'Пётр Аверин', role: 'admin', title_ru: null }],
          }),
      }),
    )

    const lead = await screen.findByLabelText('Ведущий специалист')

    // Awaited: the list is a request, and the options appear when it answers.
    expect(await within(lead).findByRole('option', { name: /Ксения Первеева/ })).toBeInTheDocument()
    expect(within(lead).queryByRole('option', { name: /Пётр Аверин/ })).toBeNull()
  })
})
