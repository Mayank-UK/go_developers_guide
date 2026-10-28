# Go Developers Guide

A runnable full-stack reference app that **is** the guide. The server (Go 1.27, standard library `net/http`, module `guide`) carries 100% of the teaching as a typed content API — 43 topics in 10 groups, 373 sections, 62 inline live demos — and the client (React 19 + Vite + TypeScript + Tailwind v4) is a presentation-only shell that renders it. Nothing is a screenshot or a promise: every handler, query, worker, and failure path the text teaches is real code in this repository, and every demo panel exercises it live against the running stack.

Fourth sibling of the guide family, after `express_developers_guide` (the canon origin — content model, devcontainer workflow, snippet discipline), `spring_boot_developers_guide`, and `fastapi_developers_guide` (the newest family reference). The full curriculum is authored and live: the server suite (build + vet + tests with `-race`, gofmt clean, snippet ratchet at zero) and the client suite (vitest + production build) are the gate, run in Docker per the verification protocol below.

## The curriculum at a glance

Ten groups, 43 topics, in curriculum order (titles as the API serves them):

- **Foundations** — The Go language · The Go runtime · HTTP and the web
- **Framework** — HTTP fundamentals with net/http · Interfaces and composition · Goroutines and channels
- **Request handling** — Routing · Middleware · Request and response bodies · Validation
- **APIs** — API design · API documentation · GraphQL · Webhooks · gRPC
- **Data** — SQL schema design · Databases and SQL from Go · sqlc and GORM · MongoDB from Go · Redis from Go
- **State & auth** — Caching · Messaging from Go · Authentication from Go · Authorization from Go · Security from Go
- **Media & realtime** — Files and media from Go · Internationalization from Go · Realtime: SSE and WebSockets · Background jobs
- **Platform** — Error handling · Dependency management · Architecture · Observability · Testing · Debugging · Environments
- **Ship** — Deployment · Performance · Reliability · Scaling
- **Capstones** — Best practices · Third-party services · Advanced architecture

The last three are the family canon carried to Go; three topics are Go-specific additions with no sibling equivalent — *interfaces-and-composition*, *goroutines-and-channels*, and the *sqlc and GORM* comparison. Each topic is concept-first prose grounded in this repo's own code, hosts its demos inline beside the sections that teach them (never queued at the page end), and closes with a curl-able terminal companion.

## Architecture

**Server** ([`server/`](server), Go module `guide`), five layers with one dependency rule:

- [`cmd/guide/`](server/cmd/guide) — the composition root: `main.go` wires every port (interface) to its adapter via constructor injection; no DI framework, no service locator
- [`internal/core/`](server/internal/core) — env config, the error taxonomy behind one error envelope, slog JSON logging with request-id correlation, the runtime `ScenarioStore`
- [`internal/content/`](server/internal/content) — the teaching as data: a typed content model (topic → sections → blocks of `text | code | demo | list`) and a registry that validates every topic at construction, so broken content fails the process at startup, never at request time
- [`internal/domain/<area>/`](server/internal/domain) — entities, ports (interfaces), and use cases per teaching area; the domain imports neither `net/http` nor any adapter; `patterns/` holds the real event store and saga runner the capstone teaches
- [`internal/data/<area>/`](server/internal/data) — the adapters: pgx v5 + goose migrations, go-redis, mongo-driver, files, messaging; DTOs never leak past a repository implementation
- [`internal/presentation/`](server/internal/presentation) — thin handlers (parse input → call one use case → map the result), middleware, and the WebSocket/SSE edges, plus the GraphQL and gRPC frontends ([`graph/`](server/internal/graph), [`grpcapi/`](server/internal/grpcapi)) served from the same use cases

The dependency rule is `presentation → domain ← data`: handlers never import the data layer, adapters implement domain ports, and only `main.go` knows which adapter answered. The same posts use cases serve REST, GraphQL, and gRPC precisely because no edge can reach past the port. Stack choices are deliberate and compared, not defaulted: `net/http` with Go 1.22+ `ServeMux` patterns as the backbone (chi/gin/echo taught by comparison only), slog stdlib, stdlib `testing` + `httptest` + testcontainers-go, PostgreSQL-only via pgx v5 — sqlc and GORM both appear, measured against each other.

**Client** ([`client/src/`](client/src)) mirrors the layering — `core/` (fetch, guards, typed DTOs), `domain/` (view models, the shared scenario slice), `data/` (the content API adapter), `ui/` (the design system, views, and a lazy-loaded registry of the 62 demo panels). It renders whatever the content API serves and holds no teaching of its own; dark theme is the default.

**Wire contract** (stable, test-pinned): `GET /api/content` → `{version: 1, groups, topics}`; `GET /api/content/{topicId}` → the full topic with sections and blocks; `GET/PATCH /api/demo/scenario`; unknown topics get a 404 in the one error envelope `{error: {code, message, requestId, details?}}` that every failure — 405 included — answers with an `x-request-id` header.

## Running it

The standard path is the VS Code devcontainer: open the repo → **Reopen in Container**. That starts the whole stack defined in [`.docker/docker-compose.yml`](.docker/docker-compose.yml): the `dev` Go toolchain container, the `client_dev` Vite container, and PostgreSQL 17 / Redis 7 / MongoDB 8. Image and tool pins follow a deliberate two-tier policy — exact where builds must reproduce, tag-level for mutable service bases ([`docs/pinning-policy.md`](docs/pinning-policy.md)). Without VS Code:

```bash
for v in gdg-go gdg-gocache gdg-npm; do docker volume create "$v"; done   # once, on a fresh machine (shared caches)
docker compose -f .docker/docker-compose.yml up -d
```

Then start the server inside the dev container (it runs there by design — the compose `dev` service is the toolchain, not an app image):

```bash
docker exec -d go_developers_guide_dev \
  bash -lc 'cd /workspaces/go_developers_guide/server && go run ./cmd/guide'
```

What comes up: the client at http://localhost:5175 (Vite proxies `/api` — HTTP and WebSocket — to the server's `:8100` inside the dev container), PostgreSQL on 5433, Redis on 6380, MongoDB on 27018; the HTTP server listens on 8100 and gRPC on 8102 (`GRPC_ADDR`), both inside the dev container. Optional services stay optional — the server boots and serves with any datastore down, and the demos that need one degrade to an honest `service_unavailable` envelope.

Smoke it:

```bash
curl -s http://localhost:5175/api/health        # {"status":"ok","env":"dev"}
curl -s http://localhost:5175/api/content | head -c 400
```

The production stack ([`deploy/docker-compose.prod.yml`](deploy/docker-compose.prod.yml): distroless server image, nginx-served client, the same three datastores) publishes the two front doors directly — API on 8100, client on 8101 — and refuses to boot until the one true secret is provided:

```bash
POSTGRES_PASSWORD=… docker compose -f deploy/docker-compose.prod.yml up -d
```

Kubernetes manifests for the same images live in [`deploy/k8s/`](deploy/k8s).

## Live demos and the scenario toggle

Every demo runs against the real substrate — the same use cases `/api/posts` serves, the same pgx pool, the same asynq worker — never a canned fixture. Demos are named `<area>:<name>` (`db:migrations`, `auth:jwt`, `reliability:breaker`), lazy-loaded by the client, and served inline beside the section that teaches them. Two endpoints make the stack teachable by hand:

```bash
# read the demo state, then flip it: fake data ⇄ live datastore
curl -s http://localhost:5175/api/demo/scenario
curl -s -X PATCH http://localhost:5175/api/demo/scenario \
  -H 'content-type: application/json' -d '{"dataMode":"live"}'
curl -s -X PATCH http://localhost:5175/api/demo/scenario \
  -H 'content-type: application/json' -d '{"injectFailure":"error"}'
```

The `ScenarioPanel` in the client header drives the same switch, and the failure injection (error, latency) feeds the error-handling and reliability demos' live failure paths. Fake mode is deterministic by contract, and every demo that reports a measurement reports what it actually measured — tests pin that performance and scaling demos invent no numbers.

## Snippets: the code is the source of truth

Every code block in a topic whose language demands sourcing (go, mod, yaml, sql, proto, ts, json, …) carries a `source:` path into this repository, and a ratchet test ([`snippet_sources_test.go`](server/internal/content/snippet_sources_test.go)) verifies each quoted snippet is an ordered, comment- and blank-normalized subsequence of the **current** file — so the guide cannot drift from the code it teaches without failing the suite. The ratchet is pinned at zero violations: an unsourced must-source block fails the build. Hand-written teaching exhibits exist only in `internal/domain/teaching/`, where no real counterpart exists.

## Verification

No hosted CI by family stance — local verification against the long-lived dev containers is the only gate (Go module and build caches ride named volumes, so these run warm):

```bash
docker exec go_developers_guide_dev bash -lc 'cd /workspaces/go_developers_guide/server && go build ./... && go vet ./... && go test ./...'
docker exec go_developers_guide_client_dev bash -lc 'cd /app && npm test && npm run build'
cd e2e && npx playwright test            # host-run e2e against the live stack (11 specs)
```

The server suite runs with `-race`, gofmt is silent, real-PG tests spawn sibling containers through the mounted Docker socket (testcontainers-go, no DinD), and smoke checks curl the running stack. Image rebuilds happen only when Dockerfiles change.

Two deeper local checks ride beside the standard suite — a coverage gate and staticcheck:

```bash
docker exec go_developers_guide_dev bash -lc 'cd /workspaces/go_developers_guide/server && tools/coverage/check.sh'   # total statement coverage floor (see server/tools/coverage/THRESHOLD.md)
docker exec go_developers_guide_dev bash -lc 'cd /workspaces/go_developers_guide/server && staticcheck ./...'          # pinned via server/staticcheck.conf; install once: go install honnef.co/go/tools/cmd/staticcheck@v0.8.2
```

## Project layout

```
server/                  Go module `guide` — the teaching server
  cmd/guide/             composition root (wires every port to its adapter)
  internal/              core/ content/ domain/ data/ presentation/ graph/ grpcapi/ apispec/
  api/                   OpenAPI spec + protobuf contracts (embedded)
  tools/                 pinned codegen tools (oapi-codegen, gqlgen)
client/                  React 19 + Vite + TypeScript presentation shell
  src/                   core/ domain/ data/ ui/
.docker/                 dev compose + dev/prod Dockerfiles
.devcontainer/           VS Code devcontainer (attaches to the dev stack)
deploy/                  prod compose + k8s manifests
```

## Status

Curriculum complete — all 43 topics, 62 demos, and orders 1–43 live (build phases 0–11 closed). Phase 12 (hardening) is closed too: host-run Playwright e2e over the live stack (11 specs across 7 files, [`e2e/`](e2e)), a 74% total-coverage floor on the server suite (`tools/coverage/check.sh`, 76.2% actual), staticcheck pinned at v0.8.2 and clean, a clean govulncheck, a go.mod snapshot drift test, and a full snippet re-quote audit behind the zero-violation ratchet. What remains of phase 13 is remediation of the senior review's findings — no curriculum work is open.
