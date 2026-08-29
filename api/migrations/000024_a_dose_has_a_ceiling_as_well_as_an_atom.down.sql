DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

ALTER TABLE IF EXISTS app.protocol_phases
    DROP CONSTRAINT IF EXISTS protocol_phases_dose_value_magnitude_check;

ALTER TABLE IF EXISTS app.dose_events
    DROP CONSTRAINT IF EXISTS dose_events_dose_value_magnitude_check;

ALTER TABLE IF EXISTS app.vials
    DROP CONSTRAINT IF EXISTS vials_total_amount_magnitude_check;

RESET ROLE;
