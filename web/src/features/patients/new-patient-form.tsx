import { useMutation, useQuery } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'

import type { Me, NewPatientBody, SpecialistBody } from '../../api'
import type { ApiClient } from '../../data/api'
import { tokens } from '../../tokens/tokens'

/** The three a care team can be made of, in the words the clinic uses for them. */
const CARE_ROLES: { id: SpecialistBody['care_role']; label: string }[] = [
  { id: 'endo', label: 'Эндокринолог' },
  { id: 'dietitian', label: 'Диетолог' },
  { id: 'nurse', label: 'Медсестра' },
]

/**
 * What the clinic says when it takes a patient on.
 *
 * Creating and inviting are one action at the API — the identifier exists only after the provider has
 * been asked — so this form's submit sends an email nobody can unsend. That is why the button is
 * disabled the moment it is pressed rather than after the answer: a second click is a second patient,
 * a second mailbox, and an address the clinic cannot then invite again.
 */
export function NewPatientForm({
  api,
  me,
  onCreated,
  onClose,
}: {
  api: ApiClient
  me: Me
  onCreated: () => void
  onClose: () => void
}) {
  const [fullName, setFullName] = useState('')
  const [email, setEmail] = useState('')
  const [careRole, setCareRole] = useState<SpecialistBody['care_role']>('endo')
  const [secondId, setSecondId] = useState('')
  const [secondRole, setSecondRole] = useState<SpecialistBody['care_role']>('dietitian')
  const [dob, setDob] = useState('')
  const [sex, setSex] = useState('')
  const [heightCm, setHeightCm] = useState('')
  const [targetWeightKg, setTargetWeightKg] = useState('')

  // The colleagues a care team may name. No policy lets a doctor read them, so the API answers this
  // through a route of its own; a form with nobody in the list is a care team of one.
  const staff = useQuery({ queryKey: ['staff'], queryFn: () => api.staff(), retry: false })

  // Only a doctor may be a specialist — measured: CreatePatient reads the profile and refuses any
  // other role. So an administrator, who may create a patient without being on the care team, names
  // who leads it instead of being put there themselves.
  const [leadId, setLeadId] = useState('')
  const iAmADoctor = me.role === 'doctor'

  const create = useMutation({
    mutationFn: (patient: NewPatientBody) => api.createPatient(patient),
    onSuccess: onCreated,
  })

  function submit(event: FormEvent) {
    event.preventDefault()

    // The one who leads, first and primary: the doctor filling the form in, or — for an
    // administrator, who is not a specialist and may not be put on a care team — the doctor they
    // named. The API refuses a doctor who is not on the care team they wrote, and at most one
    // specialist may lead, so the second is never primary and the form offers no way to make them one.
    const primaryId = iAmADoctor ? me.sub : leadId
    const specialists: SpecialistBody[] = [{ provider_id: primaryId, care_role: careRole, primary: true }]
    if (secondId !== '' && secondId !== primaryId) {
      specialists.push({ provider_id: secondId, care_role: secondRole, primary: false })
    }

    create.mutate({
      full_name: fullName.trim(),
      email: email.trim(),
      specialists,
      // Absent rather than empty: the clinical fields are «not measured yet» when the clinic did not
      // state them, and an empty string or a nought is a measurement nobody took.
      ...(dob === '' ? {} : { date_of_birth: dob }),
      ...(sex === '' ? {} : { sex: sex as NonNullable<NewPatientBody['sex']> }),
      ...(heightCm === '' ? {} : { height_cm: Number(heightCm) }),
      ...(targetWeightKg === '' ? {} : { target_weight_kg: Number(targetWeightKg) }),
    })
  }

  // Doctors only, for the same measured reason: an administrator offered in a picker of specialists
  // is a care team the database refuses after the invitation has already gone out.
  const doctors = (staff.data?.staff ?? []).filter((member) => member.role === 'doctor')
  const colleagues = doctors.filter((member) => member.user_id !== me.sub && member.user_id !== leadId)

  return (
    <section
      aria-label="Новый пациент"
      style={{
        background: tokens.paper,
        border: `1px solid ${tokens.bone}`,
        borderRadius: tokens.rLg,
        padding: '22px 24px',
        marginBottom: 28,
        fontFamily: tokens.fontBody,
        color: tokens.ink900,
      }}
    >
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 16 }}>
        <h2 style={{ fontFamily: tokens.fontDisplay, fontSize: 26, fontWeight: 400, margin: 0 }}>Новый пациент</h2>
        <button type="button" onClick={onClose} style={ghostButton}>
          Отмена
        </button>
      </header>

      <form onSubmit={submit} style={{ display: 'grid', gap: 14, gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' }}>
        <Field label="Имя и фамилия" value={fullName} onChange={setFullName} required />
        <Field label="Почта" value={email} onChange={setEmail} type="email" required />

        {!iAmADoctor && (
          <label style={label}>
            Ведущий специалист
            <select value={leadId} required onChange={(event) => setLeadId(event.target.value)} style={field}>
              <option value="">Выберите врача</option>
              {doctors.map((member) => (
                <option key={member.user_id} value={member.user_id}>
                  {member.full_name}
                  {member.title_ru === null || member.title_ru === undefined ? '' : ` · ${member.title_ru}`}
                </option>
              ))}
            </select>
          </label>
        )}

        <label style={label}>
          {iAmADoctor ? 'Кто вы для пациента' : 'Роль ведущего специалиста'}
          <select value={careRole} onChange={(event) => setCareRole(event.target.value as SpecialistBody['care_role'])} style={field}>
            {CARE_ROLES.map((role) => (
              <option key={role.id} value={role.id}>
                {role.label}
              </option>
            ))}
          </select>
        </label>

        <label style={label}>
          Ещё специалист
          <select value={secondId} onChange={(event) => setSecondId(event.target.value)} style={field}>
            <option value="">Никого</option>
            {colleagues.map((member) => (
              <option key={member.user_id} value={member.user_id}>
                {member.full_name}
                {member.title_ru === null || member.title_ru === undefined ? '' : ` · ${member.title_ru}`}
              </option>
            ))}
          </select>
        </label>

        {secondId !== '' && (
          <label style={label}>
            Роль второго специалиста
            <select
              value={secondRole}
              onChange={(event) => setSecondRole(event.target.value as SpecialistBody['care_role'])}
              style={field}
            >
              {CARE_ROLES.map((role) => (
                <option key={role.id} value={role.id}>
                  {role.label}
                </option>
              ))}
            </select>
          </label>
        )}

        <Field label="Дата рождения" value={dob} onChange={setDob} type="date" />

        <label style={label}>
          Пол
          <select value={sex} onChange={(event) => setSex(event.target.value)} style={field}>
            <option value="">Не указан</option>
            <option value="female">Женский</option>
            <option value="male">Мужской</option>
          </select>
        </label>

        <Field label="Рост, см" value={heightCm} onChange={setHeightCm} type="number" />
        <Field label="Целевой вес, кг" value={targetWeightKg} onChange={setTargetWeightKg} type="number" />

        {staff.isError && (
          <p style={{ ...noticeStyle, gridColumn: '1 / -1' }}>
            Не удалось прочитать список сотрудников — второго специалиста можно будет добавить позже.
          </p>
        )}

        {create.isError && (
          <p role="alert" style={{ ...refusalStyle, gridColumn: '1 / -1' }}>
            {refusalFor(create.error)}
          </p>
        )}

        <div style={{ gridColumn: '1 / -1' }}>
          <button type="submit" disabled={create.isPending} style={submitButton}>
            {create.isPending ? 'Создаём…' : 'Создать и пригласить'}
          </button>
        </div>
      </form>
    </section>
  )
}

/**
 * What a refusal of this endpoint means to the person who filled the form in.
 *
 * Two of them are named because two of them are ordinary: an address the clinic already knows, and a
 * provisioner that is not answering. The second one matters most — nothing was created, and a doctor
 * who believes otherwise waits for a patient who was never invited.
 */
export function refusalFor(error: unknown): string {
  const status = typeof error === 'object' && error !== null && 'status' in error ? Number(error.status) : 0

  if (status === 409) {
    return 'Этот адрес уже принадлежит пациенту или аккаунту, который клиника не приглашала.'
  }
  if (status === 503) {
    return 'Приглашение не отправлено: сервис недоступен. Пациент не создан — повторите попытку.'
  }

  return error instanceof Error && error.message !== '' ? error.message : 'Не удалось создать пациента.'
}

function Field({
  label: caption,
  value,
  onChange,
  type = 'text',
  required = false,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  type?: string
  required?: boolean
}) {
  return (
    <label style={label}>
      {caption}
      <input
        type={type}
        value={value}
        required={required}
        onChange={(event) => onChange(event.target.value)}
        style={field}
      />
    </label>
  )
}

const label = { display: 'grid', gap: 6, fontSize: 13, color: tokens.ink600 } as const

const field = {
  padding: '9px 11px',
  borderRadius: 8,
  border: `1px solid ${tokens.ink300}`,
  background: tokens.cream,
  font: 'inherit',
  color: tokens.ink900,
}

const submitButton = {
  padding: '11px 18px',
  borderRadius: 8,
  border: 'none',
  background: tokens.forest700,
  color: tokens.paper,
  font: 'inherit',
  cursor: 'pointer',
}

const ghostButton = {
  padding: '7px 13px',
  borderRadius: tokens.rPill,
  border: `1px solid ${tokens.borderStrong}`,
  background: 'transparent',
  color: tokens.ink600,
  font: 'inherit',
  cursor: 'pointer',
}

const refusalStyle = {
  margin: 0,
  color: tokens.danger,
  background: tokens.dangerBg,
  padding: '10px 12px',
  borderRadius: 8,
}

const noticeStyle = { margin: 0, color: tokens.ink600, fontSize: 13 }
