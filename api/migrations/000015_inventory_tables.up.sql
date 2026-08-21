-- The «Аптечка»: what is opened, what is sealed, what is expiring, what needs
-- reordering.
--
-- Three of those four words name a status, and there is no status column below.
-- §03's third correction derives all of them on read, and the remaining count
-- with them: remaining is total_doses minus the number of dose events drawn from
-- this vial, so there is no counter to drift out of step with the history. If a
-- later migration is about to add `status` or `remaining` here, it is undoing
-- this one.
--
-- Tables and RLS here, policies and grants in the next migration, as with both
-- pairs before it.

SET ROLE cadence_owner;

CREATE TABLE app.vials (
    id                  uuid PRIMARY KEY DEFAULT pg_catalog.gen_random_uuid(),
    patient_id          uuid NOT NULL REFERENCES app.profiles (user_id) ON DELETE CASCADE,
    -- RESTRICT, like protocol_items: deleting a reference row must not quietly
    -- empty somebody's medicine cabinet.
    compound_id         uuid NOT NULL REFERENCES app.compounds (id) ON DELETE RESTRICT,
    -- A label, not a number: «1 мг/мл» is what is printed on the vial, and the
    -- clinic transcribes it rather than computing with it. Doses are counted, not
    -- measured out of a concentration.
    concentration_label text NOT NULL CHECK (pg_catalog.length(concentration_label) BETWEEN 1 AND 50),
    total_doses         integer NOT NULL CHECK (total_doses > 0),
    -- Null until the vial is opened, and that is the whole of what «sealed» means
    -- — there is no column saying so.
    opened_at           date,
    expires_on          date NOT NULL,
    lot                 text CHECK (lot IS NULL OR pg_catalog.length(lot) BETWEEN 1 AND 50),
    location_ru         text CHECK (location_ru IS NULL OR pg_catalog.length(location_ru) BETWEEN 1 AND 100),
    disposed_at         date,
    label_photo_path    text,
    created_at          timestamptz NOT NULL DEFAULT pg_catalog.now(),
    -- A vial cannot be thrown away before it was opened, and neither date may
    -- precede the other's meaning. Written as two comparisons rather than one
    -- range, because either date may be null and a range would be NULL — and a
    -- CHECK evaluating to NULL passes, which this schema has already been bitten
    -- by once.
    CONSTRAINT vials_disposed_after_opening CHECK (
        disposed_at IS NULL OR opened_at IS NULL OR disposed_at >= opened_at
    )
);

-- Both directions the math reads. The reorder hint walks a patient's live vials
-- of one compound, and the dose-logging path resolves a vial by patient.
CREATE INDEX vials_by_patient ON app.vials (patient_id);
CREATE INDEX vials_by_patient_and_compound ON app.vials (patient_id, compound_id);

ALTER TABLE app.vials ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.vials FORCE  ROW LEVEL SECURITY;

RESET ROLE;
