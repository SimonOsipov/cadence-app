import { expect, test } from '@playwright/test'

import { CLEANUP_PREFIX, runId } from './cleanup'

/**
 * The critical path, through a browser, against the whole stack: sign in, read the clinic's roster,
 * take a patient on, and see them on it.
 *
 * Everything below the browser is real — the API, the identity provider, the provisioner and the
 * database — which is what makes this the one test that would notice a seam nobody wired up. What it
 * costs is that it cannot run anywhere the stack does not: see playwright.config.ts.
 */
const DOCTOR = process.env.SMOKE_DOCTOR_EMAIL ?? 'ksenia@clinic.example'
const PASSWORD = process.env.SEED_PASSWORD ?? 'a-seeded-password-nobody-uses'

async function signIn(page: import('@playwright/test').Page) {
  await page.goto('/')

  await page.getByLabel('Почта').fill(DOCTOR)
  await page.getByLabel('Пароль').fill(PASSWORD)
  await page.getByRole('button', { name: 'Войти' }).click()

  await expect(page.getByRole('heading', { level: 1 })).toContainText('Здравствуйте')
}

test('a doctor signs in, takes a patient on, and finds them on the roster', async ({ page }) => {
  await signIn(page)

  const journal = page.getByRole('region', { name: 'Журнал протоколов' })
  await expect(journal.getByRole('row').first()).toBeVisible()

  // The address carries the run that made it, which is what lets the teardown find exactly what this
  // run created and leave the seeded clinic alone.
  const address = `${CLEANUP_PREFIX}${runId()}@clinic.example`
  const name = `Смоук Тестовна ${runId()}`

  await page.getByRole('button', { name: 'Новый пациент' }).click()

  const form = page.getByRole('region', { name: 'Новый пациент' })
  await form.getByLabel('Имя и фамилия').fill(name)
  await form.getByLabel('Почта').fill(address)
  await form.getByRole('button', { name: 'Создать и пригласить' }).click()

  // The form closes when the patient exists, and the roster is asked again. The new patient sorts
  // wherever Russian collation puts them, which may be a page away — so the assertion is on the API's
  // own answer for that address rather than on the first page in front of us.
  await expect(form).toBeHidden()

  const roster = await page.evaluate(
    async ({ apiUrl }: { apiUrl: string }) => {
      const session = JSON.parse(sessionStorage.getItem('cadence.session') ?? '{}') as { accessToken?: string }
      const answer = await fetch(`${apiUrl}/v1/dashboard/overview?limit=100`, {
        headers: { Authorization: `Bearer ${session.accessToken ?? ''}` },
      })
      const body = (await answer.json()) as { patients: { full_name: string; invite_state: string }[] }

      return body.patients
    },
    { apiUrl: process.env.SMOKE_API_URL ?? 'http://localhost:8080' },
  )

  const created = roster.find((patient) => patient.full_name === name)

  expect(created, 'the patient this run created is not on the roster the API answers').toBeDefined()
  // Invited and nothing more, which is what a patient the clinic has just taken on looks like.
  expect(created?.invite_state).toBe('pending')
})

test('a patient is refused the dashboard', async ({ page }) => {
  await page.goto('/')

  await page.getByLabel('Почта').fill(process.env.SMOKE_PATIENT_EMAIL ?? 'marina-volkova@clinic.example')
  await page.getByLabel('Пароль').fill(PASSWORD)
  await page.getByRole('button', { name: 'Войти' }).click()

  await expect(page.getByRole('alert')).toContainText('для сотрудников клиники')
})

test('signing out leaves nothing behind', async ({ page }) => {
  await signIn(page)

  await page.getByRole('button', { name: 'Выйти' }).click()

  await expect(page.getByRole('heading', { name: 'Кабинет врача' })).toBeVisible()
  expect(await page.evaluate(() => sessionStorage.getItem('cadence.session'))).toBeNull()
})
