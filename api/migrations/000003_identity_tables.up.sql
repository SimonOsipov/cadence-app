-- The first six tables, and the shape the access model was designed around.
--
-- No policies and no grants here, on purpose. Row level security is enabled and
-- forced on every one of them, and a forced table with no policy forbids
-- everyone — the owner included. That is a safe intermediate state rather than
-- an unfinished one: between this migration and the next, a query against these
-- tables returns nothing to anybody, which is the failure direction to be in.
-- A test asserts it, so the state is measured rather than assumed.
--
-- Closed sets are a CHECK on text rather than an enum type. An added enum value
-- cannot be removed in a down migration — ALTER TYPE ... ADD VALUE has no
-- counterpart — so the first value anybody adds makes that migration one-way.
--
-- Nothing derived is stored. `initials` is absent from profiles although §03
-- lists it: it is a function of full_name, derived by one module on each
-- surface, exactly like the avatar colour. A stored copy is a second source of
-- truth that drifts on the first rename.

SET ROLE cadence_owner;

-- profiles is the anchor: one row per person the clinic knows about, whatever
-- their role.
--
-- user_id carries no foreign key to auth.users, and that is a decision rather
-- than an omission. The identity provider owns that table, its migrations
-- change it, and the policy-test database has no auth schema at all — a
-- reference would make our access model depend on somebody else's schema and
-- make it untestable without them.
CREATE TABLE app.profiles (
    user_id    uuid PRIMARY KEY,
    role       text NOT NULL CHECK (role IN ('patient', 'doctor', 'admin')),
    full_name  text NOT NULL CHECK (pg_catalog.length(full_name) BETWEEN 1 AND 200),
    -- Validated against pg_timezone_names on the service path, in Go: it is a
    -- table rather than a fixed list, and a CHECK cannot consult one.
    timezone   text,
    locale     text NOT NULL DEFAULT 'ru',
    created_at timestamptz NOT NULL DEFAULT pg_catalog.now()
);

-- The clinical card. Separate from profiles because its columns are the ones the
-- patient may not write and the clinic must: the split is what makes the column
-- grants of the next migration expressible at all.
--
-- joined_at is the moment the invitation was accepted; profiles.created_at is
-- the moment the clinic created the row. Two different facts about two different
-- people's actions, not a duplicate.
CREATE TABLE app.patient_profiles (
    user_id          uuid PRIMARY KEY REFERENCES app.profiles (user_id) ON DELETE CASCADE,
    dob              date,
    -- §03 does not define the set, so it is defined here and recorded as a
    -- decision rather than left open.
    sex              text CHECK (sex IN ('male', 'female')),
    height_cm        numeric,
    target_weight_kg numeric,
    joined_at        timestamptz
);

CREATE TABLE app.provider_profiles (
    user_id     uuid PRIMARY KEY REFERENCES app.profiles (user_id) ON DELETE CASCADE,
    title_ru    text,
    -- The anchor for multi-clinic support (§03). One clinic today; a column now
    -- costs nothing and a column later costs a backfill.
    clinic_name text
);

-- The care team: a patient may have several specialists in different roles, and
-- a doctor's access runs through this table and through nothing else. "role =
-- doctor therefore sees everyone" appears nowhere, which is what multi-clinic
-- support will rest on.
CREATE TABLE app.care_team_assignments (
    -- Defaulted so that a caller cannot forget it. gen_random_uuid is built in
    -- since PostgreSQL 13; no extension is required.
    id          uuid PRIMARY KEY DEFAULT pg_catalog.gen_random_uuid(),
    patient_id  uuid NOT NULL REFERENCES app.profiles (user_id) ON DELETE CASCADE,
    provider_id uuid NOT NULL REFERENCES app.profiles (user_id) ON DELETE CASCADE,
    care_role   text NOT NULL CHECK (care_role IN ('endo', 'dietitian', 'nurse')),
    -- NOT NULL because the partial unique index below is WHERE is_primary, and a
    -- NULL is not true: a row with a NULL here would slip past "there is one
    -- primary specialist" without anybody deciding that it should.
    is_primary  boolean NOT NULL DEFAULT false,
    since       date,
    -- Nobody is their own doctor. Cheap to state, and it closes the shape that
    -- would let a patient reach their own row through the doctor policy.
    CONSTRAINT care_team_assignments_distinct_parties CHECK (patient_id <> provider_id),
    CONSTRAINT care_team_assignments_unique_pair UNIQUE (patient_id, provider_id)
);

-- One primary specialist per patient. A partial unique index rather than a
-- constraint, because the uniqueness holds only over the rows that claim it.
CREATE UNIQUE INDEX care_team_assignments_one_primary
    ON app.care_team_assignments (patient_id) WHERE is_primary;

-- Both directions of the subquery every doctor and patient policy runs. They are
-- on the hot path of every query against every table, not just this one: the
-- policies of profiles and provider_profiles look through here too.
CREATE INDEX care_team_assignments_by_provider ON app.care_team_assignments (provider_id);
CREATE INDEX care_team_assignments_by_patient ON app.care_team_assignments (patient_id);

CREATE TABLE app.user_preferences (
    user_id        uuid PRIMARY KEY REFERENCES app.profiles (user_id) ON DELETE CASCADE,
    dose_reminders boolean NOT NULL DEFAULT true,
    lead_time_min  integer NOT NULL DEFAULT 30 CHECK (lead_time_min IN (15, 30, 60)),
    meal_reminders boolean NOT NULL DEFAULT false,
    units          text NOT NULL DEFAULT 'kg' CHECK (units IN ('kg', 'lb')),
    time_fmt       smallint NOT NULL DEFAULT 24 CHECK (time_fmt IN (24, 12)),
    weekly_report  boolean NOT NULL DEFAULT true,
    team_messages  boolean NOT NULL DEFAULT true,
    reorder_alerts boolean NOT NULL DEFAULT true
);

-- The trail of who changed what.
--
-- No foreign keys at all, and that is the point: an audit row outlives the rows
-- it describes and the people it names. A cascade from profiles would delete
-- exactly the evidence of the deletion.
--
-- Attribution is mandatory by constraint rather than by discipline: exactly one
-- of actor_id and actor_job, never both — which would name two actors for one
-- action — and never neither, which is a change nobody signed. An empty
-- actor_job is rejected for the same reason a missing one is.
--
-- GENERATED ALWAYS AS IDENTITY still permits OVERRIDING SYSTEM VALUE, so the
-- service path is able to choose an id and reorder history. Accepted
-- deliberately: the order of history is `at`, not `id`, and the service path is
-- this codebase's own Go rather than an outside actor. Recorded in the spec.
CREATE TABLE app.audit_log (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    at         timestamptz NOT NULL DEFAULT pg_catalog.now(),
    actor_id   uuid,
    actor_job  text,
    action     text NOT NULL,
    entity     text NOT NULL,
    entity_id uuid,
    patient_id uuid,
    meta       jsonb,
    CONSTRAINT audit_log_attribution CHECK (
        pg_catalog.num_nonnulls(actor_id, actor_job) = 1
        -- COALESCE unqualified, like NULLIF in the previous migration: it is
        -- grammar the parser expands rather than a function anybody can shadow,
        -- and there is no pg_catalog.coalesce to name.
        AND COALESCE(actor_job, 'x') <> ''
    )
);

-- Enabled and forced on every table, in one place so that a table added to the
-- list above and forgotten here is visible as a missing line rather than as a
-- missing word inside one.
--
-- FORCE is what extends row level security to the table's owner. Without it the
-- owner is exempt, and the owner is the role every migration runs as.
ALTER TABLE app.profiles              ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.profiles              FORCE  ROW LEVEL SECURITY;
ALTER TABLE app.patient_profiles      ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.patient_profiles      FORCE  ROW LEVEL SECURITY;
ALTER TABLE app.provider_profiles     ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.provider_profiles     FORCE  ROW LEVEL SECURITY;
ALTER TABLE app.care_team_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.care_team_assignments FORCE  ROW LEVEL SECURITY;
ALTER TABLE app.user_preferences      ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.user_preferences      FORCE  ROW LEVEL SECURITY;
ALTER TABLE app.audit_log             ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.audit_log             FORCE  ROW LEVEL SECURITY;

RESET ROLE;
