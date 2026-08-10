-- Policies and grants go together: a table left with grants and no policies is
-- reachable by nobody, and a table left with policies and no grants is the same.
-- Dropping the policies is enough to make that true, and the revokes keep the
-- catalogue honest for a cluster that stops here.
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
    IF NOT EXISTS (SELECT FROM pg_namespace WHERE nspname = 'app') THEN
        RETURN;
    END IF;

    FOR policy IN
        SELECT policyname, tablename FROM pg_policies WHERE schemaname = 'app'
    LOOP
        EXECUTE format('DROP POLICY %I ON app.%I', policy.policyname, policy.tablename);
    END LOOP;

    FOR policy IN
        SELECT c.relname AS tablename
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'app' AND c.relkind IN ('r', 'p')
    LOOP
        -- cadence_owner is deliberately absent. Its privileges on its own
        -- tables are implicit, and REVOKE ALL would materialise them into an
        -- ACL and then erase it — leaving the owner unable to write to tables it
        -- owns, in a state the up migration does not restore, because the up
        -- grants it only SELECT on profiles. A later migration doing a backfill
        -- would then fail on a cluster that had been rolled back once and
        -- succeed on a fresh one.
        EXECUTE format(
            'REVOKE ALL ON app.%I FROM cadence_patient, cadence_doctor, cadence_admin, '
            'cadence_service',
            policy.tablename);
    END LOOP;
END
$$;

RESET ROLE;
