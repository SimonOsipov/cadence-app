SET ROLE cadence_owner;

DROP INDEX app.profiles_patients_by_name;

ALTER TABLE app.profiles
    ALTER COLUMN full_name TYPE text COLLATE "default";

CREATE INDEX profiles_patients_by_name
    ON app.profiles (full_name, user_id)
    WHERE role = 'patient';

RESET ROLE;
