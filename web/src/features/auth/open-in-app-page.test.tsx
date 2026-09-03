import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { OpenInAppPage, THE_APP_DID_NOT_ANSWER, tokenFromFragment } from './open-in-app-page'

const TOKEN = 'e75b4d4f54a86c915c0afdfc5db3b5cb6eea78ba43c1ccf6bd24c5cb'

/** Runs the fallback immediately instead of after the wait a browser would take. */
const atOnce = (fn: () => void) => fn()

const never = () => {
  /* the app answered, so nothing schedules */
}

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
