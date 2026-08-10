-- The token issuance hook: the one place the product role gets into a token.
--
-- GoTrue calls it while minting an access token, hands it the event as jsonb and
-- takes back the event with the claims it should sign. Everything the request
-- path does afterwards rests on this: the seam reads cadence_role and chooses a
-- Postgres role from a closed map, and a role that never arrived is a caller the
-- seam refuses.
--
-- SECURITY DEFINER, owned by cadence_owner. Not cadence_service: the service pool
-- becomes that role, and a role that owns this function could ALTER it to drop
-- the pinned search_path or DROP it outright. No pool connects as cadence_owner,
-- which is what makes the owner a safe definer.
--
-- Reading profiles as the defining role is permitted by the narrow
-- FOR SELECT TO cadence_owner policy written in the previous migration — forced
-- row level security applies to the owner too, so without that policy this
-- function reads nothing and every token comes out without a role.

-- The intermediary. GoTrue's own database role is made a member of this one, so
-- the grant is to a name the chain controls rather than to whatever the
-- deployment happened to call its identity provider's user.
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_auth_hook') THEN
        EXECUTE 'CREATE ROLE cadence_auth_hook NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS';
    END IF;
END
$$;

-- Created by the applying role rather than by the owner: cadence_owner is
-- NOCREATEROLE, and owning the schema is a different thing from being able to
-- invent roles.
SET ROLE cadence_owner;

CREATE FUNCTION app.access_token_hook(event pg_catalog.jsonb) RETURNS pg_catalog.jsonb
    LANGUAGE plpgsql
    STABLE
    SECURITY DEFINER
    SET search_path = pg_catalog, pg_temp
AS $$
DECLARE
    claims       pg_catalog.jsonb;
    subject      text;
    product_role text;
BEGIN
    claims := event -> 'claims';

    -- Token issuance must not fail. An event this function does not understand
    -- comes back unchanged: a caller with no product role is refused by the seam
    -- with a reason of its own, while a hook that raised would take sign-in down
    -- for everybody, including the admin who would have to fix it.
    IF claims IS NULL OR pg_catalog.jsonb_typeof(claims) <> 'object' THEN
        RETURN event;
    END IF;

    subject := event ->> 'user_id';

    IF subject IS NULL OR subject !~
        '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$'
    THEN
        product_role := NULL;
    ELSE
        SELECT p.role INTO product_role
        FROM app.profiles p
        WHERE p.user_id = subject::uuid;
    END IF;

    -- Written deterministically, both ways. A user with a profile gets the role;
    -- a user without one gets the key removed — which matters because the input
    -- is formed by GoTrue from user data, so an event arriving with
    -- cadence_role already substituted must not come back out with it.
    IF product_role IS NULL THEN
        claims := claims - 'cadence_role';
    ELSE
        claims := pg_catalog.jsonb_set(
            claims, '{cadence_role}', pg_catalog.to_jsonb(product_role));
    END IF;

    -- Only that one key is touched. The mandatory claims — sub, aud, exp, iss,
    -- role — travel through untouched, and dropping any of them makes GoTrue
    -- answer 500 rather than issue a token.
    RETURN pg_catalog.jsonb_set(event, '{claims}', claims);
END;
$$;

-- CREATE FUNCTION grants EXECUTE to PUBLIC, and a SECURITY DEFINER function open
-- to PUBLIC is the definer's rights handed to everybody.
REVOKE ALL ON FUNCTION app.access_token_hook(pg_catalog.jsonb) FROM PUBLIC;

GRANT USAGE ON SCHEMA app TO cadence_auth_hook;
GRANT EXECUTE ON FUNCTION app.access_token_hook(pg_catalog.jsonb) TO cadence_auth_hook;

RESET ROLE;
