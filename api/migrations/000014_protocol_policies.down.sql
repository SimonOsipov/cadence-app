-- Policies and grants go together: a table left with grants and no policies is
-- reachable by nobody, and a table left with policies and no grants is the same.
--
-- Scoped to this migration's four tables rather than to the schema. The identity
-- pair sweeps every policy in `app` because at that point in the chain there were
-- no others; doing the same here would make rolling back this one migration
-- destroy the identity policies too, and the up migration below it would not put
-- them back.
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

DO $$
DECLARE
    target text;
    policy record;
BEGIN
    IF NOT EXISTS (SELECT FROM pg_namespace WHERE nspname = 'app') THEN
        RETURN;
    END IF;

    FOREACH target IN ARRAY ARRAY['compounds', 'protocols', 'protocol_items', 'protocol_phases']
    LOOP
        IF NOT EXISTS (
            SELECT FROM pg_class c
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'app' AND c.relname = target
        ) THEN
            CONTINUE;
        END IF;

        FOR policy IN
            SELECT policyname FROM pg_policies WHERE schemaname = 'app' AND tablename = target
        LOOP
            EXECUTE format('DROP POLICY %I ON app.%I', policy.policyname, target);
        END LOOP;

        -- cadence_owner is deliberately absent, for the reason the identity pair
        -- records: REVOKE ALL would materialise the owner's implicit privileges
        -- into an ACL and then erase it, leaving it unable to write to a table it
        -- owns, in a state no up migration restores.
        EXECUTE format(
            'REVOKE ALL ON app.%I FROM cadence_patient, cadence_doctor, cadence_admin, '
            'cadence_service',
            target);
    END LOOP;
END
$$;

RESET ROLE;
