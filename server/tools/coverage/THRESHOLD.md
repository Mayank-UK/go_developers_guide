# Coverage floor — decision record (2026-10-28)

## The gate

`check.sh` beside this file is the guide server's coverage gate. It runs the
real suite (`go test ./... -count=1 -race -covermode=atomic -coverprofile`),
prints the per-package statement-coverage table, and fails when TOTAL
statement coverage drops below the floor baked into the script (`THRESHOLD`).
This repo has no hosted CI (family stance — local verification is the only
gate), so the gate is a documented local command, run INSTEAD of the plain
suite run whenever coverage matters:

```bash
docker exec go_developers_guide_dev bash -lc \
  'cd /workspaces/go_developers_guide/server && tools/coverage/check.sh'
```

It is deliberately a script, not a test: a Go test cannot produce a coverage
profile for the tree it runs in (profiles don't exist in clean checkouts), and
spawning `go test` from inside a test would double-run the suite. The gate's
fail mode was negative-tested (a sub-floor total exits 1 with the message
above).

## Family precedent

- **spring_boot_developers_guide** gates JaCoCo bundle LINE coverage at
  `mvn verify` — measured 88.4%, bundle floor 0.85 plus package-level floors
  with documented thin-package exceptions, nothing excluded. Closest analog:
  a committed floor measured on the real build.
- **express_developers_guide** is coverage-CI-gated (86/77/87/87); the
  **fastapi** guide deferred coverage gates entirely.

This guide takes spring's shape (one committed floor, measured on the real
suite, nothing excluded) in the Go-idiomatic minimal form: one script, one
total floor, no per-package floors yet — the per-package table is printed for
humans, only the total gates.

## Numbers at adoption (2026-10-28, full `-race` atomic run, 34 packages)

- **TOTAL statement coverage: 76.2%** (10,975 / 14,406 statements).
- Everything counts at its real number, nothing excluded:
  - `guide/cmd/guide` 0.0% (231 stmts) — the composition root; exercised live
    by the running stack and smoke curls, not by unit tests.
  - Generated code at 0.0% — `graph/generated` (1,307) and `grpcapi/pb`
    (250); together with `cmd/guide` that is ~12.4% of all statements, so the
    floor is deliberately conservative relative to hand-written coverage.
  - `data/obs` 0.0% (6 stmts), `apispec` 39.5% (generated OpenAPI types plus
    the spec-drift pin) — the honest thin spots.
- Strongest real packages: `domain/authorization` and `domain/jobs` 100%,
  `domain/realtime` 96.8%, `domain/patterns` 95.4%, `domain/i18n` 94.4%,
  `domain/demos` 92.9% (4,285 stmts — the largest package), `content` 90.2%.
- First-suite data-adapter numbers: `data/posts` 87.8%, `data/auth` 80.9%,
  `data/mongodb` 77.1%, `data/db` 76.8%, `data/redis` 77.2%.

## The floor

`THRESHOLD = 74.0` in `check.sh` — **2.2 percentage points of headroom**
under the measured 76.2% baseline. Run-to-run jitter from timing-dependent
paths is well under 1 pp. Note what the headroom absorbs honestly: if a real
dependency degrades mid-run (the dev stack's MongoDB refusing index builds on
a full Docker VM disk did exactly that on 2026-10-28), the suite itself goes
red AND coverage drops — the gate fails for the right reason.

Moving the floor: raise it when coverage rises (one-line change in
`check.sh` plus a note here); never lower it to land a change. A change that
must trade coverage away is a deliberate decision, recorded here.
