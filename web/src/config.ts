/**
 * Where this build points: the API and the identity provider.
 *
 * Read once and refused at startup rather than at the first request. An unset address would otherwise
 * reach fetch as `undefined/v1/me`, and the screen would report a network failure for a build that was
 * never configured.
 *
 * Absoluteness is checked rather than assumed, because the failure it prevents was met on the first
 * deployment: a host name pasted without its scheme passed the emptiness check, reached
 * `new URL(path, base)` inside the client, and surfaced to the person signing in as «Failed to
 * construct 'URL': Invalid base URL» — a sentence naming neither the variable nor the build.
 */
function required(name: 'VITE_API_URL' | 'VITE_AUTH_URL'): string {
  const what = name === 'VITE_API_URL' ? 'API' : 'identity provider'
  const value: unknown = import.meta.env[name]

  if (typeof value !== 'string' || value.trim() === '') {
    throw new Error(`${name} is not set, so this build has no ${what} to talk to`)
  }

  const address = value.trim().replace(/\/+$/, '')

  let parsed: URL
  try {
    parsed = new URL(address)
  } catch {
    throw new Error(`${name} is "${address}", which has no scheme — an absolute address is needed, as in https://host`)
  }

  if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') {
    throw new Error(`${name} is "${address}", and only http and https reach an ${what}`)
  }

  return address
}

export function endpoints(): { apiUrl: string; providerUrl: string } {
  return { apiUrl: required('VITE_API_URL'), providerUrl: required('VITE_AUTH_URL') }
}
