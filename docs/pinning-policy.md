# Pinning policy — why some pins are exact and others are tags

This repo intentionally runs a TWO-TIER pinning policy across its Docker
images and tool installs. The split is a decision, not an oversight: an
exact pin and a floating tag each buy something different, and each tier
below states what it buys and what it costs.

## Tier 1 — exact pins (where builds must reproduce)

Anything whose OUTPUT feeds the repo, or whose absence breaks a documented
gate, is pinned to an exact version:

| What | Pin | Where | Why exact |
| --- | --- | --- | --- |
| Go toolchain | `golang:1.27.3-bookworm` | `.docker/Dockerfile.dev`, `.docker/server/Dockerfile.prod` | The toolchain builds the binary the curriculum's numbers (race verdicts, coverage floor, module snapshot) are pinned against — a silent minor bump re-baselines all of them. |
| npm | `npm@11` (installed over Node 22's bundled 10.9.9) | `.docker/Dockerfile.dev`, `.docker/client/Dockerfile{,.prod}` | Node 22 ships npm 10.9.9, which crashes in `@npmcli/arborist` on this client's lockfile regeneration (family trap, documented in those Dockerfiles); npm ≥ 11 is mandatory, and installing one known-good major keeps dev and prod installs identical. |
| buf | `v1.75.0` | `.devcontainer/devcontainer.json` (`postCreateCommand`) | The .proto compiler host — codegen output rides the container rebuild, so a newer buf silently changes generated code. |
| protoc-gen-go / protoc-gen-go-grpc | `v1.36.12` / `v1.6.2` | same `postCreateCommand`; also `tool` directives in `server/go.mod` | The generated Go/gRPC bindings must match what the module pins; the plugin's own tag is the module's. |
| staticcheck | `v0.8.2` | `server/staticcheck.conf`, README verification section | The lint gate's findings change between releases; an unpinned run is a different gate. |
| oapi-codegen, gqlgen | `tool` directives in `server/go.mod` | `server/go.mod` (run via `go tool`) | Same argument as buf: generated code must reproduce; the module lockfile pins them with the rest of the dependencies. |

The rule of thumb: **if a version bump produces a diff in this repository
(generated code, lint findings, measured numbers) or fails a documented
gate, the pin is exact.**

## Tier 2 — tag-level pins (mutable service bases)

Backing services and edge proxies run on minor-version tags:

- `postgres:17`, `redis:7-alpine`, `mongo:8` — `.docker/docker-compose.yml`
  and `deploy/docker-compose.prod.yml`
- `node:22-bookworm` — the client dev/prod builders' base (npm itself is
  Tier 1 on top of it)
- `nginx:1.27-alpine` — the prod client edge
- `gcr.io/distroless/static-debian12` — the prod server runtime

These images are RUNNING SERVICES (or one-shot builders whose output is
then tested), not inputs to repo contents. The minor tag tracks patch
fixes — including security patches — without a repo-side change, and the
suites test the SERVICE's wire behavior (pgx against real PostgreSQL,
healthchecks, probes), not the image's bytes. Exact-pinning them would
trade free security updates for reproducibility the tests already provide
by behavior.

## Tightening to digest pins

When an environment requires bit-reproducible infrastructure (audited
deployments, air-gapped registries), tighten any Tier 2 entry to a digest:

```bash
docker manifest inspect postgres:17   # read the digest
# then pin: postgres:17@sha256:<digest> in both compose files
```

The tag stays as human-readable documentation of the intended version;
the digest becomes what is actually pulled. The cost is deliberate: patch
and security updates no longer arrive silently — refresh each digest on a
schedule you own. A reasonable middle ground for this repo's stance:
digest-pin the PROD compose (`deploy/docker-compose.prod.yml`) and leave
the dev compose tag-level, so local development keeps free updates while
production is bit-reproducible.
