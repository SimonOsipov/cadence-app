-- Guarded on the role and on the relation, because every down migration in this
-- chain has to survive being applied twice and to run on a cluster the chain has
-- already been rolled off — where neither cadence_owner nor app.invites exists.
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

ALTER TABLE IF EXISTS app.invites DROP COLUMN IF EXISTS role;

RESET ROLE;
