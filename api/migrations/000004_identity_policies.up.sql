-- Six policy shapes, and every one of them carries an explicit TO.
--
-- A policy without TO applies to PUBLIC — which includes the service path, and
-- would quietly hand it the request path's row predicate. The policies registry
-- in the test suite requires polroles <> '{0}' for exactly that reason.
--
-- The role decision is made by TO and never by comparing a value. There is no
-- app.jwt_role() to compare against, and no policy body here contains the
-- literals 'patient', 'doctor' or 'admin' — a test asserts that. Privileges in
-- Postgres are bound to a role, so a policy that read the role from a claim
-- would put the decision back in the place the whole block was rewritten to take
-- it out of.
--
-- Every predicate is positive equality. Negations and coalesce are three-valued,
-- and on NULL they do not do what they look like they do: a caller with no
-- subject must match nothing, and `<>` would match everything.
--
-- One constraint on shape is load-bearing and invisible: the subquery in a
-- policy executes under the policies of the table it reads. The
-- care_team_assignments policies therefore refer to nothing but their own
-- columns — a back-reference to profiles, whose policies read
-- care_team_assignments, is infinite recursion and Postgres answers 42P17. A
-- test plants that back-reference and requires exactly that code, because
-- "there is no recursion" is otherwise green in a world where the policy was
-- forgotten.

SET ROLE cadence_owner;

-- ---------------------------------------------------------------- profiles --

-- Own row. The patient and the doctor each see themselves; only the patient may
-- edit, and the column grant below decides which columns.
CREATE POLICY profiles_own_select ON app.profiles
    FOR SELECT TO cadence_patient, cadence_doctor
    USING (user_id = app.jwt_subject());

CREATE POLICY profiles_own_update ON app.profiles
    FOR UPDATE TO cadence_patient
    USING (user_id = app.jwt_subject())
    WITH CHECK (user_id = app.jwt_subject());

-- A patient reads the profiles of their own specialists, and of nobody else.
CREATE POLICY profiles_of_my_specialists ON app.profiles
    FOR SELECT TO cadence_patient
    USING (EXISTS (
        SELECT FROM app.care_team_assignments
        WHERE care_team_assignments.provider_id = profiles.user_id
          AND care_team_assignments.patient_id = app.jwt_subject()
    ));

-- The same relation read in the other direction: a doctor reads the profiles of
-- the patients assigned to them. Adding an assignment row grants the visibility
-- and deleting it takes the visibility away, with no query edited anywhere.
CREATE POLICY profiles_of_my_patients ON app.profiles
    FOR SELECT TO cadence_doctor
    USING (EXISTS (
        SELECT FROM app.care_team_assignments
        WHERE care_team_assignments.patient_id = profiles.user_id
          AND care_team_assignments.provider_id = app.jwt_subject()
    ));

CREATE POLICY profiles_admin ON app.profiles
    FOR ALL TO cadence_admin USING (true) WITH CHECK (true);

-- The service path carries no row predicate, and that is written down rather
-- than implied: its authorization lives in Go. This is the one place in the
-- system where the database is not the last word, and the verbs it is trusted
-- with are exactly the ones granted below.
CREATE POLICY profiles_service_read ON app.profiles
    FOR SELECT TO cadence_service USING (true);
CREATE POLICY profiles_service_insert ON app.profiles
    FOR INSERT TO cadence_service WITH CHECK (true);
CREATE POLICY profiles_service_update ON app.profiles
    FOR UPDATE TO cadence_service USING (true) WITH CHECK (true);

-- The narrowest policy in the schema, and it exists for one caller: the token
-- issuance hook, which is SECURITY DEFINER owned by cadence_owner and has to
-- read a profile's role. FORCE applies row level security to the owner too, so
-- without this the hook reads nothing.
CREATE POLICY profiles_hook_read ON app.profiles
    FOR SELECT TO cadence_owner USING (true);

-- -------------------------------------------------------- patient_profiles --

CREATE POLICY patient_profiles_own_select ON app.patient_profiles
    FOR SELECT TO cadence_patient
    USING (user_id = app.jwt_subject());

CREATE POLICY patient_profiles_own_update ON app.patient_profiles
    FOR UPDATE TO cadence_patient
    USING (user_id = app.jwt_subject())
    WITH CHECK (user_id = app.jwt_subject());

CREATE POLICY patient_profiles_of_my_patients ON app.patient_profiles
    FOR SELECT TO cadence_doctor
    USING (EXISTS (
        SELECT FROM app.care_team_assignments
        WHERE care_team_assignments.patient_id = patient_profiles.user_id
          AND care_team_assignments.provider_id = app.jwt_subject()
    ));

CREATE POLICY patient_profiles_admin ON app.patient_profiles
    FOR ALL TO cadence_admin USING (true) WITH CHECK (true);

CREATE POLICY patient_profiles_service_read ON app.patient_profiles
    FOR SELECT TO cadence_service USING (true);
CREATE POLICY patient_profiles_service_insert ON app.patient_profiles
    FOR INSERT TO cadence_service WITH CHECK (true);

-- ------------------------------------------------------- provider_profiles --

CREATE POLICY provider_profiles_own_select ON app.provider_profiles
    FOR SELECT TO cadence_doctor
    USING (user_id = app.jwt_subject());

CREATE POLICY provider_profiles_of_my_specialists ON app.provider_profiles
    FOR SELECT TO cadence_patient
    USING (EXISTS (
        SELECT FROM app.care_team_assignments
        WHERE care_team_assignments.provider_id = provider_profiles.user_id
          AND care_team_assignments.patient_id = app.jwt_subject()
    ));

CREATE POLICY provider_profiles_admin ON app.provider_profiles
    FOR ALL TO cadence_admin USING (true) WITH CHECK (true);

CREATE POLICY provider_profiles_service_read ON app.provider_profiles
    FOR SELECT TO cadence_service USING (true);
CREATE POLICY provider_profiles_service_insert ON app.provider_profiles
    FOR INSERT TO cadence_service WITH CHECK (true);

-- --------------------------------------------------- care_team_assignments --

-- Their own columns and nothing else. This is the table every other table's
-- policies read through, so a predicate here that reached back into profiles
-- would make both sides recursive.
CREATE POLICY care_team_assignments_mine_patient ON app.care_team_assignments
    FOR SELECT TO cadence_patient
    USING (patient_id = app.jwt_subject());

CREATE POLICY care_team_assignments_mine_provider ON app.care_team_assignments
    FOR SELECT TO cadence_doctor
    USING (provider_id = app.jwt_subject());

CREATE POLICY care_team_assignments_admin ON app.care_team_assignments
    FOR ALL TO cadence_admin USING (true) WITH CHECK (true);

CREATE POLICY care_team_assignments_service_read ON app.care_team_assignments
    FOR SELECT TO cadence_service USING (true);
CREATE POLICY care_team_assignments_service_insert ON app.care_team_assignments
    FOR INSERT TO cadence_service WITH CHECK (true);
CREATE POLICY care_team_assignments_service_delete ON app.care_team_assignments
    FOR DELETE TO cadence_service USING (true);

-- -------------------------------------------------------- user_preferences --

-- Only the patient. A doctor has no business reading how somebody has set up
-- their reminders, and the grants table says so by leaving the row empty.
CREATE POLICY user_preferences_own_select ON app.user_preferences
    FOR SELECT TO cadence_patient
    USING (user_id = app.jwt_subject());

CREATE POLICY user_preferences_own_update ON app.user_preferences
    FOR UPDATE TO cadence_patient
    USING (user_id = app.jwt_subject())
    WITH CHECK (user_id = app.jwt_subject());

CREATE POLICY user_preferences_admin ON app.user_preferences
    FOR ALL TO cadence_admin USING (true) WITH CHECK (true);

CREATE POLICY user_preferences_service_read ON app.user_preferences
    FOR SELECT TO cadence_service USING (true);
CREATE POLICY user_preferences_service_insert ON app.user_preferences
    FOR INSERT TO cadence_service WITH CHECK (true);

-- --------------------------------------------------------------- audit_log --

-- Read only, and only for the admin. There is no FOR ALL shape on this table and
-- no UPDATE or DELETE policy for anybody: append-only is a property of the
-- schema against both paths, and against the migration chain it is detectable
-- rather than preventable — the chain runs as the owner and could drop FORCE.
CREATE POLICY audit_log_admin_read ON app.audit_log
    FOR SELECT TO cadence_admin USING (true);

-- The actor on the row has to be the actor the seam published. Compared as text
-- so that neither side can raise: a cast of a setting a caller can overwrite is
-- a 22P02 from inside a policy.
--
-- This is a "do not forget to name the actor" property, not protection against
-- forgery: code that can write the row can write the setting. It is worth
-- exactly that much and is recorded as being worth exactly that much.
CREATE POLICY audit_log_service_insert ON app.audit_log
    FOR INSERT TO cadence_service
    WITH CHECK (
        -- Lowered on the published side: a uuid renders canonically in lower
        -- case, while the seam accepts a subject case-insensitively. Two
        -- components disagreeing about what counts as the same subject would
        -- fail the whole patient-creation transaction, as a policy refusal that
        -- names nothing.
        actor_id::text = pg_catalog.lower(
            NULLIF(pg_catalog.current_setting('app.actor_id', true), ''))
        OR actor_job = NULLIF(pg_catalog.current_setting('app.actor_job', true), '')
    );

-- ------------------------------------------------------------------ grants --

-- Per role, and where it matters per column. There are no default privileges to
-- fall back on: a verb absent from this list is a verb nobody holds.
--
-- SELECT is table-wide wherever it appears; the column lists are on UPDATE,
-- which is where the difference between "may see" and "may change" lives.

-- profiles: the patient may correct their own name, timezone and locale. Not
-- role — that is the escalation the seven-role split exists to prevent — and
-- not user_id, which is their identity.
GRANT SELECT ON app.profiles TO cadence_patient, cadence_doctor, cadence_service;
GRANT UPDATE (full_name, timezone, locale) ON app.profiles TO cadence_patient;
GRANT INSERT, UPDATE ON app.profiles TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.profiles TO cadence_admin;
-- The hook's defining role reads profiles and nothing else.
GRANT SELECT ON app.profiles TO cadence_owner;

-- patient_profiles: the clinical fields are entered by the clinic. The patient
-- owns one number on this card — the target they set for themselves.
GRANT SELECT ON app.patient_profiles TO cadence_patient, cadence_doctor, cadence_service;
GRANT UPDATE (target_weight_kg) ON app.patient_profiles TO cadence_patient;
GRANT INSERT ON app.patient_profiles TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.patient_profiles TO cadence_admin;

GRANT SELECT ON app.provider_profiles TO cadence_patient, cadence_doctor, cadence_service;
GRANT INSERT ON app.provider_profiles TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.provider_profiles TO cadence_admin;

-- The patient may read their care team and change nothing about it. Deleting an
-- assignment would be a way to hide from the doctor.
GRANT SELECT ON app.care_team_assignments TO cadence_patient, cadence_doctor, cadence_service;
GRANT INSERT, DELETE ON app.care_team_assignments TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.care_team_assignments TO cadence_admin;

GRANT SELECT, UPDATE ON app.user_preferences TO cadence_patient;
GRANT SELECT, INSERT ON app.user_preferences TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.user_preferences TO cadence_admin;

-- audit_log: appended by the service path, read by the admin, and nothing else
-- by anybody. No UPDATE and no DELETE exist to be granted.
GRANT INSERT ON app.audit_log TO cadence_service;
GRANT SELECT ON app.audit_log TO cadence_admin;

-- Nothing on sequences, deliberately. audit_log.id is GENERATED ALWAYS AS
-- IDENTITY and its sequence leaks the number of values ever handed out — an
-- upper bound on inserts that no policy filters, because row level security
-- cannot be enabled on a sequence at all.

RESET ROLE;
