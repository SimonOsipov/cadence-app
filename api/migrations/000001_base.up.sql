-- The first migration owns no data. It establishes who is allowed to do what,
-- because every later migration and every RLS policy is written against that
-- arrangement.
--
-- Three roles, not one:
--
--   cadence_owner          owns the application schema and every object in it.
--                          NOLOGIN: nothing connects as the owner, migrations
--                          reach it with SET ROLE.
--   cadence_app            LOGIN, owns nothing. This is DATABASE_URL — the role
--                          the API connects with on every request.
--   cadence_authenticated  NOLOGIN, the target of SET ROLE inside a request.
--                          Table privileges are granted here, and the RLS
--                          policies of M2 will be written for it.
--
-- A single role that both owns the tables and serves the requests is not a
-- lesser version of this — it is a hole, because the owner of a table may
-- disable row level security on it. Separating ownership from connection is the
-- whole point, and it costs nothing to do now and a migration of every policy
-- to do later.
--
-- Applied by a bootstrap role with CREATEROLE and CREATEDB that is *not* a
-- superuser: that is what `postgres` is on Supabase. Nothing here may require
-- superuser.

-- Roles are objects of the cluster, not of the database. A second database in
-- the same cluster — every test database, for instance — finds them already
-- created, so each CREATE ROLE is guarded rather than assumed.
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        CREATE ROLE cadence_owner
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;
    END IF;

    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_authenticated') THEN
        CREATE ROLE cadence_authenticated
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;
    END IF;

    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_app') THEN
        -- No password here on purpose: the credential is provisioned outside the
        -- repository, in Supabase and in Railway variables. A LOGIN role without
        -- a password cannot authenticate, so forgetting to provision it locks
        -- the API out rather than letting it in.
        --
        -- NOINHERIT is the load-bearing attribute. cadence_app is a member of
        -- cadence_authenticated, but without inheritance that membership is
        -- usable only through an explicit SET ROLE. A query that skips the
        -- impersonation seam therefore holds no table privileges at all and
        -- fails loudly, instead of quietly running with the union of both roles'
        -- rights and no policy in sight.
        CREATE ROLE cadence_app
            LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;
    END IF;
END
$$;

-- Declared, not merely created. The guards above skip a role that already
-- exists, and a cluster where cadence_app was created by hand with INHERIT
-- would take the grant below and quietly turn the impersonation seam into an
-- option: every query would hold the privileges of cadence_authenticated
-- without ever asking for them. The chain states what it depends on every time
-- it runs, so an existing role converges instead of persisting.
--
-- Only LOGIN and INHERIT are set here. SUPERUSER and BYPASSRLS may be changed
-- only by a role that holds them, so a bootstrap role that is neither cannot
-- clear them — the block below therefore verifies them and stops the chain
-- instead of pretending to fix them.
ALTER ROLE cadence_owner NOLOGIN;
ALTER ROLE cadence_authenticated NOLOGIN;
ALTER ROLE cadence_app LOGIN NOINHERIT;

DO $$
DECLARE
    offenders text;
BEGIN
    SELECT string_agg(rolname, ', ' ORDER BY rolname) INTO offenders
    FROM pg_roles
    WHERE rolname IN ('cadence_owner', 'cadence_app', 'cadence_authenticated')
      AND (rolsuper OR rolbypassrls OR rolcreaterole OR rolcreatedb);

    IF offenders IS NOT NULL THEN
        RAISE EXCEPTION
            'roles carry privileges the request path must never hold: %. '
            'Drop them and let the chain create them, or clear the attributes as a superuser.',
            offenders;
    END IF;
END
$$;

-- Withdrawn before it is granted, and this is not ceremony.
--
-- From PostgreSQL 16 inheritance is a property of the membership, not only of
-- the member role, and a membership is keyed by its grantor: the same role can
-- be granted twice by two different roles, and the member inherits if *any* of
-- those grants says so. A GRANT only ever updates the row belonging to the role
-- issuing it, so re-granting cannot undo a permissive grant somebody else made.
-- Revoking first collapses every grant this role has authority over, and the
-- single grant below is then the whole truth.
REVOKE cadence_authenticated FROM cadence_app;

DO $$
DECLARE
    inheriting integer;
BEGIN
    -- Earlier versions have no INHERIT option on a grant and follow the role
    -- attribute, which the ALTER above has already set.
    IF current_setting('server_version_num')::int < 160000 THEN
        EXECUTE 'GRANT cadence_authenticated TO cadence_app';

        RETURN;
    END IF;

    EXECUTE 'GRANT cadence_authenticated TO cadence_app WITH INHERIT FALSE';

    -- The revoke above reaches only the grants this role has authority over. A
    -- membership granted by a superuser survives it, and no amount of granting
    -- undoes someone else's row — so the chain stops rather than reporting a
    -- separation it did not achieve. This is the whole failure mode the spec was
    -- rewritten around: an arrangement that looks right and enforces nothing.
    --
    -- Dynamic SQL because pg_auth_members.inherit_option does not exist before
    -- PostgreSQL 16, and a static reference would fail to parse there even
    -- inside a branch that never runs.
    EXECUTE $q$
        SELECT count(*)
        FROM pg_auth_members m
        JOIN pg_roles granted ON granted.oid = m.roleid
        JOIN pg_roles member ON member.oid = m.member
        WHERE granted.rolname = 'cadence_authenticated'
          AND member.rolname = 'cadence_app'
          AND m.inherit_option
    $q$ INTO inheriting;

    IF inheriting > 0 THEN
        RAISE EXCEPTION
            'cadence_app inherits cadence_authenticated through a grant this role cannot revoke, '
            'so the impersonation seam would be optional. Revoke it as the grantor or as a superuser: '
            'REVOKE cadence_authenticated FROM cadence_app;';
    END IF;
END
$$;

DO $$
BEGIN
    -- The bootstrap role has to be able to become the owner in order to create
    -- objects owned by it.
    --
    -- Granted unconditionally rather than behind a membership check, and the
    -- reason is a trap: from PostgreSQL 16 a CREATEROLE role is granted the
    -- roles it creates with SET FALSE, so pg_has_role reports MEMBER while
    -- SET ROLE is still refused. A check for membership would skip the grant
    -- that fixes exactly that. The statement is idempotent.
    EXECUTE 'GRANT cadence_owner TO CURRENT_USER';

    -- CONNECT and CREATE are database-scoped, and the database name is not
    -- known to a migration — it is cadence-dev in one environment and a
    -- throwaway name in the test harness.
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO cadence_app', current_database());
    EXECUTE format('GRANT CREATE ON DATABASE %I TO cadence_owner', current_database());
END
$$;

-- Everything below is created by the owner, not by whoever is applying the
-- chain. Every later migration must open the same way, or its tables end up
-- owned by the bootstrap role and the separation above becomes decorative.
-- This is not left to discipline: the integration suite sweeps pg_class and
-- fails on any table in the schema that the owner does not own.
SET ROLE cadence_owner;

CREATE SCHEMA IF NOT EXISTS app;

RESET ROLE;

-- Entering the schema is allowed; creating in it is not. Schema changes arrive
-- through this chain and through nothing else — not through the Supabase
-- dashboard, and not through the API.
GRANT USAGE ON SCHEMA app TO cadence_app, cadence_authenticated;

-- Default privileges attach to the role that creates the object, and that role
-- has to be named. Without FOR ROLE they bind to the role applying the chain —
-- which is the bootstrap role, not the owner — and the failure is silent: the
-- tables of the next migration simply are not readable by the request path, and
-- the first symptom is a permission error in a feature nobody connected to this
-- line.
--
-- Privileges go to cadence_authenticated rather than to cadence_app: the
-- request path holds them only while it is impersonating, which is exactly when
-- the RLS policies of M2 apply.
ALTER DEFAULT PRIVILEGES FOR ROLE cadence_owner IN SCHEMA app
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO cadence_authenticated;

ALTER DEFAULT PRIVILEGES FOR ROLE cadence_owner IN SCHEMA app
    GRANT USAGE, SELECT ON SEQUENCES TO cadence_authenticated;
