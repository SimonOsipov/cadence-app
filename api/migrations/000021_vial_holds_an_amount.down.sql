-- Dropped by its owner. No REVOKE, unlike 000020: dropping a column takes its
-- grants with it, and revoking a column grant after the column is gone is an error
-- rather than a no-op.

DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

ALTER TABLE IF EXISTS app.dose_events
    DROP CONSTRAINT IF EXISTS dose_events_dose_value_scale_check;

-- The four CHECKs on vials are not named: DROP COLUMN removes a constraint that reads
-- the column, the two-column one included. The dose_events line above is named because
-- its column stays.
ALTER TABLE IF EXISTS app.vials DROP COLUMN IF EXISTS held_back_at;
ALTER TABLE IF EXISTS app.vials DROP COLUMN IF EXISTS amount_unit;
ALTER TABLE IF EXISTS app.vials DROP COLUMN IF EXISTS total_amount;

-- opened_at is deliberately not un-backfilled: the up migration derived it from
-- dose events that are still there, so the value it wrote is true whichever
-- direction the chain is walked. Guessing which rows to blank would lose facts.

RESET ROLE;
