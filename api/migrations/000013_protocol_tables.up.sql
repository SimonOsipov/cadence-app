-- The prescription: what is taken, when, how much, and up to which dose.
--
-- Same arrangement as the identity pair — tables and RLS here, policies and
-- grants in the next migration. A forced table with no policy refuses everyone
-- including its owner, so the state between these two files is safe rather than
-- half-built.
--
-- No materialized schedule table appears here and none ever will: occurrences are
-- generated from these four tables in api/internal/protocol, and only facts that
-- happened are rows. Nothing derived is stored — there is no `remaining`, no
-- adherence and no occurrence status anywhere below.

-- Into the default schema, not into app: the functions registry reconciles
-- everything in app against a declared list, and this extension would arrive
-- there as several dozen undeclared functions. The chain touches public nowhere
-- else.
--
-- Trusted since PostgreSQL 13, so the bootstrap role installs it without being a
-- superuser — which matters because managed Postgres hands nobody one. Asserted
-- against the pinned image rather than taken from the documentation.
CREATE EXTENSION IF NOT EXISTS btree_gist;

SET ROLE cadence_owner;

-- The drug reference. Seeded by code, extended by name.
--
-- `code` is nullable and that is the whole point of it: a compound the migration
-- seeds carries one, so re-running the seed is idempotent and renaming `name_ru`
-- cannot produce a second row; a compound a doctor types in has none. The KMP
-- type drops the field in phase 4 — it is written here and read by the seed, and
-- by nobody else.
--
-- Units are a CHECK on text rather than an enum type, like every closed set in
-- this schema: ALTER TYPE ... ADD VALUE has no counterpart in a down migration.
CREATE TABLE app.compounds (
    id            uuid PRIMARY KEY DEFAULT pg_catalog.gen_random_uuid(),
    code          text UNIQUE,
    name_ru       text NOT NULL CHECK (pg_catalog.length(name_ru) BETWEEN 1 AND 200),
    default_unit  text NOT NULL CHECK (default_unit IN ('мг', 'мкг')),
    route         text NOT NULL CHECK (pg_catalog.length(route) BETWEEN 1 AND 50),
    -- Data, not presentation: which glyph a compound gets is the clinic's choice,
    -- and each surface adapts the name into its own icon set.
    icon          text NOT NULL CHECK (pg_catalog.length(icon) BETWEEN 1 AND 50),
    created_at    timestamptz NOT NULL DEFAULT pg_catalog.now()
);

-- «Повтор по регистру возвращает существующий compoundId, а не создаёт второй» is
-- an acceptance criterion, and this is what makes it enforceable rather than a
-- habit of one function. lower(text) is immutable, so it may carry an index.
CREATE UNIQUE INDEX compounds_one_row_per_name ON app.compounds (pg_catalog.lower(name_ru));

-- The course. start_date is what the cycle week derives from — every date in the
-- schedule is (start_date + n days), which is why the generator takes no clock.
CREATE TABLE app.protocols (
    id             uuid PRIMARY KEY DEFAULT pg_catalog.gen_random_uuid(),
    patient_id     uuid NOT NULL REFERENCES app.profiles (user_id) ON DELETE CASCADE,
    start_date     date NOT NULL,
    duration_weeks integer NOT NULL CHECK (duration_weeks BETWEEN 1 AND 104),
    status         text NOT NULL CHECK (status IN ('active', 'completed', 'cancelled')),
    -- The prescriber outlives their own account: a doctor who leaves the clinic
    -- must not take the courses they wrote with them.
    created_by     uuid REFERENCES app.profiles (user_id) ON DELETE SET NULL,
    notes          text,
    created_at     timestamptz NOT NULL DEFAULT pg_catalog.now()
);

-- One active protocol per patient, recorded as a decision and enforced here.
-- Partial, because the uniqueness holds only over the rows that claim it: a
-- patient may have any number of completed or cancelled courses behind them.
--
-- Lifting this later is a migration, a change to the client's ProtocolPlan (which
-- takes exactly one), and a decision about what «today's dose» means with two
-- courses running. It costs more to remove than to add, which is why it is here.
CREATE UNIQUE INDEX protocols_one_active_per_patient
    ON app.protocols (patient_id) WHERE status = 'active';

CREATE INDEX protocols_by_patient ON app.protocols (patient_id);

-- What is taken and on which days. `times` is an array because §03 makes it one:
-- BPC-157 at 08:00 and at 20:00 are two occurrences, logged apart, which is why
-- an occurrence is keyed (item, date, time) rather than (item, date).
CREATE TABLE app.protocol_items (
    id           uuid PRIMARY KEY DEFAULT pg_catalog.gen_random_uuid(),
    protocol_id  uuid NOT NULL REFERENCES app.protocols (id) ON DELETE CASCADE,
    kind         text NOT NULL CHECK (kind IN ('injection', 'supplement', 'weigh_in')),
    -- Nullable: a weigh-in is not a drug. RESTRICT rather than CASCADE — deleting
    -- a reference row must not silently empty a live prescription.
    compound_id  uuid REFERENCES app.compounds (id) ON DELETE RESTRICT,
    cadence      text NOT NULL CHECK (cadence IN ('weekly', 'daily', 'n_per_week')),
    -- ISO: 1 is Monday, 7 is Sunday, matching kotlinx.datetime's isoDayNumber.
    -- Go's time.Weekday counts Sunday from zero, so the API converts through one
    -- tested helper rather than through an "obvious" cast.
    days_of_week smallint[] NOT NULL DEFAULT '{}',
    times        time[] NOT NULL,
    -- §03 gives this a column of its own. Only injections are logged today, but
    -- an injection the clinic has stopped logging is what the column is for, and
    -- deriving it from `kind` would make that unsayable.
    loggable     boolean NOT NULL DEFAULT true,
    CONSTRAINT protocol_items_days_are_iso
        CHECK (days_of_week <@ ARRAY[1, 2, 3, 4, 5, 6, 7]::smallint[]),
    -- The two fields are read together by the generator, so they are constrained
    -- together here: a daily item that also names weekdays, or a weekly one that
    -- names none, would make `fallsOn` and `dosesPerWeek` disagree about the same
    -- row. Measured as a hazard in review before it was constrained here.
    -- cardinality and not array_length: array_length of an empty array is NULL,
    -- NULL >= 1 is NULL, and a CHECK that evaluates to NULL passes. Measured —
    -- both of these constraints were written with array_length first and accepted
    -- exactly the rows they exist to refuse.
    CONSTRAINT protocol_items_cadence_matches_days CHECK (
        (cadence = 'daily' AND pg_catalog.cardinality(days_of_week) = 0)
        OR (cadence <> 'daily' AND pg_catalog.cardinality(days_of_week) >= 1)
    ),
    CONSTRAINT protocol_items_has_a_slot
        CHECK (pg_catalog.cardinality(times) >= 1)
);

CREATE INDEX protocol_items_by_protocol ON app.protocol_items (protocol_id);

-- The titration band: 0,25 (w1–4) → 0,5 (w5–8) → 1,0 (w9–12). A dose is a number
-- and a unit; «0,25 мг» is a rendering.
CREATE TABLE app.protocol_phases (
    id               uuid PRIMARY KEY DEFAULT pg_catalog.gen_random_uuid(),
    protocol_item_id uuid NOT NULL REFERENCES app.protocol_items (id) ON DELETE CASCADE,
    -- Week 1 is the first seven days of the course. The lower bound is not
    -- decoration: a phase opening at week 0 draws a dose band a week before the
    -- protocol began, while the generator answers «not prescribed» for those
    -- days — two readers of one row disagreeing. Found by review of step 1 and
    -- closed in both places.
    from_week        integer NOT NULL CHECK (from_week >= 1),
    to_week          integer NOT NULL,
    dose_value       numeric NOT NULL CHECK (dose_value > 0),
    dose_unit        text NOT NULL CHECK (dose_unit IN ('мг', 'мкг')),
    CONSTRAINT protocol_phases_runs_forwards CHECK (to_week >= from_week),
    -- Phases of one item never overlap, and the database is what says so — Go
    -- checks the same thing before inserting so that a doctor gets a 422 naming
    -- the row rather than a raw 23P01, and this stays the guard against the race
    -- Go cannot see.
    --
    -- Gaps are legal and deliberately not constrained: four weeks on, four off,
    -- four on is a wash-out, and the client already draws it that way.
    --
    -- Equality on uuid inside a GiST index is what needs btree_gist: the built-in
    -- GiST covers ranges and geometry, not ordinary scalars.
    CONSTRAINT protocol_phases_do_not_overlap EXCLUDE USING gist (
        protocol_item_id WITH =,
        pg_catalog.int4range(from_week, to_week, '[]') WITH &&
    )
);

CREATE INDEX protocol_phases_by_item ON app.protocol_phases (protocol_item_id);

-- Enabled and forced on each, in one block so that a table added above and
-- forgotten here is a missing line rather than a missing word inside one.
ALTER TABLE app.compounds        ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.compounds        FORCE  ROW LEVEL SECURITY;
ALTER TABLE app.protocols        ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.protocols        FORCE  ROW LEVEL SECURITY;
ALTER TABLE app.protocol_items   ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.protocol_items   FORCE  ROW LEVEL SECURITY;
ALTER TABLE app.protocol_phases  ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.protocol_phases  FORCE  ROW LEVEL SECURITY;

RESET ROLE;
