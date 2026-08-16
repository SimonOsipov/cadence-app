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

## Описание

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

## Что уже реализовано (DONE)

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

## Технические детали

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

## Архитектурное решение

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

## Дельты компонентов

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

> [!deviation] 2026-08-16 — the gap between two emails is `GOTRUE_SMTP_MAX_FREQUENCY`, not `GOTRUE_MAILER_MAX_FREQUENCY`
> Spec said: the production `GOTRUE_MAILER_MAX_FREQUENCY`, and, in the acceptance criteria and the `data-layer.md` delta, that `/recover` hits the per-user `GOTRUE_MAILER_MAX_FREQUENCY`. Actually done: the deployment and the harness set `GOTRUE_SMTP_MAX_FREQUENCY`. Why: measured against v2.194.0 on 2026-08-16 — with `GOTRUE_MAILER_MAX_FREQUENCY` set to 2s the provider still refused a second `/recover` "after 56 seconds", its one-minute default; with the `SMTP_` name it refused for the two seconds it was given. The name the spec carries is one GoTrue reads and ignores, which looks exactly like a limit in force.
> **This has to be corrected in the criteria and in the delta, not only here**: the `data-layer.md` delta line is copied into `architecture/components/data-layer.md` on finalization, and a component note naming a variable the provider ignores is how the next deployment gets no limit at all.
> A second limit on the same route was measured at the same time, and it changes what the spec says about `/invite`: `GOTRUE_RATE_LIMIT_EMAIL_SENT` is a quota per hour for the whole instance, and it **does** cover the admin `/invite` — measured by running the harness at a quota of two, where the third email of a run, an invitation, was refused. Both limits answer `over_email_send_rate_limit` and differ only in the message, so a test asserting the code alone cannot say which one it saw.
> So the sentence to carry forward is narrower than the spec's: the **gap** does not cover `/invite`; the **quota** does, and it counts the clinic rather than the doctor — which is why the invitation limit is still ours, per doctor and per window. Because the quota can refuse what our own limiter allowed, the deployment now sets it (200/hour) and a test requires it to be at least `identity.InvitesPerWindow`. The ceiling that actually matters is the SMTP provider's and is not known: SKL-01 has not been chosen.
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

> [!deviation] 2026-08-16 — four decisions this step took that its paragraph does not name
> The step conforms: four columns, `SET ROLE cadence_owner`, §04's grants, both owner policies with the pinned `WITH CHECK`, and every registry updated. What follows is what had to be decided inside it, recorded because each one is a claim a later step or a component note will read as settled.
> 1. **`invites` has no `role` column, and that absence belongs in the detail section's list.** §03 names one and the partner's build plan §DOC-10 refers to «the §03 `invites.role` field» for `POST /v1/providers`. It is not stored because the profile written in the same transaction carries the role, and a second copy is the derived state this project does not keep. The detail section's «what is absent and why» enumerates `status`, `payload` and `gotrue_user_id`; this is the fourth.
> 2. **`invited_by uuid NOT NULL REFERENCES app.profiles (user_id)` with no `ON DELETE`, and this closes an admin path for good.** Every other person table cascades from `profiles`; this one refuses. Deleting a doctor who has ever recorded an invitation answers `23503` to the admin, who holds `DELETE` through `profiles_admin`. Cascading was rejected because the invite record is the only evidence that an account is ours to claim: erase it and every retry against that account answers 409 forever, which is the state this whole block exists to make curable. **The refusal is permanent, not temporary**: no role holds `DELETE` on `invites` and no `DELETE` policy exists, so nothing on any application path can retire the row that blocks the deletion — a doctor who has sent one invitation is undeletable until a migration says otherwise. `identity.md`'s «the person tables cascade from `profiles` on delete» needs the exception, and its known-limitation section, which contemplates profile deletion as the future revocation path, has to record that such a procedure must provide a route for `invites` rows before it can retire a member of staff. Asserted by `TestAnAdminCannotRemoveADoctorWhoseInvitationsStand`, with the control that a doctor no invitation names still goes.
> 3. **`invites_doctor_insert` carries `WITH CHECK (invited_by = app.jwt_subject())`.** §04 grants the doctor `create` and names no predicate. Without one every doctor can record an invitation in a colleague's name, which is exactly the column the admin reads this table for. It is the same own-row shape the rest of the request path uses, and it is a new policy shape the `identity.md` delta does not list.
> 4. **`audit_log_bootstrap_insert` does not reconcile the actor against a published setting** — it pins `actor_job IS NOT NULL` and nothing else, because the one-off command runs under the migration role and through no seam, so there is no `app.actor_job` for a policy to compare against. `audit.md` invariant 4 states reconciliation unconditionally and now has an exception: it holds on the service path; the bootstrap policy pins only that the author is a job and never a person. The integrity consequence — an author that can `TRUNCATE` — was already recorded in this spec's `audit.md` delta; the reconciliation exception was not.
>
> Two smaller shapes, for the record: this landed as two migrations (`000008_invites`, `000009_owner_writes_the_first_admin`) because the two down paths are unrelated, and `invites.email` carries `CHECK (email = btrim(lower(email)) AND email <> '')` so that «normalized» is a property of the data rather than a habit of one caller — `btrim` is the narrower claim, since it removes spaces where the Go fold's `TrimSpace` removes every kind of whitespace.
todoist: "6h9JrX5MFm3GwhWH"

### step-4: `cmd/bootstrap-admin`

A one-off command under the migration role: the first admin and an audit row with
`actor_job`; refusal when an admin already exists. It comes before the endpoints:
their tests need an admin, and there is no other producer of one.

> [!deviation] 2026-08-16 — five decisions this step took that its paragraph does not name
> The step conforms: `cmd/bootstrap-admin` writes the profile and the audit row in one transaction under the migration role, signs with `actor_job`, and refuses a second administrator. What follows is what had to be decided inside it, each one a claim a later step or a component note will read as settled.
> 1. **The command does not invite anybody. The account exists at the provider first, and its identifier is an argument** — `bootstrap-admin <account> "<full name>"`. It could not record an invitation in any case: `app.invites` carries no policy for `cadence_owner`, which is the barrier — the `invited_by` reference is the weaker half, since a row written after the profile in the same transaction would satisfy it. What makes that absence harmless is that this command never *claims* an account — the identifier is given to it, so a run that fails is repeated with the same identifier and nothing is consumed, and an identifier somebody lost is what the provisioner's lookup answers with. The operator's sequence is therefore: `POST /invite` on the provisioner, then this command with the identifier it answered. `identity.md`'s invariant 3 — «There is no public registration. An account appears only by the clinic's invitation.» — still holds, because that invitation is sent. What does not hold is that `invites` is a complete record of them: **the first administrator's invitation leaves no row there**, so the address the clinic knows them by is nowhere in `app`, and the dashboard's «who let this address in» will not find it. The account is also unclaimable for the same reason, which costs nothing only because nothing ever claims it.
> 2. **The audit row's action is `admin.bootstrap`, not `provider.create`.** The spec's acceptance line names `provider.create` for «the staff creation transaction», which is `POST /v1/providers` in step-6 — and step-6 will write that string for every doctor. This row is about an administrator, who has no `provider_profiles` row and is not a provider; sharing the string would leave the two acts separable only on `actor_job`, a column most audit queries do not project. `action` is free text with no `CHECK` and no registry, so **step-6 inherits this**: `provider.create` is its, and only its.
> 3. **Nothing serialises two simultaneous runs.** Both would find no administrator and both would write one — `app.profiles` is unique on `user_id` and on nothing else, so the check-then-insert is the only guard. Accepted rather than solved: this is a command a person runs once per environment. The one-line alternative is the `pg_advisory_xact_lock` step-5 takes on an address, and whoever decides it is worth taking here should say so in a later step rather than assume this was overlooked.
> 4. **A successful run against the wrong identifier is a dead end, and only the migration role can leave it.** A failed run costs nothing — the identifier is repeated. A run that *succeeds* on a well-shaped but mis-pasted uuid writes an administrator nobody can sign in as, and there is no application path back: the command itself then refuses every later run, `cadence_owner` has policies for only `SELECT` and `INSERT` on `profiles` under FORCE RLS so its `UPDATE` and `DELETE` fall on zero rows, and `profiles_admin`'s `DELETE` needs an admin token that can no longer be issued to anybody. The escape is the migration role altering the arrangement by hand — a policy added, or `NO FORCE` for the length of one statement — which is a schema edit outside the chain and therefore a thing to do once, deliberately, and record. Bounded rather than closed: `IsUUIDShaped` refuses a malformed identifier, and nothing can tell a well-formed wrong one from a right one.
> 5. **The command assumes `cadence_owner` explicitly rather than inheriting it.** Measured: a migration role that inherits reaches 000009's policies without the `SET ROLE`, because a policy `TO cadence_owner` matches every member — so the statement looks dead and is not. A `NOINHERIT` migration role, which the chain's own «every migration opens with `SET ROLE cadence_owner`» convention implies is allowed, is refused without it. Asserted by `TestTheCommandWorksUnderAMigrationRoleThatInheritsNothing`; the mutation that removes the line survives every other test in the package.
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

> [!deviation] 2026-08-16 — where the implementation departs from this step's paragraph, and why
> The step conforms in its order — the lock is taken first, the lookup, the invitation and the deletion all happen outside a transaction, the server sets the role, the timezone validation now accepts absence, and the NUT-01 point is marked in the transaction. Eleven things had to be decided or diverged inside it, and each is a claim a later step or a component note will read as settled.
> 1. **The record of the invitation is committed in its own transaction, before the patient's — the diagram puts `invites` inside the single transaction, and that placement makes «cured by a retry» unreachable.** A creation that fails after the mail has gone out rolls its transaction back; with the record inside it, the retry finds an account the clinic has no record of and answers `409` forever, which is the state this whole block exists to make curable. So the order is: invite → commit (`invites` + `audit invite.send`) → commit (profile, card, care team, preferences, `audit patient.create`). The window does not close — the commit always comes after the side effect — it shrinks to the gap between the invitation and that first commit. Pinned by mutation: moving the record after the patient's transaction fails `TestACreationInterruptedAfterTheInvitationIsCuredByARetry` at the line that asks whether the record exists.
> 1a. **The same window has a second mouth, and it is closed under the lock rather than left open.** `POST /invite` creates the account and sends the mail *before* its answer comes back, so a call that fails on the way home — the client's ten-second bound, a dropped connection — leaves an account with no record either. The address is looked up again while the lock still holds, and an account **nobody has been inside** is recorded as ours; the request is still refused, because whether the mail left is not knowable from here, and the doctor's retry then finds the record and invites again. The condition is what keeps this from taking a stranger's account: from this side, «our account whose answer was lost» and «somebody else's account created in that second» are the same picture, and claiming deletes. So an account that is already confirmed or has signed in is *not* recorded — the address stays refused as somebody else's, which is the cost of the safe direction and is the one residual case where a lost answer still burns an address. Both halves are asserted: `TestAnInvitationWhoseAnswerWasLostIsStillRecorded` and `TestAnAccountSomebodyHasOpenedIsNotClaimedByAFailedInvitation`.
> 2. **`422 email_exists → 409` is not implemented, and cannot be from this side.** Measured on 2026-08-16: `cmd/provisioner` answers every refusal the identity provider gives with a fixed `502` body (`routes.go`'s `failed` → `refuse`, which replaces the reason for any status ≥ 500), so the API cannot tell «this address is already taken» from «the provisioner is down». Both map to `503`. What the criterion wants happens by another road: the address is looked up under the lock before anything is invited, so the ordinary path answers `409` from the lookup rather than from a refusal, and a retry after a race does the same. Satisfying its letter means changing that component to surface the taken address as its own status — a change to the trust boundary's answer shape, and the owner's to make.
> 3. **An unknown field yields `422`, not the `400` the criterion names.** Measured against huma v2.39 on 2026-08-16: the schema it generates carries `additionalProperties: false` and its validator answers `422` for a violation of it; `400` is what a malformed body gets. Both carry `/problems/validation`, and neither is the `403` the criterion was written to rule out. Asserted by `TestABodyThatChoosesARoleIsRefusedRatherThanIgnored`, which sends `"role":"admin"` and requires the refusal to name the field.
> 4. **The advisory lock is session-level and taken on the *request* pool, while the writes go to the service pool.** A transaction-scoped lock would end at the first commit and leave the retry unserialised, so it is held on a connection of its own for as long as the provider takes to answer. It is not the service pool's connection because that is a deadlock: with `serviceMaxConns` requests in flight, every one holding a lock connection and every one waiting for a second connection from the same pool, only another holder can release one.
> 5. **Two `403`s the step names no status for.** A doctor who leaves themselves off the care team is refused — they would create a patient no screen shows them — and so is a token whose role is a patient's or absent. Both are `/problems/forbidden` with Russian `detail`, and both are refused before the provider is asked anything, so a refused request never burns an address.
> 6. **The transport gained three problem types and one rule changed.** `/problems/forbidden`, `/problems/conflict` and `/problems/unavailable`, and `Problem.normalise` now picks *which* fixed detail a 5xx carries by type instead of always `internalDetail`. The rule it does not weaken is the one that matters: a 5xx still says only what `httpserver` decided, never the caller's words. The `503`'s sentence is deliberately about nothing in particular — «сервис временно недоступен, повторите запрос» — because `problemTypeFor` gives that type to *every* 503 the API will ever answer, and because this side cannot honestly say the invitation was not sent: it is sent before the answer comes back.
> 7. **The deletion is audited, under an action this step names.** Claiming an account that has been opened destroys it and every session on it at the identity provider — the one act of this flow performed against another system — and `audit.md`'s list for this block (`patient.create`, `invite.send`, `provider.create`) predates the third provisioner operation. It is written as `account.delete` in a transaction of its own, before the invitation that replaces it, so the record does not depend on the rest of the creation going through. `action` is free text with no `CHECK` and no registry, so this settles the vocabulary the same way step-4's `admin.bootstrap` did.
> 8. **`invite.send` says «an invitation was asked for», not «a mail was delivered».** Delivery is the provider's and is never reported back, and the row is written for a request whose answer was lost as well. One row per ask, so a second invitation to one account is a second row.
> 9. **The `422`s share one sentence, except one.** A specialist who is not a doctor gets its own Russian detail, because the identifier came from a picker and the doctor cannot find that by rereading the form; two leading specialists, an unknown timezone, a malformed identifier and an address that folds to nothing share «проверьте поля формы». None of them names the offending field in `errors`, deliberately — this body carries a person's name and clinical figures, and the transport's rule is that a submitted value never travels back.
> 10. **A claim that deletes leaves the old `invites` row behind.** The provider assigns a new identifier to the recreated account, and no role holds `DELETE` on `invites` — by decision, in step-3 — so one mailbox can carry two rows, of which only the newest names a live account. Reading «who invited this address» has to take the row whose `user_id` still exists, and the dashboard's future invite-state view inherits that. The rows are not duplicates: each is a true record of an invitation this clinic sent.
>
> Not done here and named rather than left silent: `identity.InviteLimit` still has no caller, so the criterion «the invite limit is imposed on our side, per doctor and per window» is unsatisfied. It is not in this step's paragraph or its acceptance list, and wiring it would add a `429` to the contract that nothing asked for. The invite routes are this one and step-6's, and whichever of them takes it should say so.
todoist: "6h9JrXH2mCxfrXPq"

### step-6: `POST /v1/providers`

Admin only, `role='doctor'` only. Tests: a doctor gets 403; an attempt to create an
admin is rejected by the service-path policy and surfaced clearly rather than as a
`500`.

> [!deviation] 2026-08-16 — what this step decided beyond its two sentences, and why
> The step conforms: only an admin may ask, the route writes `doctor` and nothing else, a doctor is answered `403`, and an attempt to write an administrator is refused by 000006's policy and surfaced as a Russian `403` rather than a `500`. Eight things had to be decided inside it.
> 1. **The route invites, so it writes an `invites` row and an `invite.send` audit row — the acceptance criterion gives the staff transaction `provider.create` alone.** A doctor with no account at the identity provider can never sign in, and an invitation is the only producer of one; step-8's cycle begins «an admin creates a doctor». So this is the second invite route and it takes the whole spine of step-5 — the advisory lock on the folded address, the lookup, the invitation or the claim outside a transaction, then the record of the invitation in its own transaction and the person in the next. The criterion's letter is not met; its subject — which rows are written where, and what a rollback removes — is.
> 2. **The rows a staff creation signs carry no `patient_id`.** `invite.send` and `account.delete` are the same act for either person and are written by the shared spine, so a `creation` value now travels through it saying whether the person being created is a patient, and `writeAudit` takes the column as a parameter. Without it a doctor's identifier lands in the column a patient's trail is read by, and `audit_log` carries no foreign key at all — so nothing fails, the row simply reads as one about a patient. Pinned by mutation: removing the guard fails `TestTheRowsThatSignAStaffCreationNameTheAdminAndNoPatient/invite.send`, and removing `forPatient` from the patient route fails `TestBothRowsOfACreationAreSignedByTheDoctor`.
> 3. **The role travels as data to the INSERT rather than as a literal inside it.** The closed set is migration 000006's `WITH CHECK`, and a literal in the statement would leave that policy's refusal unreachable from this package — untestable, and unanswerable as anything but a 500. The handler passes a constant; the database stays the authority. `TestTheServicePathRefusesToWriteAnAdmin` and its control `TestTheServicePathWritesADoctor` differ in that one value and in nothing else.
> 4. **Which `42501` was heard is read out of Postgres' message, and that is a named weakness.** A missing grant on the table, a missing grant on a column and a row no policy would admit share one SQLSTATE; the audit row's own actor policy raises the third of them on this same transaction, which is why the reading also requires the message to name `"profiles"`. The message was measured against the live container: `new row violates row-level security policy for table "profiles"`. Nothing in this repository pins the server's `lc_messages`, so on a cluster initialised with a translated locale the reading falls through and the refusal becomes the 500 this step exists to remove. Recorded rather than fixed — pinning it is a deployment decision.
> 5. **A doctor gets no `user_preferences` row.** No policy on that table admits `cadence_doctor` — 000004 gives it the patient, the admin and the service path, and its header says a doctor has no business reading how somebody set up their reminders — so a row written for staff would be a row the person it describes may not read. Whatever adds notifications for doctors inherits the absence.
> 6. **`clinic_name` is not on the wire and stays NULL.** With one clinic there is no name that distinguishes anybody, and a field the dashboard would have to invent a value for is worse than an absent one. Not the multi-clinic anchor, whatever the column's own migration comment suggests: §03's escape hatch names a future `provider_profiles.clinic_id`, and `clinic_name` is a column it lists with nothing hanging off it.
> 7. **The `409` says «человек», not «пациент», and the shared refusals are deferred one at a time.** `refusalFor`'s own default names the act for the log, and an unrecognised failure on this route logged as «creating a patient» sends the next engineer to the other handler.
> 8. **`identity.InviteLimit` still has no caller, and this was the last route that could have taken it.** Step-5 deferred it to here in as many words. It is not taken here either: it is in neither this step's paragraph nor its acceptance list, and wiring it would add a `429` to the contract that nothing asked for. What has changed is that there is nobody left inside this spec to defer to, so the criterion «the invite limit is imposed on our side, per doctor and per window» is **unsatisfied and now owned by a later block** rather than by a step of this one.
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

## Открытые вопросы

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
