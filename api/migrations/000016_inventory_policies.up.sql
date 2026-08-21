-- One table, and the seam it is written for is the other one. A protocol is a
-- cross-actor write — the doctor writes it about the patient — and travels the
-- service path. A vial is the patient writing about themselves, which is the
-- request path, RLS, and no audit row: cadence_patient has no privilege on
-- audit_log and is not going to be given one.
--
-- What that means for this migration is that the patient's own predicate is not
-- a read filter here, it is the write rule as well — so every one of their
-- policies carries WITH CHECK, and USING and WITH CHECK say the same thing. A
-- patient may not write a row about somebody else, and may not move one to them.

SET ROLE cadence_owner;

CREATE POLICY vials_own_select ON app.vials
    FOR SELECT TO cadence_patient
    USING (patient_id = app.jwt_subject());

-- Split by verb rather than written as FOR ALL, so that a verb the patient is not
-- meant to hold is a missing policy rather than a missing grant. The grant is the
-- second lock; this is the first.
--
-- WITH CHECK on both halves of the UPDATE: without it on the USING side a patient
-- edits only their own row, and without it on the CHECK side they may hand that
-- row to somebody else. The pair is what makes «may not move a row» true here
-- without a column grant — the patient's predicate names the subject, which the
-- service path's cannot.
CREATE POLICY vials_own_insert ON app.vials
    FOR INSERT TO cadence_patient
    WITH CHECK (patient_id = app.jwt_subject());
CREATE POLICY vials_own_update ON app.vials
    FOR UPDATE TO cadence_patient
    USING (patient_id = app.jwt_subject())
    WITH CHECK (patient_id = app.jwt_subject());

-- No DELETE, and it is a decision rather than an omission: a vial is disposed of
-- by setting disposed_at, and the dose events drawn from it have to keep pointing
-- at something. «Move into reserve» and «dispose» are both UPDATEs.

CREATE POLICY vials_of_my_patients ON app.vials
    FOR SELECT TO cadence_doctor
    USING (EXISTS (
        SELECT FROM app.care_team_assignments
        WHERE care_team_assignments.patient_id = vials.patient_id
          AND care_team_assignments.provider_id = app.jwt_subject()
    ));

CREATE POLICY vials_admin ON app.vials
    FOR ALL TO cadence_admin USING (true) WITH CHECK (true);

-- The service path reads and inserts: the seed fills a development clinic, and
-- the dose-logging transaction of step 8 has to resolve the vial a dose came out
-- of. It does not update — a vial is the patient's own record of their own
-- medicine cabinet, and nothing on the service path has business rewriting one.
CREATE POLICY vials_service_read ON app.vials
    FOR SELECT TO cadence_service USING (true);
CREATE POLICY vials_service_insert ON app.vials
    FOR INSERT TO cadence_service WITH CHECK (true);

-- ------------------------------------------------------------------ grants --

-- Table-wide on this one, and the reason is the shape above: the patient's own
-- policies carry the subject in both USING and WITH CHECK, so a row cannot change
-- owner through them. That is the guarantee the protocol tables had to buy with
-- column grants, because the service path's policies carry no subject at all.
GRANT SELECT, INSERT, UPDATE ON app.vials TO cadence_patient;
GRANT SELECT ON app.vials TO cadence_doctor, cadence_service;
GRANT INSERT ON app.vials TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.vials TO cadence_admin;

RESET ROLE;
