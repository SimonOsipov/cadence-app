-- Scoped to this migration's one table: a sweep of the schema would make rolling
-- back this migration destroy the policies of every pair below it, which no up
-- migration would restore.
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
        WHERE n.nspname = 'app' AND c.relname = 'journal_entries'
    ) THEN
        RETURN;
    END IF;

    FOR policy IN
        SELECT policyname FROM pg_policies WHERE schemaname = 'app' AND tablename = 'journal_entries'
    LOOP
        EXECUTE format('DROP POLICY %I ON app.journal_entries', policy.policyname);
    END LOOP;

    -- cadence_owner deliberately absent, for the reason 000004 records.
    REVOKE ALL ON app.journal_entries
        FROM cadence_patient, cadence_doctor, cadence_admin, cadence_service;
END
$$;

RESET ROLE;
