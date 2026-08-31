-- measurements — §03's «biomarkers, body, weight — one table for all of it».
--
-- The unit is a function of the metric, so it is constrained as a pair rather than as
-- a closed set of its own: 'weight' with 'cm' satisfies two single-column sets and
-- means nothing. The pairs are the wire units — the Russian «кг» is a render concern.
--
-- No range on `value`: the plausible one differs per metric, so a CHECK here would be
-- a second definition of what `measurements.Bounds` owns.
-- Finiteness is a different question and is answered below.

SET ROLE cadence_owner;

CREATE TABLE app.measurements (
    id          uuid PRIMARY KEY DEFAULT pg_catalog.gen_random_uuid(),
    patient_id  uuid NOT NULL REFERENCES app.profiles (user_id) ON DELETE CASCADE,
    metric      text NOT NULL CHECK (metric IN (
        'weight', 'hrv', 'rhr', 'sleep', 'bodyfat', 'waist', 'hip', 'chest')),
    -- numeric carries NaN and both infinities, and none of them is a reading.
    value       numeric NOT NULL CONSTRAINT measurements_value_is_finite
                CHECK (value > '-Infinity'::numeric AND value < 'Infinity'::numeric),
    unit        text NOT NULL,
    measured_at timestamptz NOT NULL,
    -- §03's `source manual|healthkit|health_connect`, defaulted so that the patient
    -- needs no grant on the column: withheld, a hand-written row cannot claim to have
    -- come off a watch.
    source      text NOT NULL DEFAULT 'manual'
                CHECK (source IN ('manual', 'healthkit', 'health_connect')),
    -- The health platform's own id for the sample, and the whole of the import's
    -- idempotency. There is no importer yet and the patient holds no grant on it.
    external_id text,
    note        text CHECK (
        note IS NULL
        OR (pg_catalog.length(note) BETWEEN 1 AND 2000 AND note ~ '[^[:space:]]')
    ),
    created_at  timestamptz NOT NULL DEFAULT pg_catalog.now(),
    CONSTRAINT measurements_unit_belongs_to_its_metric CHECK (
        (metric, unit) IN (
            ('weight', 'kg'), ('hrv', 'ms'), ('rhr', 'bpm'), ('sleep', '/100'),
            ('bodyfat', '%'), ('waist', 'cm'), ('hip', 'cm'), ('chest', 'cm'))
    )
);

-- Every read of this table is one patient's one metric over a window, newest first.
CREATE INDEX measurements_by_patient_and_metric
    ON app.measurements (patient_id, metric, measured_at DESC);

-- §03's UNIQUE(patient, metric, external_id), with the patient promoted to lead it for
-- the reason 000019 records on the dose stream's slot key: uniqueness is checked before
-- row security, so a key space shared across patients answers by error code whether
-- somebody else holds a sample. There is no importer yet, and no patient grant reaches
-- external_id — the service and admin roles are granted the whole row — so today this
-- reserves the shape rather than refusing a row.
CREATE UNIQUE INDEX measurements_one_per_external_sample
    ON app.measurements (patient_id, metric, external_id)
    WHERE external_id IS NOT NULL;

ALTER TABLE app.measurements ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.measurements FORCE  ROW LEVEL SECURITY;

RESET ROLE;
