-- Back to the two service policies as 000004 wrote them: WITH CHECK (true), so
-- no row predicate at all on the service path.
--
-- Guarded on the schema and applied through DROP IF EXISTS, because every down
-- migration in this chain is required to survive being applied twice and to run
-- on a cluster the chain has already been rolled off.

DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'app' AND c.relname = 'profiles'
    ) THEN
        RETURN;
    END IF;

    DROP POLICY IF EXISTS profiles_service_insert ON app.profiles;
    CREATE POLICY profiles_service_insert ON app.profiles
        FOR INSERT TO cadence_service WITH CHECK (true);

    DROP POLICY IF EXISTS profiles_service_update ON app.profiles;
    CREATE POLICY profiles_service_update ON app.profiles
        FOR UPDATE TO cadence_service USING (true) WITH CHECK (true);
END
$$;

RESET ROLE;
