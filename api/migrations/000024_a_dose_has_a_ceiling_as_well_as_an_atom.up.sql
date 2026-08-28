-- The three operands were bounded from below and left open above.
--
-- 000021 and 000023 say how fine a dose may be because the cabinet counts whole
-- micrograms. Nothing says how large, and the count is int64: measured on arm64, a phase
-- of 1e19 мг passes the writer and every CHECK, AmountOf saturates to MaxInt64 and the
-- day card answers zero injections left; the same row read on amd64 wraps to MinInt64,
-- the arithmetic reads it as no dose and the count disappears instead. One value, two
-- answers, chosen by the machine that ran the read.
--
-- A gram is the ceiling because nothing the clinic injects comes near it — semaglutide
-- tops out at 2,4 мг, tirzepatide at 15 мг, BPC-157 at 250 мкг — and a gram is 10⁶ мкг
-- against a MaxInt64 of 9,2 × 10¹⁸, thirteen orders of margin. All three operands here
-- rather than the divisor alone: bounding two of the three and leaving the third is what
-- 000023 was written to repair.

SET ROLE cadence_owner;

ALTER TABLE app.protocol_phases
    ADD CONSTRAINT protocol_phases_dose_value_magnitude_check CHECK (
        (dose_unit = 'мг'  AND dose_value <= 1000)
        OR (dose_unit = 'мкг' AND dose_value <= 1000000)
    );

ALTER TABLE app.dose_events
    ADD CONSTRAINT dose_events_dose_value_magnitude_check CHECK (
        (dose_unit = 'мг'  AND dose_value <= 1000)
        OR (dose_unit = 'мкг' AND dose_value <= 1000000)
    );

-- The vial is a container and not a dose, so its ceiling is its own and a hundred times
-- higher. The product runs hormone protocols beside the peptide ones: testosterone
-- cypionate comes at 250 мг/мл in a 10 мл multi-dose vial, which is 2500 мг — two and a
-- half times a dose ceiling, and a doctor meeting it would simply be unable to register
-- the vial. A hundred grams keeps eleven orders of margin against MaxInt64 micrograms.
ALTER TABLE app.vials
    ADD CONSTRAINT vials_total_amount_magnitude_check CHECK (
        (amount_unit = 'мг'  AND total_amount <= 100000)
        OR (amount_unit = 'мкг' AND total_amount <= 100000000)
    );

RESET ROLE;
