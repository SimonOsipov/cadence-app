-- The diary's arrangement on a table of readings: request seam, RLS, no audit row,
-- the subject written out in both halves because this table has an owner column.
--
-- No UPDATE for anybody but the admin: a reading is a clinical fact, and a correction
-- is a new reading. What is new here is DELETE — the first one a patient holds in this
-- schema — and it is bounded twice over, to their own rows and to the ones they typed
-- in. An imported sample is the health platform's fact and reappears on the next sync,
-- so deleting it would be undone rather than honoured.
--
-- That predicate is the seventh form in the registry: the first that filters on
-- ownership and on a second column of the row at once.

SET ROLE cadence_owner;

CREATE POLICY measurements_own_select ON app.measurements
    FOR SELECT TO cadence_patient
    USING (patient_id = app.jwt_subject());

CREATE POLICY measurements_own_insert ON app.measurements
    FOR INSERT TO cadence_patient
    WITH CHECK (patient_id = app.jwt_subject());

CREATE POLICY measurements_own_manual_delete ON app.measurements
    FOR DELETE TO cadence_patient
    USING (patient_id = app.jwt_subject() AND source = 'manual');

CREATE POLICY measurements_of_my_patients ON app.measurements
    FOR SELECT TO cadence_doctor
    USING (EXISTS (
        SELECT FROM app.care_team_assignments
        WHERE care_team_assignments.patient_id = measurements.patient_id
          AND care_team_assignments.provider_id = app.jwt_subject()
    ));

CREATE POLICY measurements_admin ON app.measurements
    FOR ALL TO cadence_admin USING (true) WITH CHECK (true);

-- The service path reads and inserts: the seed fills a development clinic, and the
-- import writer of a later milestone lands here. It does not rewrite or delete —
-- correcting a reading is the clinic's act, through the admin.
CREATE POLICY measurements_service_read ON app.measurements
    FOR SELECT TO cadence_service USING (true);
CREATE POLICY measurements_service_insert ON app.measurements
    FOR INSERT TO cadence_service WITH CHECK (true);

-- ------------------------------------------------------------------ grants --

-- By column on the patient's side, following the precedent every clinical step has
-- returned to. Four are withheld: `id` and `created_at` are the row's provenance,
-- `source` is what makes the delete boundary meaningful — with the grant, a patient
-- could write 'healthkit' onto their own hand-typed row — and `external_id` belongs to
-- an importer that does not exist.
--
-- No UPDATE column list at all, rather than a narrow one: there is no UPDATE grant.
GRANT SELECT, DELETE ON app.measurements TO cadence_patient;
GRANT INSERT (patient_id, metric, value, unit, measured_at, note)
    ON app.measurements TO cadence_patient;

GRANT SELECT ON app.measurements TO cadence_doctor;
GRANT SELECT, INSERT ON app.measurements TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.measurements TO cadence_admin;

RESET ROLE;
