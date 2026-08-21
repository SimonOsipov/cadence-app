-- Dropped by their owner, in reverse dependency order. The indexes, the CHECKs
-- and the exclusion constraint go with the tables.
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cadence_owner') THEN
        EXECUTE 'SET ROLE cadence_owner';
    END IF;
END
$$;

DROP TABLE IF EXISTS app.protocol_phases;
DROP TABLE IF EXISTS app.protocol_items;
DROP TABLE IF EXISTS app.protocols;
DROP TABLE IF EXISTS app.compounds;

RESET ROLE;

-- The extension goes too, and it goes last: the exclusion constraint above is its
-- only user, and leaving it installed would make this down migration a partial
-- one — the next reader could not tell whether the extension predated the chain.
-- Outside the owner's role, because the extension belongs to whoever installed it
-- rather than to cadence_owner.
DROP EXTENSION IF EXISTS btree_gist;
