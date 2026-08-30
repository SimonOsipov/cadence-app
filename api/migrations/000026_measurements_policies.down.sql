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

REVOKE ALL ON app.measurements FROM cadence_patient, cadence_doctor, cadence_service, cadence_admin;

DROP POLICY IF EXISTS measurements_service_insert ON app.measurements;
DROP POLICY IF EXISTS measurements_service_read ON app.measurements;
DROP POLICY IF EXISTS measurements_admin ON app.measurements;
DROP POLICY IF EXISTS measurements_of_my_patients ON app.measurements;
DROP POLICY IF EXISTS measurements_own_manual_delete ON app.measurements;
DROP POLICY IF EXISTS measurements_own_insert ON app.measurements;
DROP POLICY IF EXISTS measurements_own_select ON app.measurements;

RESET ROLE;
