import type { ApiClient } from './api'

/**
 * An API a test can hand a screen, answering only what that test is about.
 *
 * Everything else refuses by name rather than by returning something empty: a screen that reaches for
 * a call the test did not arrange should say so, not draw a clinic with nobody in it.
 */
export function stubApi(answers: Partial<ApiClient> = {}): ApiClient {
  const refuse = (call: string) => () => Promise.reject(new Error(`this test does not answer ${call}`))

  return {
    me: answers.me ?? refuse('me'),
    roster: answers.roster ?? refuse('roster'),
    staff: answers.staff ?? refuse('staff'),
    createPatient: answers.createPatient ?? refuse('createPatient'),
  }
}
