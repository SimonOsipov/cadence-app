-- One table, and the seam it is written for is the other one. A protocol is a
-- cross-actor write — the doctor writes it about the patient — and travels the
-- service path. A vial is the patient writing about themselves, which is the
-- request path, RLS, and no audit row: cadence_patient has no privilege on
-- audit_log and is not going to be given one.
--
-- The shape is not new. The identity block already writes three tables this way,
-- and `ownRowWrite` in the policy registry is this predicate in both USING and
-- WITH CHECK, applied to profiles, patient_profiles and user_preferences. Two of
-- those three are narrowed by column, and that precedent is the one to follow —
-- see the grants at the foot of this file.
--
-- Two things here are new and are what to read carefully. It is the first INSERT
-- cadence_patient holds at all. And it is the first patient-written table whose
-- owner is an ordinary column rather than the row's primary key: on profiles and
-- user_preferences, «change owner» means «change the key», and here it does not.

SET ROLE cadence_owner;

CREATE POLICY vials_own_select ON app.vials
    FOR SELECT TO cadence_patient
    USING (patient_id = app.jwt_subject());

-- Split by verb rather than written as FOR ALL, so that a verb the patient is not
-- meant to hold is a missing policy rather than a missing grant. The grant is the
-- second lock; this is the first.
--
-- The subject appears in USING and in WITH CHECK, and the two answer different
-- questions: WITH CHECK refuses a row that would become somebody else's, USING
-- refuses one that is already theirs. That pair is what makes «may not move a row»
-- true here without a column grant — the patient's predicate names the subject,
-- which the service path's cannot.
--
-- Measured, and worth knowing before editing either: USING here is defence in
-- depth rather than the sole guard. PostgreSQL applies the SELECT policies to an
-- UPDATE whose WHERE reads a column, so taking another patient's row is refused
-- twice over. Widen either one alone and the theft is still refused; only widening
-- both lets the row move. That is why the suite's theft assertion fires on the
-- pair and the visibility assertions fire on the SELECT policy — neither clause is
-- distinguishable on its own while the other stands.
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

-- The patient's write is by column, and for two reasons that have nothing to do
-- with ownership — that part the policies above do hold, which is why `patient_id`
-- is deliberately still in both lists: the predicate is the mechanism here, not the
-- grant, and taking it out of the grant would hide which one is working.
--
-- `id` is withheld because dose events point at it from step 8, and because a
-- primary key the caller chooses is a way to ask whether a guessed one exists: a
-- uniqueness check runs before row security and answers 23505 either way.
--
-- `created_at` is withheld because it is the row's provenance rather than a field
-- of the form: a value the patient sets is a value that can be a lie.
--
-- `label_photo_path` is **not** withheld, and that is the second decision here.
-- Withholding it was the first repair, and it left the column unwritable on every
-- product path — the patient writes vials through this seam, the service path has
-- no UPDATE, and a photo is attached after the vial exists. M4 would have met
-- 42501 and fixed it by moving the write to the service seam, undoing the seam
-- decision quietly. The key is constrained in 000015 instead, to the patient's own
-- prefix, which holds for every role rather than for the ones a grant names.
GRANT SELECT ON app.vials TO cadence_patient;
GRANT INSERT (patient_id, compound_id, concentration_label, total_doses,
              opened_at, expires_on, lot, location_ru, label_photo_path)
    ON app.vials TO cadence_patient;
GRANT UPDATE (patient_id, compound_id, concentration_label, total_doses,
              opened_at, expires_on, lot, location_ru, disposed_at, label_photo_path)
    ON app.vials TO cadence_patient;
GRANT SELECT ON app.vials TO cadence_doctor, cadence_service;
GRANT INSERT ON app.vials TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.vials TO cadence_admin;

RESET ROLE;
