-- The service path may not write an admin.
--
-- A compromised API holds cadence_service_app and can invite an arbitrary
-- address. That is harmless only while it cannot make the invitee an admin, and
-- until here the two service policies on profiles carried no predicate at all —
-- so "invite, then write the profile with role = 'admin'" was one transaction.
-- Refusing it in Go would be refusing it inside the process assumed compromised.
--
-- What is constrained is the value in the row, not the caller. WITH CHECK looks
-- at the row being written, so this is the closed set of roles the service path
-- may bring into being; the role decision is still made by TO and by nothing
-- else, and no claim is read here.
--
-- This withdraws one sentence of 000004's header — "no policy body here contains
-- the literals 'patient', 'doctor' or 'admin' — a test asserts that". That file
-- is immutable, so the correction lives here: the rule it was standing in for is
-- that the *caller's* role is never compared against a value, and it still holds.
-- The test named there was narrowed accordingly and now says both halves —
-- TestNoPolicyBodyDecidesTheCallersRole declares the two policies permitted to
-- name a role at all, and forbids any policy from naming one in a body that also
-- reads a claim.
--
-- 'admin' stays in the column's CHECK. Admins exist — they are just not made
-- from the service path. What this closes is the escalation the review found:
-- a compromised API holds cadence_service_app, so inviting an arbitrary address
-- is harmless only once that address cannot also be written an admin profile.
--
-- It is not "only a one-off command makes admins", and the difference is worth
-- being exact about: profiles_admin is FOR ALL TO cadence_admin WITH CHECK
-- (true), and cadence_admin is a role the request path does transition into for
-- a caller holding an admin token. An existing admin can therefore still make
-- another one. Closing that is a separate decision about the request path and
-- is not this migration's.
--
-- USING on the UPDATE policy stays unrestricted: the service path may still read
-- and rewrite an admin's row, it may only not produce one. Narrowing USING would
-- take away the ability to correct an admin's name.

SET ROLE cadence_owner;

DROP POLICY IF EXISTS profiles_service_insert ON app.profiles;
CREATE POLICY profiles_service_insert ON app.profiles
    FOR INSERT TO cadence_service
    WITH CHECK (role IN ('patient', 'doctor'));

DROP POLICY IF EXISTS profiles_service_update ON app.profiles;
CREATE POLICY profiles_service_update ON app.profiles
    FOR UPDATE TO cadence_service
    USING (true)
    WITH CHECK (role IN ('patient', 'doctor'));

RESET ROLE;
