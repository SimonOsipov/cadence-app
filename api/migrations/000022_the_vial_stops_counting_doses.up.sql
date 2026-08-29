-- The count goes, and the amount becomes the only quantity a vial carries.
--
-- 000021 landed the amount beside total_doses so nothing had to change at once. The Go
-- side reads the amount now, so the count is a second answer to one question — and §03's
-- own rule is that nothing derived is stored.
--
-- SET NOT NULL is deliberately allowed to fail. 000021 leaves total_amount empty on a
-- vial it could not convert — no dose was ever drawn from it in the compound's own unit —
-- and an empty column is exactly the signal a human should read before the chain moves
-- on. The alternative was inventing a multiplier, which nothing downstream could detect.

SET ROLE cadence_owner;

ALTER TABLE app.vials ALTER COLUMN total_amount SET NOT NULL;
ALTER TABLE app.vials ALTER COLUMN amount_unit  SET NOT NULL;

ALTER TABLE app.vials DROP COLUMN total_doses;

RESET ROLE;
