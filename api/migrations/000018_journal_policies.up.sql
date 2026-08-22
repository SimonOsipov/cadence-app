-- The diary is the patient writing about themselves, like the medicine cabinet and
-- unlike the protocol: request seam, RLS, no audit row.
--
-- The predicate is written out in full rather than leaned on a parent, and that is
-- the rule step 3's record left for this step: a table may defer «whose» to the
-- table it references only when it has no owner column of its own. This one has
-- `patient_id`, so it answers for itself — and it is cheaper as well as clearer,
-- because the column is half the primary key.
--
-- The subject stands in USING and in WITH CHECK, so a row cannot be created as
-- somebody else's and cannot be handed away. Measured on vials and true for the same
-- reason here: PostgreSQL adds the SELECT policies to an UPDATE only when the
-- statement reads a column, so the update's USING is the unconditional guard and an
-- UPDATE with no WHERE would otherwise sweep the table.

SET ROLE cadence_owner;

CREATE POLICY journal_entries_own_select ON app.journal_entries
    FOR SELECT TO cadence_patient
    USING (patient_id = app.jwt_subject());

CREATE POLICY journal_entries_own_insert ON app.journal_entries
    FOR INSERT TO cadence_patient
    WITH CHECK (patient_id = app.jwt_subject());

-- The WITH CHECK cannot fail for this role as the grants stand, which is not the
-- same as never being asked: a standalone hand-away is refused as a permission
-- before any policy sees it (patient_id is withheld from UPDATE, measured), but on
-- the upsert of step 8 no grant is consulted for a column absent from the SET list,
-- and this policy is what answers — over an unchanged patient_id, so both halves
-- are the same expression. It is the lock that holds if the column is ever granted.
CREATE POLICY journal_entries_own_update ON app.journal_entries
    FOR UPDATE TO cadence_patient
    USING (patient_id = app.jwt_subject())
    WITH CHECK (patient_id = app.jwt_subject());

-- No DELETE for anybody but the admin: a day is edited, not unwritten, and the
-- side-effect flag the doctor sees is computed off this history.

-- The second conjunct is redundant while care_team_assignments_mine_provider stands
-- — the subquery runs under the caller, so RLS on that table already narrows it to
-- this doctor's rows (measured: removing it changes no test's behaviour). Kept, and
-- not to be «fixed» back out: it is what holds if that policy is ever widened.
CREATE POLICY journal_entries_of_my_patients ON app.journal_entries
    FOR SELECT TO cadence_doctor
    USING (EXISTS (
        SELECT FROM app.care_team_assignments
        WHERE care_team_assignments.patient_id = journal_entries.patient_id
          AND care_team_assignments.provider_id = app.jwt_subject()
    ));

CREATE POLICY journal_entries_admin ON app.journal_entries
    FOR ALL TO cadence_admin USING (true) WITH CHECK (true);

-- The service path reads and inserts, and does not update: the seed fills a
-- development clinic, and nothing on that path has business rewriting a day somebody
-- wrote about themselves.
CREATE POLICY journal_entries_service_read ON app.journal_entries
    FOR SELECT TO cadence_service USING (true);
CREATE POLICY journal_entries_service_insert ON app.journal_entries
    FOR INSERT TO cadence_service WITH CHECK (true);

-- ------------------------------------------------------------------ grants --

-- By column on the patient's side, following the precedent the identity block set
-- and step 3 returned to. Two are withheld:
--
--   `created_at` — the row's provenance rather than a field of the form;
--   `patient_id` on UPDATE — not because the policy needs help, but because half of
--   it is the primary key and moving a row between patients here is not an edit of
--   a day, it is a different row. It stays writable on INSERT, where the WITH CHECK
--   is what decides.
--
-- `source` is *not* withheld, and that is worth stating because it looks like it
-- should be. The rule is «set once, never rewritten», and no grant expresses it: the
-- handler and the caller reach the table as the same role, so the database cannot
-- tell which of them wrote the value. It is held by the merge, in Go, and tested
-- there. What a patient can do by writing it directly is mislabel their own day —
-- in their own feed and in their doctor's, who reads the same row — which is
-- cosmetic and about a day that is theirs.
GRANT SELECT ON app.journal_entries TO cadence_patient;
GRANT INSERT (patient_id, entry_date, mood, energy, sleep, tags, note, source)
    ON app.journal_entries TO cadence_patient;
GRANT UPDATE (entry_date, mood, energy, sleep, tags, note, source)
    ON app.journal_entries TO cadence_patient;

GRANT SELECT ON app.journal_entries TO cadence_doctor, cadence_service;
GRANT INSERT ON app.journal_entries TO cadence_service;
GRANT SELECT, INSERT, UPDATE, DELETE ON app.journal_entries TO cadence_admin;

RESET ROLE;
