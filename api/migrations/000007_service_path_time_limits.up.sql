-- Time limits on the role the service path logs in as.
--
-- On cadence_service_app and not on cadence_service, and the difference is not a
-- preference: a role's settings are applied at login, from session_user. SET
-- ROLE does not adopt the target role's defaults, so the same two lines written
-- against cadence_service would be a limit nothing ever applies — green in the
-- catalogue and absent from every session.
-- TestTheServicePathLimitsSitOnTheRoleThatLogsIn measures both halves.
--
-- Milliseconds, because a bare integer is what both settings take, and because
-- the pool's startup packet carries the same two numbers spelled the same way —
-- see serviceRuntimeParams, and the test that keeps the two copies one number.
--
-- A default rather than a barrier: both settings are USERSET and a session may
-- raise its own. Closing that needs a role without the SET privilege and is
-- probably impossible without a superuser; it is an open question in the spec
-- rather than an oversight here.

ALTER ROLE cadence_service_app SET statement_timeout = 5000;
ALTER ROLE cadence_service_app SET idle_in_transaction_session_timeout = 15000;
