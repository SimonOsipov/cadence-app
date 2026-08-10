-- Dropped by its owner, the way it was created. The bootstrap role could drop it
-- as a member of cadence_owner, but a rollback that runs under a different role
-- from the one that applied is how ownership drifts in the first place.
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

DROP FUNCTION IF EXISTS app.jwt_subject();

RESET ROLE;
