SET ROLE cadence_owner;

ALTER TABLE app.invites DROP COLUMN IF EXISTS role;

RESET ROLE;
