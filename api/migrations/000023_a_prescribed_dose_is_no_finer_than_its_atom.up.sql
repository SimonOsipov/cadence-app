-- The prescribed dose is the cabinet's divisor, and it had no bound.
--
-- 000021 bounded the scale of a vial's amount and of a logged dose, because both are
-- operands of the subtraction. It missed the third: app.protocol_phases.dose_value is
-- what PhaseDose reads, and step 2 made it what remaining substance is divided by.
--
-- Measured on the unbounded column: a phase of 0,0001 мг converts to zero micrograms,
-- so the day card silently stops answering how many injections are left and the reorder
-- hint disappears; a phase of 0,0006 мг converts to one microgram and the card answers
-- two thousand injections out of a 2 мг vial. Both pass CHECK (dose_value > 0).

SET ROLE cadence_owner;

-- A disjunction and not a CASE on the unit, for the reason 000021 records over the
-- same bound on vials: a CASE with no arm for a unit yields NULL, and a CHECK over
-- NULL passes, so a unit nobody prescribes would carry any scale at all through.
ALTER TABLE app.protocol_phases
    ADD CONSTRAINT protocol_phases_dose_value_scale_check CHECK (
        (dose_unit = 'мг'  AND pg_catalog.scale(dose_value) <= 3)
        OR (dose_unit = 'мкг' AND pg_catalog.scale(dose_value) = 0)
    );

RESET ROLE;
