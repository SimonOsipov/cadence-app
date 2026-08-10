DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

DROP FUNCTION IF EXISTS app.access_token_hook(pg_catalog.jsonb);

RESET ROLE;

-- Dropping the role takes its memberships with it, and the identity provider's
-- own database user is one of them. Nothing in the chain can put that back: the
-- membership is provisioning, granted by `make dev-up` locally and by the
-- deployment's last step remotely. So `migrate down` followed by `migrate up`
-- leaves a stack where every sign-in answers 500 until that grant is re-run —
-- which is a thing to know before rolling back a deployment, not after.
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_auth_hook') THEN
        EXECUTE 'GRANT cadence_auth_hook TO CURRENT_USER';
        EXECUTE 'DROP OWNED BY cadence_auth_hook CASCADE';
        EXECUTE 'DROP ROLE cadence_auth_hook';
    END IF;
END
$$;
