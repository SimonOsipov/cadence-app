#!/usr/bin/env bash
# Ralph — the autonomous loop. Each iteration hands the agent the same prompt;
# the agent picks one story out of prd.json, implements it with TDD, runs the
# gate and commits. The loop stops when the agent emits <promise>COMPLETE</promise>
# or when it runs out of iterations.
#
# Usage: scripts/ralph/ralph.sh [max_iterations]   (default 10)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PRD_FILE="$SCRIPT_DIR/prd.json"
PROGRESS_FILE="$SCRIPT_DIR/progress.txt"
# prompt.md, not CLAUDE.md: the repository's .gitignore keeps every CLAUDE.md
# local, and this prompt has to be checked in.
PROMPT_FILE="$SCRIPT_DIR/prompt.md"
ARCHIVE_DIR="$SCRIPT_DIR/archive"
LAST_BRANCH_FILE="$SCRIPT_DIR/.last-branch"

MAX_ITERATIONS=${1:-10}
# Validated rather than trusted: an unchecked value reaches a loop that spends
# real money on an autonomous agent.
if ! [[ "$MAX_ITERATIONS" =~ ^[1-9][0-9]*$ ]]; then
    echo "max_iterations must be a positive integer, got '$MAX_ITERATIONS'" >&2
    exit 1
fi

for tool in jq git claude; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "$tool is not installed — the loop needs it" >&2
        exit 1
    fi
done

if [ ! -f "$PRD_FILE" ]; then
    echo "no prd.json — generate one from an approved spec with /ralph-prd first" >&2
    exit 1
fi
if [ ! -f "$PROMPT_FILE" ]; then
    echo "no prompt.md next to this script" >&2
    exit 1
fi

declared_branch=$(jq -r '.branchName // empty' "$PRD_FILE")
if [ -z "$declared_branch" ]; then
    echo "prd.json has no branchName" >&2
    exit 1
fi

# Two independent checks, because each one alone has been enough to let an
# autonomous loop commit onto the wrong branch: what prd.json asks for, and
# what the working tree is actually on. The prefix is required on both — naming
# only `main` as forbidden would still allow master, develop or a feature branch.
require_ralph_branch() {
    case "$1" in
        ralph/?*) return 0 ;;
        *)
            echo "refusing: '$1' is not a ralph/* branch — Ralph commits only there" >&2
            return 1
            ;;
    esac
}

require_ralph_branch "$declared_branch"

actual_branch=$(git -C "$REPO_ROOT" rev-parse --abbrev-ref HEAD)
require_ralph_branch "$actual_branch"

if [ "$actual_branch" != "$declared_branch" ]; then
    echo "refusing: checked out '$actual_branch' but prd.json declares '$declared_branch'" >&2
    exit 1
fi

# The agent is told to commit its story's changes. Anything already dirty here
# would be swept into that commit without anyone deciding to — untracked strays
# included, which is exactly what the loop's `git add` would pick up. The loop's
# own state is gitignored, so a clean tree really is clean.
if [ -n "$(git -C "$REPO_ROOT" status --porcelain)" ]; then
    echo "refusing: the working tree has uncommitted changes — commit or stash them first" >&2
    git -C "$REPO_ROOT" --no-pager status --short >&2
    exit 1
fi

# A new branch means a new run: keep the previous run's record instead of
# letting it blend into this one.
if [ -f "$LAST_BRANCH_FILE" ]; then
    last=$(cat "$LAST_BRANCH_FILE")
    if [ "$last" != "$declared_branch" ]; then
        stamp="$(date +%Y-%m-%d)-${last#ralph/}"
        mkdir -p "$ARCHIVE_DIR/$stamp"
        cp -f "$PRD_FILE" "$ARCHIVE_DIR/$stamp/prd.json"
        # mv, not cp-then-rm: on a failed copy the previous run's log — the only
        # record of a blocked run — must survive rather than be deleted.
        [ -f "$PROGRESS_FILE" ] && mv "$PROGRESS_FILE" "$ARCHIVE_DIR/$stamp/progress.txt"
        echo "archived the previous run to $ARCHIVE_DIR/$stamp"
    fi
fi
echo "$declared_branch" > "$LAST_BRANCH_FILE"

if [ ! -f "$PROGRESS_FILE" ]; then
    {
        echo "# Ralph progress log"
        echo "Branch: $declared_branch"
        echo "Started: $(date)"
        echo "---"
    } > "$PROGRESS_FILE"
fi

open_stories=$(jq '[.stories[] | select(.passes == false)] | length' "$PRD_FILE")
echo "Ralph: branch $declared_branch, $open_stories open stories, up to $MAX_ITERATIONS iterations"

for ((i = 1; i <= MAX_ITERATIONS; i++)); do
    echo
    echo "=============================================================="
    echo "  iteration $i / $MAX_ITERATIONS"
    echo "=============================================================="

    # A crashed or missing agent must not read as an unproductive iteration.
    # `pipefail` from the top of the script makes the assignment carry the
    # agent's non-zero status even though `tee` succeeds — PIPESTATUS would not:
    # a command substitution is one command, so it holds a single element, and
    # that element is the pipeline's status, not the agent's.
    set +e
    output=$(claude --dangerously-skip-permissions --print < "$PROMPT_FILE" 2>&1 | tee /dev/stderr)
    agent_status=$?
    set -e

    if [ "$agent_status" -ne 0 ]; then
        echo "the agent exited with status $agent_status — stopping instead of burning iterations" >&2
        exit "$agent_status"
    fi

    # The agent may have checked out something else mid-run.
    actual_branch=$(git -C "$REPO_ROOT" rev-parse --abbrev-ref HEAD)
    if [ "$actual_branch" != "$declared_branch" ]; then
        echo "stopping: the working tree moved to '$actual_branch', expected '$declared_branch'" >&2
        exit 1
    fi

    if echo "$output" | grep -q "<promise>COMPLETE</promise>"; then
        echo
        echo "Ralph finished every story on iteration $i."
        exit 0
    fi

    sleep 2
done

echo
echo "Ralph hit the iteration limit with stories still open. See $PROGRESS_FILE."
exit 1
