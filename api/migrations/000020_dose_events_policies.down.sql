-- Dropped by its owner. Revoking the grants as well: a policy removed while a grant
-- stands leaves a role holding a verb on a forced table, which is deny-all in effect
-- and misleading in the registry.
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

REVOKE ALL ON app.dose_events FROM cadence_patient, cadence_doctor, cadence_service, cadence_admin;

DROP POLICY IF EXISTS dose_events_service_insert ON app.dose_events;
DROP POLICY IF EXISTS dose_events_service_read ON app.dose_events;
DROP POLICY IF EXISTS dose_events_admin ON app.dose_events;
DROP POLICY IF EXISTS dose_events_of_my_patients ON app.dose_events;
DROP POLICY IF EXISTS dose_events_own_insert ON app.dose_events;
DROP POLICY IF EXISTS dose_events_own_select ON app.dose_events;

RESET ROLE;
