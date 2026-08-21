-- Dropped by its owner. The indexes and the CHECKs go with the table; nothing
-- here is shared with another migration.
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

DROP TABLE IF EXISTS app.vials;

RESET ROLE;
