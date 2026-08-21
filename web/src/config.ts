/**
 * Where this build points: the API and the identity provider.
 *
 * Read once and refused at startup rather than at the first request. An unset address would otherwise
 * reach fetch as `undefined/v1/me`, and the screen would report a network failure for a build that was
 * never configured.
 */
function required(name: 'VITE_API_URL' | 'VITE_AUTH_URL'): string {
  const value: unknown = import.meta.env[name]

  if (typeof value !== 'string' || value.trim() === '') {
    throw new Error(`${name} is not set, so this build has no ${name === 'VITE_API_URL' ? 'API' : 'identity provider'} to talk to`)
  }

  return value.replace(/\/+$/, '')
}

export function endpoints(): { apiUrl: string; providerUrl: string } {
  return { apiUrl: required('VITE_API_URL'), providerUrl: required('VITE_AUTH_URL') }
}
