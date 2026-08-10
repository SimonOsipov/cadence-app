-- The one function the access policies call.
--
-- Every policy of every table keys on "who is asking", and this is where that
-- question is answered: it reads the claims the impersonation seam published
-- into the transaction and returns the subject as a uuid.
--
-- There is deliberately no companion app.jwt_role(). The product role is not a
-- value a policy compares against — it is the Postgres role the transaction is
-- running as, chosen by the seam from a closed map, and the policies key on TO.
-- A function returning the role would invite exactly the shape the whole block
-- was rewritten to remove: a policy deciding access by comparing a claim.

SET ROLE cadence_owner;

-- STABLE, not IMMUTABLE. The value depends on a setting rather than on the
-- arguments, and IMMUTABLE would let the planner fold the call at plan time —
-- so a prepared statement reused across two requests on one pooled connection
-- would answer the second with the first caller's subject. That is not a
-- performance nuance; it is one patient reading another's rows, and it is
-- asserted by a test that runs the same statement twice under two subjects.
--
-- search_path is pinned and every name is qualified. Without it, a caller able
-- to create a temporary object could shadow a function this body calls and have
-- it run under the definer's rights; the function is not SECURITY DEFINER today,
-- but the pinning costs nothing and the next edit might make it one.
CREATE FUNCTION app.jwt_subject() RETURNS pg_catalog.uuid
    LANGUAGE sql
    STABLE
    SET search_path = pg_catalog, pg_temp
AS $$
    -- Nothing here may raise, and there are two casts that could.
    --
    -- The json cast is guarded by IS JSON OBJECT rather than by nullif alone.
    -- nullif covers one value — the empty string the service seam leaves behind,
    -- because set_config has no way to unset — and every other non-JSON value
    -- raises 22P02: `garbage`, a truncated object, anything at all. That is not
    -- a hypothetical input. request.jwt.claims has USERSET context, so a caller
    -- can overwrite it inside their own transaction and nothing can forbid it;
    -- the spec says as much, and a seam test already proves it is reachable.
    -- Once this call sits inside every policy, an unparseable value would be a
    -- 500 on every table for that session, where the design's answer is
    -- "no rows".
    --
    -- The uuid cast is guarded by the shape check. uuid_in raises on a value
    -- that is not one, and the regex is a strict subset of what it accepts —
    -- the canonical hyphenated form and nothing else, matching what the seam
    -- validates before it publishes. The seam is the first of the two guards;
    -- this is the one that holds when the claims are overwritten.
    --
    -- IS JSON is PostgreSQL 16, which this makes a floor. The chain's role
    -- grants still carry a pre-16 branch; nothing else here does, and the
    -- deployment is 17.
    SELECT CASE
        WHEN claimed OPERATOR(pg_catalog.~)
             '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$'
        THEN claimed::pg_catalog.uuid
    END
    FROM (
        -- NULLIF unqualified because it is grammar rather than a function:
        -- there is no pg_catalog.nullif to name, and nothing can shadow a
        -- construct the parser expands into a CASE.
        SELECT CASE
                   WHEN raw IS JSON OBJECT
                   THEN raw::pg_catalog.json OPERATOR(pg_catalog.->>) 'sub'
               END AS claimed
        FROM (
            SELECT NULLIF(
                       pg_catalog.current_setting('request.jwt.claims', true), ''
                   ) AS raw
        ) AS setting
    ) AS claims
$$;

-- CREATE FUNCTION grants EXECUTE to PUBLIC. Without the revoke the grant below
-- would say nothing: every role in the cluster could already call it, and the
-- registry that reconciles function privileges would be reconciling a list
-- nobody was bound by.
REVOKE ALL ON FUNCTION app.jwt_subject() FROM PUBLIC;

-- The four roles a transaction can be running as. Not the connection roles:
-- cadence_app and cadence_service_app hold nothing on this schema and reach
-- nothing in it without the seam, and this function is no exception to that.
GRANT EXECUTE ON FUNCTION app.jwt_subject()
    TO cadence_patient, cadence_doctor, cadence_admin, cadence_service;

RESET ROLE;
