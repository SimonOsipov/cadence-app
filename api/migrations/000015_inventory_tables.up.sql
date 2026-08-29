-- The «Аптечка»: what is opened, what is sealed, what is expiring, what needs
-- reordering.
--
-- Three of those four words name a status, and there is no status column below.
-- §03's third correction derives all of them on read, and what is left in a vial
-- with them: a subtraction over the history rather than a stored counter, so there
-- is nothing that can drift out of step with it. What is subtracted changed —
-- dose counts here, substance from 000022 on — and the rule did not. If a later
-- migration is about to add `status` or `remaining` here, it is undoing this one.
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
    -- clinic transcribes it rather than computing with it. What a vial holds and
    -- what a dose takes out of it are stated, never derived from this string.
    concentration_label text NOT NULL CHECK (pg_catalog.length(concentration_label) BETWEEN 1 AND 50),
    -- Dropped by 000022, which makes what a vial holds an amount of substance.
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
    -- CHECK keeps one seam, and it is a stronger statement than a grant: it holds
    -- for every role, including the owner, the admin, the service path and any role
    -- a future migration adds — measured, the superuser is refused too.
    label_photo_path    text,
    created_at          timestamptz NOT NULL DEFAULT pg_catalog.now(),
    -- A shape and not a prefix, and the difference was measured. `LIKE prefix || '/%'`
    -- was the first form: it pins where the string *begins*, not where the key
    -- *points*, and `%` matches `..` happily. A key of `<own>/../<other>/label.jpg`
    -- satisfied it, and both natural ways to build a URL from a stored key resolve
    -- it — `path.Join(bucket, key)` and `path.Clean` each yield the other patient's
    -- object. The signer of step 10 checks that the row is visible and then signs
    -- what the key says, so the check would have passed on a genuinely-own row.
    --
    -- The regex admits one to four segments, each beginning with an alphanumeric,
    -- so `..` is not a segment, `//` is not one either, and there is no trailing
    -- slash, no newline and no bare prefix naming no object. The uuid text is
    -- regex-literal — uuid_out renders lowercase hex and hyphens, none of which is a
    -- metacharacter in either `~` or `LIKE` — and that is a property of the column's
    -- type rather than of this line, which is why it is written here.
    --
    -- Two consequences for step 10, which is the one that signs links and the one
    -- that will read this. The key layout is fixed here — `{patient_id}/…`, with no
    -- bucket segment — so a key qualified by bucket does not fit the column and
    -- moving to one is a migration. And the protection is positional rather than
    -- normalizing: it refuses a key that climbs, it does not rewrite one, so the
    -- signer must not normalize the path before signing it either.
    --
    -- The length bound lives in the pattern now rather than beside it: four
    -- segments of at most 101 characters after a 36-character uuid is 440, and a
    -- second bound would be a second number to keep in step.
    --
    -- Named explicitly because it reads two columns: an unnamed one becomes
    -- `vials_check`, which tells a caller nothing and tells a test even less — the
    -- refusal tables in this project pin the constraint name, and «the wrong
    -- neighbour fired» is a defect this schema has already had once.
    CONSTRAINT vials_photo_key_is_under_its_own_prefix CHECK (
        label_photo_path IS NULL
        OR label_photo_path ~ ('^' || patient_id::text || '(/[A-Za-z0-9][A-Za-z0-9._-]{0,100}){1,4}$')
    ),
    -- A vial cannot be thrown away before it was opened, and neither date may
    -- precede the other's meaning. Written as two comparisons rather than one
    -- range, because either date may be null and a range would be NULL — and a
    -- CHECK evaluating to NULL passes, which this schema has already been bitten by
    -- twice: once on array_length returning NULL for an empty array, and once on
    -- cardinality counting a NULL element.
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
