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
-- tops out at 2,4 мг, tirzepatide at 15 мг, BPC-157 at 250 мкг — and it sits eleven
-- orders of magnitude below the overflow it exists to prevent. All three operands here
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

-- The vial holds an amount rather than a dose, and the same ceiling fits: it is a
-- container of substance, and a gram of peptide in one vial is already absurd.
ALTER TABLE app.vials
    ADD CONSTRAINT vials_total_amount_magnitude_check CHECK (
        (amount_unit = 'мг'  AND total_amount <= 1000)
        OR (amount_unit = 'мкг' AND total_amount <= 1000000)
    );

RESET ROLE;
