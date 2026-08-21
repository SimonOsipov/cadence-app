-- Scoped to this migration's one table, for the reason 000014 records: a sweep of
-- the schema would make rolling back this migration destroy the policies of the
-- two pairs below it, which no up migration would restore.
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

DO $$
DECLARE
    policy record;
BEGIN
    IF NOT EXISTS (
        SELECT FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'app' AND c.relname = 'vials'
    ) THEN
        RETURN;
    END IF;

    FOR policy IN
        SELECT policyname FROM pg_policies WHERE schemaname = 'app' AND tablename = 'vials'
    LOOP
        EXECUTE format('DROP POLICY %I ON app.vials', policy.policyname);
    END LOOP;

    -- cadence_owner deliberately absent: REVOKE ALL would materialise its
    -- implicit privileges into an ACL and then erase it.
    REVOKE ALL ON app.vials FROM cadence_patient, cadence_doctor, cadence_admin, cadence_service;
END
$$;

RESET ROLE;
