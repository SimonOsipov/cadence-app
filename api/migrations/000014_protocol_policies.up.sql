-- Five of the six shapes identity uses, on four more tables, plus a seventh that
-- is new here and recorded as a deviation on step 2: a reference table open to
-- both request roles. data-layer.md invariant 2 enumerates six and says «and no
-- others», so widening it is an architecture change and not a detail — the note
-- is rewritten at finalization rather than extended.
--
-- Every policy carries an explicit TO. A policy without it applies to PUBLIC — which includes
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

-- One level deeper than the item's own policy, and it does not repeat it: a phase
-- carries no patient column of its own, and the subquery above runs under
-- protocol_items' policies, which already answer «this patient's».
--
-- This is not what the identity block does — every relation policy in 000004
-- writes its predicate out in full, and none of them is two levels deep. Nor is
-- it what protocol_items does thirty lines above. It is a decision taken here, and
-- these are its two standing costs:
--
--   * both bodies below mention no subject at all, so the guarantee is entirely
--     transitive: a protocol_items policy that stops depending on the subject
--     opens this table with no edit visible in this file;
--   * the two are identical and differ only in TO, so adding a role to either
--     one's TO is the whole of the change.
--
-- Measured rather than argued: a caller with no subject published reads nothing
-- from any of the four tables, and the deepest one is reached by primary key
-- without escaping its predicate.
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

-- Per role, and where it matters per column. A verb absent from this list is a
-- verb nobody holds: there are no default privileges to fall back on.
--
-- Three of the column lists below are on UPDATE and one is on SELECT, and the two
-- kinds are there for different reasons — the UPDATE lists keep a row from
-- changing owner, the SELECT list keeps a timestamp from being an oracle.

-- The reference is readable by column rather than by table. `code` says whether a
-- compound was seeded or typed in by the clinic and `created_at` says when, so
-- the two together tell a patient that somebody was prescribed something new, to
-- the second. No row of another patient is reachable through it.
--
-- Narrowed, not closed, and the difference is worth stating: an unordered SELECT
-- still returns heap order, which is insertion order, so a patient who looks
-- twice still sees which compound is new. What goes is the timestamp. The
-- remaining precision depends on `id` staying random — a time-ordered uuid would
-- put `created_at` back into a column the patient reads.
GRANT SELECT (id, name_ru, default_unit, route, icon)
    ON app.compounds TO cadence_patient, cadence_doctor;
-- No UPDATE, deliberately: a name collision must not rewrite a reference row that
-- other patients' protocols already point at. The consequence for step 6, which
-- has to resolve a typed-in compound to an existing id: the one-round-trip idiom
-- `INSERT … ON CONFLICT … DO UPDATE … RETURNING` is refused here, so the shape is
-- `INSERT … ON CONFLICT DO NOTHING` followed by a SELECT on the conflict path.
GRANT SELECT, INSERT ON app.compounds TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.compounds TO cadence_admin;

-- The service path carries no row predicate, so what keeps it from moving a row
-- across a patient boundary **in place** is the grant — and for that, the grant is
-- the only thing that can. A policy cannot say «patient_id did not change»: WITH
-- CHECK sees the new row and there is no OLD to compare it against.
--
-- In place is the whole of the claim. Copy-and-delete reaches the same end state
-- and nothing here prevents it: INSERT … SELECT onto another patient's course,
-- then DELETE the originals, all within the verbs the write path needs. DELETE
-- has no column form, so a grant cannot narrow it, and the service path carries
-- no row predicate by design. That half is Go's, in step 6 — measured, not
-- assumed: the sequence was run and the rows moved.
--
-- Measured before it was written this way: with a table-wide UPDATE, one
-- statement moved another patient's item onto this patient's course, and the
-- patient then read the moved row through their own policy — correct policy
-- behaviour on a row that had become theirs. The same shape as 000006, where an
-- escalation «Go will prevent» was closed in the database instead.
--
-- INSERT stays table-wide: it names the parent once, at creation, and editing a
-- course replaces items and phases rather than repointing them.
--
-- No DELETE on protocols for the service path: a course ends by becoming
-- 'completed' or 'cancelled', and its history has to survive that.
GRANT SELECT ON app.protocols TO cadence_patient, cadence_doctor, cadence_service;
GRANT INSERT ON app.protocols TO cadence_service;
GRANT UPDATE (start_date, duration_weeks, status, notes)
    ON app.protocols TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.protocols TO cadence_admin;

GRANT SELECT ON app.protocol_items TO cadence_patient, cadence_doctor, cadence_service;
GRANT INSERT, DELETE ON app.protocol_items TO cadence_service;
GRANT UPDATE (kind, compound_id, cadence, days_of_week, times, loggable)
    ON app.protocol_items TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.protocol_items TO cadence_admin;

GRANT SELECT ON app.protocol_phases TO cadence_patient, cadence_doctor, cadence_service;
GRANT INSERT, DELETE ON app.protocol_phases TO cadence_service;
GRANT UPDATE (from_week, to_week, dose_value, dose_unit)
    ON app.protocol_phases TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.protocol_phases TO cadence_admin;

-- Nothing on sequences, and there are none to grant on: every primary key here
-- is a defaulted uuid rather than an identity column.

RESET ROLE;
