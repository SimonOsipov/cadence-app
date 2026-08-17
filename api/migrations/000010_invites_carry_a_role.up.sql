-- What the invitation was for. 000008 left this out on the grounds that the role
-- "is on the profile written beside it" — true of the finished state, and not of
-- the path to it. The profile is written in the transaction *after* this row, and
-- deliberately: an invitation cannot be unsent, so the record of it commits first
-- or an interruption leaves an address nothing on this side recognises.
--
-- Measured 2026-08-17 on the cycle database, before this column existed: a staff
-- creation interrupted between its two commits left `invite yes, profile no`, a
-- doctor's POST /v1/patients read that as its own half-finished patient and
-- answered 201 — and the patient profile it then wrote made every later request
-- for the address answer 409 through RefuseAlreadyOnboarded. The staff address
-- became permanently unusable, which is the same failure the commit ordering was
-- chosen to prevent, reached from the other side.
--
-- No DEFAULT, and that is the load-bearing half of this column rather than a
-- preference. A default makes the column silently fillable: an INSERT that forgets
-- the parameter compiles, runs, and records a guess as a fact — reopening the
-- window this closes, in a column whose whole job is to keep it shut. Without one,
-- such a statement fails loudly. NOT NULL costs a reset of any database holding
-- rows; there is nothing to backfill from, because a row with no profile is
-- exactly the row whose role is unknown.

SET ROLE cadence_owner;

-- The same closed set as profiles.role: the two describe one fact at two moments,
-- and a role invitable here but not writable there would be an invitation that
-- cannot be completed.
ALTER TABLE app.invites
    ADD COLUMN role text NOT NULL
    CONSTRAINT invites_role_is_known CHECK (role IN ('patient', 'doctor', 'admin'));

RESET ROLE;
