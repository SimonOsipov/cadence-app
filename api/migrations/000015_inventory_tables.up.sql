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
    -- A key into a store that has **no** row level security, so the constraint below
    -- is not tidiness: the endpoint that signs a link checks that the *row* is
    -- visible to the caller and then signs whatever the *path* says. A patient
    -- storing their own vial with a key under somebody else's prefix would pass a
    -- check that was never about the path.
    --
    -- Closed here rather than by withholding the column, and that is a decision.
    -- Withholding was the first repair and it made the column unwritable on every
    -- product path: the patient writes vials through the request seam, the service
    -- path holds no UPDATE at all, and attaching a photo happens after the vial
    -- exists — so M4 would have met 42501 and fixed it the cheap way, by moving the
    -- whole write to the service seam and silently undoing the seam decision. The
    -- CHECK keeps one seam and makes the key unable to point outside its own
    -- prefix, which is a stronger statement than a grant: it holds for every role,
    -- including the admin and the service path.
    label_photo_path    text,
    created_at          timestamptz NOT NULL DEFAULT pg_catalog.now(),
    -- A vial cannot be thrown away before it was opened, and neither date may
    -- precede the other's meaning. Written as two comparisons rather than one
    -- range, because either date may be null and a range would be NULL — and a
    -- CHECK evaluating to NULL passes, which this schema has already been bitten
    -- by twice — once on array_length returning NULL for an empty array, and once on
    -- cardinality counting a NULL element.
    -- Named explicitly because it reads two columns: an unnamed one becomes
    -- `vials_check`, which tells a caller nothing and tells a test even less — the
    -- refusal tables in this project pin the constraint name, and «the wrong
    -- neighbour fired» is a defect this schema has already had once.
    CONSTRAINT vials_photo_key_is_under_its_own_prefix CHECK (
        label_photo_path IS NULL
        OR (pg_catalog.length(label_photo_path) BETWEEN 1 AND 500
            AND label_photo_path LIKE patient_id::text || '/%')
    ),
    CONSTRAINT vials_disposed_after_opening CHECK (
        disposed_at IS NULL OR opened_at IS NULL OR disposed_at >= opened_at
    )
);

-- One index, not two. The reorder hint walks a patient's live vials of one
-- compound and the dose-logging path resolves a vial by patient, and a leading
-- column serves both — a separate (patient_id) index would be a strict prefix of
-- this one, and the cascade from profiles uses it too.
CREATE INDEX vials_by_patient_and_compound ON app.vials (patient_id, compound_id);

ALTER TABLE app.vials ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.vials FORCE  ROW LEVEL SECURITY;

RESET ROLE;
