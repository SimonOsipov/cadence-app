import { act, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { OpenInAppPage, THE_APP_DID_NOT_ANSWER, WAIT_FOR_THE_APP_MS, tokenFromFragment } from './open-in-app-page'

const TOKEN = 'a-token-a-test-made-up'

/** Runs the fallback immediately instead of after the wait a browser would take. */
const atOnce = (fn: () => void) => {
  fn()

  return undefined
}

const never = () => undefined

describe('the page an invitation lands on', () => {
  it('hands the fragment token to the app, verbatim', () => {
    const opened: string[] = []

    render(<OpenInAppPage kind="accept" fragment={`#token_hash=${TOKEN}`} open={(url) => opened.push(url)} schedule={never} />)

    expect(opened).toEqual([`cadence://accept?token_hash=${TOKEN}`])
  })

  it('sends the recovery link to its own destination', () => {
    const opened: string[] = []

    render(<OpenInAppPage kind="recover" fragment={`#token_hash=${TOKEN}`} open={(url) => opened.push(url)} schedule={never} />)

    expect(opened).toEqual([`cadence://recover?token_hash=${TOKEN}`])
  })

  // The whole reason the token rides in the fragment: a browser does not send that part to any
  // server, so the one thing this page must never do is put it back into a request.
  it('never puts the token into a request', () => {
    const fetcher = vi.fn()
    vi.stubGlobal('fetch', fetcher)

    render(<OpenInAppPage kind="accept" fragment={`#token_hash=${TOKEN}`} open={() => {}} schedule={never} />)

    expect(fetcher).not.toHaveBeenCalled()
    vi.unstubAllGlobals()
  })

  // What this page exists for: with no app installed the scheme opens nothing at all, and a patient
  // is left looking at a blank tab with no idea what went wrong.
  it('says what to do when nothing answered', () => {
    render(<OpenInAppPage kind="accept" fragment={`#token_hash=${TOKEN}`} open={() => {}} schedule={atOnce} stillHere={() => true} />)

    expect(screen.getByText(THE_APP_DID_NOT_ANSWER)).toBeInTheDocument()
  })

  // The other half: when the app did answer, the browser tab is behind it, and a sentence telling
  // the patient to install what they already have is the last thing they would see of it.
  it('says nothing when the app answered', () => {
    render(<OpenInAppPage kind="accept" fragment={`#token_hash=${TOKEN}`} open={() => {}} schedule={atOnce} stillHere={() => false} />)

    expect(screen.queryByText(THE_APP_DID_NOT_ANSWER)).not.toBeInTheDocument()
  })

  it('explains a link with nothing in it rather than opening the app', () => {
    const opened: string[] = []

    render(<OpenInAppPage kind="accept" fragment="" open={(url) => opened.push(url)} schedule={never} />)

    expect(opened).toEqual([])
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })
})

describe('reading the fragment', () => {
  it('takes the token whether or not the hash carries its leading sign', () => {
    expect(tokenFromFragment(`#token_hash=${TOKEN}`)).toBe(TOKEN)
    expect(tokenFromFragment(`token_hash=${TOKEN}`)).toBe(TOKEN)
  })

  // Everything else a browser can put there. A page that answers a token for any of these hands a
  // stranger's string to the app on the strength of a link somebody else wrote.
  it('reads nothing out of anything else', () => {
    for (const fragment of ['', '#', '#token_hash=', '#access_token=abc', '#other=abc', '#token_hashy=abc']) {
      expect(tokenFromFragment(fragment)).toBeNull()
    }
  })
})

describe('the defaults the routes actually ship', () => {
  // Every test above injects `schedule` and `stillHere`, so the configuration a patient meets has
  // never run. Without the wait the «не установлено» sentence flashes under everyone who does have
  // the app, in the instant before the system brings it forward.
  it('says nothing until the wait is over, then says it', () => {
    vi.useFakeTimers()

    // `open` is still injected: jsdom refuses to redefine `window.location.assign`, so the one
    // default that stays unexercised anywhere is the navigation itself, and the by-hand pass is
    // what measures it. The wait and the visibility check are the shipped ones.
    render(<OpenInAppPage kind="accept" fragment={`#token_hash=${TOKEN}`} open={() => {}} />)

    act(() => void vi.advanceTimersByTime(WAIT_FOR_THE_APP_MS - 1))
    expect(screen.queryByText(THE_APP_DID_NOT_ANSWER)).not.toBeInTheDocument()

    act(() => void vi.advanceTimersByTime(1))
    expect(screen.getByText(THE_APP_DID_NOT_ANSWER)).toBeInTheDocument()

    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  // A token is moved across a parser boundary, and the one character that changes its meaning
  // there is the one a fragment can legally carry.
  it('hands over a token that would otherwise split the link', () => {
    const opened: string[] = []

    render(<OpenInAppPage kind="accept" fragment="#token_hash=aa%23bb" open={(url) => opened.push(url)} schedule={never} />)

    expect(opened).toEqual(['cadence://accept?token_hash=aa%23bb'])
  })
})
