-- Back to an owner that writes neither table. DROP POLICY IF EXISTS still needs
-- the relation to exist, so each drop is guarded on its table rather than on the
-- policy: this has to run on a cluster the chain has already been rolled off,
-- and twice in a row.
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'app' AND c.relname = 'profiles'
    ) THEN
        DROP POLICY IF EXISTS profiles_bootstrap_insert ON app.profiles;
    END IF;

    IF EXISTS (
        SELECT FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'app' AND c.relname = 'audit_log'
    ) THEN
        DROP POLICY IF EXISTS audit_log_bootstrap_insert ON app.audit_log;
    END IF;
END
$$;

RESET ROLE;
