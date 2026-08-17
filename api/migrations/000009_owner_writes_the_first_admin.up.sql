-- The floor under the chicken-and-egg: an admin is created by an admin, and the
-- first one has nobody to create them.
--
-- cmd/bootstrap-admin runs under the migration role, a member of cadence_owner.
-- FORCE extends row level security to the owner, and a forced table with no
-- INSERT policy refuses it — so until here the one-off command had no way to
-- write either of the two rows it exists to write.
--
-- Each policy is as narrow as that job. The owner may bring an admin into being
-- and no other role: patients and doctors are the service path's to create,
-- under its own policies and its own audit, and a second producer of them would
-- be a second thing to reason about. And the owner may sign only as a job, never
-- in a person's name.
--
-- Honestly: a role that can TRUNCATE the table is a weaker author for an audit
-- row than the service path is. audit.md already names that boundary as
-- detectable rather than preventable, and this does not move it — what it does
-- is keep the weaker author from writing a row that reads as a human's.

SET ROLE cadence_owner;

CREATE POLICY profiles_bootstrap_insert ON app.profiles
    FOR INSERT TO cadence_owner
    WITH CHECK (role = 'admin');

-- IS NOT NULL rather than an equality against a published actor, because this
-- command runs through no seam and there is nothing to reconcile against. The
-- table's own CHECK already refuses two actors at once and an empty job, so what
-- is left to say here is which of the two columns this role may use.
CREATE POLICY audit_log_bootstrap_insert ON app.audit_log
    FOR INSERT TO cadence_owner
    WITH CHECK (actor_job IS NOT NULL);

RESET ROLE;
