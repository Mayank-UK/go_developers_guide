#!/usr/bin/env bash
# Coverage gate for the guide server — the Go counterpart of the spring
# guide's JaCoCo bundle floor (this family has no hosted CI: local
# verification is the only gate). It runs the REAL suite with atomic
# coverage instrumentation, prints the per-package statement-coverage
# table, and fails when TOTAL statement coverage falls below THRESHOLD.
# The floor carries >= 2 percentage points of headroom under the measured
# baseline; tools/coverage/THRESHOLD.md records the numbers, the
# rationale, and the rule for moving the floor.
#
# Run it INSTEAD of the plain suite run whenever you want coverage
# enforced (it is the same suite, instrumented):
#   docker exec go_developers_guide_dev bash -lc \
#     'cd /workspaces/go_developers_guide/server && tools/coverage/check.sh'
set -euo pipefail
cd "$(dirname "$0")/../.."

# Total statement coverage floor, percent. Baseline measured 2026-10-28:
# see THRESHOLD.md beside this script. Every package counts at its real
# number — generated code and the untested composition root included,
# nothing is excluded. Raise the floor when coverage rises; never lower
# it to land a change.
readonly THRESHOLD=74.0

profile="$(mktemp /tmp/gdg-cover.XXXXXX)"
trap 'rm -f "$profile"' EXIT

echo "== go test ./... -count=1 -race -covermode=atomic (coverage gate) =="
go test ./... -count=1 -race -covermode=atomic -coverprofile="$profile"

echo
echo "== per-package statement coverage (atomic) =="
awk 'NR>1 {path=$1; sub(/:.*/, "", path); sub(/\/[^\/]*$/, "", path);
          total[path]+=$2; if ($3>0) covered[path]+=$2}
     END {for (k in total)
            printf "%7.1f%%  %6d/%-6d %s\n", 100*covered[k]/total[k], covered[k], total[k], k}' \
    "$profile" | sort -rn

total="$(go tool cover -func="$profile" | awk '/^total:/ {sub(/%/, "", $NF); print $NF}')"
echo
awk -v t="$total" -v th="$THRESHOLD" 'BEGIN {
    if (t+0 < th+0) {
        printf "coverage gate FAILED: total %s%% is below the %.1f%% floor — add tests or see tools/coverage/THRESHOLD.md\n", t, th
        exit 1
    }
    printf "coverage gate PASSED: total %s%% >= floor %.1f%% (headroom %.1f pp)\n", t, th, t-th
}'
