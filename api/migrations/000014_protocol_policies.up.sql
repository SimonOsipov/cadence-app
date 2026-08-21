-- The same six shapes as identity, on four more tables, and every one of them
-- carries an explicit TO. A policy without it applies to PUBLIC — which includes
-- the service path, and would hand it the request path's row predicate.
--
-- No policy body below contains 'patient', 'doctor' or 'admin'. The role decision
-- is TO and nothing else; a test asserts the absence.
--
-- The reach of a doctor runs through care_team_assignments and through nothing
-- else. «role = doctor therefore sees every patient» appears nowhere, which is
-- what multi-clinic support will rest on.
--
-- Depth is new here and worth naming. A phase's policy reads protocol_items,
-- whose policy reads protocols, whose policy reads care_team_assignments, whose
-- policies read only their own columns. Each subquery runs under the policies of
-- the table it reads, so the chain terminates — and it terminates because
-- care_team_assignments refers to nothing. A back-reference anywhere along it is
-- 42P17 rather than a leak, which is a loud failure and still a failure.

SET ROLE cadence_owner;

-- --------------------------------------------------------------- compounds --

-- The one table in this migration that is not patient-scoped: a drug reference
-- is the same for everybody, and there is nothing in a row to filter on. Written
-- as USING (true) rather than left without a policy, because a forced table with
-- no policy refuses everyone.
CREATE POLICY compounds_read ON app.compounds
    FOR SELECT TO cadence_patient, cadence_doctor USING (true);

CREATE POLICY compounds_admin ON app.compounds
    FOR ALL TO cadence_admin USING (true) WITH CHECK (true);

-- A compound the clinic types in is written on the service path, in the same
-- transaction as the protocol that needed it.
CREATE POLICY compounds_service_read ON app.compounds
    FOR SELECT TO cadence_service USING (true);
CREATE POLICY compounds_service_insert ON app.compounds
    FOR INSERT TO cadence_service WITH CHECK (true);

-- --------------------------------------------------------------- protocols --

CREATE POLICY protocols_own_select ON app.protocols
    FOR SELECT TO cadence_patient
    USING (patient_id = app.jwt_subject());

-- Adding an assignment row grants the visibility and deleting it takes the
-- visibility away, with no query edited anywhere.
CREATE POLICY protocols_of_my_patients ON app.protocols
    FOR SELECT TO cadence_doctor
    USING (EXISTS (
        SELECT FROM app.care_team_assignments
        WHERE care_team_assignments.patient_id = protocols.patient_id
          AND care_team_assignments.provider_id = app.jwt_subject()
    ));

CREATE POLICY protocols_admin ON app.protocols
    FOR ALL TO cadence_admin USING (true) WITH CHECK (true);

-- A protocol is a cross-actor write — the doctor writes it about the patient —
-- so it travels the service seam and leaves an audit row. The row predicate is
-- absent on purpose: that authorization lives in Go, and the verbs the path is
-- trusted with are exactly the ones granted at the foot of this file.
CREATE POLICY protocols_service_read ON app.protocols
    FOR SELECT TO cadence_service USING (true);
CREATE POLICY protocols_service_insert ON app.protocols
    FOR INSERT TO cadence_service WITH CHECK (true);
CREATE POLICY protocols_service_update ON app.protocols
    FOR UPDATE TO cadence_service USING (true) WITH CHECK (true);

-- ---------------------------------------------------------- protocol_items --

CREATE POLICY protocol_items_own_select ON app.protocol_items
    FOR SELECT TO cadence_patient
    USING (EXISTS (
        SELECT FROM app.protocols
        WHERE protocols.id = protocol_items.protocol_id
          AND protocols.patient_id = app.jwt_subject()
    ));

CREATE POLICY protocol_items_of_my_patients ON app.protocol_items
    FOR SELECT TO cadence_doctor
    USING (EXISTS (
        SELECT FROM app.protocols
        JOIN app.care_team_assignments
          ON care_team_assignments.patient_id = protocols.patient_id
        WHERE protocols.id = protocol_items.protocol_id
          AND care_team_assignments.provider_id = app.jwt_subject()
    ));

CREATE POLICY protocol_items_admin ON app.protocol_items
    FOR ALL TO cadence_admin USING (true) WITH CHECK (true);

-- DELETE is here and not on protocols: editing a course removes items and phases
-- from it, and a course itself is cancelled rather than deleted — a deleted
-- protocol would take the dose history's foreign key with it.
CREATE POLICY protocol_items_service_read ON app.protocol_items
    FOR SELECT TO cadence_service USING (true);
CREATE POLICY protocol_items_service_insert ON app.protocol_items
    FOR INSERT TO cadence_service WITH CHECK (true);
CREATE POLICY protocol_items_service_update ON app.protocol_items
    FOR UPDATE TO cadence_service USING (true) WITH CHECK (true);
CREATE POLICY protocol_items_service_delete ON app.protocol_items
    FOR DELETE TO cadence_service USING (true);

-- --------------------------------------------------------- protocol_phases --

CREATE POLICY protocol_phases_own_select ON app.protocol_phases
    FOR SELECT TO cadence_patient
    USING (EXISTS (
        SELECT FROM app.protocol_items
        WHERE protocol_items.id = protocol_phases.protocol_item_id
    ));

-- One level deeper than the item's own policy, and it does not repeat it: the
-- subquery above runs under protocol_items' policies, which already answer «this
-- patient's». Repeating the join here would be a second copy of the rule to keep
-- in step, and the identity block chose the same way.
CREATE POLICY protocol_phases_of_my_patients ON app.protocol_phases
    FOR SELECT TO cadence_doctor
    USING (EXISTS (
        SELECT FROM app.protocol_items
        WHERE protocol_items.id = protocol_phases.protocol_item_id
    ));

CREATE POLICY protocol_phases_admin ON app.protocol_phases
    FOR ALL TO cadence_admin USING (true) WITH CHECK (true);

CREATE POLICY protocol_phases_service_read ON app.protocol_phases
    FOR SELECT TO cadence_service USING (true);
CREATE POLICY protocol_phases_service_insert ON app.protocol_phases
    FOR INSERT TO cadence_service WITH CHECK (true);
CREATE POLICY protocol_phases_service_update ON app.protocol_phases
    FOR UPDATE TO cadence_service USING (true) WITH CHECK (true);
CREATE POLICY protocol_phases_service_delete ON app.protocol_phases
    FOR DELETE TO cadence_service USING (true);

-- ------------------------------------------------------------------ grants --

-- Table-wide throughout: there is no column here that one role may see and
-- another may change, which is what the identity block's column grants exist for.
-- A verb absent from this list is a verb nobody holds; there are no default
-- privileges to fall back on.

GRANT SELECT ON app.compounds TO cadence_patient, cadence_doctor, cadence_service;
GRANT INSERT ON app.compounds TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.compounds TO cadence_admin;

-- No DELETE for the service path: a course ends by becoming 'completed' or
-- 'cancelled', and its history has to survive that.
GRANT SELECT ON app.protocols TO cadence_patient, cadence_doctor, cadence_service;
GRANT INSERT, UPDATE ON app.protocols TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.protocols TO cadence_admin;

GRANT SELECT ON app.protocol_items TO cadence_patient, cadence_doctor, cadence_service;
GRANT INSERT, UPDATE, DELETE ON app.protocol_items TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.protocol_items TO cadence_admin;

GRANT SELECT ON app.protocol_phases TO cadence_patient, cadence_doctor, cadence_service;
GRANT INSERT, UPDATE, DELETE ON app.protocol_phases TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.protocol_phases TO cadence_admin;

-- Nothing on sequences, and there are none to grant on: every primary key here
-- is a defaulted uuid rather than an identity column.

RESET ROLE;
