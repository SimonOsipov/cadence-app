import { afterEach, describe, expect, it, vi } from 'vitest'

import { endpoints } from './config'

afterEach(() => {
  vi.unstubAllEnvs()
})

function build(api: string, auth = 'https://auth.example') {
  vi.stubEnv('VITE_API_URL', api)
  vi.stubEnv('VITE_AUTH_URL', auth)
}

describe('endpoints', () => {
  it('reads both addresses', () => {
    build('https://api.example', 'https://auth.example')

    expect(endpoints()).toEqual({ apiUrl: 'https://api.example', providerUrl: 'https://auth.example' })
  })

  it('refuses an unset address', () => {
    build('', 'https://auth.example')

    expect(() => endpoints()).toThrow(/VITE_API_URL is not set/)
  })

  // The one this file exists for: a host name without its scheme reached a doctor as
  // «Failed to construct 'URL': Invalid base URL», thrown from inside the API client.
  it('refuses a host name with no scheme, and says which variable', () => {
    build('porollo-dev-cadence-app-bbb3.twc1.net')

    expect(() => endpoints()).toThrow(/VITE_API_URL .* no scheme/)
  })

  it('refuses a scheme that reaches nothing', () => {
    build('ftp://api.example')

    expect(() => endpoints()).toThrow(/only http and https/)
  })

  it('names the identity provider rather than the API when that one is wrong', () => {
    build('https://api.example', 'auth.example')

    expect(() => endpoints()).toThrow(/VITE_AUTH_URL/)
  })

  it('drops a trailing slash, so a path is not joined onto a double one', () => {
    build('https://api.example///')

    expect(endpoints().apiUrl).toBe('https://api.example')
  })
})
