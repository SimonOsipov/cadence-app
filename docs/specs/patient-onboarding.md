---
type: spec
project: cadence
status: approved
priority: p1
created: 2026-07-30
todoist_parent: "6h9JrRJC22fGwMHH"
components: [api, identity, audit, data-layer]
proposal: "[[20-Projects/cadence/architecture/proposals/invites-and-onboarding|architecture/proposals/invites-and-onboarding]]"
---

<!-- SNAPSHOT (read-only copy). Master: 20-Projects/cadence/specs/patient-onboarding.md in vault prll-vault. Edit the vault note, then re-export — never edit here. -->

# Patient onboarding: creation, invitation, sign-in by link

## Summary

The second of the two onboarding blocks. The doctor creates a patient, a Russian
email goes out, the patient signs in via the link and sees exactly their own data.

The spec was rewritten after the fourth review round, and that round reversed a
decision from the previous draft: **`invites` is back**. Removing the table
destroyed not derived state but a fact — there is no column holding an address
anywhere in the `app` schema, and `auth.users` is declared permanently unreadable.
Without a recorded address, three things become impossible at once: knowing whose
invite is pending; resending it; showing the address on the dashboard. The new
shape is a log of **our action**, not a mirror of GoTrue's state.

The same round revealed that an external call inside the transaction made a retry
**destructive**, and that the claim rule was a dead end with no way out. Both are
fixed: the calls moved outside, under an advisory lock, and `provisioner` gained a
third operation.

## User Story

**As a** clinic doctor
**I want** to create a patient in one operation, see that the invite went out, and
be able to resend it
**So that** an account only ever appears by invitation, my creation is named in the
audit under my name, and a failure does not burn the patient's address forever

## Acceptance Criteria

- [ ] The test harness brings up GoTrue with the hook registered; the cycle tests run against **the database GoTrue is connected to** — otherwise the hook will not find `app.profiles` — and clean up both `app.*` and `auth.users` after themselves
- [ ] The incompleteness of hermeticity is recorded: `auth` state is shared within the package, and tests are separated by unique addresses. IDN-17 demands more, and that is named as not done rather than done
- [ ] In tests the invite link is taken from `auth.users.confirmation_token` through a **harness** connection, not by a query; the assumption "the link is written to the log" is unverified and unused
- [ ] Link lifetime: the production `GOTRUE_MAILER_OTP_EXP` value is chosen and recorded as what the derived state's TTL reuses; the harness **overrides** it, otherwise the expiry test would wait out the production lifetime
- [ ] Rate limits are named as measured: `/recover` hits the per-user `GOTRUE_MAILER_MAX_FREQUENCY`, while the admin `/invite` is **not covered at all** by the mail limiter — so the invite limit is imposed on our side, per doctor and per window
- [ ] `POST /recover` is permitted before the first sign-in and recorded as accepted; it was measured that on its own it does not confirm the account and does not touch the invite token, but that following `verify?type=recovery` does confirm it
- [ ] There is nothing to disable email change via `PUT /user` — v2.194.0 has no such key. Accepted; the divergence of `invites.email` from `auth.users.email` is recorded, and reconciliation is left to whoever reads the state
- [ ] The address is lowercased **on our side before the lookup**: `?filter=` is case-sensitive, and `/invite` locks the address; mixed case would otherwise return an empty lookup for an existing account and kill a live link
- [ ] `invites` is created: `user_id`, the normalized address, who invited, and when. No `status`, no `payload`, no `gotrue_user_id`. Grants and policies per §04, entries in all three registries, and the policy test suite extended — an acceptance rule
- [ ] External calls happen **outside** the transaction; retries are serialized by an advisory lock on the normalized address. The transaction contains only inserts
- [ ] A double click is safe: the loser sees the committed profile and answers `409` **without calling** `/invite` — it was measured that `/invite` on an unconfirmed account rotates the token and would have killed a live link
- [ ] Claiming: our invite record exists and there is no row in `profiles` → finish the creation. If the account is confirmed or has signed in — first **delete** through `provisioner`, then invite again; otherwise the new patient would inherit the previous person's permanent password
- [ ] No invite record but an account exists → `409`: that is not our half-finished onboarding, it is somebody else's account
- [ ] A failure at commit after a successful invite is cured by a retry, and the test verifies that **after the invitee has opened the link** — otherwise the test is green only because nobody clicks in the harness
- [ ] Failure mapping: `422 email_exists` and `23505` on the `profiles` primary key → `409`; an unavailable `provisioner` → `503` with its own problem type. All three are reachable through an ordinary scenario
- [ ] `POST /v1/patients` sets `role = 'patient'` itself; an unknown field in the body yields `400`, not `403`
- [ ] `POST /v1/providers` is available to admins only and creates `role = 'doctor'` only: admins come into being through a one-off command, because the service-path policy does not let `role='admin'` through
- [ ] The doctor must assign themselves; may add a second specialist; only an admin can assign a patient exclusively to somebody else — all three branches are covered
- [ ] `profiles.timezone` is empty at creation; the identity block's service-path validation is amended to accept its absence, and the amendment is named
- [ ] A token without `cadence_role` yields `403` with its own problem type, never `5xx`
- [ ] `patient.create` and `invite.send` are written in the patient creation transaction and name the doctor; `provider.create` — in the staff creation transaction. A rollback removes its own transaction's rows
- [ ] `cmd/bootstrap-admin` works under the migration role; for that, owner `INSERT` policies are added on `profiles` and `audit_log` with a `WITH CHECK` pinning `role='admin'` and `actor_job`, and **both registry tables are updated** — the block changes the access model, and that is an act of acceptance
- [ ] The command refuses to run when an admin already exists, and writes nothing
- [ ] The templates, their subjects, and the sender name are in Russian; the `detail` fields of the `409` and `503` responses a doctor will see are in Russian. GoTrue's own error and redirect strings stay English, and that is recorded as deferred to the client surfaces
- [ ] The redirect allow-list is set per environment; a disallowed `redirect_to` falls back to `SITE_URL`, so the allow-list is required for the thing to work at all
- [ ] The full cycle is hermetic: an admin creates a doctor, the doctor creates a patient, the patient signs in by link, the token carries the right role, and visibility is exactly what the policy suite proved
- [ ] The acceptance criterion "the Russian email arrives" is named as waiting on the SMTP provider from SKL-01 (the task was rewritten on 07-29 to Timeweb and owns it) and is not marked done
- [ ] The edits to existing tests listed in the steps are made; `make gate` and `make test-integration` are green; `openapi.json` is regenerated in the same commit

## Scope / Non-scope

**In scope.** A GoTrue container in the harness. GoTrue limits and address
normalization. The `invites` table with its policies and registries. Owner
policies for `bootstrap-admin`. `POST /v1/patients` and `POST /v1/providers` with
the advisory lock, claiming, and deletion. Russian templates, subjects, sender,
redirect addresses, Russian `detail` fields for refusals shown to the doctor.
`cmd/bootstrap-admin`. Cycle tests.

**Out of scope.** `GET /v1/dashboard/overview` v0 and person seeds. Deriving
invite state — wherever the reader appears; the data for it now exists.
**`patient_profiles.joined_at` stays without a writer**: acceptance happens
entirely inside GoTrue, the API never learns of it, and we have no hook on first
sign-in. Named as owned by the block that adds first-sign-in handling. **Setting a
password from §04 step 2 is not implemented**: it was measured that sign-in by link
requires no password; the person sets one themselves via `PUT /user`, and that is
GoTrue's behaviour, not our flow. `nutrition_targets` — NUT-01 extends this same
transaction and fills it in; the extension point is marked. The SMTP provider — the
manual SKL-01. The screens on both surfaces. Russifying GoTrue's own strings.
Reconciling `invites.email` with `auth.users.email`.

## What already exists (DONE)

**Strictly speaking — none of what is listed below.** Both prerequisite blocks are
`approved` but **not implemented**: today the repository holds only
`000001_base`, `testsupport` still exports `AuthenticatedRole`, and
`config.ServiceURL` is not validated. This block comes third and rests on:

- **Identity** — seven roles, two seams, `app.jwt_subject()`, six tables, six
  policy shapes, three registries, the audit actor.
- **Trust Boundary** — `provisioner` with **three** operations, a client whose
  interface is owned by the consumer, `WITH CHECK (role IN ('patient','doctor'))`,
  zero access to `auth`, two keys, time bounds, the image by digest.

**Measured against live GoTrue on 2026-07-30** — what this block rests on:

- `/verify?type=invite` → 303 with a session, sets **both** `confirmed_at` **and**
  `last_sign_in_at`, clears `confirmation_token`. No password is set.
- From that session, `PUT /user` **sets a permanent password**, after which the
  password grant works. Hence the need to delete when claiming.
- `/invite` on a confirmed account → `422 email_exists` with no changes. On an
  unconfirmed one → 200, **the token rotates**, `invited_at` is overwritten,
  `recovery_token` does not rotate.
- `/recover` is public, does not itself confirm, and does not touch the invite
  token; following `verify?type=recovery` confirms and also extinguishes a pending
  invite.
- `?filter=` is a substring match and **case-sensitive**; `/invite` locks the address.
- `GOTRUE_MAILER_AUTOCONFIRM=true` does **not** auto-confirm invitations.
- `/signup` and `/otp` on an unknown address → `422 signup_disabled`, and no user
  is created. There is no enumeration oracle.
- A table owner under `FORCE RLS` with no `INSERT` policy is refused.

## Technical detail

### The harness

A GoTrue container per package with **its own persistent database**, to which our
chain has been applied: the hook resolves over GoTrue's connection, so
`app.profiles` must live there too. The cycle tests run against it and clean up
`app.*` and `auth.users` after themselves; the per-test `NewDatabase` databases
continue to live separately and never touch GoTrue — which is why
`DROP DATABASE … WITH (FORCE)` does not sever its connection.

Hermeticity is incomplete **by decision**: addresses are separated, `auth` state is
shared. IDN-17 requires eliminating both address collisions and schema residue;
that will not be done, and it is written down as such.

`GOTRUE_MAILER_OTP_EXP` is overridden in the harness with a short value: the expiry
test would otherwise wait out the production lifetime.

### `invites`

`user_id uuid PK` (which is also the GoTrue user identifier), `email text`
normalized, `invited_by uuid → profiles`, `invited_at timestamptz`.

What is absent and why: `status` — derived; `payload` — the name and assignments
are written to their own tables in the same transaction; `gotrue_user_id` — the
profile identifier equals the user identifier, so there is nothing to duplicate.

Grants per §04: doctor `INSERT`, admin `SELECT`, service path `INSERT` and
`SELECT`, patient nothing. Address uniqueness is **not** imposed: GoTrue provides
uniqueness, and our row may be recreated on a repeated creation.

### The order of creation

```
advisory lock on the normalized address
  ├ outside the transaction: lookup through provisioner
  │   ├ no account                            → invite
  │   ├ exists, our record exists, no profile
  │   │   ├ unconfirmed, never signed in      → invite again (the old link dies)
  │   │   └ confirmed or has signed in        → delete, then invite
  │   ├ exists, our record exists, profile exists → 409
  │   └ exists, no record of ours              → 409 (somebody else's account)
  └ transaction: profiles(user_id from GoTrue) · patient_profiles ·
    care_team_assignments · user_preferences · invites · audit
```

The lock is taken before everything and released on completion: it serializes a
double click and two requests for one address. The loser sees the committed profile
and answers `409` **without calling** `/invite` — otherwise it would rotate the
token and kill the link of the patient the winner just created.

Deleting when claiming a confirmed account is not hygiene: it was measured that a
permanent password can be set from the link's session, so without deletion the new
patient would inherit the previous person's credential.

The window between the invitation and the commit remains — it is irreducible, the
commit always comes after the side effect. But it is now **curable**: the invite
record lets a repeated request recognize its own account and finish the job instead
of running into `422`.

`profiles.user_id` is the identifier **from GoTrue**. One generated locally would
have produced green tests and an empty sign-in: the hook would not find the profile.

The extension point for NUT-01 (`nutrition_targets`) is marked with a comment in
the transaction.

### Failures

`422 email_exists` and `23505` on the `profiles` primary key → `409`.
An unavailable `provisioner` → `503` with its own problem type: it is the most
likely production failure, and without the mapping it yields `500` and an orphan
with an unknown outcome. A token without `cadence_role` → `403`.

The shape is `problem+json`, with `detail` in Russian: this is the first refusal a
doctor will see in the interface. The response is the same for doctor and admin.

### `cmd/bootstrap-admin`

Under the migration role. It belongs to `cadence_owner`, but that role has only
`SELECT` on `profiles` and nothing on `audit_log`, and `FORCE RLS` rejects an
owner's insert without a policy. So the block **adds** owner `INSERT` policies with
a `WITH CHECK` pinning `role='admin'` and `actor_job`, and updates both registry
tables. The registries are exhaustive — a superfluous grant fails a test — which is
why this is acceptance, not a detail.

Honestly: an audit row written by a role capable of `TRUNCATE` is weaker than the
rest in integrity terms. That is the same boundary `audit.md` already names as
detectable rather than preventable.

The chicken-and-egg is described in the build plan as GOL-04, not DEP-02.

## Architecture decision

The architecture was accepted by the proposal
[[20-Projects/cadence/architecture/proposals/invites-and-onboarding|invites-and-onboarding]]. This block implements P4, P7, P10, P11, and the
**reversed, then rewritten** P6, plus the fourth-round decisions: P14 (the third
`provisioner` operation), P15 (calls outside the transaction under an advisory
lock), P16 (address normalization), P17 (nothing to disable email change with), P18
(password from the link's session), P19 (rate limits), P20 (grants under
bootstrap), P21 (the ADR after both blocks).

The main lesson of the previous draft is worth writing down: it declared the
endpoint "idempotent rather than atomic" and backed that with a test that was green
only because in the harness the invitee never clicks the link. Real idempotency
required three things at once — an invite record, a lock on the address, and the
ability to delete an account.

## Component deltas

### identity.md
- ADDED: `invites` exists and stores `user_id`, the normalized address, who invited, and when. No `status`, no `payload`, no `gotrue_user_id`; address uniqueness is provided by GoTrue, not by our index
- ADDED: to contracts — `POST /v1/patients` (the server sets the role) and `POST /v1/providers` (admin only, `role='doctor'` only)
- ADDED: invariant — the profile identifier equals the GoTrue user identifier; the order is invite-or-claim first, outside the transaction, under an advisory lock on the normalized address
- ADDED: invariant — claiming is permissible only when **our** invite record exists and there is no profile; a confirmed account, or one that has signed in, is deleted before claiming, because a permanent password can be set from the link's session
- MODIFIED: invariant 3 — acceptance by link rather than by password is **true only until the user's first action**: `PUT /user` from that same session sets a permanent password. The second acceptance channel is the public `POST /recover`
- MODIFIED: "Data" — `profiles.timezone` is empty until first sign-in, and validation accepts its absence; `patient_profiles.joined_at` stays without a writer and that is named
- ADDED: to contracts — the first admin comes into being through a one-off command under the migration role, not by invitation

### api.md
- ADDED: to "Contracts" — `POST /v1/patients` and `POST /v1/providers`
- ADDED: to the error shape — `422 email_exists` and `23505` on the `profiles` primary key → `409`; an unavailable `provisioner` → `503` with its own type; a token without `cadence_role` → `403`, never `5xx`; the `detail` of refusals a doctor sees is in Russian
- ADDED: invariant — external calls execute **outside** the transaction, and retries are serialized by an advisory lock on the normalized address: inside the transaction, an external call made a retry destructive, because `/invite` rotates the token
- ADDED: the invite limit is imposed on our side, per doctor and per window: the admin `/invite` is not covered by GoTrue's mail limiter

### data-layer.md
- ADDED: to the Auth contract — the production `GOTRUE_MAILER_OTP_EXP` is chosen and reused as the TTL of the derived state; the harness overrides it
- ADDED: to the Auth contract — rate limits are named as measured: `GOTRUE_MAILER_MAX_FREQUENCY` applies to `/recover` per user, and the admin `/invite` is not covered at all
- ADDED: to the Auth contract — `POST /recover` is public and permitted before the first sign-in; it does not confirm on its own, but following `verify?type=recovery` confirms and extinguishes a pending invite
- ADDED: to the Auth contract — there is nothing to disable email change via `PUT /user`; the divergence of `invites.email` from `auth.users.email` is accepted, and reconciliation is left to whoever reads the state
- ADDED: to the trust boundary — **SMTP holds credentials**: the link is itself a session credential
- ADDED: to the test harness — a GoTrue container with its own persistent database, to which the chain has been applied, because the hook resolves over its connection; hermeticity is incomplete, IDN-17 is not satisfied, and that is named

### audit.md
- ADDED: to the list of actions — `patient.create`, `invite.send` (the patient transaction), `provider.create` (the staff transaction)
- ADDED: to contracts — the first admin is audited with `actor_job` under the migration role; the integrity of that row is weaker than the rest, because the role that wrote it is capable of `TRUNCATE`, and that is the same boundary already named as detectable

## Decomposition

### step-1: A GoTrue container in the test harness

GoTrue per package with its own persistent database, to which the chain has been
applied, and with the hook registered. The cycle tests run against it and clean up
`app.*` and `auth.users`; the per-test databases stay separate. The incompleteness
of hermeticity and the non-satisfaction of IDN-17 are recorded in a comment on the
spot. `OTP_EXP` is overridden in the harness.

Edits to existing code: `testsupport/postgres.go` — `StartCluster` and
`NewDatabase` gain a neighbour without changing the semantics of per-test
databases; `base_migration_integration_test.go:TestMain` — the only caller of
`StartCluster`, and GoTrue must be **optional** for that package, otherwise all of
its tests pay for the container; the comment in
`coexistence_integration_test.go` about a future `profiles` reference to `auth` is
now definitively obsolete.
todoist: "6h9JrX2qVQcWfjWH"

### step-2: GoTrue limits and address normalization

The production `GOTRUE_MAILER_OTP_EXP`, `GOTRUE_MAILER_MAX_FREQUENCY`, and the
invite limit on our side. Lowercasing the address before the lookup. `/recover`
recorded as accepted. The impossibility of disabling email change recorded.

Tests: mixed case finds an existing account; an expired link is rejected (against
the overridden lifetime); the invite limit fires; `/recover` on an unconfirmed
account does not itself confirm, while following the recovery link confirms and
extinguishes the invite — the properties are **asserted**.
todoist: "6h9JrX7CJxQHVPGH"

### step-3: The `invites` migration and the owner policies

`invites` with the columns from the detail section, `SET ROLE cadence_owner`,
grants and policies per §04, entries in the registries, an extension of the policy
test suite. Also here: owner `INSERT` policies on `profiles` and `audit_log` for
`bootstrap-admin`, with `WITH CHECK` on `role='admin'` and `actor_job`, and updates
to both registry tables.

Tests: the doctor creates and does not read; the admin reads and does not write;
the patient nothing; the owner inserts only an admin and only with `actor_job`; the
registries fail on divergence.
todoist: "6h9JrX5MFm3GwhWH"

### step-4: `cmd/bootstrap-admin`

A one-off command under the migration role: the first admin and an audit row with
`actor_job`; refusal when an admin already exists. It comes before the endpoints:
their tests need an admin, and there is no other producer of one.
todoist: "6h9JrX8W8hj8Gjcq"

### step-5: `POST /v1/patients`

The order from the detail section: the advisory lock, lookup and invite or claim
with deletion outside the transaction, a short transaction of inserts and audit.
The server sets the role, an unknown field yields `400`. Mapping of `422`, `23505`,
and provisioner unavailability. `detail` in Russian. The identity block's timezone
validation is amended to accept absence. The NUT-01 extension point is marked.
`make openapi` in the same commit.

Tests: a double click is serialized and the loser does not call `/invite`; a
failure at commit is cured by a retry **after the invitee has opened the link**;
claiming a confirmed account deletes it and invites again; an account with no
record of ours yields `409`; mixed case does not create a second patient; the
doctor must assign themselves and may add a second specialist.
todoist: "6h9JrXH2mCxfrXPq"

### step-6: `POST /v1/providers`

Admin only, `role='doctor'` only. Tests: a doctor gets 403; an attempt to create an
admin is rejected by the service-path policy and surfaced clearly rather than as a
`500`.
todoist: "6h9JrXH9VHrcHQRq"

### step-7: Russian templates, subjects, sender, redirect addresses

Three templates, subjects, and a sender name in Russian; the variables are named
and the files are in the repository. The redirect allow-list per environment.
GoTrue's own strings stay English — recorded as deferred.

The acceptance criterion "the email arrives" is named as waiting on SKL-01 and is
not marked done.
todoist: "6h9JrXM3qg28RwhH"

### step-8: Integration tests for the cycle

Hermetically: an admin creates a doctor, the doctor creates a patient, the patient
signs in by link, the token carries the right role, and visibility is the same one
the policy suite proved. The refusal branches: an attempt to create an admin, a
doctor on `/v1/providers`, public registration, claiming somebody else's account, a
retry after the link has been clicked.

Also here, a test **asserting** the residual property: after a claim with deletion
the previous sessions are dead, whereas without deletion they would have been alive.
todoist: "6h9JrXQq59rcmcCH"

## Open questions

> [!question] `patient_profiles.joined_at` stays without a writer: acceptance
> happens entirely inside GoTrue, and we have no hook on first sign-in. The
> candidates are writing it on the first authenticated request (which needs a path
> outside `WithCaller`, because nesting the seams is forbidden) or handing it to
> the timezone capture, which also happens on first sign-in. Decided by the block
> that adds first-sign-in handling; named here so it does not get lost.

> [!question] The numbers for link lifetime and rate limits are chosen at step 2
> with nothing to base them on: it is a pilot, and the creation rate is unknown. We
> choose conservatively and record them as subject to revision after the first
> weeks.

> [!question] The ADR for this proposal is condensed after **both** blocks, not the
> first: the decisions are implemented here, and removing-then-restoring a §03
> entity is a change to the partner's canonical document, which by convention only
> an ADR makes. Who exactly writes it — the second block's `/implement` or a
> separate step — is not decided.
