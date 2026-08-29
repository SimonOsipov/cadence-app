DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

ALTER TABLE IF EXISTS app.protocol_phases
    DROP CONSTRAINT IF EXISTS protocol_phases_dose_value_scale_check;

RESET ROLE;
