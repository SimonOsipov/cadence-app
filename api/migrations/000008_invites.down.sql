-- The policies and the grants go with the table; nothing here is shared with
-- another one. Guarded on the role, because every down migration in this chain
-- has to survive being applied twice and to run on a cluster the chain has
-- already been rolled off.
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

DROP TABLE IF EXISTS app.invites;

RESET ROLE;
