# Protecting the `main` branch

`main.json` is the branch protection configuration. It lives in the repository
rather than in GitHub's settings for the same reason the migration chain does:
a change to it shows up in a diff and gets discussed, instead of just happening.

**Only the repository owner can apply it.** Repositories under a personal GitHub
account have exactly two access levels — owner, and collaborator with write
access; the `admin` and `maintain` roles exist only on organization
repositories. That is a platform limitation, not a setting somebody forgot to
turn on.

---

## Order. It matters

1. **This branch merges into `main`.** Until it does, `main.json` does not exist
   on `main`, and the command below — run from a fresh clone — will not find it.
2. **CI runs at least once, and the check names are reconciled against the real
   ones.** The seven names in `main.json` were derived from `ci.yml`, but no
   check has ever run on this repository, which means the names were *derived,
   not observed*. A required check whose name GitHub has never reported stays
   `Pending` forever and blocks every pull request — so the reconciliation is
   mandatory, and it happens before the command is handed to the owner:

   ```bash
   gh run list --limit 1 --json databaseId --jq '.[0].databaseId' \
     | xargs -I{} gh api repos/SimonOsipov/cadence-app/actions/runs/{}/jobs \
         --jq '.jobs[].name' | sort
   # the output must match:
   jq -r '.rules[] | select(.type=="required_status_checks")
          | .parameters.required_status_checks[].context' \
     .github/rulesets/main.json | sort
   ```
3. Only then — the command for the owner.

## What the owner has to do — one command

```bash
gh api --method POST repos/SimonOsipov/cadence-app/rulesets \
  --input .github/rulesets/main.json
```

From the root of a cloned repository. Requires [`gh`](https://cli.github.com/),
authenticated as the owner account.

## Acceptance is ours, and it is not "applied" but "works"

`gh api .../rulesets --jq '.[].name'` prints `main` for a rule in `evaluate`
mode, for a rule with a non-empty `bypass_actors`, and for one with a stale
check list — that is, for all three states the local script was written against.
So behaviour is what gets verified, not presence:

```bash
# 1. a direct push to main must be rejected
git push origin HEAD:main            # expect the server to refuse

# 2. merging with a red check must be rejected
#    (a branch with a deliberately failing gate, a PR, an attempt to merge)
```

`write` access is enough for both checks — which is precisely why acceptance
stayed with us even though applying the rule went to the owner.

## What this turns on

- **Direct pushes to `main` stop working.** Changes arrive only through a pull
  request. That includes the owner — there are no exceptions, and that is not an
  oversight: a gate with an exception protects against everyone except the
  person most likely to be in a hurry.
- **Merging requires green checks.** Seven CI checks; the full list is in
  `main.json`. The branch must be up to date with `main`.
- **`main`'s history cannot be rewritten, and the branch cannot be deleted.**
- **No approval is required.** There is one developer, and a rule nobody can
  satisfy is a rule people start routing around. A pull request and green CI are
  mandatory; a second person's signature is not.

## What changes in day-to-day habits

- Everything that used to be a `git push` to `main` is now a branch and a pull
  request. You can merge your own PR as soon as CI is green.
- **Every conversation in a PR must be marked resolved before merging.** This
  holds regardless of approvals not being required: an unanswered comment blocks
  the button.
- The branch must be up to date with `main` — if `main` has moved ahead, the PR
  has to be updated before it can merge.

## How to roll back

```bash
# find the id
gh api repos/SimonOsipov/cadence-app/rulesets --jq '.[] | "\(.id) \(.name)"'
# delete
gh api --method DELETE repos/SimonOsipov/cadence-app/rulesets/<id>
```

Or weaken it temporarily without deleting: in `main.json`, replace
`"enforcement": "active"` with `"evaluate"` and update the rule via
`--method PUT .../rulesets/<id>`. In `evaluate` mode violations are recorded but
nothing is blocked.

---

## Why the check list must not be edited by hand

`scripts/gate/ruleset.sh` reconciles `main.json` against
`.github/workflows/ci.yml` and fails when they diverge. The check is part of the
shared gate and of CI, and it catches two mistakes, each of them quiet and
expensive:

- **A required check matching no job will never report.** GitHub leaves it
  `Pending`, and no pull request merges again — because of a typo in a name.
- **A job that runs but is not required gates nothing.** The branch looks
  protected and is not, which is worse than being openly unprotected.

Recorded as deliberately not done: the required checks have no `integration_id`
pinned, so a check counts as satisfied by name from any source, not only from
GitHub Actions. Pinning requires a magic number we have not verified against a
live run — and this entire page exists so the owner is never handed a command
with an unverified claim inside it. Revisit after the first name reconciliation
in item 2.

Separately, the script verifies that `on:` in `ci.yml` carries no path filter. A
job skipped by `if:` reports as successful and does not block the merge; a
workflow skipped by a path filter does not report at all — and every required
check hangs. That is why path filtering in this repository lives inside, in the
`changes` job, and must stay there.
