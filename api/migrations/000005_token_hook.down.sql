DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

DROP FUNCTION IF EXISTS app.access_token_hook(pg_catalog.jsonb);

RESET ROLE;

DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_auth_hook') THEN
        EXECUTE 'GRANT cadence_auth_hook TO CURRENT_USER';
        EXECUTE 'DROP OWNED BY cadence_auth_hook CASCADE';
        EXECUTE 'DROP ROLE cadence_auth_hook';
    END IF;
END
$$;
