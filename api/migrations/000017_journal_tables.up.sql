-- The wellbeing diary: one entry per day, and the day is the identity.
--
-- PRIMARY KEY (patient_id, entry_date) rather than a surrogate with a unique index
-- beside it. §03 says «one entry per day, always», and a key that says so is a key
-- an upsert can name: the merge of step 8 is `ON CONFLICT (patient_id, entry_date)`,
-- and with a surrogate id there would be nothing to conflict on.
--
-- The cycle day is not here. It is derived from the protocol's start date, and a
-- stored copy would go stale the first time a course is edited.

SET ROLE cadence_owner;

CREATE TABLE app.journal_entries (
    patient_id uuid NOT NULL REFERENCES app.profiles (user_id) ON DELETE CASCADE,
    entry_date date NOT NULL,
    -- Five-point scales, and nullable: «пропущено» is a real answer and is not the
    -- same as a one. Off the scale the loss would be silent — the client maps an
    -- unknown number to «no answer», so a stored 7 reads back as nothing said.
    mood       smallint CHECK (mood BETWEEN 1 AND 5),
    energy     smallint CHECK (energy BETWEEN 1 AND 5),
    sleep      smallint CHECK (sleep BETWEEN 1 AND 5),
    -- §03's «tags[] (7 fixed)», and the seven are the side effects: the KMP type is a
    -- typealias onto SideEffect rather than a second enum, because two sets that must
    -- never differ are one set. The Go side puts the closed set in this context and
    -- dosing reads it, which is the direction the write already runs in — one patient
    -- action writes a dose event and this row.
    tags       text[] NOT NULL DEFAULT '{}',
    note       text CHECK (note IS NULL OR pg_catalog.length(note) BETWEEN 1 AND 2000),
    -- §03's `source manual|dose`. Set once and never rewritten: an entry born of the
    -- dose wizard says so in the feed, and a later edit by hand does not make that
    -- untrue. The rule lives in the merge, in Go — the database cannot tell a handler
    -- that set it from a caller that did, because both arrive as the same role.
    source     text NOT NULL CHECK (source IN ('manual', 'dose')),
    created_at timestamptz NOT NULL DEFAULT pg_catalog.now(),
    CONSTRAINT journal_entries_one_per_day PRIMARY KEY (patient_id, entry_date),
    -- cardinality and not array_length: array_length of an empty array is NULL, and a
    -- CHECK evaluating to NULL passes. This schema has been bitten by that twice.
    CONSTRAINT journal_entries_tags_are_a_flat_named_list CHECK (
        CASE WHEN pg_catalog.array_ndims(tags) IS NULL OR pg_catalog.array_ndims(tags) = 1
             THEN tags <@ ARRAY['nausea', 'fatigue', 'headache', 'bloating',
                                'insomnia', 'site', 'appetite']::text[]
             ELSE false
        END
    ),
    -- An entry that says nothing would put a day the patient never filled into the
    -- feed and into the heatmap. Written as three total tests rather than a chain of
    -- comparisons, so that no branch of it can evaluate to NULL.
    CONSTRAINT journal_entries_say_something CHECK (
        pg_catalog.num_nonnulls(mood, energy, sleep) > 0
        OR pg_catalog.cardinality(tags) > 0
        OR note IS NOT NULL
    )
);

-- The feed reads a patient's days newest first, and the trend charts read a window
-- of them. The primary key already orders by (patient, date) ascending; this is the
-- same relation read the other way.
CREATE INDEX journal_entries_by_patient_newest_first
    ON app.journal_entries (patient_id, entry_date DESC);

ALTER TABLE app.journal_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.journal_entries FORCE  ROW LEVEL SECURITY;

RESET ROLE;
