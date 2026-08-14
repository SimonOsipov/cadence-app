#!/usr/bin/env bash
# Probes a deployed harness by its public name and requires that no route there
# resolves to provisioner.
#
# "Only the first service in the manifest is proxied" is a routing property of
# the platform, not a property of this repository, and prose is not accepted for
# it: the component that holds the GoTrue admin key must be unreachable from the
# public internet, and the only way to know is to ask the public name.
#
# It is not part of scripts/gate/: a gate runs on every machine and in CI, and
# there is nothing to probe until a deployment exists. Run it by hand after a
# deploy, and after any change to the manifest's service order.
#
#   PROVISIONER_SHARED_SECRET=… scripts/probe/provisioner-is-not-proxied.sh https://cadence-api.example.ru
#
# The secret is carried because a probe without one would be answered by the
# guard with a 401 — and a 401 is also what the API answers a request with no
# bearer token, so the probe would pass identically against a provisioner that
# *is* proxied. It comes from the environment rather than from an argument:
# argv is visible in `ps` to every account on the machine, and lands in shell
# history besides.
set -euo pipefail

BASE=${1:-}
SECRET=${PROVISIONER_SHARED_SECRET:-}

if [ -z "$BASE" ] || [ -z "$SECRET" ]; then
    echo "usage: PROVISIONER_SHARED_SECRET=<a current secret> $0 <public base url>" >&2
    exit 2
fi

BASE=${BASE%/}

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

# The control, first. Without it a typo in the hostname — or a deployment that
# is simply down — produces a refusal on every path below and a green probe that
# measured nothing.
echo "==> the public name serves the API"
health=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 15 "$BASE/healthz") \
    || fail "$BASE is not answering at all; this probe would otherwise pass by measuring nothing"
[ "$health" = "200" ] || fail "$BASE/healthz answered $health — that is not the API, so nothing below is a measurement"

# Every operation provisioner mounts, including the one that exists only outside
# production. The list is duplicated from cmd/provisioner/routes.go on purpose:
# this file has to disagree with the code the moment an operation is added, and
# a route added without being probed is a route reachable without being checked.
for path in /invite /users/lookup /users/lookup-batch /users/delete /users/password; do
    echo "==> $path does not reach provisioner"

    # A body every operation refuses at validation, on purpose. The one run of
    # this script that matters is the one that fails, and a payload that a
    # reachable provisioner would act on would create an account and send a
    # patient an invitation on the way to reporting that it should not have
    # been able to. Reachability is visible in the 400 either way.
    answer=$(curl -sS --max-time 15 \
        -X POST "$BASE$path" \
        -H 'Content-Type: application/json' \
        -H "X-Cadence-Provisioner-Secret: $SECRET" \
        -d '{"email":"not-an-address","ids":["not-an-identifier"],"id":"not-an-identifier"}' \
        -w '\n%{http_code}')

    status=$(printf '%s' "$answer" | tail -n1)
    body=$(printf '%s' "$answer" | sed '$d')

    # A provisioner that answered would say so in one of three ways: an account
    # document, its own error shape, or a 204 from a write. The API in front
    # answers 401 or 404 for all of these paths, and its body is a problem
    # document.
    case "$status" in
        200 | 204 | 400 | 409 | 502)
            fail "POST $path answered $status — provisioner is reachable from the public name: $body"
            ;;
    esac

    case "$body" in
        *'"account"'* | *'"accounts"'*)
            fail "POST $path answered with an account document — provisioner is reachable from the public name"
            ;;
    esac

    echo "    $status, and no provisioner answer in the body"
done

echo "probe: green — no route on $BASE resolves to provisioner"
