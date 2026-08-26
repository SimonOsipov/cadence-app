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

ALTER TABLE app.protocol_phases
    ADD CONSTRAINT protocol_phases_dose_value_scale_check CHECK (
        CASE dose_unit
            WHEN 'мг'  THEN pg_catalog.scale(dose_value) <= 3
            WHEN 'мкг' THEN pg_catalog.scale(dose_value) = 0
        END
    );

RESET ROLE;
