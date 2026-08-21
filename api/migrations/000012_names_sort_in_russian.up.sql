-- The roster is ordered by name, and until this migration «by name» meant by bytes: measured on the
-- image the suite runs, Ёлкина sorted before Анна and every lowercase name after every uppercase one.
-- The product's language is Russian and the registry is a doctor reading down a list, so the order is
-- part of the answer rather than of the storage.
--
-- ICU rather than a libc locale: the deployment's own locale is not ours to choose — the test image is
-- musl and the cluster on Timeweb will not be — and a collation that travels with the database is the
-- only one whose order is the same in both places.
--
-- The index is dropped and rebuilt around the change on purpose. A collation is part of the index's
-- own definition, so an index left in place would be ordered by the collation it was built under, and
-- the ORDER BY that no longer matches it would stop using it silently.

SET ROLE cadence_owner;

DROP INDEX app.profiles_patients_by_name;

ALTER TABLE app.profiles
    ALTER COLUMN full_name TYPE text COLLATE "ru-RU-x-icu";

CREATE INDEX profiles_patients_by_name
    ON app.profiles (full_name, user_id)
    WHERE role = 'patient';

RESET ROLE;
