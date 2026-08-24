-- Dropped by its owner, and the parents' composite witnesses go with it: they were
-- added for this table's foreign keys and nothing else references them.
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

DROP TABLE IF EXISTS app.dose_events;

ALTER TABLE IF EXISTS app.vials
    DROP CONSTRAINT IF EXISTS vials_belong_to_their_patient;
ALTER TABLE IF EXISTS app.protocol_items
    DROP CONSTRAINT IF EXISTS protocol_items_belong_to_their_protocol;
ALTER TABLE IF EXISTS app.protocols
    DROP CONSTRAINT IF EXISTS protocols_belong_to_their_patient;

RESET ROLE;
