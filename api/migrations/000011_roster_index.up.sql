-- The ordering the roster pages by, named so that «keyset pagination over a stable ordering» is a
-- property of the schema rather than of one query's ORDER BY.
--
-- Partial on the role because that is the only role this index is read for: staff never appear in a
-- registry of patients, and a doctor reading their own row through profiles_own_select would
-- otherwise be a row in their own roster.
--
-- (full_name, user_id) and not full_name alone: names are not unique, and a cursor keyed on a
-- non-unique column either skips the second Иванов or returns them twice.

SET ROLE cadence_owner;

CREATE INDEX profiles_patients_by_name
    ON app.profiles (full_name, user_id)
    WHERE role = 'patient';

RESET ROLE;
