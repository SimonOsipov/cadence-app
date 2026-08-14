-- Back to the cluster's own defaults for both settings.
--
-- Guarded on the role existing, because every down migration in this chain is
-- required to survive being applied twice and to run on a cluster the chain has
-- already been rolled off — where cadence_service_app is gone and an
-- unconditional ALTER ROLE is an error rather than a no-op. RESET itself is
-- twice-safe: a setting that is not there resets quietly.

DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_service_app') THEN
        EXECUTE 'ALTER ROLE cadence_service_app RESET statement_timeout';
        EXECUTE 'ALTER ROLE cadence_service_app RESET idle_in_transaction_session_timeout';
    END IF;
END
$$;
