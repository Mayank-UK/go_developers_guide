package content

import (
	"errors"
	"fmt"
)

// This file is topic 41 best-practices (group capstones, curriculum
// order 41 — the guide's FIRST capstone): the synthesis that closes
// the curriculum's teaching arc. Forty topics built a runnable
// reference app — a typed content registry, thin handlers, one error
// envelope, real stores, honest tests, a shippable image — and this
// topic walks the DISCIPLINES that produced it, area by area, each
// grounded in the repo's own real code and each cross-referenced back
// to the topic that taught it: API discipline to topics 11 and 12,
// security to 25, data to 16 through 20, reliability to 39,
// observability to 33, performance to 38, operations to 36 and 37.
// The topic's demo, best:checklist, is real machinery: a self-audit
// whose verdict rows are computed ONCE at boot from machinery that
// actually executed — live probes through the real handler chain, real
// gofmt/ratchet/-race tool runs, real file reads — and served from the
// snapshot, honest fails where a check cannot run and manual verdicts
// where no automation can honestly judge.

// bestPracticesTopic builds topic 41.
func bestPracticesTopic() (Topic, error) {
	var buildErr error
	collect := func(what string, err error) {
		if err != nil {
			buildErr = errors.Join(buildErr, fmt.Errorf("%s: %w", what, err))
		}
	}
	text := func(id, body string) TextBlock {
		block, err := NewTextBlock(id, body)
		collect("text "+id, err)
		return block
	}
	code := func(id, language, caption, source, snippet string) CodeBlock {
		block, err := NewCodeBlock(id, language, caption, source, snippet)
		collect("code "+id, err)
		return block
	}
	demo := func(id, title, description string) DemoBlock {
		block, err := NewDemoBlock(id, title, description)
		collect("demo "+id, err)
		return block
	}
	list := func(id string, ordered bool, items ...string) ListBlock {
		block, err := NewListBlock(id, ordered, items)
		collect("list "+id, err)
		return block
	}
	section := func(id, title, lead string, blocks ...Block) Section {
		assembled, err := NewSection(id, title, lead, blocks)
		collect("section "+id, err)
		return assembled
	}

	sections := []Section{
		// ── 1. Setup and toolchain ──────────────────────────────
		section("setup", "Setup and toolchain",
			"The floor everything stands on: one manifest of exact pins, one format gate, one container-only build world.",
			text("bp-setup-pitch",
				"Every practice in this capstone stands on a floor laid in the first hour: a module manifest that pins the toolchain and every dependency to exact versions, a formatter that is not a suggestion but a gate, and a build world that exists only in containers so no machine is special. Go makes the first two almost unfairly cheap — the toolchain directive and the require block ARE the contract, there is no separate lockfile to keep honest because go.sum's checksums pin the graph the manifest names. The repo's manifest states the whole policy in its opening lines, and the tool directives carry the codegen tools the API topics (12, 13, 15) generate with:"),
			code("bp-setup-gomod-quote", "mod",
				"The manifest's opening contract: the toolchain pinned, requires exact, codegen tools declared.",
				"server/go.mod",
				"module guide\n"+
					"go 1.27.3\n"+
					"// …\n"+
					"require (\n"+
					"	github.com/hibiken/asynq v0.26.0\n"+
					"	github.com/jackc/pgx/v5 v5.11.0\n"+
					"	github.com/pressly/goose/v3 v3.29.0\n"+
					"	github.com/redis/go-redis/v9 v9.23.0\n"+
					"// …\n"+
					")\n"+
					"// …\n"+
					"tool (\n"+
					"	github.com/99designs/gqlgen\n"+
					"	github.com/sqlc-dev/sqlc/cmd/sqlc\n"+
					")"),
			text("bp-setup-format",
				"The format gate is the second half of the floor, and it is deliberately boring: gofmt is non-negotiable, `gofmt -l .` must print NOTHING, and the verification one-liner in this topic's terminal companion enforces it on every run. A formatter that everyone runs kills the review-thread arguments about spacing before they start, and — the quieter benefit — a byte-stable tree is what makes this guide's own snippet discipline possible, because quoted lines do not drift when nobody rewraps them. The third pillar is the container world: the dev containers are the only place anything builds, named cache volumes keep the first dependency download the last, and the whole gate is one command anyone can run identically. None of this setup earns a feature ticket or a screenshot, which is exactly why it has to be structural — a practice that depends on somebody remembering is a practice that is already gone. The setup checklist as this repo keeps it:"),
			list("bp-setup-list",
				false,
				"Exact pins, no ranges — the go directive names the toolchain, every require carries a full version, and go.sum's checksums close the graph.",
				"Codegen tools as directives — gqlgen, sqlc, goose, oapi-codegen, and the protoc plugins are declared IN the manifest, so `go tool` runs the same versions everywhere (topic 31 walked the whole story).",
				"Format as a gate — gofmt -l printing nothing is part of the definition of done, not an aspiration; a dirty file fails the run.",
				"Vet in the loop — go vet rides the same one-liner, catching the printf mistakes and copy-lock bugs a compiler shrugs at.",
				"Containers only — no host toolchain state matters; the dev container's world is the world, and a fresh machine reaches it in one command (topic 36's environments discipline).",
				"Stdlib first — every dependency must out-argue the standard library before it lands; the not-dos section owns the deliberate refusals.",
			),
		),
		// ── 2. API discipline ───────────────────────────────────
		section("api-discipline", "API discipline",
			"Thin handlers, one envelope, statuses decided in exactly one place — the shape topics 11 and 12 taught, kept everywhere.",
			text("bp-api-pitch",
				"API discipline is what makes forty-odd routes readable in an afternoon: every handler does the same three things in the same order — parse and validate at the boundary, call ONE use case, map the result to a response — and no handler ever contains business logic. The posts resource (topic 11's canonical surface, the substrate half the guide reuses) states the whole shape in a screen: the detail handler is five lines of wiring around one use-case call, and a miss arrives as the typed taxonomy error and leaves through the ONE error writer:"),
			code("bp-api-handler-quote", "go",
				"The house handler shape: parse or take the path value, one use-case call, taxonomy error or JSON — nothing else.",
				"server/internal/presentation/posts.go",
				"// ServeHTTP implements http.Handler for reading one post.\n"+
					"func (h *PostDetailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"	post, err := h.get.Execute(r.Context(), r.PathValue(\"id\"))\n"+
					"	if err != nil {\n"+
					"		core.WriteError(w, r, err, h.logger)\n"+
					"		return\n"+
					"	}\n"+
					"	writeJSON(w, http.StatusOK, newPostResponse(post))\n"+
					"}"),
			text("bp-api-envelope",
				"The envelope is the other half of the discipline. Every failure in the whole app — validation, auth, missing, conflict, throttled, degraded, internal — renders as ONE JSON shape with a code, a message, the request id, and optional details, and the HTTP status is decided in exactly one place: the kind-to-status table in the taxonomy (topic 30 built it; the wire contract recorded it at Phase 1). No handler ever invents a status or a code, which is what makes failures countable — a metrics dashboard over status codes agrees with the logs, which agree with the client's error rendering. The table itself is small enough to memorize, which is the point:"),
			code("bp-api-status-quote", "go",
				"Statuses decided once: the taxonomy's kind-to-status map is the only place an HTTP status is chosen.",
				"server/internal/core/errors.go",
				"var httpStatusByKind = map[Kind]int{\n"+
					"	KindValidationFailed:   http.StatusUnprocessableEntity,\n"+
					"	KindUnauthorized:       http.StatusUnauthorized,\n"+
					"	KindForbidden:          http.StatusForbidden,\n"+
					"	KindNotFound:           http.StatusNotFound,\n"+
					"	KindConflict:           http.StatusConflict,\n"+
					"	KindRateLimited:        http.StatusTooManyRequests,\n"+
					"	KindServiceUnavailable: http.StatusServiceUnavailable,\n"+
					"	KindInternal:           http.StatusInternalServerError,\n"+
					"}"),
			text("bp-api-contract",
				"The contract's lifecycle is disciplined too, because discipline at creation is only half the practice. The OpenAPI document (topic 12) generates from the code and a drift test pins it to the routes, so the spec cannot quietly disagree with reality; the GraphQL schema (topic 13) and the protobuf (topic 15) are compiled artifacts whose generators run from the manifest's pinned tool directives; and the client validates every payload against its own typed DTOs, so wire drift surfaces loudly rather than half-rendering. Versioning stays boring on purpose — the content API says version 1 in every payload, and a version 2 is earned by shipping a breaking change you can no longer avoid, not by anticipating ones you might."),
		),
		// ── 3. Security ─────────────────────────────────────────
		section("security", "Security",
			"Never opt-in defense — uniform cost at the login, limits at the edge, secrets as environment values. Topic 25 taught it; the audit re-proves it.",
			text("bp-security-pitch",
				"Security practices fail when they are features somebody has to remember to enable, so this repo wires its defaults into structure instead: the defensive headers and the rate limiter wrap the security surfaces at the composition root, the error envelope carries no stack traces, and the login path burns UNIFORM work whether the failure is a wrong password or a nonexistent user. That last one is the subtle practice worth restating at capstone level: response timing is a side channel, and a login that short-circuits on unknown usernames teaches an attacker which accounts exist. The repo's fix is the decoy hash — the domain engine verifies against it whenever no stored hash exists, so every rejection costs one full argon2id run:"),
			code("bp-security-uniform-quote", "go",
				"Timing-uniform verification: absence burns the same argon2 work as error, so a stopwatch learns nothing.",
				"server/internal/domain/auth/password.go",
				"// VerifyUniform is the timing-uniform credential check every login path\n"+
					"// uses: an empty stored hash still burns one full verification against\n"+
					"// the decoy, and the answer is false either way — callers learn nothing\n"+
					"// about which half failed, and neither does a stopwatch.\n"+
					"func (h *Hasher) VerifyUniform(password, encoded string) bool {\n"+
					"	if encoded == \"\" {\n"+
					"		encoded = DecoyHash\n"+
					"	}\n"+
					"	return h.Verify(password, encoded) == nil\n"+
					"}"),
			text("bp-security-list-framing",
				"The rest of the security checklist as this repo keeps it — each row the audit panel re-proves from the code that implements it:"),
			list("bp-security-list",
				false,
				"Uniform verification cost — unknown users burn the decoy hash (the quote above), a bug class the login suite pins with a lower-bound timing test.",
				"Rate limiting at the edge — the x/time token bucket keyed per client (topic 25), with 429s carrying Retry-After so the throttled client knows when to return.",
				"Headers that are not opt-in — the security surfaces wrap in the headers middleware OUTSIDE the limiter, so even a 429 reply carries the defensive set.",
				"Input as data — parameterized SQL everywhere (topics 16-18), streaming uploads that never trust a filename (topic 26), validation at the boundary before any store (topic 10).",
				"Secrets as environment values — typed config reads them at boot (topic 36), the prod compose rail refuses to render without them (topic 37), and display surfaces redact (topic 25's PII demo).",
				"Deny by default — the authorization middleware (topic 24) demands identity and permission explicitly; nothing is reachable-then-locked.",
			),
		),
		// ── 4. Data ─────────────────────────────────────────────
		section("data", "Data",
			"The schema has a history and input never becomes code — topics 16 through 20's disciplines in one place.",
			text("bp-data-pitch",
				"Data practice in this repo comes down to two commitments: the schema changes only through migrations, and user input reaches the database only as data. The first is goose's chain (topic 16) — SQL files, sequentially numbered, embedded in the binary, applied at boot — so every schema decision is a reviewable artifact with an up and a down, and the database a fresh machine builds is byte-identical to the one production runs. The initial migration's posts table carries the house style in a dozen lines: UUID primary keys minted app-side as UUIDv7, CHECK constraints at creation (never only in the app), timestamptz always:"),
			code("bp-data-table-quote", "sql",
				"Named decisions, reviewable as code: the schema's first table states the house style.",
				"server/internal/data/db/migrations/00001_init.sql",
				"CREATE TABLE posts (\n"+
					"    -- uuid, not bigserial: post URLs must not be enumerable, and ids are\n"+
					"    -- minted app-side as UUIDv7 (time-ordered, index-friendly — the ids\n"+
					"    -- topic walks the whole trade-off). DEFAULT gen_random_uuid() is the\n"+
					"    -- safety net for rows inserted by hand.\n"+
					"    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),\n"+
					"    title       text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),\n"+
					"    body        text NOT NULL,\n"+
					"    -- timestamptz, never timestamp: every instant is a moment on the\n"+
					"    -- timeline; naive timestamps invite the UTC-vs-local bug class.\n"+
					"    created_at  timestamptz NOT NULL DEFAULT now(),\n"+
					"    updated_at  timestamptz NOT NULL DEFAULT now()\n"+
					");"),
			text("bp-data-second",
				"The second commitment is parameterization, and in Go it is nearly free — the database/sql and pgx APIs take a query string and arguments separately, so the discipline is cultural, not syntactic: $1 placeholders in every handwritten query (topic 17), sqlc generating the same shape from SQL you wrote first (topic 18), and rendered SQL appearing only in DISPLAY paths like the safety demo's probe, never on an execute path. Around the two commitments sit the practices the data topics each contributed: one dialect (PostgreSQL) learned deeply with real-instance tests through testcontainers; transactions that own their whole story — the outbox row and the business write commit together or not at all (topic 22); indexes as per-query decisions with their costs stated (topics 16, 17, 40); and the optional stores (MongoDB, Redis) held to optional-service discipline — their absence degrades a demo, never a boot."),
		),
		// ── 5. Reliability ──────────────────────────────────────
		section("reliability", "Reliability",
			"Bounded everything — deadlines, retries, shutdown — and a race stance that is executed, not asserted. Topic 39's frame, kept.",
			text("bp-reliability-pitch",
				"Reliability practice is mostly decisions about what happens when things go away, and this repo's answers are consistent: bound the wait, degrade the feature not the process, drain on the way out. The context deadline is the primitive everything reduces to — every use case takes ctx first, every external call respects it, and the timeouts topic (39) walked the whole ladder from a per-request deadline through retry-with-jitter to the circuit breaker. The composition root's shutdown path is the discipline in miniature: BOTH surfaces drain, gRPC first because its RPCs are short, HTTP second inside a bounded window, and a stuck drain surfaces as an error instead of a zombie process:"),
			code("bp-reliability-drain-quote", "go",
				"The drain every rollout depends on: both surfaces, bounded, on the signal path.",
				"server/cmd/guide/main.go",
				"	grpcServer.GracefulStop()\n"+
					"	logger.Info(\"grpc server stopped\")\n"+
					"	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)\n"+
					"	defer cancel()\n"+
					"	if err := srv.Shutdown(shutdownCtx); err != nil {\n"+
					"		return fmt.Errorf(\"graceful shutdown: %w\", err)\n"+
					"	}\n"+
					"	logger.Info(\"server stopped\")"),
			text("bp-reliability-race",
				"The race stance deserves its own paragraph because Go makes it executable: the race detector rides the standard verification run, and this guide treats a data race as a bug to fix, never a flake to re-run. The concurrency topics (02, 06) taught the hygiene — goroutines that own their lifetimes, channels that close exactly once, contexts that propagate cancellation — and the testing topic (34) taught how to prove it under -race. What the audit panel adds is the stance made visible: the race row does not READ a policy document, it EXECUTES a real `go test -race` run of the race exhibit's own tests at boot and serves the exit code. That is the transferable habit for any practice you claim: own a check that inflicts it, or the claim is a comment with ambitions."),
		),
		// ── 6. Observability ────────────────────────────────────
		section("observability", "Observability",
			"JSON logs from line one, one id per request, health that lies about nothing — topic 33's three answers.",
			text("bp-obs-pitch",
				"Observability practice answers three questions — what happened, in what order, and where do I look — and this repo answers them with the smallest machinery that honestly can. Logs are structured JSON from the first line of the process: one object per event, machine-parseable, stdout as the container-native sink, no log files anywhere. The entire pipeline is one constructor the composition root calls once, which is itself the practice — logging configuration is a single seam, not a per-file import of opinions:"),
			code("bp-obs-logging-quote", "go",
				"The entire logging setup: one constructor, JSON on stdout, level from typed config.",
				"server/internal/core/logging.go",
				"func NewLogger(cfg Config) *slog.Logger {\n"+
					"	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{\n"+
					"		Level: cfg.LogLevel,\n"+
					"	}))\n"+
					"}"),
			text("bp-obs-request-id",
				"The correlation id is the second answer: the request-id middleware stamps one on EVERY response — success or envelope — and binds it into the context every logger downstream can read, so a user report of \"request 4f2c broke\" maps to exact log lines (the wire contract guarantees the id rides the error envelope too). The third answer is the health pair: /api/health reports the PROCESS and nothing else, so an orchestrator's liveness probe never fails because a database is slow; /api/ready pings the real dependencies inside bounded timeouts, so a load balancer stops routing to a pod that cannot serve. Metrics and tracing ride the same honesty — a hand-rolled Prometheus exposition and a W3C tracecontext propagator (topic 33), stdlib both, with pprof mounted behind a dedicated internal path for the profiles topic 35 walked. Level discipline closes it: INFO for what an operator greps, WARN for handled failures the taxonomy already classified, ERROR for the unhandled — and nothing logged twice at two levels. The test of the whole stack is a stranger's: run any request against the live server and the JSON lines should read like a narration of that one request — if following a single call through your logs takes more than one grep, the logs are decoration, not observability."),
		),
		// ── 7. Performance ──────────────────────────────────────
		section("performance", "Performance",
			"Measure first, bound the edges, keep the defaults until the numbers arrive — topic 38's stance, held.",
			text("bp-perf-pitch",
				"Performance practice in this repo starts with restraint: measure before tuning, and keep the shipping defaults boring until real load says otherwise. The two structural bounds the server ships are chosen before any benchmark ran — bounded IO at the connection level, because a slowloris client should be disconnected by policy rather than tolerated until memory dies, and one process-wide pool per external store, because connection arithmetic multiplies faster than intuition. The server's own timeouts state the first practice in five fields:"),
			code("bp-perf-timeouts-quote", "go",
				"Bounded IO from day one: the slow-client abuse class dies at the connection, not in a dashboard.",
				"server/cmd/guide/main.go",
				"	srv := &http.Server{\n"+
					"		Addr:    fmt.Sprintf(\":%d\", cfg.Port),\n"+
					"		Handler: handler,\n"+
					"		// Bounded reads/writes defeat slow-client abuse while leaving ample\n"+
					"		// headroom for the guide's demos.\n"+
					"		ReadHeaderTimeout: 5 * time.Second,\n"+
					"		ReadTimeout:       10 * time.Second,\n"+
					"		WriteTimeout:      15 * time.Second,\n"+
					"		IdleTimeout:       60 * time.Second,\n"+
					"	}"),
			text("bp-perf-measured",
				"Wherever this guide quotes a number, the number was measured in the process serving it — that is the practice the performance topic (38) pinned with its own no-invented-numbers test, and the later topics inherited: the breaker serves only time.Since-measured timings (39), the scaling round serves counted distribution and swap facts (40), the benchmark demo serves parsed benchstat output with its command and run config. Pool sizing and GC knobs stay at defaults ON PURPOSE until a real workload complains — defaults documented as deliberate beat tuning nobody can reproduce. And the two cheapest levers stay first in line ahead of any tuning: an index that removes a scan (the schema topic's doctrine), and streaming that never buffers in memory what can flow (the files topic's uploads, the realtime topic's frames). The performance checklist is short because the honest list is short."),
		),
		// ── 8. Operations ───────────────────────────────────────
		section("operations", "Operations",
			"The stack as files you can diff and dry-run — topics 36 and 37's disciplines, from compose to distroless.",
			text("bp-ops-pitch",
				"Operational practice is what lets somebody else run what you built, and this repo's posture is files-you-can-diff end to end: the dev stack is a compose file whose services declare their OWN healthchecks (so a human asking \"is postgres up\" gets the same answer the orchestrator would), the cache volumes are named and external on purpose (so a fresh machine pays its dependency download exactly once, and the file explains what happens if you forget), and the production story is a multi-stage build landing in a distroless image — the static binary topic 37 shipped. The healthcheck block is the whole stance in six lines:"),
			code("bp-ops-healthcheck-quote", "yaml",
				"Truth as config: a backing service states how its own health is probed, for humans and orchestrators alike.",
				".docker/docker-compose.yml",
				"    healthcheck:\n"+
					"      # pg_isready ships in the postgres image itself.\n"+
					"      test: [\"CMD-SHELL\", \"pg_isready -U postgres -d guide\"]\n"+
					"      interval: 10s\n"+
					"      timeout: 5s\n"+
					"      retries: 5\n"+
					"      start_period: 10s"),
			text("bp-ops-volumes",
				"The external cache volumes carry the workflow economics in the same file — and the comment block beside them is deliberate operational practice too: a stack that explains its own failure modes is a runbook you cannot lose. Rendering before running is the other half: `docker compose config` is the pre-flight, and the production compose file's required-variable rail refuses to RENDER without a real database password, so the secrets conversation happens before any container starts, not during an incident. Parity is the closing discipline — dev, prod, and the k8s manifests run the SAME images and the same binary shape, every difference a value in a file (a port, an env pointer, a replica count), never a branch of the codebase. One-command onboarding rides the same file set: the devcontainer names the whole world, and the day a fresh machine needs a runbook, the repo owes a fix."),
			code("bp-ops-volumes-quote", "yaml",
				"External caches: declared, reused, and explained where they are declared.",
				".docker/docker-compose.yml",
				"  gdg-go:\n"+
					"    name: gdg-go\n"+
					"    external: true\n"+
					"  gdg-gocache:\n"+
					"    name: gdg-gocache\n"+
					"    external: true"),
		),
		// ── 9. Git and review ───────────────────────────────────
		section("git-and-review", "Git and review",
			"Small diffs, machines first, a ratchet that only tightens — and a human who reads what machines cannot.",
			text("bp-git-pitch",
				"The version-control practices here are disciplines of smallness and honesty: small diffs, atomic commits, and a review culture where machines check what machines can check so human attention spends itself on design intent. This repo's strongest git-adjacent practice is the snippet ratchet — the test that proves every quoted line in every one of the forty-one topics is a real line of a real file, so the teaching cannot rot silently while the code moves under it. The belt is two lines, and the contract lives in the constant's own comment:"),
			code("bp-git-ratchet-quote", "go",
				"The review force-multiplier: a ratchet that only turns one way.",
				"server/internal/content/snippet_sources_test.go",
				"// RATCHET is the maximum allowed violations. Never raise it; drive\n"+
					"// it to 0 and keep it there.\n"+
					"const RATCHET = 0"),
			text("bp-git-review-culture",
				"Around the machine belt sits the human one. Every diff gets read by a person before it lands — not to re-verify what the suite proves, but to ask the questions automation cannot: is this the simplest shape that solves it, does the name say the thing, will the next reader have the context this change assumes. The audit panel below is honest about this boundary — it has a review-your-diffs row, and that row comes back verdict MANUAL with a one-line why, because a checklist that pretends to verify judgment is lying with extra steps. History that answers WHY is the quieter third practice: commit messages and the planning file carry decisions and their reasons, so archaeology starts at a rationale, not at a diff — and when a practice is deliberately NOT kept, the next section's anti-checklist makes that a first-class artifact instead of an omission someone discovers in a code review six months later."),
		),
		// ── 10. Human practices ─────────────────────────────────
		section("human-practices", "Human practices",
			"The judgments no checklist can make for you — stated plainly, because a practices synthesis that omits them is a list of chores.",
			text("bp-human-pitch",
				"The practices that matter most are the ones no automation can verify, and this topic refuses to pretend otherwise. First, the threat model is revisited whenever the attack surface changes — a new endpoint, a new dependency, a new store — because a threat model written once is a photograph of a system that no longer exists; topic 25 taught the habit, and only people can keep it. Second, production secrets are actually overridden: the compose rail enforces PRESENCE, but whether the value behind the variable is strong, rotated, and owned is an operator fact, and treating the rail as the whole practice is how 'changeme' ships to production. Third, onboarding stays honest — when a new machine cannot reach a running stack in one command, that is a bug in the repo, not a rite of passage for the newcomer."),
			text("bp-human-communication",
				"Communication is the other half. Failures get reported with their request id, because \"it broke\" is an anecdote while \"request 4f2c broke\" is a greppable diagnosis — the observability section's whole design exists to make that sentence cheap. Divergences get written down where the next reader trips over them: this repo's not-dos ride the audit payload as first-class rows with reasons and artifacts, and the same expectation applies to a fork in the road a teammate will otherwise re-litigate from scratch. Estimates stay guesses in public — a definition of done that names the command that must pass is a communication practice disguised as bookkeeping, because the whole team reads the same definition of finished. Treat every checklist, this topic's included, as a budget statement rather than a score: passes are sunk discipline, failures are scheduled work, manuals are the agenda of your next conversation with your team. None of these rows will ever appear green on a dashboard, and that is the point — the practices a panel cannot check are the ones that decide whether the panel tells the truth."),
		),
		// ── 11. The anti-checklist ──────────────────────────────
		section("anti-checklist", "The anti-checklist",
			"This section carries the topic's demo — the live self-audit, with the repo's deliberate not-dos riding the same payload.",
			text("bp-anti-pitch",
				"An honest practices guide ends with what it deliberately did NOT do, because a checklist without its complement teaches cargo cult. This repo's not-dos are decisions with reasons: no hosted CI or Dependabot by default (the local docker verification one-liners are the only gate — a family stance, revisited only on explicit request); stdlib-first everywhere (no chi, gin, echo, testify, or zap — the commercial options live in comparison prose, and each one must out-argue the standard library before it lands); JWT hand-rolled as stdlib HS256 (the format is small enough that a dependency buys nothing a screen of honest code cannot); and browser E2E deferred to the hardening phase (scripted curl tours and payload assertions bridge the gap and say so). The demo below presents the checklist and its complement together — the audit's own rows, and the four not-dos with their reasons and artifacts."),
			text("bp-anti-demo-pitch",
				"The demo is real machinery, not narration. At process boot — once, in the background, bounded — the audit drives five REAL requests through the same health, scenario, and posts handlers behind the same production middleware chain the live server serves; runs three REAL tools (gofmt -l over the server tree, the snippet-ratchet suite, a -race run of the race exhibit's tests); and reads the real files (go.mod for pins, the composition root for the drain wiring, the migrations directory, the decoy-burn markers). Every machine row carries the evidence that produced it; a check that cannot execute reports failed with its reason — never passed — and the two practices no automation can judge report manual with a why. The row-shaping rule that keeps it honest is small enough to quote:"),
			code("bp-anti-row-quote", "go",
				"No verdict without evidence — a row that produced no evidence fails itself.",
				"server/internal/domain/demos/best_checklist.go",
				"func bestVerdictRow(id, practice, kind string, evidence []string, ok bool) BestChecklistItemRow {\n"+
					"	verdict := BestVerdictFail\n"+
					"	if ok {\n"+
					"		verdict = BestVerdictPass\n"+
					"	}\n"+
					"	if len(evidence) == 0 {\n"+
					"		evidence = []string{\"the check produced no evidence — reported as failed, never guessed\"}\n"+
					"		verdict = BestVerdictFail\n"+
					"	}\n"+
					"	return BestChecklistItemRow{ID: id, Practice: practice, CheckKind: kind,\n"+
					"		Verdict: verdict, Evidence: evidence}\n"+
					"}"),
			demo("best:checklist", "The best-practices self-audit",
				"GET /api/demo/best/checklist serves the whole-guide self-audit, computed ONCE at "+
					"process boot and served from the snapshot. Fifteen rows across nine areas "+
					"(setup, API discipline, security, data, reliability, observability, "+
					"operations, git and review, human practices), every row carrying its check "+
					"kind and its real evidence: 'live' rows drove real requests through the "+
					"real handlers behind the production chain (the health contract, the 404 "+
					"envelope with its request-id match, a 422 at the boundary, the scenario "+
					"contract, this process's resolved config); 'exec' rows ran the real tools "+
					"(gofmt -l, the snippet-ratchet suite, a -race run) with bounded timeouts; "+
					"'file' rows read the repo (go.mod pins, the graceful-drain wiring, the "+
					"embedded migrations, the argon2 decoy markers, the compose healthchecks — "+
					"this last degrading to an honest manual when the repo root is not "+
					"visible); 'manual' rows claim nothing and say why. The anti-checklist "+
					"rides the same payload — four deliberate not-dos with reasons and "+
					"artifacts — beside a summary that counts pass, fail, and manual without "+
					"spinning. Reload and the verdicts replay identically: the audit is a "+
					"fact about boot."),
			text("bp-anti-handler-reading",
				"The handler that serves it is as thin as every other handler in the stack — the audit's machinery lives in the domain builder and the handler's boot-time collection pass, and the endpoint can never turn a broken check into a 500: until the snapshot exists it serves an honest computing placeholder with the same wire shape:"),
			code("bp-anti-handler-quote", "go",
				"The audit endpoint: snapshot or placeholder, never a hang, never a fabricated panel.",
				"server/internal/presentation/demo_best.go",
				"func (h *BestChecklistHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"	if panel := h.snapshot(); panel != nil {\n"+
					"		writeJSON(w, http.StatusOK, *panel)\n"+
					"		return\n"+
					"	}\n"+
					"	writeJSON(w, http.StatusOK, demos.BestChecklistPanel{\n"+
					"		Mode: \"computing: the boot audit is still executing — the panel you want is one \" +\n"+
					"			\"reload away (probes, tool runs, and file reads are bounded; nothing waits on \" +\n"+
					"			\"this request)\",\n"+
					"		Areas:    []demos.BestChecklistAreaRow{},\n"+
					"		NotDones: []demos.BestNotDoneRow{},\n"+
					"	})\n"+
					"}"),
		),
		// ── 12. Terminal companion ──────────────────────────────
		section("terminal-companion", "Terminal companion: the practices over curl",
			"The synthesis from a shell: the curriculum closed, the audit panel live, the gate that guards it, and the diff-provable snapshot.",
			text("bp-terminal-pitch",
				"The tour walks this topic's real surfaces on the running stack. The determinism claim is itself checkable: the audit computes once at boot, so two GETs serve byte-identical panels — the diff below should print nothing but the echo. Everything else reads what the machinery produced: the summary counts, one live row's evidence up close, an honest manual's why, the not-dos, and finally the one-liner that IS this repo's gate."),
			code("bp-terminal-tour-1", "bash", "", "",
				"# stop 1 — the curriculum this capstone closes: the capstones group arrives\n"+
					"curl -s localhost:8100/api/content | grep -o '\"id\":\"best-practices\"[^}]*'\n"+
					"curl -s localhost:8100/api/content | grep -c '\"groupId\":\"capstones\"\\|\"group\":\"capstones\"'\n"+
					"\n"+
					"# the full shape: twelve sections, the demo inline in the anti-checklist\n"+
					"curl -s localhost:8100/api/content/best-practices | grep -o '\"id\":\"[a-z-]*\"' | tail -n 13"),
			code("bp-terminal-tour-2", "bash", "", "",
				"# stop 2 — the self-audit: the summary line, then the verdicts per area\n"+
					"curl -s localhost:8100/api/demo/best/checklist | grep -o '\"summary\":{[^}]*}'\n"+
					"curl -s localhost:8100/api/demo/best/checklist | grep -o '\"checkKind\":\"[a-z]*\"' | sort | uniq -c\n"+
					"\n"+
					"# determinism, proven against the live endpoint — expect the echo, nothing else:\n"+
					"diff <(curl -s localhost:8100/api/demo/best/checklist) \\\n"+
					"     <(curl -s localhost:8100/api/demo/best/checklist) && echo deterministic"),
			code("bp-terminal-tour-3", "bash", "", "",
				"# stop 3 — evidence up close: a live row's proof, and an honest manual's why\n"+
					"curl -s localhost:8100/api/demo/best/checklist | grep -A4 '\"id\":\"error-envelope-on-miss\"'\n"+
					"curl -s localhost:8100/api/demo/best/checklist | grep -o '\"whyManual\":\"[^\"]*\"' | head -n 2\n"+
					"# the anti-checklist rides the same payload\n"+
					"curl -s localhost:8100/api/demo/best/checklist | grep -o '\"notDone\":\"[^\"]*\"'\n"+
					"\n"+
					"# unknown topics stay honest, as they have all guide long:\n"+
					"curl -s -i localhost:8100/api/content/best-practices-nope | head -n 6"),
			code("bp-terminal-tour-4", "bash", "", "",
				"# stop 4 — the repo's actual gate: format, build, vet, tests — one breath\n"+
					"docker exec go_developers_guide_dev bash -lc \\\n"+
					"  'cd /workspaces/go_developers_guide/server && gofmt -l . && go build ./... && go vet ./... && go test ./...'\n"+
					"# client side, same discipline\n"+
					"docker exec go_developers_guide_client_dev bash -lc 'cd /app && npm test && npm run build'\n"+
					"\n"+
					"# stop 5 — from the HOST, the dry-run the deploy path owes you:\n"+
					"docker compose -f .docker/docker-compose.yml config > /dev/null && echo \"dev renders\"\n"+
					"\n"+
					"# stop 6 — this topic through the client's dev proxy, as the browser reaches it:\n"+
					"curl -s localhost:5175/api/content/best-practices | grep -c '\"kind\":\"demo\"'"),
		),
	}

	return Topic{
		ID:    "best-practices",
		Title: "Best practices",
		Subtitle: "The capstone synthesis: every discipline this guide taught, audited against the repo that taught it — " +
			"setup and toolchain, API discipline, security, data, reliability, observability, performance, operations, " +
			"git and review, and the human practices no checklist can verify, each cross-referenced back to the topic " +
			"that taught it and grounded in real code; an anti-checklist of deliberate not-dos with their reasons; and " +
			"a live self-audit whose every verdict was computed at boot from machinery that actually executed — real " +
			"probes, real tool runs, real file reads — with honest fails where a check cannot run and manual verdicts " +
			"where none can honestly judge.",
		Group:    "capstones",
		Order:    41,
		Sections: sections,
	}, buildErr
}
