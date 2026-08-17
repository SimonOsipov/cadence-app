-- The record that an address was invited by this clinic, and the reason a
-- creation interrupted between the invitation and the commit can be finished
-- rather than refused forever.
--
-- The window is irreducible: the invitation is a side effect of another system
-- and the commit always comes after it. What this table changes is what a repeat
-- of the same request can know — an account that exists and carries a record of
-- ours is ours to claim, and an account with no record is somebody else's.
--
-- What §03 lists and this does not store: `status`, which is derived from the
-- account and the link; `payload`, whose name and assignments are rows the same
-- transaction writes; `role`, which is on the profile written beside it; and a
-- separate `gotrue_user_id`, because user_id is that identifier already.
--
-- Address uniqueness is deliberately not imposed. The provider holds it for live
-- accounts, and claiming a confirmed account deletes it and invites again, which
-- produces a second user_id for one address — a unique index here would refuse
-- exactly the recovery it looks like it protects.

SET ROLE cadence_owner;

CREATE TABLE app.invites (
    -- The identifier the provider assigned, which is also profiles.user_id: one
    -- generated here would leave the token issuance hook resolving nothing.
    user_id    uuid PRIMARY KEY,
    email      text NOT NULL,
    -- No ON DELETE. Removing a doctor who has invitations outstanding is refused
    -- rather than allowed to erase the records those accounts are claimed
    -- through; the alternative turns a claimable account into a permanent 409.
    invited_by uuid NOT NULL REFERENCES app.profiles (user_id),
    invited_at timestamptz NOT NULL DEFAULT pg_catalog.now(),
    -- The spelling identity.NormalizeAddress produces, held by the schema rather
    -- than by the discipline of one caller: an unfolded row is a lookup key that
    -- never matches, and the request that follows finds no record of its own
    -- account. btrim removes spaces where the fold's TrimSpace removes every kind
    -- of whitespace, so this is the narrower of the two claims.
    CONSTRAINT invites_email_is_folded CHECK (
        email = pg_catalog.btrim(pg_catalog.lower(email)) AND email <> ''
    )
);

ALTER TABLE app.invites ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.invites FORCE  ROW LEVEL SECURITY;

-- §04's row for this table: the doctor creates, the admin reads, the patient
-- holds nothing at all. The service path is added to it because the transaction
-- that writes the record runs there and has to recognise its own on a retry.

-- The row says who invited, and a doctor may say only themselves. Without the
-- predicate every doctor can attribute an invitation to a colleague, which is
-- precisely what the admin reads this table to find out.
CREATE POLICY invites_doctor_insert ON app.invites
    FOR INSERT TO cadence_doctor
    WITH CHECK (invited_by = app.jwt_subject());

CREATE POLICY invites_admin_read ON app.invites
    FOR SELECT TO cadence_admin USING (true);

CREATE POLICY invites_service_read ON app.invites
    FOR SELECT TO cadence_service USING (true);
CREATE POLICY invites_service_insert ON app.invites
    FOR INSERT TO cadence_service WITH CHECK (true);

-- No UPDATE and no DELETE for anybody, so neither exists to be granted: a record
-- that can be rewritten cannot witness what it was created to witness. A repeated
-- invitation of the same account leaves the row it already wrote.
GRANT INSERT ON app.invites TO cadence_doctor;
GRANT SELECT ON app.invites TO cadence_admin;
GRANT SELECT, INSERT ON app.invites TO cadence_service;

RESET ROLE;
