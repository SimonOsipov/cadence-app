-- Rolling the count back reconstructs it; it does not recover it.
--
-- The number a clinic wrote on a box is gone once the column is dropped, so this derives
-- it the way 000021 derived the amount — from the first dose drawn from the vial in the
-- compound's own unit — and falls back to one where no such dose exists. A vial rolled
-- back and rolled forward again therefore keeps its amount and may not keep its count.
-- Naming that is cheaper than pretending the inverse is exact.

DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

ALTER TABLE app.vials ADD COLUMN IF NOT EXISTS total_doses integer;

-- FORCE binds the owner, and cadence_migrator inherits it, so this UPDATE would be
-- filtered to zero rows and report success. Same shape as 000021's backfill, same guard:
-- row_security = off turns a table left off the list into a failure rather than silence.
SET row_security = off;
ALTER TABLE app.vials       NO FORCE ROW LEVEL SECURITY;
ALTER TABLE app.compounds   NO FORCE ROW LEVEL SECURITY;
ALTER TABLE app.dose_events NO FORCE ROW LEVEL SECURITY;

UPDATE app.vials v
SET total_doses = GREATEST(1, pg_catalog.round(v.total_amount / COALESCE(
        (SELECT d.dose_value
         FROM app.dose_events d
         WHERE d.vial_id = v.id AND d.dose_unit = c.default_unit
         ORDER BY d.injected_at, d.created_at, d.id
         LIMIT 1),
        v.total_amount))::integer)
FROM app.compounds c
WHERE c.id = v.compound_id AND v.total_doses IS NULL;

ALTER TABLE app.dose_events FORCE ROW LEVEL SECURITY;
ALTER TABLE app.compounds   FORCE ROW LEVEL SECURITY;
ALTER TABLE app.vials       FORCE ROW LEVEL SECURITY;
RESET row_security;

DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

ALTER TABLE app.vials ALTER COLUMN total_doses SET NOT NULL;
-- Dropped before it is added: the down chain is applied twice by the idempotency test,
-- and ADD CONSTRAINT has no IF NOT EXISTS.
ALTER TABLE app.vials DROP CONSTRAINT IF EXISTS vials_total_doses_check;
ALTER TABLE app.vials
    ADD CONSTRAINT vials_total_doses_check CHECK (total_doses > 0);

ALTER TABLE app.vials ALTER COLUMN total_amount DROP NOT NULL;
ALTER TABLE app.vials ALTER COLUMN amount_unit  DROP NOT NULL;

GRANT INSERT (total_doses) ON app.vials TO cadence_patient;
GRANT UPDATE (total_doses) ON app.vials TO cadence_patient;

RESET ROLE;
