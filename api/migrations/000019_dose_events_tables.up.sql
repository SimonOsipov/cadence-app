-- dose_events — «the core clinical fact stream» (§03). The one table everything
-- else is measured against: adherence, a vial's remaining count, the missed-dose
-- sweep, and the day's «с дозой» mark all read it and none of them stores a copy.
--
-- Three references and each of them belongs to somebody. A dose event that pointed
-- at another patient's course or another patient's vial would be a row legitimately
-- its owner's, accepted by every policy, and wrong — the shape step 2 met as
-- reparenting and closed with column grants. Here it is closed in the schema
-- instead, by chaining composite foreign keys down from the patient:
--
--     (patient_id, protocol_id)      → protocols (patient_id, id)
--     (protocol_id, protocol_item_id) → protocol_items (protocol_id, id)
--     (patient_id, vial_id)          → vials (patient_id, id)
--
-- which is why protocol_id is a column here at all. It is derivable from the item,
-- so it looks like the stored-derived state this project forbids — it is not: it is
-- half a key, and it is the half that makes the chain expressible. A CHECK cannot
-- carry a subquery and a policy guards only the request path, whereas a foreign key
-- binds the owner, the admin, the service path and the superuser alike.

SET ROLE cadence_owner;

-- The three parents need the composite the chain references. Each is implied by an
-- existing key (id is already unique), so these add a witness rather than a rule.
ALTER TABLE app.protocols
    ADD CONSTRAINT protocols_belong_to_their_patient UNIQUE (patient_id, id);
ALTER TABLE app.protocol_items
    ADD CONSTRAINT protocol_items_belong_to_their_protocol UNIQUE (protocol_id, id);
ALTER TABLE app.vials
    ADD CONSTRAINT vials_belong_to_their_patient UNIQUE (patient_id, id);

CREATE TABLE app.dose_events (
    id                 uuid PRIMARY KEY DEFAULT pg_catalog.gen_random_uuid(),
    patient_id         uuid NOT NULL REFERENCES app.profiles (user_id) ON DELETE CASCADE,
    protocol_id        uuid NOT NULL,
    protocol_item_id   uuid NOT NULL,
    -- Optional: §03 marks it `vial_id?`, and the wizard's picker may be skipped.
    -- Its absence costs the vial's count one dose, which is a truth about what is
    -- known rather than an error.
    vial_id            uuid,
    -- The compound the dose is attributed to, snapshot at write. Step 2 left this
    -- decision here, and the argument is its own: §03 gives this table `dose_value`
    -- and `dose_unit` so the number survives an edit of the course — attribution is
    -- the heavier clinical fact, and it was the one still riding on a mutable
    -- protocol_items.compound_id. Nullable because an item need not name one.
    compound_id        uuid REFERENCES app.compounds (id) ON DELETE RESTRICT,
    -- The occurrence this event answers. There is no schedule table, so this is the
    -- whole of the match: a generated occurrence and a logged event meet on
    -- (item, date, slot). The time is absent for an item with no named slot.
    scheduled_for_date date NOT NULL,
    scheduled_for_time time,
    -- When the injection happened, which is not when the row was written: a dose
    -- logged from the retry queue lands hours after the fact.
    injected_at        timestamptz NOT NULL,
    dose_value         numeric NOT NULL CHECK (dose_value > 0),
    dose_unit          text NOT NULL CHECK (dose_unit IN ('мг', 'мкг')),
    -- Ten zones, and §03 names three of them before writing «…». The rest are the
    -- body map the frozen prototype draws; nullable because the wizard's zone step
    -- may be skipped like the rest of it.
    site_code          text CHECK (site_code IN (
        'l-abdomen', 'r-abdomen', 'l-delt', 'r-delt', 'l-glute',
        'r-glute', 'l-thigh', 'r-thigh', 'l-lback', 'r-lback')),
    mood               smallint CHECK (mood BETWEEN 1 AND 5),
    -- The same seven as journal_entries.tags, which is one set and not two: one
    -- patient action writes both rows. Written a third time here because a CHECK
    -- cannot reference another table's, and reconciled with the other two by test.
    side_effects       text[] NOT NULL DEFAULT '{}',
    note               text CHECK (
        note IS NULL
        OR (pg_catalog.length(note) BETWEEN 1 AND 2000 AND note ~ '[^[:space:]]')
    ),
    -- Shaped, not prefixed: `<patient>/..` starts with the right characters and
    -- resolves under path.Join to another patient's object. Measured on vials in
    -- step 3; the same key layout and the same guard.
    photo_path         text,
    -- The client's own key, and the whole of the idempotency: a retry from the
    -- offline queue carries the key it generated when the patient tapped save, so
    -- the repeat finds the row rather than writing a second dose.
    client_request_id  text NOT NULL CHECK (
        pg_catalog.length(client_request_id) BETWEEN 8 AND 128
        AND client_request_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]*$'
    ),
    created_at         timestamptz NOT NULL DEFAULT pg_catalog.now(),
    CONSTRAINT dose_events_one_per_client_key UNIQUE (patient_id, client_request_id),
    -- Named, and at table level because it reads a second column: an inline CHECK
    -- that does so becomes a table constraint anyway, under the name PostgreSQL
    -- invents — `dose_events_check`, which the next such constraint would collide
    -- with and which tells a reader nothing about what refused their row.
    CONSTRAINT dose_events_photo_key_is_under_its_own_prefix CHECK (
        photo_path IS NULL
        OR photo_path ~ ('^' || patient_id::text || '(/[A-Za-z0-9][A-Za-z0-9._-]{0,100}){1,4}$')
    ),
    -- cardinality and not array_length: array_length of an empty array is NULL, and
    -- a CHECK evaluating to NULL passes. This schema has been bitten by that twice.
    CONSTRAINT dose_events_side_effects_are_a_flat_named_list CHECK (
        CASE WHEN pg_catalog.array_ndims(side_effects) IS NULL
                  OR pg_catalog.array_ndims(side_effects) = 1
             THEN side_effects <@ ARRAY['nausea', 'fatigue', 'headache', 'bloating',
                                        'insomnia', 'site', 'appetite']::text[]
             ELSE false
        END
    ),
    CONSTRAINT dose_events_belong_to_their_course
        FOREIGN KEY (patient_id, protocol_id)
        REFERENCES app.protocols (patient_id, id) ON DELETE CASCADE,
    CONSTRAINT dose_events_answer_an_item_of_that_course
        FOREIGN KEY (protocol_id, protocol_item_id)
        REFERENCES app.protocol_items (protocol_id, id) ON DELETE CASCADE,
    -- MATCH SIMPLE, which is the default and is wanted: with vial_id NULL the pair
    -- is not checked, and that is «no vial named» rather than «any vial will do».
    CONSTRAINT dose_events_drawn_from_their_own_vial
        FOREIGN KEY (patient_id, vial_id)
        REFERENCES app.vials (patient_id, id) ON DELETE RESTRICT
);

-- The race the idempotency key cannot see: two different keys aiming at one slot,
-- which is what a patient tapping save on two devices produces.
--
-- The WHERE clause buys index size and not behaviour, and it is worth saying so
-- because it reads like the thing that lets a slotless day carry two doses. It is
-- not: NULLs are distinct in a unique index, so a total index would admit them too
-- (measured on postgres:16-alpine — two NULL slots accepted, two named ones
-- refused). What the clause does is keep rows the index can never constrain out of
-- the b-tree.
CREATE UNIQUE INDEX dose_events_one_per_slot
    ON app.dose_events (protocol_item_id, scheduled_for_date, scheduled_for_time)
    WHERE scheduled_for_time IS NOT NULL;

-- Today and the schedule read a patient's window of days; the missed-dose sweep
-- reads the same relation. Newest-first is the primary key's business elsewhere —
-- here the key is a surrogate, so this index is the access path and not a duplicate.
CREATE INDEX dose_events_by_patient_and_day
    ON app.dose_events (patient_id, scheduled_for_date DESC);

-- remaining = total_doses − count(dose_events WHERE vial_id = …), which is a count
-- per vial and the one read that does not start from the patient.
CREATE INDEX dose_events_by_vial ON app.dose_events (vial_id) WHERE vial_id IS NOT NULL;

ALTER TABLE app.dose_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.dose_events FORCE  ROW LEVEL SECURITY;

RESET ROLE;
