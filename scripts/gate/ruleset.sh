#!/usr/bin/env bash
# Checks the committed branch-protection ruleset against the workflow it is
# supposed to protect. It does NOT talk to GitHub: this runs on every machine
# and in CI, where nobody has the admin token, and it answers the only question
# that can be answered offline — is the configuration self-consistent.
#
# The two failures it exists to catch are both silent and both severe.
#
# A required check whose name matches no job never reports. GitHub leaves it
# Pending forever and every pull request becomes unmergeable — by a typo.
#
# A job that runs but is not required gates nothing. The branch looks protected
# and is not, which is worse than being visibly unprotected.
set -euo pipefail

cd "$(dirname "$0")/../.."

WORKFLOW=.github/workflows/ci.yml
RULESET=.github/rulesets/main.json
CODEOWNERS=.github/CODEOWNERS

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

echo "==> files present"
for f in "$WORKFLOW" "$RULESET" "$CODEOWNERS"; do
    [ -f "$f" ] || fail "$f is missing"
done

echo "==> ruleset is valid JSON"
python3 -c "import json,sys; json.load(open('$RULESET'))" \
    || fail "$RULESET is not valid JSON — the partner would paste a broken payload"

# Everything below is one Python pass: parsing the same two files repeatedly in
# shell would be the third place that has to agree about their shape.
echo "==> ruleset agrees with the workflow"
python3 - "$WORKFLOW" "$RULESET" <<'PY'
import json
import re
import sys

workflow_path, ruleset_path = sys.argv[1], sys.argv[2]
lines = open(workflow_path, encoding="utf-8").read().splitlines()
problems = []

# A deliberately small YAML reader. PyYAML is not guaranteed on a runner and a
# dependency for one file is a dependency to keep working; the shape it relies
# on is asserted below, so a workflow that stops matching fails loudly rather
# than being read wrongly.
def block(pattern):
    """Lines belonging to a top-level block, without its header.

    Blank lines and full-line comments do not end the block. They used to: a
    comment at column zero looks exactly like the next top-level key to a
    reader that stops at the first non-indented character, and in a repository
    where every file is commented that is a one-line edit away at all times.
    """
    out, inside = [], False
    for line in lines:
        if not inside:
            if re.match(pattern, line):
                inside = True
            continue
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        if not line[0].isspace():
            break
        out.append(line)
    return out

# 1. A path filter on the trigger makes every required check hang forever.
#    Filtering inside, per job, is what this workflow does and must keep doing:
#    a job skipped by `if:` reports success, a workflow skipped by `paths:`
#    reports nothing at all.
#
#    `on` is matched with optional quotes because YAML 1.1 reads a bare `on` as
#    a boolean, so `"on":` is a legitimate — and yamllint-recommended — spelling.
#    Flow style (`on: {…}`) does not match the pattern at all, which is why the
#    block is asserted non-empty rather than trusted: a reader that silently
#    returns nothing would turn this whole check into a no-op, which is the
#    exact failure mode the check exists to prevent elsewhere.
trigger = block(r'^"?on"?:\s*$')
if not trigger or not any(re.match(r"\s+pull_request:", line) for line in trigger):
    problems.append(
        "could not read the `on:` trigger of "
        f"{workflow_path} — it is written in a shape this script does not "
        "parse (flow style, or renamed). The paths-filter check below is "
        "therefore not running at all. Fix the reader before trusting a green "
        "run; a silent no-op here is worse than a failure."
    )
for line in trigger:
    if re.match(r"\s+paths(-ignore)?:", line):
        problems.append(
            "the `on:` trigger has a paths filter. A workflow skipped by path "
            "filtering never reports, so every required check below would sit "
            "Pending and no pull request could ever merge. Filter inside the "
            "jobs with `if:` instead — a skipped job reports success."
        )

# 2. Job display names: what GitHub reports, and therefore what a ruleset names.
jobs = {}
job_id = None
for line in block(r"^jobs:\s*$"):
    if re.match(r"^  [A-Za-z0-9_-]+:\s*$", line):
        job_id = line.strip().rstrip(":")
        jobs[job_id] = job_id  # a job without `name:` reports its id
    elif job_id and (m := re.match(r"^    name:\s*(.+?)\s*$", line)):
        jobs[job_id] = m.group(1).strip("'\"")
    elif job_id and re.match(r"^    (strategy|matrix):", line):
        # A matrix job reports one check run per combination, named
        # "Display name (value, value)". The bare name in the ruleset would
        # then match nothing and block every pull request — the same permanent
        # deadlock a typo causes, from an edit that looks entirely routine.
        problems.append(
            f"job {job_id!r} uses a matrix. GitHub names a matrix job's check "
            "runs per combination, so the bare context in the ruleset would "
            "never report. Either drop the matrix or list every generated "
            "name — and teach this script how you did it."
        )

if len(jobs) < 3:
    problems.append(
        f"only {len(jobs)} jobs parsed out of {workflow_path} — the reader in "
        "this script no longer matches the workflow's shape, so every check "
        "below is meaningless. Fix the reader before trusting a green run."
    )

ruleset = json.load(open(ruleset_path, encoding="utf-8"))
rules = {r["type"]: r for r in ruleset.get("rules", [])}

checks = rules.get("required_status_checks", {}).get("parameters", {}).get("required_status_checks", [])
required = {c["context"] for c in checks}
expected = set(jobs.values())

# 3. Both directions, because they fail differently and both fail silently.
for missing in sorted(expected - required):
    problems.append(
        f"job {missing!r} runs in CI but is not required by the ruleset — it "
        "gates nothing, and the branch only looks protected"
    )
for unknown in sorted(required - expected):
    problems.append(
        f"the ruleset requires {unknown!r}, which matches no job in "
        f"{workflow_path}. That check will never report, and every pull "
        "request will be blocked forever"
    )

# 4. The rules that make "protected" mean what it says.
if ruleset.get("enforcement") != "active":
    problems.append("enforcement is not `active` — the ruleset is documentation, not a gate")
if "pull_request" not in rules:
    problems.append("no pull_request rule — a direct push to main would still be allowed")
if "non_fast_forward" not in rules:
    problems.append("no non_fast_forward rule — history on main could be rewritten")
if "deletion" not in rules:
    problems.append("no deletion rule — main could be deleted outright")
if ruleset.get("bypass_actors"):
    problems.append(
        "bypass_actors is not empty — somebody is exempt from the gate, which "
        "is the one thing a gate cannot afford"
    )

if problems:
    print("the committed ruleset and the workflow disagree:\n", file=sys.stderr)
    for p in problems:
        print(f"  - {p}\n", file=sys.stderr)
    sys.exit(1)

print(f"    {len(required)} required checks, all matching jobs in {workflow_path}:")
for name in sorted(required):
    print(f"      {name}")
PY

echo "ruleset gate: green"
