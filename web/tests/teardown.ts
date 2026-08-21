import { execFileSync } from 'node:child_process'

import { CLEANUP_PREFIX } from './cleanup'

/**
 * Deletes what the run created, in the local database.
 *
 * The decision the spec left open, taken here: cleanup goes through the database rather than through
 * the provisioner. Deleting an account there is conditioned on the absence of a profile — that
 * condition is what stands between «reuse an address a rolled-back transaction burned» and «delete any
 * account» — and a patient this test creates has one. The alternatives were to weaken that condition
 * for a test environment, which is a production surface changed for a test, or this: the smoke test
 * runs against a local harness whose database it already owns.
 *
 * Both schemas, because a patient is rows in `app` and an account in `auth`, and either left behind
 * is an address the next run cannot invite.
 */
export default function teardown(): void {
  const like = `${CLEANUP_PREFIX}%`

  const sql = `
    DELETE FROM app.care_team_assignments
     WHERE patient_id IN (SELECT user_id FROM app.invites WHERE email LIKE '${like}');
    DELETE FROM app.user_preferences
     WHERE user_id IN (SELECT user_id FROM app.invites WHERE email LIKE '${like}');
    DELETE FROM app.patient_profiles
     WHERE user_id IN (SELECT user_id FROM app.invites WHERE email LIKE '${like}');
    DELETE FROM app.audit_log
     WHERE entity_id IN (SELECT user_id FROM app.invites WHERE email LIKE '${like}');
    DELETE FROM app.profiles
     WHERE user_id IN (SELECT user_id FROM app.invites WHERE email LIKE '${like}');
    DELETE FROM app.invites WHERE email LIKE '${like}';
    DELETE FROM auth.users WHERE email LIKE '${like}';
  `

  // Through the container rather than through a Postgres client of its own: the dashboard has no
  // database dependency and acquiring one for a teardown would put a driver in the bundle's tree.
  execFileSync(
    'docker',
    ['compose', '-f', '../api/docker-compose.yml', 'exec', '-T', 'postgres',
      'psql', '-q', '-v', 'ON_ERROR_STOP=1', '-U', 'cadence', '-d', 'cadence', '-c', sql],
    { stdio: 'inherit' },
  )
}
