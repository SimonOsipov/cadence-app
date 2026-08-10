-- Rolling the base migration back has to remove cluster objects, not just
-- objects of this database, and it has to be safe to run twice: `migrate down`
-- against a database that never had the chain applied is a normal thing to do
-- by accident.
--
-- Order matters. Default privileges and grants are dependencies of the roles
-- that hold them, so DROP ROLE fails while any of them survive. DROP OWNED BY
-- clears both — but only within the current database, which is why a role that
-- owns objects in a second database of the same cluster still cannot be
-- dropped. In deployment there is one database per project, and in the test
-- harness each database is dropped as its test ends.

-- The grants come first, before anything is dropped. DROP OWNED BY requires the
-- privileges of the role whose objects are being dropped, and SET ROLE requires
-- the SET option — and the membership a CREATEROLE role receives implicitly
-- when it creates a role carries neither: from PostgreSQL 16 that grant is made
-- with INHERIT FALSE and SET FALSE. An explicit grant defaults both to the
-- member's own attributes.
--
-- Ordering them ahead of the schema drop matters when the role rolling back is
-- not the role that applied: relying on the membership the up migration left
-- behind works until the day it does not.
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_app') THEN
        EXECUTE 'GRANT cadence_app TO CURRENT_USER';
    END IF;

    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_authenticated') THEN
        EXECUTE 'GRANT cadence_authenticated TO CURRENT_USER';
    END IF;

    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'GRANT cadence_owner TO CURRENT_USER';
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
        DROP SCHEMA IF EXISTS app CASCADE;
        EXECUTE 'RESET ROLE';
    ELSE
        DROP SCHEMA IF EXISTS app CASCADE;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_app')
        AND EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_authenticated')
    THEN
        REVOKE cadence_authenticated FROM cadence_app;
    END IF;

    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_app') THEN
        EXECUTE format('REVOKE CONNECT ON DATABASE %I FROM cadence_app', current_database());
        DROP OWNED BY cadence_app CASCADE;
        DROP ROLE cadence_app;
    END IF;

    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_authenticated') THEN
        DROP OWNED BY cadence_authenticated CASCADE;
        DROP ROLE cadence_authenticated;
    END IF;

    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE format('REVOKE CREATE ON DATABASE %I FROM cadence_owner', current_database());
        -- Also clears the default privileges declared FOR ROLE cadence_owner,
        -- which outlive the schema they were declared in.
        DROP OWNED BY cadence_owner CASCADE;
        DROP ROLE cadence_owner;
    END IF;
END
$$;
