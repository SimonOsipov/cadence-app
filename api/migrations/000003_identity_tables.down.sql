-- Dropped by their owner, in reverse dependency order. The indexes and
-- constraints go with the tables; nothing here is shared with another schema.
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

DROP TABLE IF EXISTS app.audit_log;
DROP TABLE IF EXISTS app.user_preferences;
DROP TABLE IF EXISTS app.care_team_assignments;
DROP TABLE IF EXISTS app.provider_profiles;
DROP TABLE IF EXISTS app.patient_profiles;
DROP TABLE IF EXISTS app.profiles;

RESET ROLE;
