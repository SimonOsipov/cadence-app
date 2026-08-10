-- The first migration owns no data. It establishes who is allowed to do what,
-- because every later migration and every RLS policy is written against that
-- arrangement.
--
-- Seven roles, and the shape of them is the access model:
--
--   cadence_owner        owns the application schema and every object in it.
--                        NOLOGIN: nothing connects as the owner, migrations
--                        reach it with SET ROLE.
--   cadence_app          LOGIN, owns nothing. This is DATABASE_URL — the role
--                        the API connects with on every request.
--   cadence_patient      NOLOGIN, the targets of the impersonation seam inside
--   cadence_doctor       a request — SET LOCAL ROLE, spelled as an assignment to
--   cadence_admin        the role parameter so it can take a bound argument.
--                        Table privileges are granted to them, and the RLS
--                        policies will be written for them.
--   cadence_service_app  LOGIN, owns nothing. This is DATABASE_SERVICE_URL —
--                        a second pool, for the paths that write rows no policy
--                        lets a request write.
--   cadence_service      NOLOGIN, the target of SET LOCAL ROLE on that path.
--
-- The product role is a Postgres role rather than a value a policy compares
-- against, and that is the decision the arrangement exists for. Privileges in
-- Postgres are bound to a role: while the product role was only a claim, every
-- product role was the same Postgres role, so a grant the admin needed reached
-- the patient too, and the escalation could be closed neither by a policy nor by
-- a column grant. It also means overwriting request.jwt.claims no longer yields
-- an admin — the residual property is subject substitution within one's own
-- role. The seam asserts that directly, in
-- TestWithCallerKeepsTheRoleWhenTheClaimsAreOverwritten; the same property
-- against the policies arrives with the regression suite.
--
-- A single role that both owns the tables and serves the requests is not a
-- lesser version of this — it is a hole, because the owner of a table may
-- disable row level security on it. Separating ownership from connection is the
-- whole point, and it costs nothing to do now and a migration of every policy
-- to do later.
--
-- Applied by a bootstrap role with CREATEROLE and CREATEDB that is *not* a
-- superuser: that is what managed Postgres hands out. Nothing here may require
-- superuser — and one consequence is load-bearing: a CREATEROLE role cannot
-- grant BYPASSRLS, so no role escapes a policy, the service path included. That
-- assumption is probed by a test rather than quoted.

-- Roles are objects of the cluster, not of the database. A second database in
-- the same cluster — every test database, for instance — finds them already
-- created, so each CREATE ROLE is guarded rather than assumed.
--
-- No password on either LOGIN role, on purpose: the credential is provisioned
-- outside the repository, in the deployment's variables. Under password
-- authentication — scram-sha-256, which is what the deployment uses — a LOGIN
-- role without one cannot authenticate, so forgetting to provision it locks the
-- API out rather than letting it in. On a cluster whose pg_hba trusts local
-- connections, as the development image does, that is no barrier at all and is
-- not relied on as one.
--
-- NOINHERIT on the connection roles is the load-bearing attribute. Each is a
-- member of its impersonation targets, but without inheritance that membership
-- is usable only through an explicit SET ROLE. A query that skips the
-- impersonation seam therefore holds no table privileges at all and fails
-- loudly, instead of quietly running with the union of both roles' rights and
-- no policy in sight.
DO $$
DECLARE
    role_name text;
BEGIN
    FOREACH role_name IN ARRAY ARRAY[
        'cadence_owner', 'cadence_patient', 'cadence_doctor', 'cadence_admin', 'cadence_service'
    ] LOOP
        IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = role_name) THEN
            EXECUTE format(
                'CREATE ROLE %I NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS', role_name);
        END IF;
    END LOOP;

    FOREACH role_name IN ARRAY ARRAY['cadence_app', 'cadence_service_app'] LOOP
        IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = role_name) THEN
            EXECUTE format(
                'CREATE ROLE %I LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS',
                role_name);
        END IF;
    END LOOP;
END
$$;

-- Declared, not merely created. The guards above skip a role that already
-- exists, and a cluster where cadence_app was created by hand with INHERIT
-- would take the grants below and quietly turn the impersonation seam into an
-- option: every query would hold the privileges of all three product roles
-- without ever asking for them. The chain states what it depends on every time
-- it runs, so an existing role converges instead of persisting.
--
-- Only LOGIN and INHERIT are set here. SUPERUSER and BYPASSRLS may be changed
-- only by a role that holds them, so a bootstrap role that is neither cannot
-- clear them — the block therefore verifies them and stops the chain instead of
-- pretending to fix them.
DO $$
DECLARE
    role_name text;
    offenders text;
BEGIN
    FOREACH role_name IN ARRAY ARRAY[
        'cadence_owner', 'cadence_patient', 'cadence_doctor', 'cadence_admin', 'cadence_service'
    ] LOOP
        EXECUTE format('ALTER ROLE %I NOLOGIN', role_name);
    END LOOP;

    FOREACH role_name IN ARRAY ARRAY['cadence_app', 'cadence_service_app'] LOOP
        EXECUTE format('ALTER ROLE %I LOGIN NOINHERIT', role_name);
    END LOOP;

    SELECT string_agg(rolname, ', ' ORDER BY rolname) INTO offenders
    FROM pg_roles
    WHERE rolname IN (
        'cadence_owner', 'cadence_app', 'cadence_service_app',
        'cadence_patient', 'cadence_doctor', 'cadence_admin', 'cadence_service'
    )
      AND (rolsuper OR rolbypassrls OR rolreplication OR rolcreaterole OR rolcreatedb);

    IF offenders IS NOT NULL THEN
        RAISE EXCEPTION
            'roles carry privileges the request path must never hold: %. '
            'REPLICATION belongs on that list beside SUPERUSER and BYPASSRLS: a role that streams '
            'WAL reads every row of every table without a policy ever running. '
            'Drop them and let the chain create them, or clear the attributes as a superuser.',
            offenders;
    END IF;
END
$$;

-- The bootstrap role has to be able to become the owner in order to create
-- objects owned by it.
--
-- Granted unconditionally rather than behind a membership check, and the reason
-- is a trap: from PostgreSQL 16 a CREATEROLE role is granted the roles it
-- creates with SET FALSE, so pg_has_role reports MEMBER while SET ROLE is still
-- refused. A check for membership would skip the grant that fixes exactly that.
-- The statement is idempotent.
--
-- CONNECT and CREATE are database-scoped, and the database name is not known to
-- a migration — it is cadence-dev in one environment and a throwaway name in
-- the test harness.
DO $$
BEGIN
    EXECUTE 'GRANT cadence_owner TO CURRENT_USER';

    EXECUTE format('GRANT CONNECT ON DATABASE %I TO cadence_app', current_database());
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO cadence_service_app', current_database());
    EXECUTE format('GRANT CREATE ON DATABASE %I TO cadence_owner', current_database());
END
$$;

-- cadence_authenticated is abolished. An intermediate group role standing
-- between the connection and the tables would bring back exactly the
-- indistinguishability of the product roles that the arrangement above exists
-- to remove.
--
-- Removed rather than left alone, because leaving it is not neutral: on a
-- cluster where an earlier version of this chain ran it still holds USAGE on the
-- schema, still carries default privileges pointing at it, and cadence_app is
-- still a member — so SET ROLE cadence_authenticated would keep working and keep
-- bypassing the per-role grants. If it cannot be dropped, because it owns
-- something in another database of the cluster, Postgres raises here and the
-- chain stops. That is the honest outcome: a surviving bypass is worse than a
-- refused migration.
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_authenticated') THEN
        RETURN;
    END IF;

    EXECUTE 'GRANT cadence_authenticated TO CURRENT_USER';
    EXECUTE 'REVOKE cadence_authenticated FROM cadence_app';

    IF EXISTS (SELECT FROM pg_namespace WHERE nspname = 'app') THEN
        EXECUTE 'ALTER DEFAULT PRIVILEGES FOR ROLE cadence_owner IN SCHEMA app REVOKE ALL ON TABLES FROM cadence_authenticated';
        EXECUTE 'ALTER DEFAULT PRIVILEGES FOR ROLE cadence_owner IN SCHEMA app REVOKE ALL ON SEQUENCES FROM cadence_authenticated';
    END IF;

    EXECUTE 'DROP OWNED BY cadence_authenticated CASCADE';
    EXECUTE 'DROP ROLE cadence_authenticated';
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
-- grants below are then the whole truth.
--
-- cadence_service is revoked from cadence_app and never granted to it. The two
-- paths are separated by session_user rather than by a statement: the service
-- path writes rows the request path's policies forbid, so a request-path
-- connection able to assume cadence_service would make the separation
-- decorative.
REVOKE cadence_patient FROM cadence_app;
REVOKE cadence_doctor FROM cadence_app;
REVOKE cadence_admin FROM cadence_app;
REVOKE cadence_service FROM cadence_app;
REVOKE cadence_service FROM cadence_service_app;

DO $$
DECLARE
    unexpected text;
    inheriting text;
BEGIN
    -- Earlier versions have no INHERIT option on a grant and follow the role
    -- attribute, which the ALTER above has already set. The pre-16 branch is
    -- written as plain statements; the 16+ branch has to be dynamic, because
    -- WITH INHERIT FALSE does not parse at all on 15.
    IF current_setting('server_version_num')::int < 160000 THEN
        GRANT cadence_patient TO cadence_app;
        GRANT cadence_doctor TO cadence_app;
        GRANT cadence_admin TO cadence_app;
        GRANT cadence_service TO cadence_service_app;
    ELSE
        EXECUTE 'GRANT cadence_patient TO cadence_app WITH INHERIT FALSE';
        EXECUTE 'GRANT cadence_doctor TO cadence_app WITH INHERIT FALSE';
        EXECUTE 'GRANT cadence_admin TO cadence_app WITH INHERIT FALSE';
        EXECUTE 'GRANT cadence_service TO cadence_service_app WITH INHERIT FALSE';
    END IF;

    -- The membership graph is small enough to state exhaustively, and stating it
    -- exhaustively is the point: a surplus edge is how a role acquires somebody
    -- else's grants without anybody deciding that it should.
    --
    -- Both directions, and neither is optional.
    --
    -- An edge *into* one of these roles is somebody else's privileges arriving
    -- at ours: GRANT pg_execute_server_program TO cadence_app is programs
    -- running on the database host, reached from the request path without the
    -- seam ever being called, and no cadence_* name appears in it anywhere —
    -- which is why the granted side cannot be filtered.
    --
    -- An edge *out of* one is our privileges arriving at somebody else: GRANT
    -- cadence_patient TO analyst lets a stranger SET ROLE into the product role
    -- and hold every grant the patient holds.
    --
    -- Two carve-outs on the outward direction, both of them the chain's own
    -- footprint rather than anybody's decision: the admin_option memberships a
    -- CREATEROLE role receives over the roles it creates, and the membership in
    -- cadence_owner this file grants to CURRENT_USER a few blocks up so that
    -- objects can be created under the owner. Refusing either would mean
    -- refusing the chain that just ran.
    SELECT string_agg(
               format('%s -> %s', granted.rolname, member.rolname),
               ', ' ORDER BY granted.rolname, member.rolname)
      INTO unexpected
      FROM pg_auth_members m
      JOIN pg_roles granted ON granted.oid = m.roleid
      JOIN pg_roles member ON member.oid = m.member
     WHERE (
               member.rolname IN (
                   'cadence_owner', 'cadence_app', 'cadence_service_app',
                   'cadence_patient', 'cadence_doctor', 'cadence_admin', 'cadence_service')
               OR (granted.rolname IN (
                       'cadence_owner', 'cadence_app', 'cadence_service_app',
                       'cadence_patient', 'cadence_doctor', 'cadence_admin', 'cadence_service')
                   AND NOT m.admin_option
                   AND member.rolname <> current_user)
           )
       AND (granted.rolname, member.rolname) NOT IN (
               ('cadence_patient', 'cadence_app'),
               ('cadence_doctor', 'cadence_app'),
               ('cadence_admin', 'cadence_app'),
               ('cadence_service', 'cadence_service_app'));

    IF unexpected IS NOT NULL THEN
        RAISE EXCEPTION
            'the cluster carries memberships this chain did not grant: %. '
            'Each hands one of these roles privileges nobody wrote a grant for, reachable from a '
            'connection role through SET ROLE; revoke them as the grantor or as a superuser.',
            unexpected;
    END IF;

    IF current_setting('server_version_num')::int < 160000 THEN
        RETURN;
    END IF;

    -- The revokes above reach only the grants this role has authority over. A
    -- membership granted by a superuser survives them, and no amount of granting
    -- undoes someone else's row — so the chain stops rather than reporting a
    -- separation it did not achieve. This is the failure mode the spec was
    -- rewritten around: an arrangement that looks right and enforces nothing.
    --
    -- Dynamic SQL is not strictly required for the column reference: plpgsql
    -- resolves names lazily, through SPI at first execution, so a never-taken
    -- branch mentioning inherit_option compiles fine on 15. It keeps the
    -- early-return contract local instead — the query cannot be executed on a
    -- version where the column is absent, whatever a later edit does to the
    -- guard above. Contrast the GRANT earlier in this block, where WITH INHERIT
    -- FALSE is a grammar error and is caught on 15 even in a dead branch.
    EXECUTE $q$
        SELECT string_agg(
                   format('%s -> %s', granted.rolname, member.rolname),
                   ', ' ORDER BY granted.rolname, member.rolname)
        FROM pg_auth_members m
        JOIN pg_roles granted ON granted.oid = m.roleid
        JOIN pg_roles member ON member.oid = m.member
        WHERE (granted.rolname, member.rolname) IN (
                  ('cadence_patient', 'cadence_app'),
                  ('cadence_doctor', 'cadence_app'),
                  ('cadence_admin', 'cadence_app'),
                  ('cadence_service', 'cadence_service_app'))
          AND m.inherit_option
    $q$ INTO inheriting;

    IF inheriting IS NOT NULL THEN
        RAISE EXCEPTION
            'these memberships inherit through a grant this role cannot revoke: %, '
            'so the impersonation seam would be optional. Revoke them as the grantor or as a '
            'superuser.',
            inheriting;
    END IF;
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

-- IF NOT EXISTS skips, and a skip is not convergence. A cluster where the schema
-- was created by hand under another owner takes every statement below without
-- complaint, and the result is the hole the preamble names: the owner of a table
-- may disable row level security on it, so an app schema owned by cadence_app is
-- an access model the request path can switch off. The only signal Postgres
-- gives is a NOTICE, which golang-migrate discards.
--
-- The membership is granted first because ALTER SCHEMA ... OWNER TO needs the
-- privileges of the current owner, and the implicit membership a CREATEROLE role
-- holds over the roles it created carries neither INHERIT nor SET. If the
-- current owner is a role this one cannot grant itself, the GRANT raises and the
-- chain stops — which is right: a schema owned by a stranger is not a state to
-- proceed from.
DO $$
DECLARE
    schema_owner text;
    strays text;
BEGIN
    SELECT pg_get_userbyid(nspowner) INTO schema_owner
    FROM pg_namespace WHERE nspname = 'app';

    IF schema_owner <> 'cadence_owner' THEN
        -- The grant is only needed when the applying role does not already hold
        -- the owner's privileges — it does when it created the schema itself,
        -- and it does when somebody has already made it a member.
        IF NOT pg_has_role(current_user, schema_owner, 'USAGE') THEN
            BEGIN
                EXECUTE format('GRANT %I TO CURRENT_USER', schema_owner);
            EXCEPTION WHEN insufficient_privilege THEN
                RAISE EXCEPTION
                    'schema app is owned by %, and this role cannot take ownership from it. '
                    'The schema was created outside this chain; a table created in it would '
                    'belong to a role that can disable row level security on it. '
                    'Run ALTER SCHEMA app OWNER TO cadence_owner as a superuser, then apply again '
                    '(the chain will be marked dirty and needs `migrate force`).',
                    schema_owner;
            END;
        END IF;

        EXECUTE 'ALTER SCHEMA app OWNER TO cadence_owner';
    END IF;

    -- Ownership of the schema says nothing about what is already inside it. A
    -- table left by whoever created the schema keeps its own owner, keeps row
    -- level security off, and is invisible to every check above — while the
    -- pg_class sweep that would catch it lives in the test suite and never runs
    -- against a deployed cluster.
    --
    -- Not reassigned, refused: these carry data, and a migration that silently
    -- changes who owns somebody's table is worse than one that stops.
    SELECT string_agg(format('%s (%s)', c.relname, pg_get_userbyid(c.relowner)), ', '
                      ORDER BY c.relname)
      INTO strays
      FROM pg_class c
      JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'app'
       AND c.relkind IN ('r', 'p', 'v', 'm', 'f', 'S')
       AND pg_get_userbyid(c.relowner) <> 'cadence_owner';

    IF strays IS NOT NULL THEN
        RAISE EXCEPTION
            'schema app already contains objects this chain does not own: %. '
            'Their owner can disable row level security on them. Reassign them to cadence_owner '
            'or drop them, then apply again.',
            strays;
    END IF;
END
$$;

-- Ownership converged; now the privileges. ALTER SCHEMA ... OWNER TO moves the
-- owner and leaves every grant the previous owner issued exactly where it was,
-- so a schema created by hand with GRANT ALL ... TO cadence_app comes out of the
-- block above owned by cadence_owner and still handing the request path CREATE
-- on it — which is the whole hole, one step further along.
--
-- Written as a sweep of the ACL rather than a list of REVOKEs, because the point
-- is not to undo the grants somebody thought of: it is that the only privileges
-- on this schema are the ones the next statement issues.
DO $$
DECLARE
    grantee text;
BEGIN
    FOR grantee IN
        SELECT DISTINCT
               CASE WHEN a.grantee = 0 THEN 'PUBLIC' ELSE pg_get_userbyid(a.grantee) END
          FROM pg_namespace n, aclexplode(n.nspacl) a
         WHERE n.nspname = 'app'
    LOOP
        CONTINUE WHEN grantee IN (
            'cadence_owner', 'cadence_patient', 'cadence_doctor', 'cadence_admin',
            'cadence_service');

        IF grantee = 'PUBLIC' THEN
            EXECUTE 'REVOKE ALL ON SCHEMA app FROM PUBLIC';
        ELSE
            EXECUTE format('REVOKE ALL ON SCHEMA app FROM %I', grantee);
        END IF;
    END LOOP;
END
$$;

-- Entering the schema is allowed; creating in it is not. Schema changes arrive
-- through this chain and through nothing else — not through a database control
-- panel, and not through the API.
--
-- Granted to the impersonation targets and to nobody else. The connection roles
-- never touch the schema outside the seam, and a role holding nothing on the
-- schema fails with 42501 the moment it tries — which is the failure this
-- arrangement wants to be loud.
GRANT USAGE ON SCHEMA app TO cadence_patient, cadence_doctor, cadence_admin, cadence_service;

-- There are deliberately no default privileges. ALTER DEFAULT PRIVILEGES cannot
-- distinguish one product role from another — it grants the same verbs on every
-- future table to whoever is named — which is the opposite of the per-role, and
-- where needed per-column, matrix this access model is built on. Every grant is
-- written in the migration of the table it belongs to.
--
-- Nothing is granted on sequences either, and that is a decision rather than an
-- omission: an identity column's sequence leaks the number of values ever handed
-- out — an upper bound on inserts, counting the rolled back and the deleted —
-- and no policy filters it, because row level security cannot be enabled on a
-- sequence at all.
