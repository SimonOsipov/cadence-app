-- Rolling the base migration back has to remove cluster objects, not just
-- objects of this database, and it has to be safe to run twice: `migrate down`
-- against a database that never had the chain applied is a normal thing to do
-- by accident.
--
-- Order matters. Grants are dependencies of the roles that hold them, so DROP
-- ROLE fails while any of them survive. DROP OWNED BY clears them — but only
-- within the current database, which is why a role that owns objects in a second
-- database of the same cluster still cannot be dropped. In deployment there is
-- one database per project, and in the test harness each database is dropped as
-- its test ends.
--
-- cadence_authenticated is in the list although the chain no longer creates it:
-- rolling back on a cluster where an older version of the chain ran has to leave
-- nothing behind either.

-- The grants come first, before anything is dropped. DROP OWNED BY requires the
-- privileges of the role whose objects are being dropped, and SET ROLE requires
-- the SET option — and the membership a CREATEROLE role receives implicitly when
-- it creates a role carries neither: from PostgreSQL 16 that grant is made with
-- INHERIT FALSE and SET FALSE.
--
-- An explicit grant sets the SET option to true unconditionally, and takes the
-- INHERIT option from the member's own rolinherit. Measured, because the
-- difference decides whether this file works: a bootstrap role created NOINHERIT
-- gets SET but not INHERIT from the grants below, and then DROP OWNED BY fails
-- with 'permission denied to drop objects' and the rollback stops half way. The
-- chain therefore requires an inheriting role to apply and to roll back, which
-- is what a managed cluster hands out and what the harness creates.
--
-- Ordering them ahead of the schema drop matters when the role rolling back is
-- not the role that applied: relying on the membership the up migration left
-- behind works until the day it does not.
DO $$
DECLARE
    role_name text;
BEGIN
    FOREACH role_name IN ARRAY ARRAY[
        'cadence_app', 'cadence_service_app',
        'cadence_patient', 'cadence_doctor', 'cadence_admin', 'cadence_service',
        'cadence_authenticated', 'cadence_owner'
    ] LOOP
        IF EXISTS (SELECT FROM pg_roles WHERE rolname = role_name) THEN
            EXECUTE format('GRANT %I TO CURRENT_USER', role_name);
        END IF;
    END LOOP;
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

-- The memberships are revoked before the roles are dropped. Dropping a role
-- would clear them anyway; doing it explicitly keeps the failure legible when it
-- is this role's own grant that cannot go.
--
-- It is not a detector of foreign grants, and saying so would be false: a
-- REVOKE that matches no row belonging to this grantor raises a WARNING and
-- succeeds, and golang-migrate does not surface warnings. Catching a membership
-- the chain cannot revoke is the up migration's job, where it raises.
DO $$
DECLARE
    membership record;
BEGIN
    FOR membership IN
        SELECT granted.rolname AS granted, member.rolname AS member
        FROM pg_auth_members m
        JOIN pg_roles granted ON granted.oid = m.roleid
        JOIN pg_roles member ON member.oid = m.member
        WHERE granted.rolname IN (
                  'cadence_patient', 'cadence_doctor', 'cadence_admin',
                  'cadence_service', 'cadence_authenticated')
          AND member.rolname IN ('cadence_app', 'cadence_service_app')
    LOOP
        EXECUTE format('REVOKE %I FROM %I', membership.granted, membership.member);
    END LOOP;
END
$$;

DO $$
DECLARE
    role_name text;
BEGIN
    -- The connection roles first: they hold the database-scoped grants, and
    -- those have to go before the role does.
    FOREACH role_name IN ARRAY ARRAY['cadence_app', 'cadence_service_app'] LOOP
        IF EXISTS (SELECT FROM pg_roles WHERE rolname = role_name) THEN
            EXECUTE format('REVOKE CONNECT ON DATABASE %I FROM %I', current_database(), role_name);
            EXECUTE format('DROP OWNED BY %I CASCADE', role_name);
            EXECUTE format('DROP ROLE %I', role_name);
        END IF;
    END LOOP;

    FOREACH role_name IN ARRAY ARRAY[
        'cadence_patient', 'cadence_doctor', 'cadence_admin', 'cadence_service',
        'cadence_authenticated'
    ] LOOP
        IF EXISTS (SELECT FROM pg_roles WHERE rolname = role_name) THEN
            EXECUTE format('DROP OWNED BY %I CASCADE', role_name);
            EXECUTE format('DROP ROLE %I', role_name);
        END IF;
    END LOOP;

    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE format('REVOKE CREATE ON DATABASE %I FROM cadence_owner', current_database());
        -- Also clears any default privileges declared FOR ROLE cadence_owner,
        -- which outlive the schema they were declared in. The chain no longer
        -- declares any; a cluster that ran an older version still carries them.
        EXECUTE 'DROP OWNED BY cadence_owner CASCADE';
        EXECUTE 'DROP ROLE cadence_owner';
    END IF;
END
$$;
