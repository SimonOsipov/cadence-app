-- The dose is the patient writing about themselves, like the diary and the medicine
-- cabinet: request seam, RLS, no audit row. The predicate is written out in full
-- rather than leaned on the course — step 2's rule is that a table defers «whose» to
-- a parent only when it has no owner column of its own, and this one has patient_id.
--
-- No UPDATE and no DELETE for anybody but the admin. A logged dose is a fact: the
-- prototype's un-log toggle is replaced by a dose-detail view (§08's build plan), a
-- vial's remaining count is a subtraction over these rows, and adherence is read off
-- them. A mistake is corrected by the clinic, not by the row disappearing.
--
-- The subject stands in USING and in WITH CHECK, so a row cannot be created as
-- somebody else's. What it cannot be created as at all is a row pointing into
-- another patient's course or vial — that is the schema's job here, held by the
-- composite foreign keys in 000019 rather than by any predicate, because a policy
-- guards the request path and a key guards every path.

SET ROLE cadence_owner;

CREATE POLICY dose_events_own_select ON app.dose_events
    FOR SELECT TO cadence_patient
    USING (patient_id = app.jwt_subject());

CREATE POLICY dose_events_own_insert ON app.dose_events
    FOR INSERT TO cadence_patient
    WITH CHECK (patient_id = app.jwt_subject());

CREATE POLICY dose_events_of_my_patients ON app.dose_events
    FOR SELECT TO cadence_doctor
    USING (EXISTS (
        SELECT FROM app.care_team_assignments
        WHERE care_team_assignments.patient_id = dose_events.patient_id
          AND care_team_assignments.provider_id = app.jwt_subject()
    ));

CREATE POLICY dose_events_admin ON app.dose_events
    FOR ALL TO cadence_admin USING (true) WITH CHECK (true);

-- The service path reads and inserts: the seed fills a development clinic, and the
-- missed-dose sweep reads. It does not rewrite — nothing on that path has business
-- editing an injection somebody reported giving themselves.
CREATE POLICY dose_events_service_read ON app.dose_events
    FOR SELECT TO cadence_service USING (true);
CREATE POLICY dose_events_service_insert ON app.dose_events
    FOR INSERT TO cadence_service WITH CHECK (true);

-- ------------------------------------------------------------------ grants --

-- By column on the patient's side, following the precedent identity set and every
-- clinical step has returned to. `created_at` is withheld: it is the row's
-- provenance, not a field of the wizard. Everything else the wizard collects is
-- insertable, `client_request_id` included — the key is the client's by design, and
-- the uniqueness that makes it idempotent is scoped to the patient, so a key
-- borrowed from another patient collides with nothing and reveals nothing.
--
-- No UPDATE column list at all, rather than a narrow one: there is no UPDATE grant.
GRANT SELECT ON app.dose_events TO cadence_patient;
GRANT INSERT (patient_id, protocol_id, protocol_item_id, vial_id, compound_id,
              scheduled_for_date, scheduled_for_time, injected_at,
              dose_value, dose_unit, site_code, mood, side_effects, note,
              photo_path, client_request_id)
    ON app.dose_events TO cadence_patient;

GRANT SELECT ON app.dose_events TO cadence_doctor, cadence_service;
GRANT INSERT ON app.dose_events TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.dose_events TO cadence_admin;

RESET ROLE;
