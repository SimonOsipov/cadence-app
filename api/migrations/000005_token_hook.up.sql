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
-- FOR SELECT TO cadence_owner policy written in 000004_identity_policies —
-- forced row level security applies to the owner too, so without that policy this
-- function reads nothing and every token comes out without a role.

-- The intermediary. GoTrue's own database role is made a member of this one, so
-- the grant is to a name the chain controls rather than to whatever the
-- deployment happened to call its identity provider's user.
--
-- Created by the applying role rather than by the owner: cadence_owner is
-- NOCREATEROLE, and owning the schema is a different thing from being able to
-- invent roles. Hence this block runs before the SET ROLE below.
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_auth_hook') THEN
        EXECUTE 'CREATE ROLE cadence_auth_hook NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS';
    END IF;
END
$$;

SET ROLE cadence_owner;

CREATE FUNCTION app.access_token_hook(event pg_catalog.jsonb) RETURNS pg_catalog.jsonb
    LANGUAGE plpgsql
    STABLE
    SECURITY DEFINER
    SET search_path = pg_catalog, pg_temp
AS $$
DECLARE
    uuid_shape constant text :=
        '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$';
    claims       pg_catalog.jsonb;
    subject      text;
    claimed      text;
    product_role text;
BEGIN
    claims := event -> 'claims';

    -- An event this function does not understand comes back byte for byte. That
    -- is narrower than "issuance cannot fail", and the difference is worth being
    -- exact about: GoTrue refuses such an output itself, with 500 either way —
    -- "output claims field is missing" when the key is absent, a JSON type error
    -- when it is present and not an object. What the guard buys is that the
    -- database never *raises* — a raised exception is a different, louder 500,
    -- and it would take sign-in down for reasons that have nothing to do with
    -- the caller.
    --
    -- It warns, because the alternative is invisible. An event whose shape
    -- changed under a GoTrue upgrade would otherwise mean every token silently
    -- losing its product role, indistinguishable in the log from the ordinary
    -- case of a user who has no profile yet.
    IF claims IS NULL OR pg_catalog.jsonb_typeof(claims) <> 'object' THEN
        RAISE WARNING 'access_token_hook: unrecognised event, claims are %',
            coalesce(pg_catalog.jsonb_typeof(claims), 'absent');

        RETURN event;
    END IF;

    subject := event ->> 'user_id';
    claimed := claims ->> 'sub';

    IF subject IS NULL OR subject !~ uuid_shape THEN
        RAISE WARNING 'access_token_hook: user_id is %, issuing no product role',
            coalesce(subject, 'absent');

        product_role := NULL;

    -- The profile is looked up by user_id; the subject the token will carry is
    -- claims.sub. GoTrue fills both from one user record, so they cannot diverge
    -- today — but this is a SECURITY DEFINER function, and "the role of one user
    -- with the subject of another" is the one output it must never produce. The
    -- seam downstream trusts the pair: it publishes sub for row level security
    -- and picks the Postgres role from cadence_role. On a disagreement the hook
    -- names no role rather than choosing which half to believe.
    -- CASE rather than `claimed !~ uuid_shape OR claimed::uuid <> …`: Postgres
    -- does not promise the order in which the arms of an OR are evaluated, and
    -- the cast of a malformed sub raising 22P02 is precisely what the paragraph
    -- above says cannot happen. The order is made structural instead of hoped
    -- for.
    -- The parentheses around CASE are not style: plpgsql reads an ELSIF condition
    -- up to the first unparenthesised THEN, and without them the THEN belonging
    -- to the CASE ends the condition — "syntax error at end of input", at
    -- migration time.
    ELSIF claimed IS NOT NULL AND (CASE
              WHEN claimed !~ uuid_shape THEN true
              ELSE claimed::pg_catalog.uuid <> subject::pg_catalog.uuid
          END)
    THEN
        RAISE WARNING 'access_token_hook: event names user_id % and sub %, issuing no product role',
            subject, claimed;

        product_role := NULL;
    ELSE
        SELECT p.role INTO product_role
        FROM app.profiles p
        WHERE p.user_id = subject::pg_catalog.uuid;
    END IF;

    -- Written deterministically, both ways. A user with a profile gets the role;
    -- a user without one gets the key removed — which matters because the input
    -- is formed by GoTrue from user data, so an event arriving with
    -- cadence_role already substituted must not come back out with it.
    --
    -- No warning on this path: an invited account that has not been provisioned
    -- is an ordinary state, not a fault.
    IF product_role IS NULL THEN
        claims := claims - 'cadence_role';
    ELSE
        claims := pg_catalog.jsonb_set(
            claims, '{cadence_role}', pg_catalog.to_jsonb(product_role));
    END IF;

    -- Only that one key is touched, and it is set inside the claims rather than
    -- beside them: GoTrue signs event.claims, so a role written at the top level
    -- of the event is a role that reaches no token.
    --
    -- The claims GoTrue insists on travel through untouched. It validates this
    -- function's output against MinimumViableTokenSchema (v2.194.0), whose
    -- required members are aud, exp, iat, sub, email, phone, role, aal,
    -- session_id and is_anonymous; dropping any of them is answered with 500
    -- rather than a token. `iss` is deliberately not in that list — dropping it
    -- yields a signed token with no issuer, which this API's verifier refuses
    -- instead. Two different failures, and the difference matters to whoever has
    -- to find one.
    RETURN pg_catalog.jsonb_set(event, '{claims}', claims);
END;
$$;

-- CREATE FUNCTION grants EXECUTE to PUBLIC, and a SECURITY DEFINER function open
-- to PUBLIC is the definer's rights handed to everybody.
REVOKE ALL ON FUNCTION app.access_token_hook(pg_catalog.jsonb) FROM PUBLIC;

-- USAGE as well as EXECUTE: a function cannot be called without USAGE on the
-- schema holding it, so the hook role reaches `app` — and nothing in it. That is
-- narrower than it sounds and is asserted rather than described: no privilege on
-- any table, no other function, measured in the policy and grant registries.
GRANT USAGE ON SCHEMA app TO cadence_auth_hook;
GRANT EXECUTE ON FUNCTION app.access_token_hook(pg_catalog.jsonb) TO cadence_auth_hook;

RESET ROLE;
