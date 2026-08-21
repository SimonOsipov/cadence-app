-- The ordering the roster pages by, named so that «keyset pagination over a stable ordering» is a
-- property of the schema rather than of one query's ORDER BY.
--
-- Partial because 'patient' is the only role the index is ever read for. It is not what keeps staff
-- out of the registry — the predicate in the query is, and it says so there.
--
-- (full_name, user_id) and not full_name alone: names are not unique, and a cursor keyed on a
-- non-unique column either skips the second Иванов or returns them twice.

SET ROLE cadence_owner;

CREATE INDEX profiles_patients_by_name
    ON app.profiles (full_name, user_id)
    WHERE role = 'patient';

RESET ROLE;
