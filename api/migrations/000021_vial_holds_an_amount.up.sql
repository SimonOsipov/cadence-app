-- The vial holds an amount of substance, not a number of injections.
--
-- §03's third correction made a vial's remainder a subtraction of counts; the proposal
-- `a-vial-holds-an-amount` makes it a subtraction of milligrams, because a count is
-- right only while the dose never changes, and a titrating course changes it.
--
-- Additive: total_doses stays and no existing writer changes, so the tree is green
-- and the contract is untouched.

SET ROLE cadence_owner;

-- Bounded per unit because the atom differs: the cabinet's arithmetic is integer
-- micrograms, so a tail finer than the unit's atom is lost on conversion and a value
-- that converts to zero never empties a vial.
--
-- First, so the backfill below can rely on it: total_doses × dose_value keeps the
-- numeric's scale, and an unbounded dose would carry that straight into total_amount.
ALTER TABLE app.dose_events
    ADD CONSTRAINT dose_events_dose_value_scale_check CHECK (
        CASE dose_unit
            WHEN 'мг'  THEN pg_catalog.scale(dose_value) <= 3
            WHEN 'мкг' THEN pg_catalog.scale(dose_value) = 0
        END
    );

ALTER TABLE app.vials ADD COLUMN total_amount numeric;
ALTER TABLE app.vials ADD COLUMN amount_unit  text;
ALTER TABLE app.vials ADD COLUMN held_back_at date;

-- FORCE is lifted for the length of the backfill, on every table the two statements
-- read. app.vials, app.compounds, app.dose_events and app.profiles all FORCE row
-- security, and FORCE binds the owner — and cadence_migrator inherits cadence_owner
-- (000001:145), so RESET ROLE does not escape it either. Measured: under the role that
-- actually applies the chain both UPDATEs touched zero rows and reported success.
--
-- row_security = off is the guard rather than the mechanism: with FORCE lifted the
-- owner is exempt and it is satisfied, but a table left out of the list below raises
-- instead of silently filtering.
--
-- The restore below is safe on a mid-file failure only because golang-migrate sends
-- the file as one implicit transaction; `x-multi-statement=true` on the migration URL
-- would split it into autocommit statements and a failure here would leave four
-- tables unforced on a live database.
SET row_security = off;
ALTER TABLE app.vials       NO FORCE ROW LEVEL SECURITY;
ALTER TABLE app.compounds   NO FORCE ROW LEVEL SECURITY;
ALTER TABLE app.dose_events NO FORCE ROW LEVEL SECURITY;
ALTER TABLE app.profiles    NO FORCE ROW LEVEL SECURITY;

-- The unit comes from the compound, not a literal: a BPC-157 vial stamped 'мг' would
-- be unusable for every dose that drug is ever given in.
--
-- The amount stays NULL where no same-unit dose has been drawn, rather than falling
-- back to a multiplier of one. A fallback would write the injection count into the
-- amount column — for BPC-157, dosed at 250 мкг, that is out by a factor of 250,
-- positive, and indistinguishable from an honest backfill. Step 2's SET NOT NULL finds
-- a NULL; it never finds an invented number.
UPDATE app.vials v
SET amount_unit = c.default_unit,
    total_amount = v.total_doses * (
        SELECT d.dose_value
        FROM app.dose_events d
        WHERE d.vial_id = v.id AND d.dose_unit = c.default_unit
        -- Two events may share an instant on a hand-built stand row; without the
        -- second and third keys the multiplier is whichever the heap returned.
        ORDER BY d.injected_at, d.created_at, d.id
        LIMIT 1)
FROM app.compounds c
WHERE c.id = v.compound_id;

-- IsDrawableFor admits a sealed vial and no product path writes opened_at, so a
-- part-used vial reads as sealed. This backfills the rows that exist; both write paths
-- — the named vial and the automatic one — are closed in step 3.
--
-- The day is the patient's, not the server's — an injection at 01:00 in Moscow
-- belongs to that day, and every other read in this feature says so.
UPDATE app.vials v
SET opened_at = e.first_draw
FROM (
    SELECT d.vial_id, min((d.injected_at AT TIME ZONE p.timezone)::date) AS first_draw
    FROM app.dose_events d
    JOIN app.profiles p ON p.user_id = d.patient_id
    WHERE d.vial_id IS NOT NULL
    GROUP BY d.vial_id
) e
WHERE e.vial_id = v.id
  AND v.opened_at IS NULL
  -- A vial thrown away before its first recorded dose would violate
  -- vials_disposed_after_opening from 000015 and fail the whole migration. Measured.
  AND (v.disposed_at IS NULL OR v.disposed_at >= e.first_draw);

ALTER TABLE app.profiles    FORCE ROW LEVEL SECURITY;
ALTER TABLE app.dose_events FORCE ROW LEVEL SECURITY;
ALTER TABLE app.compounds   FORCE ROW LEVEL SECURITY;
ALTER TABLE app.vials       FORCE ROW LEVEL SECURITY;
RESET row_security;

-- Nullable: the Go side still writes total_doses, and NOT NULL now would rewrite every
-- vial fixture in three packages twice — once here and once in the rewrite that drops
-- total_doses. The tightening travels with that rewrite. Until it lands the three
-- CHECKs below are vacuous, because a CHECK over NULL passes.
ALTER TABLE app.vials
    ADD CONSTRAINT vials_total_amount_check CHECK (total_amount > 0),
    -- Written as a disjunction rather than a CASE on the unit: a CASE with no arm
    -- for a NULL unit yields NULL, a CHECK over NULL passes, and 0,0001 with no unit
    -- named would walk straight through the bound. An absent amount still passes —
    -- that is the nullable window, not a hole.
    ADD CONSTRAINT vials_total_amount_scale_check CHECK (
        total_amount IS NULL
        OR (amount_unit = 'мг'  AND pg_catalog.scale(total_amount) <= 3)
        OR (amount_unit = 'мкг' AND pg_catalog.scale(total_amount) = 0)
    ),
    ADD CONSTRAINT vials_amount_unit_check CHECK (amount_unit IN ('мг', 'мкг')),
    -- Held back and thrown away are opposite ends of the same lifecycle. Disposal
    -- clears the flag rather than meeting this constraint, so a held-back vial can
    -- still be discarded.
    ADD CONSTRAINT vials_not_held_back_while_disposed
        CHECK (held_back_at IS NULL OR disposed_at IS NULL);

-- Additive, not a replacement: GRANT never revokes, so a column dropped from this
-- list would stay granted. The list is written whole for reading; the registry test
-- in platform/database owns what is actually held.
GRANT INSERT (patient_id, compound_id, concentration_label, total_doses,
              total_amount, amount_unit,
              opened_at, expires_on, lot, location_ru, disposed_at, label_photo_path)
    ON app.vials TO cadence_patient;
-- held_back_at is UPDATE-only, unlike disposed_at in 000016: holding a vial back is an
-- act on one that already exists, and no form has to create a row already held back.
GRANT UPDATE (patient_id, compound_id, concentration_label, total_doses,
              total_amount, amount_unit, held_back_at,
              opened_at, expires_on, lot, location_ru, disposed_at, label_photo_path)
    ON app.vials TO cadence_patient;

RESET ROLE;
