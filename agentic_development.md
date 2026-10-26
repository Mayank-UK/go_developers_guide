# AI defined section:

## Current state
- Product — runnable full-stack reference app that IS the guide (path /Users/mak/Development/personal/go/go_developers_guide). Server (Go + net/http stdlib, module `guide`) carries 100% of the
  teaching as a typed content API; client (React 19 + Vite + TS + Tailwind v4, react_developers_guide architecture) is a presentation-only shell that never teaches React. Siblings: ../express
  (canon origin), ../springboot, ../fastapi (NEWEST reference — when unsure, look THERE first, then spring, then express).
- Repo — `server/` + `client/` + `e2e/` (host-run Playwright, Phase 12) + `deploy/k8s/` (Phase 10) + `.docker/` + `.devcontainer/`; NO `.github/` (no hosted CI/Dependabot — family stance; local
  verification is the only gate). Ports (co-run with the family stacks): server 8100, client dev 5175, client prod 8101, PG 5433, redis 6380, mongo 27018.
- Layers — `server/cmd/guide/main.go` = composition root (constructor injection, no DI framework) · `internal/core/` (env config, error taxonomy + envelope, slog JSON + request-id middleware,
  Result-equivalent where it earns its keep, ScenarioStore) · `internal/content/` (typed topic registry → sections → blocks, startup validation) · `internal/domain/<area>/` (entities, ports =
  interfaces, use cases; `patterns/` = event store + saga) · `internal/data/<area>/` (repo impls: pgx/redis/mongo/files/messaging) · `internal/presentation/` (handlers, middleware, websocket).
  JSON camelCase via struct tags. Client: identical family architecture (core/domain/data/ui; snake_case views, kebab-case core/domain/data + shared ui), clone-and-adapt from the fastapi/spring
  client; theme key 'gdg-theme', DARK default.
- Verification (DoD) — VERIFICATION PROTOCOL v2, in force from day one (spring's container-churn must NOT recur): DEFAULT = docker exec on the
  long-lived containers, ZERO ephemeral churn; image rebuilds only when Dockerfiles change; scoped tests mid-loop + ONE full suite per topic; smoke =
  curl the RUNNING stack. Server: `docker exec go_developers_guide_dev bash -lc 'cd /workspaces/go_developers_guide/server && go build ./... && go vet ./... && go test ./...'`
  (gofmt -l silent; caches in named volumes gdg-go + gdg-gocache keep execs warm). Client: `docker exec go_developers_guide_client_dev bash -lc 'cd /app && npm test && npm run build'`
  (node_modules in its anon volume; in-container `npm install` ONLY when package.json changed, NEVER `npm ci`). Stack-down fallback only: ephemeral
  `docker run --rm -v … -v gdg-go:/go/pkg/mod -v gdg-gocache:/root/.cache/go-build golang:<pinned>` (flock-wrapped). `docker compose -f
  .docker/docker-compose*.yml config` when Docker files change. Every new demo endpoint needs a Go test; snippet ratchet stays 0.
- THE CODE IS THE SOURCE OF TRUTH — every must-source code block (go/mod/yaml/sql/proto/ts/json/…) carries `source: '<repo-root-relative path>'` and passes the snippet-source test (ordered
  subsequence after comment/blank normalization; `// …` elisions + trailing teaching comments allowed; `<!-- … -->` elisions do NOT normalize away — spring gotcha; RATCHET = 0 — unsourced
  must-source block FAILS the suite). New teaching = real implementation first, then quote; compilable exhibits only in `internal/domain/teaching/` when no real counterpart exists.
  Bash/command/output blocks exempt; never relabel a language to dodge the check. QUOTE-STABILITY: editing a quoted source line breaks the ratchet — keep quoted signatures stable or re-quote.
- Layering — dependency rule: presentation → domain ← data; handlers never import the data layer; DTOs never leak past repository implementations (exemption: teaching-exhibit handlers may import
  data/ directly, marked teaching-exception). Thin handlers: parse/validate input → call a use case → map the result to an HTTP response; no business logic in handlers.
- Errors + DI — one error taxonomy end-to-end: typed error values (errors.Is/As-able) → central error-response writer → one consistent HTTP error envelope; nothing swallowed; every failure logged
  with a request ID; panics are for programmer bugs only — recovered by middleware, never control flow. DI at the composition root: main.go wires ports (interfaces) to adapters via constructor
  injection; fake/live wiring switchable at runtime for demos (ScenarioStore, PATCH /api/demo/scenario).
- Teaching anatomy — each concept section = teach → text → sourced code → ITS live demo INLINE (never queued at page end); every topic closes with a terminal companion (text + curl tour, bash
  blocks); no screen ships with only a CodeBlock. Depth: concept-first, real examples + working demos; family median ~3k words/topic, prose floor 2500 test-pinned. SELF-CONTAINED (family-wide):
  no programming fundamentals assumed — prerequisites (language basics incl. control flow/loops/functions/OOP, data structures, formatting) taught explicitly in the language topic; audit every
  topic for assumed knowledge before DONE; go-language MUST open with fastapi's python-basics + python-data-and-strings equivalents.
- Block selection — prose is the DEFAULT; one paragraph per text block (no \n\n stitching); a ≥3-parallel-point paragraph → 1–3 framing sentences + a list block (ordered only when sequential;
  "Lead term — explanation."; 3–8 items, no nesting); tables only if de-facto tabular; sections never mash sub-topics; narrative paragraphs stay prose; sweeps meaning-preserving (wording +
  numbers verbatim); author this way from day one — no retrofit sweep. Family consistency: react guide's tokens + screen anatomy; fastapi client = the content-screen reference.
- Conventions — gofmt non-negotiable; short package names; exported ids documented; errors wrapped %w; context.Context first param on anything that can block; `-race` in standard test runs.
  Content model: topic → sections → blocks (text | code | demo | list), Go structs + JSON tags, CONTENT_API_VERSION = 1; the server content registry is the single topic-id source; wire contract =
  the fastapi guide's VERBATIM incl. the list block {kind:"list", id, ordered, items 3–8, "Lead term — explanation."} (full contract in ../fastapi/fastapi_developers_guide agentic_development.md;
  record here at Phase 1 close). Markers: ✅ done · 🚧 in progress · ⏳ pending.
- Stack decisions — Go latest stable + stdlib net/http backbone (ServeMux 1.22+ patterns; chi/gin/echo by comparison only, not used) · PostgreSQL-ONLY via pgx v5 (real-PG tests via
  testcontainers-go) · sqlc AND GORM both in topic 18 (sqlc = typed queries, GORM = ORM comparison) · slog stdlib (zap/zerolog by comparison) · stdlib testing + httptest + testcontainers-go, NO
  testify (YAGNI, revisit if earned) · root module `guide`. Canon additions: interfaces-and-composition, goroutines-and-channels, sqlc-gorm; Reactivity N/A (concurrency lives in 02+06).
- Directives — (1) STRICTLY SEQUENTIAL: one subagent at a time, never parallel; quality/detail over speed. (2) Whole setup on Docker. (3) REVIEW GATE: STOP for developer review after Phase 0 + 1;
  later phases per approval cadence. (4) QUALITY BAR: 10/10 to a 15+yr engineer — no shortcuts, no dead code, no "good enough for now". (5) STACK STAYS UP: compose runs CONTINUOUSLY in active dev
  — NO compose down at task end; teardown only on explicit request / long pauses. (6) Verification = PROTOCOL v2, above. (7) agentic_development.md is MASTER-OWNED — subagents never edit it; they
  report facts, master bookkeeps. (8) PER-TOPIC sequential authoring from Phase 3, exemplar first; template = fastapi's TOPIC AUTHORING PATTERN (mirror at 3.1 close).
- State — FRESH START: repo has only this file (no code/git/infra); this file is the single planning source. Pin at bootstrap (verify latest stable): Go (expect 1.27.x, Oct 2026) + golang image,
  pgx v5, goose or golang-migrate (decide at 0.B), gqlgen, grpc-go, asynq, go-redis v9, mongo-driver; client = fastapi client's pins; images postgres:17 / redis:7-alpine / mongo:8 /
  node:22-bookworm. Family traps: npm 10.9.9 arborist crash on the client lockfile → npm ≥ 11 in-container; pre-create cache volumes on fresh machines (`docker volume create gdg-go gdg-gocache
  gdg-npm`); test-file basenames unique per package; subagents killed after 600s idle → poll long runs; docker -v ALWAYS absolute paths; rename workspace dirs BETWEEN sessions only.
- Curriculum — 43 topics / 10 groups / ~62 demos / 14 phases; canon from the family; `+` go-new, `=` canon-adapted; per-topic sections + demos → devlog.md (2026-10-24; read before authoring).
  - Foundations: 01 go-language · 02 go-runtime · 03 http-web | Framework: 04 go-http-fundamentals · 05 interfaces-and-composition + · 06 goroutines-and-channels +
  - Request handling: 07 routing · 08 middleware · 09 request-response · 10 validation | APIs: 11 api-design · 12 api-documentation · 13 graphql · 14 webhooks · 15 grpc
  - Data: 16 sql-schema · 17 databases-sql · 18 sqlc-gorm + · 19 mongodb · 20 redis | State & auth: 21 caching · 22 messaging · 23 authentication · 24 authorization · 25 security
  - Media & realtime: 26 files-media · 27 internationalization · 28 realtime · 29 background-jobs | Platform: 30 error-handling · 31 dependency-management · 32 architecture · 33 observability
  - 34 testing · 35 debugging · 36 environments | Ship: 37 deployment · 38 performance · 39 reliability · 40 scaling | Capstones: 41 best-practices · 42 third-party · 43 advanced-architecture

## Active tasks
- All 14 build phases ⏳ pending — full breakdown in the Developer section's Development tasks; next up: Phase 0 — Bootstrap (0.A → 0.B → 0.C).

# Developer defined section:

- Project requirements
    - Build a full Go teaching-guide project developed exactly like the sibling guides (../express/express_developers_guide, ../springboot/spring_boot_developers_guide, ../fastapi/fastapi_developers_guide — same product concept: server carries all teaching as content API, React presentation shell, devcontainer workflow, snippet source-of-truth discipline, scenario toggles, inline demos), exhaustively covering every backend topic for Go. Common backend topics carry over from the family canon; Go-specific topics added (interfaces-and-composition, goroutines-and-channels, sqlc-gorm split).
    - Frontend uses React following ../react/react_developers_guide architecture and simplicity; clone-and-adapt from the fastapi guide's client (newest family reference).
    - Make the guide as comprehensive as possible: every topic covered in detail — explanations, examples, and demos (~3k-word per-topic median like the siblings).
    - The full curriculum (43 topics → sections → demos) and 14 build phases live in the Curriculum section + Development tasks of this file; that file is the contract.
    - I will add my requirements to this file marked as "⏳" and the AI will keep finishing them and marking them "✅" on completion.

- Generic instructions:
    - Build UI using the design system implemented in the application; reuse existing components before creating new ones; single-file components stay as functions inside the view; multi-file ones go to global ui/components.
    - Follow the proper architecture: use cases, repositories (ports = interfaces in internal/domain/), repository implementations in internal/data/, remote clients, DTOs, DAOs, exception/error handling — mapped to Go idioms (constructor injection at main.go, errors as values, context propagation).
    - If confused about how to do things, check how it is done in other parts of this app or in the sibling guides (fastapi first, then spring, then express); the goal is consistency.
    - Quality principles: DRY, KISS, YAGNI, SOLID, separation of concerns, single source of truth; encapsulation/abstraction/composition-over-inheritance/dependency injection; modularity, maintainability, readability, extensibility, defensive programming (input is hostile), immutability, pure functions; self-documenting naming, consistent formatting, error handling; concurrency safety (race detector clean); least privilege, input validation, secret management; unit + integration tests, test behavior not implementation; incremental refactoring; right algorithms/data structures/caching/batching.
    - Keep agentic_development.md lean: update after every task, drop stale info. This is the ONLY planning/history file (no devlog unless it outgrows).

- Development tasks:
    - Phase 0 — Bootstrap ⏳
        - 🚧 0.A Infra: .gitignore/.zcodeignore/.dockerignore, README stub, git init, .devcontainer, .docker (Dockerfile.dev golang+node; server dev/prod images — prod = multi-stage distroless static binary; client dev/prod + nginx; dev + prod compose with gdg-* cache volumes + co-run ports) — compose configs + Dockerfile.dev build verified; bring the stack UP at the end and LEAVE IT UP (directive 5). Launched 2026-10-26 (subagent, background).
        - ⏳ 0.B Server skeleton: go mod init guide + pinned toolchain, cmd/guide/main.go + internal/ layout, typed env config, slog JSON, /api/health handler, go test/vet/gofmt green (via docker exec from 0.A onward — Protocol v2)
        - ⏳ 0.C Client skeleton: clone-and-adapt the fastapi guide's client (lockfile regeneration via npm ≥ 11 in-container — the arborist trap), 5175 + /api proxy → 8100, tokens + dark theme ('gdg-theme'), health ping through layers, vitest green
    - Phase 1 — Content engine ⏳: typed content model (topic → sections → blocks discriminated on kind) + registry with startup validation; GET /api/content (+ detail); ScenarioStore + GET/PATCH /api/demo/scenario; error taxonomy + one envelope + recovery middleware + request-id middleware; snippet matcher + ratchet harness (canary test proves it bites); wire contract = fastapi's, recorded here at close. REVIEW GATE: STOP for developer review after this phase.
    - Phase 2 — Client shell ⏳: generic TopicView renderer, DemoHost + lazy demo-registry, group nav, friendly degradation (clone fastapi's Phase 2 shape)
    - Phase 3 — Fundamentals ⏳ (SIX sequential per-topic delegations, exemplar first): 3.1 go-language — EXEMPLAR, fixes the authoring pattern (mirror fastapi's TOPIC AUTHORING PATTERN, adapted: content/topics/<snake_id>.go exporting Topic, demos in internal/domain/demos/, handlers in internal/presentation/demo/<area>.go, prose floor 2500) · 3.2 go-runtime · 3.3 http-web · 3.4 go-http-fundamentals · 3.5 interfaces-and-composition · 3.6 goroutines-and-channels
    - Phase 4 — Request handling ⏳: 07 routing · 08 middleware · 09 request-response · 10 validation
    - Phase 5 — APIs ⏳: 11 api-design (posts substrate) · 12 api-documentation (oapi-codegen spec-first) · 13 graphql (gqlgen) · 14 webhooks · 15 grpc (grpc-go)
    - Phase 6 — Data ⏳: 16 sql-schema (goose) · 17 databases-sql (pgx) · 18 sqlc-gorm (testcontainers-go) · 19 mongodb · 20 redis — testcontainers wiring (docker.sock + host override) lands here
    - Phase 7 — Cache/msg + auth ⏳: 21 caching · 22 messaging · 23 authentication · 24 authorization · 25 security
    - Phase 8 — Files/i18n/realtime/jobs ⏳: 26 files-media · 27 internationalization · 28 realtime (SSE + WebSockets) · 29 background-jobs (asynq)
    - Phase 9 — Platform I ⏳ (pairs): 30 error-handling + 31 dependency-management · 32 architecture + 33 observability · 34 testing + 35 debugging · 36 environments
    - Phase 10 — Platform II ⏳: 37 deployment (real deploy/k8s/) · 38 performance · 39 reliability (breaker) · 40 scaling
    - Phase 11 — Capstones ⏳: 41 best-practices · 42 third-party · 43 advanced-architecture (patterns: saga + event store)
    - Phase 12 — Hardening ⏳: flake fixes, govulncheck + module snapshot pins, shared POST /api/demo/reset, jq-free terminal tours, host-run Playwright e2e/, coverage gates, staticcheck decision, snippet re-quote audit, README rewrite
    - Phase 13 — 10/10 quality ⏳ (only on explicit request): reset rate-limit, prod fail-fast config, stricter lint gates, coverage branch gates
    - Topic DONE = content (detailed text + sourced snippets) + working demo(s) inline + Go tests green (+ race where relevant) + snippet spec green + terminal companion (curl tour) verified against the running stack.
