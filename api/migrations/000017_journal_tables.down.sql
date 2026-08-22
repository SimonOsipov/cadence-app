-- Dropped by its owner. The index, the CHECKs and the composite primary key go with
-- the table.
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

DROP TABLE IF EXISTS app.journal_entries;

RESET ROLE;
