package content

import (
	"errors"
	"fmt"
)

// This file is topic 36 environments (group platform, curriculum order
// 36): how one Go codebase serves dev, staging, and prod from the same
// bytes while everything that differs becomes CONFIG. The one-codebase
// section quotes this repo's own dev instance (the devcontainer + the
// dev compose) as the concrete "environment as data" artifact; the
// twelve-factor section walks all twelve factors honestly — ten hits,
// three deliberate differs, each verdict naming the real machinery
// behind it. The typed-config section is core.LoadConfig itself: the
// GDG_* contract, dev-safe defaults, validation-at-boot with the
// failing variable NAMED, and the fail-fast boot — quoted, then made
// inspectable by the topic's demo (env:inspector): THIS process's
// resolved config with per-variable environment-vs-default attribution,
// credentials redacted by a table-tested policy, the scorecard as data,
// and the ScenarioStore's CURRENT state as the live flag panel.
// Env-specific behavior is the honest list of what actually changes
// dev→prod (and the bigger honest fact: the process contains no env
// ifs — the PLATFORM differs, not the code). Secrets covers what never
// goes in code or git, the .gitignore discipline, compose/devcontainer
// injection as the dev story, and secret managers by comparison.
// Feature flags teach the ScenarioStore as the guide's REAL flag
// machinery — runtime-flippable, typed, honest — against the
// flags-as-config trade-off. The terminal companion curls the inspector
// live, proves redaction and a flag flip on the wire, and drives a
// scratch binary through GDG_PORT/GDG_LOG_LEVEL overrides and the
// fail-fast refusals.

// environmentsTopic builds topic 36.
func environmentsTopic() (Topic, error) {
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
		// ── 1. One codebase, many environments ────────────────────
		section("one-codebase", "One codebase, many environments",
			"Dev, staging, and prod run the SAME bytes; everything that differs between them is config. This repo's dev instance — the devcontainer and its compose file — is the concrete artifact of that rule.",
			text("env-cb-bytes",
				"THE RULE FIRST, because it orders everything else in this topic: ONE CODEBASE, many environments — the artifact a build produces is immutable, and every difference between dev, staging, and prod is carried by CONFIGURATION injected at run time, never by editing the artifact per place. Go makes this discipline unusually easy to hold, and unusually easy to verify: ``go build`` emits a single static binary — no interpreter directory, no virtualenv to copy, no runtime to install beside it — so ``the same bytes`` is not an aspiration but a checksum you can compare between the CI build and the container that serves prod. The complement is just as concrete: the moment a binary is special-cased for one environment (``just rebuild it with the prod flag turned on in the source``), there are two programs wearing one name, and a bug report from prod no longer reproduces on the binary you can inspect. This guide's whole stack — devcontainer, both compose files, the distroless prod image — is one codebase's set of environment DECLARATIONS."),
			code("env-cb-devcontainer", "json",
				"The dev environment as data: the devcontainer points at the compose file and the service to attach to — no code anywhere in it.",
				".devcontainer/devcontainer.json",
				"  \"name\": \"go_developers_guide\",\n"+
					"  \"dockerComposeFile\": \"../.docker/docker-compose.yml\",\n"+
					"  \"service\": \"dev\",\n"+
					"  \"runServices\": [\"dev\", \"client_dev\", \"postgres\", \"redis\", \"mongo\"],\n"+
					"  \"workspaceFolder\": \"/workspaces/go_developers_guide\",\n"+
					"  \"shutdownAction\": \"stopCompose\","),
			text("env-cb-what-varies",
				"WHAT VARIES between the instances is a short, honest list — and everything on it is data in a file beside the code. The BACKING SERVICES: dev's PostgreSQL, Redis, and Mongo run as compose services with published host ports (PG on 5433, redis on 6380, mongo on 27018 — the family's co-run slots), while the prod compose puts the same engines on an internal network with NO published ports at all. The CONFIG INJECTION: dev's container needs no ``GDG_*`` variables at all, because the code's defaults already point at the compose services by name (the next section's case study), while a real deployment injects DSNs and addresses as environment variables at boot. The TOOLCHAIN: dev runs inside a fat devcontainer with Go, Node, and the codegen tools mounted; prod ships a distroless image with nothing but the binary. What NEVER varies: the module, the embedded migrations, the content registry — and the engines themselves (postgres:17, redis:7, mongo:8 are pinned IDENTICALLY in both compose files, the dev/prod parity discipline the scorecard grades later)."),
			code("env-cb-compose", "yaml",
				"The dev stack's own config injection: the datastore's dev-grade credentials and published port, declared once beside the code.",
				".docker/docker-compose.yml",
				"  postgres:\n"+
					"    image: postgres:17\n"+
					"    container_name: go_developers_guide_postgres_dev\n"+
					"    restart: unless-stopped\n"+
					"    environment:\n"+
					"      - POSTGRES_USER=postgres\n"+
					"      - POSTGRES_PASSWORD=guide\n"+
					"      - POSTGRES_DB=guide\n"+
					"    ports:\n"+
					"      - 5433:5432"),
			text("env-cb-honest",
				"THE HONEST READING of that compose block is the paragraph this section has been building: an environment is a declaration — ``this process, these backing services, these injected values, these published edges`` — written as reviewable text beside the code it deploys. The credentials in the quote are DEV-GRADE BY DESIGN: the compose file pins them because the dev stack's network is local-only, and the prod compose carries the comment that the env rails replace them — the differences between environments are written down where a reviewer reads them, not discovered where a pager reports them. Staging, for this guide, is the same story one more time: the same binary, a third declaration, datastore addresses that point at rehearsal instances. When a behavior difference CANNOT be expressed as config — when the code itself must branch — that is not an environment anymore; that is the feature-flags section's subject, with its own honesty rules."),
		),
		// ── 2. Twelve-factor, walked honestly ─────────────────────
		section("twelve-factor", "The twelve-factor app, walked honestly",
			"The twelve-factor methodology is a 2011 checklist for config-in-environment deployments. This section walks all twelve factors against THIS repo — ten hits, three deliberate differs, no dogma — and the demo renders the same scorecard as data.",
			text("env-tf-what",
				"WHAT THE DOCUMENT IS, and how to read it without dogma: twelve-factor app (Heroku, 2011) is a methodology for building SaaS-deployable services, distilled into twelve rules whose shared spine is SEPARATION — code from config, app from backing services, build from release from run. It was written for a world of elastic PaaS deploys, which means some factors read as universal law and others as dated vocabulary (``backing services``, ``disposability``) — but every one of them is answerable by a concrete artifact in a concrete repo, which is exactly what this section does. The walk below grades THIS guide honestly: a factor is a HIT only when real machinery in this codebase earns it, and a deliberate DIFFERS is recorded with its trade-off, not hidden — the scorecard in this topic's demo renders the same twelve verdicts as data, so the panel and the prose can never drift apart."),
			text("env-tf-code-side",
				"THE CODE SIDE — factors I through IV — is where the repo is strongest, because Go's toolchain enforces half of it mechanically. I. CODEBASE: one git tree serves every environment; the devcontainer and both compose files from the previous section ARE the many deploys, differing only in config. II. DEPENDENCIES: ``go.mod`` and ``go.sum`` declare everything, minimal-version selection resolves it, and the dependency topic's govulncheck pass audits it — there is no way for a package to arrive implicitly. III. CONFIG: everything that differs between environments flows through ``GDG_*`` and ``GRPC_ADDR`` environment variables into ``core.LoadConfig`` — no config file is baked into any image, and the next section dissects that loader as this topic's case study. IV. BACKING SERVICES: PostgreSQL, MongoDB, and Redis attach as URLs and addresses, swappable without code changes — this repo even swaps them at RUN TIME through the scenario store's fake/live switch, one step beyond the factor's ask."),
			text("env-tf-runtime-shape",
				"THE RUNTIME SHAPE — factors V, VI, VII, IX, and XI — describes the process itself, and the composition root is the evidence for each. V. BUILD, RELEASE, RUN: one build step produces identical bytes (the prod Dockerfile's compiler stage), a release is an image tag, and run is config injection — the three stages never smear into each other. VI. PROCESSES: the process is stateless — state lives in the datastores; what the process holds is caches, and the one deliberate exception (the in-memory ScenarioStore) is demo machinery, flagged as such, never product state. VII. PORT BINDING: the server IS the listener — ``net/http`` binds ``GDG_PORT`` inside the container, exporting HTTP beside the gRPC listener, with no external web server required to host it. IX. DISPOSABILITY: boot is bounded (a 30-second window for dials and migrations), SIGTERM drains both serving surfaces inside timeouts, and the debugging topic's nets keep every wait bounded — the process is restartable at any moment. XI. LOGS: slog JSON to stdout, always and only — the container collects the stream; the process never opens a log file, never rotates one, never formats for a console it does not own."),
			text("env-tf-ops-side",
				"THE OPERATIONS SIDE — factors VIII, X, and XII — is where the honest grades live, because this guide deliberately takes three differs, and each one is a recorded trade-off rather than an oversight. VIII. CONCURRENCY says scale the process, and split divergent workloads into separate processes; this guide hosts the background-jobs worker INSIDE the API process so one container teaches everything — pedagogy over fleet shape. X. DEV/PROD PARITY says keep the environments as similar as possible; the guide pins identical datastore engine versions in both compose files, but the runtimes deliberately diverge — a fat devcontainer toolchain against a distroless prod image — the accepted cost of dev ergonomics. XII. ADMIN PROCESSES says one-off tasks run in an environment identical to the app; the guide's goose migrations are EMBEDDED in the binary and run at boot, so the admin process and the app process are the same process — the single-binary trade-off the migrations decision recorded. Each differs is defensible; the discipline is writing it down:"),
			list("env-tf-differs",
				false,
				"VIII. Concurrency — the jobs worker shares the API's process so one container teaches the whole stack; a real fleet splits them so API replicas and workers scale (and fail) independently.",
				"X. Dev/prod parity — datastore engines are pinned identical, but the runtimes differ on purpose: a fat devcontainer toolchain for dev ergonomics versus a distroless image holding nothing but the binary in prod.",
				"XII. Admin processes — goose migrations are embedded and run at boot, so there is no separate migration command to keep environment-identical; the deliberate single-binary trade-off.",
			),
			text("env-tf-no-dogma",
				"NO DOGMA is the reading rule: the factors are strong defaults with named escape hatches, and the failure modes this section guards against are symmetric. The first is CARGO-CULTING — adopting a factor's vocabulary without its substance (``we're twelve-factor, the config is in YAML files inside the image`` misses factor III entirely) — and the cure is grading against artifacts, as this walk did. The second is its mirror, PURITY THEATER — treating a differs as a defect and quietly splitting the worker into its own deployment the week the guide ships, without asking whether the fleet actually needs it — and the cure is the recorded trade-off: a differs you can NAME and justify is healthier than a hit you cannot. The demo's scorecard encodes exactly this stance as data — every verdict clickable back to the machinery that earned it."),
		),
		// ── 3. Typed env config [demo env:inspector] ──────────────
		section("typed-env-config", "Typed environment config: core.LoadConfig is the case study",
			"Environment variables are strings; a Go server wants typed, validated, immutable config. This repo's own loader — GDG_* names, dev-safe defaults, validation that names the failing variable and fails the boot — is the case study, and the demo inspects THIS process's resolved config live.",
			text("env-cfg-strings",
				"THE PROBLEM THE LOADER SOLVES is a type gap at the process boundary: environment variables arrive as STRINGS — always, only — while the server needs an int for the port, a ``slog.Level`` for the log gate, and a DSN-shaped string for the database. Reading ``os.Getenv`` ad hoc at use sites scatters that conversion across the codebase, defers every malformed value to whatever request first trips over it, and leaves no single place where ``the process's configuration`` exists as a thing you can print, test, or hand to a colleague. The typed loader is the fix, and its shape is a family pattern: read ONCE at boot through a parameterized ``getenv`` (tests inject values instead of mutating the process environment — the seam that made the config tests table-driven), resolve every variable against a default, VALIDATE the result, and return one immutable struct that the rest of the process reads by field. The names are the contract this guide's operators see:"),
			code("env-cfg-names", "go",
				"The contract: one GDG_-prefixed variable per knob, mirroring the family guides' prefixes.",
				"server/internal/core/config.go",
				"const (\n"+
					"	envPort        = \"GDG_PORT\"\n"+
					"	envEnv         = \"GDG_ENV\"\n"+
					"	envLogLevel    = \"GDG_LOG_LEVEL\"\n"+
					"	envGRPCAddr    = \"GRPC_ADDR\"\n"+
					"	envDatabaseURL = \"GDG_DATABASE_URL\"\n"+
					"	envMongoURI    = \"GDG_MONGO_URI\"\n"+
					"	envRedisAddr   = \"GDG_REDIS_ADDR\"\n"+
					")"),
			text("env-cfg-defaults",
				"THE DEFAULTS are the dev-environment story in miniature, and their design rule is worth stating: DEV-SAFE AND SECRET-FREE, so the stack boots anywhere with zero configuration. The port defaults to 8100, the environment to ``dev``, the log gate to info, and — the interesting ones — the datastore addresses default to the compose file's SERVICE NAMES (``postgres:5432``, ``redis:6379``, ``mongo:27017``), which resolve only inside the dev network and carry credentials only in the same dev-grade shape the compose file already declares. That is why the devcontainer from the first section injects no ``GDG_*`` variables at all: the defaults ARE the dev instance's values. An unset or WHITESPACE-ONLY variable falls back — ``strings.TrimSpace`` runs before the emptiness check — because an empty override is far more often a CI template's unfilled placeholder than a deliberate wish. Two defaults also document their own honesty: PostgreSQL is part of the stack (its default is dialed at boot, and unreachable fails the boot), while Mongo boot-connects and DEGRADES, and Redis is fully lazy — the validity of those addresses surfaces in their own demos, not in config."),
			code("env-cfg-validation", "go",
				"Validation at boot: a malformed GDG_PORT is refused with the variable named — never coerced, never warned-and-continued.",
				"server/internal/core/config.go",
				"	if raw := strings.TrimSpace(getenv(envPort)); raw != \"\" {\n"+
					"		port, err := strconv.Atoi(raw)\n"+
					"		if err != nil {\n"+
					"			return Config{}, fmt.Errorf(\"%s %q: not a valid port number\", envPort, raw)\n"+
					"		}\n"+
					"		if port < 1 || port > 65535 {\n"+
					"			return Config{}, fmt.Errorf(\"%s %q: port must be between 1 and 65535\", envPort, raw)\n"+
					"		}\n"+
					"		cfg.Port = port\n"+
					"	}"),
			text("env-cfg-failfast",
				"FAIL FAST is the policy the whole wiring enforces, and it is worth reading the composition root's first lines as its proof: a config load error is returned up through ``run`` and becomes ``main``'s minimal stderr JSON fatal with a NON-ZERO exit — before any listener binds, before any dial, before the process can pretend to be healthy. The log level gets the same treatment (``slog.Level.UnmarshalText`` accepts exactly debug/info/warn/error; anything else names ``GDG_LOG_LEVEL`` and exits). The DSN is the deliberate exception that teaches the boundary: config VALIDATES what belongs to config (a port is a number in range — that is config's grammar) and DEFERS what belongs to another package (a DSN's syntax is the driver's to judge, at the dial the boot performs immediately after) — the honest split between ``validated at boot`` and ``validated at first use``, with nothing silently in between:"),
			code("env-cfg-boot", "go",
				"The boot: config once, logger from it, and any error a fatal exit before serving starts.",
				"server/cmd/guide/main.go",
				"	cfg, err := core.LoadConfig(os.Getenv)\n"+
					"	if err != nil {\n"+
					"		return fmt.Errorf(\"load config: %w\", err)\n"+
					"	}\n"+
					"	logger := core.NewLogger(cfg)"),
			text("env-cfg-demo-intro",
				"THE DEMO MAKES THE CASE STUDY INSPECTABLE — and it is built the way this guide builds everything: the implementation is real, and the topic quotes it. The payload below is THIS process's resolved configuration, derived at request time from the LIVE config the handler was wired with at boot: every variable's name, its VALIDATED value, and whether it came from the ENVIRONMENT or the DEFAULT — attribution re-reading the same environment through the same rule the loader applies (set-and-non-blank means environment; anything else means the default was kept). One policy decision in that table is load-bearing enough to quote before you look: SENSITIVE VALUES ARE REDACTED — a DSN's password and its entire query never reach the wire, because showing your config is a debugging habit worth having, and only a redaction POLICY makes it a safe one:"),
			code("env-cfg-redaction", "go",
				"The demo's redaction policy, as code: username, scheme, host, port and path survive; the password and the whole query never do.",
				"server/internal/domain/demos/env_inspector.go",
				"func RedactCredential(raw string) string {\n"+
					"	parsed, err := url.Parse(raw)\n"+
					"	if err != nil || parsed.Scheme == \"\" {\n"+
					"		return hidden\n"+
					"	}\n"+
					"	if parsed.User != nil {\n"+
					"		parsed.User = url.User(parsed.User.Username())\n"+
					"	}\n"+
					"	parsed.RawQuery = \"\"\n"+
					"	parsed.ForceQuery = false\n"+
					"	parsed.Fragment = \"\"\n"+
					"	return parsed.String()\n"+
					"}"),
			demo("env:inspector", "The env inspector: this process's resolved config, live",
				"GET /api/demo/env/inspector serves the server's own resolved configuration as a table — every variable: its Go field, its env name, its validated value, and its source (environment or default), with the two credential-bearing slots (GDG_DATABASE_URL, GDG_MONGO_URI) REDACTED by the policy quoted above: scheme, username, host, port and path shown; password and query never. Beside the table: the twelve-factor scorecard — the same twelve verdicts this topic's prose walked, as data — and the flag panel, the ScenarioStore's CURRENT state, snapshotted at request time. Flip it with PATCH /api/demo/scenario and reload: the panel follows, live values, not fixtures."),
			code("env-cfg-handler", "go",
				"The wiring behind the panel: constructor injection — the real cfg and store at construction, no globals, no re-loading.",
				"server/internal/presentation/demo_env.go",
				"func NewEnvInspectorHandler(cfg core.Config, store *core.ScenarioStore, logger *slog.Logger) *EnvInspectorHandler {\n"+
					"	return &EnvInspectorHandler{cfg: cfg, store: store, logger: logger, getenv: os.Getenv}\n"+
					"}\n"+
					"\n"+
					"// …\n"+
					"func (h *EnvInspectorHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {\n"+
					"	writeJSON(w, http.StatusOK, demos.BuildEnvInspectorPayload(h.cfg, h.getenv, h.store.Snapshot()))\n"+
					"}"),
			text("env-cfg-redact-honesty",
				"READ THE POLICY'S EDGES as tests, because they are: the suite table-tests ``RedactCredential`` against a postgres DSN with a password in userinfo, a mongo URI with credentials and an ``authSource`` query, and a cloud-style DSN carrying its token in a ``password=`` query parameter — and it marshals the WHOLE payload to JSON and asserts the credentials appear nowhere, not in any field, note, or caption. Two design details carry the weight: the query is dropped ENTIRELY rather than filtered key-by-key, because a query-parameter denylist is a scheme-by-scheme maintenance race the policy refuses to run; and a value that does not parse as a URL is hidden outright (``(hidden)``), never echoed on trust — the rule from the secrets section arriving early, stated as code. The attribution column has its own honesty note, stated on the payload itself: VALUES are what the process booted with (validated, typed, immutable since), while ATTRIBUTION re-reads the environment at request time — set a variable after boot and the row honestly reports ``environment`` beside the boot-time value, the one moment the two columns can diverge."),
		),
		// ── 4. Env-specific behavior ──────────────────────────────
		section("env-specific-behavior", "Environment-specific behavior: the honest list",
			"What actually changes between dev and prod in a well-factored service — and the bigger fact this repo leans on: the process contains no environment ifs; the PLATFORM differs, not the code.",
			text("env-beh-env-field",
				"THE ONE THING THE CODE KNOWS about its environment is the ``Env`` field — a free-form name (``dev``, ``staging``, ``prod``) that exists to be REPORTED, not branched on. It flows exactly where an operator needs it: the health probe answers with it, so a load balancer or a curious colleague can ask ``which environment is this`` of any running instance and get the boot-time truth; and the boot's own log line carries it beside the address (the ``server listening`` line with its ``env`` attribute), so the first line of every process's output states where it believes it is. The health handler is small enough to quote whole, because its shape IS the lesson — the env is a value the process HOLDS and SERVES, not a switch it consults:"),
			code("env-beh-health", "go",
				"The env field's whole job: reported by the probe, never branched on by the code.",
				"server/internal/presentation/health.go",
				"// HealthHandler serves GET /api/health, the family liveness probe.\n"+
					"type HealthHandler struct {\n"+
					"	env string\n"+
					"}\n"+
					"\n"+
					"func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {\n"+
					"	// …\n"+
					"	_ = json.NewEncoder(w).Encode(healthResponse{Status: \"ok\", Env: h.env})\n"+
					"}"),
			list("env-beh-changes",
				false,
				"Log level — dev runs debug or info as the operator prefers (GDG_LOG_LEVEL), prod settles at info; the level is a boot-time config value, so changing it is a restart, by design.",
				"pprof exposure — the /internal/ mount exists in every environment, but the FENCE is what differs: dev reaches it from the workstation, prod admits nothing under /internal/ past the ingress — the same binary, a different network edge.",
				"Error detail — this guide serves the same honest envelope everywhere by deliberate stance; where a stricter deployment hides internal detail, the change lives in the envelope writer, once — not in an if per handler.",
				"Rate limits — the security demo's limiter is deliberately demo-scale (2/s sustained) so a curl loop can reach the 429; production tunes the numbers, not the mechanism.",
				"Datastore addresses — dev's compose publishes host ports and the defaults point at service names; prod's datastores sit on an internal network with no published ports, and the DSNs arrive as injected environment variables.",
			),
			code("env-beh-prod", "yaml",
				"The prod declaration: the same server image — its environment injected by the platform, DSN on the required-variable secret rail — one published port, and an internal network the datastores never leave.",
				"deploy/docker-compose.prod.yml",
				"  server:\n"+
					"    build:\n"+
					"      context: ./../\n"+
					"      dockerfile: ./.docker/server/Dockerfile.prod\n"+
					"    image: gdg_server_prod\n"+
					"    container_name: go_developers_guide_server_prod\n"+
					"    restart: unless-stopped\n"+
					"    environment:\n"+
					"      GDG_ENV: prod\n"+
					"      GDG_LOG_LEVEL: info\n"+
					"      GDG_REDIS_ADDR: redis:6379\n"+
					"      GDG_MONGO_URI: mongodb://mongo:27017\n"+
					"      GDG_DATABASE_URL: postgres://guide:${POSTGRES_PASSWORD:?export POSTGRES_PASSWORD before compose up}@postgres:5432/guide?sslmode=disable\n"+
					"    ports:\n"+
					"      - \"8100:8100\"\n"+
					"    networks:\n"+
					"      - app_internal"),
			text("env-beh-no-ifs",
				"THE BIGGER HONEST FACT is what that list does not contain: anywhere the code consults its environment to choose behavior. There is no ``if cfg.Env == \"prod\"`` in this codebase — not because services never need env-specific behavior, but because the twelve-factor stance assigns those differences to the right layer: the PLATFORM (compose files, ingress rules, injected config) differs between environments, while the PROCESS stays one program with one code path, tested once. When a difference genuinely belongs in code — a behavior that must flip WITHOUT a restart, per-request, under an operator's hand — that is not environment config anymore; it is the feature-flag decision, and this guide's answer to it is real machinery: the scenario store the next-but-one section dissects. The border between the three — platform differences, config values, runtime flags — is the whole discipline of this topic, and every item on the honest list above knows which side of each border it lives on."),
		),
		// ── 5. Secrets beyond .env ────────────────────────────────
		section("secrets", "Secrets beyond .env",
			"What never goes in code or git, the .gitignore discipline that enforces it, how the dev environment injects its values, and how secret managers earn their place — by comparison, not by fashion.",
			text("env-sec-never",
				"WHAT NEVER GOES IN CODE OR GIT is a shorter list than people hope, and it admits no exceptions: CREDENTIALS — anything whose leakage breaks an authentication boundary. Not because git history is hostile by default, but because it is FOREVER: a secret pushed once lives in the object database across clones, forks, and backups long after the working tree has moved on, and rotation becomes the only remediation. The local-file habit this rule produces — the ``.env`` file, loaded by some dev tooling — is fine as a CONVENTION and fatal as a repository contents, so this repo states the rule where git enforces it, in the ignore file, with the one deliberate exception the family pattern allows: a committed, PUBLIC-BY-DESIGN template (the client's ``.env.example``), whose values are placeholders, never secrets:"),
			code("env-sec-gitignore", "text",
				"The ignore rule and its one sanctioned exception — templates are public, values are not.",
				"",
				"# Env files stay out of git — committed non-secret templates (if any arise)\n"+
					"# get an explicit `!path` negation here, fastapi-family style. The client\n"+
					"# build-env template is public-by-design (Umami placeholders, empty = off).\n"+
					".env*\n"+
					"!client/.env.example"),
			list("env-sec-what-counts",
				false,
				"DSN passwords — the credential half of any postgres/mongo connection string, including percent-encoded forms; this guide's dev-grade pair (postgres/guide) is dev-network-only, and the prod compose says so where a reviewer reads it.",
				"API keys and tokens — third-party service credentials, webhook signing secrets, cloud DSNs whose whole authority is one query parameter.",
				"Signing keys — JWT secrets and cookie seals; this guide's auth console MINTS ephemeral 256-bit keys in memory precisely so no real key sits in demo code, and a real deployment injects them as environment variables.",
				"Private keys and certificates — TLS server keys, CA keys, code-signing material; the rule is identical to passwords with worse blast radius.",
				"Anything a redaction policy would hide — the practical test: if the config table's policy would redact it before display (the demo's rule), it is a secret, whatever its type is called.",
			),
			text("env-sec-injection",
				"THE DEV STORY IS INJECTION, not files: the compose environment block and the devcontainer you met in the first section ARE the dev secret store — values declared in a local-only declaration, handed to processes as environment variables at start. The application's contract never changes: ``core.LoadConfig`` reads ``getenv``; WHO fills the environment is the platform's business. That single seam is what makes the ladder climb cheap: dev composes the values in; a real deployment's orchestrator (compose again, Kubernetes secrets as env, a task-definition secret) injects them from a store with access control and audit; and a full SECRET MANAGER (Vault, AWS Secrets Manager, GCP Secret Manager) adds rotation, leasing, and per-service authorization on top — the app still just reads environment variables at boot (or fetches on demand through the manager's SDK, a coupling the env-injection variant avoids). The honest trade-off, stated once: env-injected secrets are values in a PROCESS SNAPSHOT — visible in /proc, in crash dumps, in the inspector-style tooling this very topic builds, which is exactly why this topic's demo redacts by policy rather than by hope."),
			text("env-sec-lifecycle",
				"THE LIFECYCLE DISCIPLINE rounds the section out, because injection is the beginning of a secret's life, not the end. ROTATE on a schedule and on every suspicion — the env-injection shape makes rotation a restart, which disposability (factor IX) keeps cheap. EXPOSE MINIMALLY: one variable per secret, scoped to the service that needs it, never a shared god-.env. AUDIT YOURSELF before every commit: the practical rule this topic practices is a grep of the diff for format calls and literals touching config — the demo's redaction tests exist because ``NEVER print a secret`` needs a machine to hold the line when the human is tired. And PREPARE FOR THE BAD DAY: know, per secret, the rotation procedure and the blast radius — a leaked dev-grade pair (this repo's own) costs a local network; a leaked signing key costs every token it ever signed. The security topic reprises these duties at the threat-model level; here, the rule is the repo's own: secrets flow IN through the environment, and NOTHING sensitive flows OUT through a response, a log line, or a demo payload."),
		),
		// ── 6. Feature flags ──────────────────────────────────────
		section("feature-flags", "Feature flags: the ScenarioStore is the real machinery",
			"A feature flag is a behavior switch flippable at run time, without a restart. This guide ships the real thing — the ScenarioStore: typed enums, validated patches, live everywhere the demos read it — and this section teaches its trade-offs honestly.",
			text("env-ff-what",
				"THE DISTINCTION FROM CONFIG is the defining property: config is read AT BOOT and changing it means a restart, while a feature flag is behavior that flips WHILE THE PROCESS RUNS, per operator action, without a deploy. The use cases follow from that property — gradual rollout (enable for a canary, widen), kill switches (disable a degraded dependency's path), and rehearsal (exercise the failure path before production meets it) — and so do the hazards, which is why the machinery deserves the same rigor as config. This guide's flags are REAL, not illustrative: every demo in the guide reads the ScenarioStore, ``PATCH /api/demo/scenario`` flips it through the production HTTP surface, and the switch's consequences are live — fake mode answers from the in-memory adapter while live mode reads the migrated PostgreSQL, and the error/latency injections really do change what the next request experiences. The store's discipline is in its patch path — typed enums, validated, atomic copy-in/copy-out under a mutex:"),
			code("env-ff-store", "go",
				"Flags as typed values: an unknown mode is refused with the legal values named — the same fail-fast stance config holds at boot, held here per PATCH.",
				"server/internal/core/scenario.go",
				"	if patch.DataMode != nil && !patch.DataMode.Valid() {\n"+
					"		return s.current, ValidationFailed(\n"+
					"			\"Invalid scenario patch: unknown data mode.\",\n"+
					"			map[string]any{\"dataMode\": string(*patch.DataMode), \"expected\": []string{string(DataModeFake), string(DataModeLive)}},\n"+
					"		)\n"+
					"	}\n"+
					"	// …\n"+
					"	next := s.current\n"+
					"	if patch.DataMode != nil {\n"+
					"		next.DataMode = *patch.DataMode\n"+
					"	}\n"+
					"	if patch.InjectFailure != nil {\n"+
					"		next.InjectFailure = *patch.InjectFailure\n"+
					"	}\n"+
					"	s.current = next\n"+
					"	return next, nil"),
			text("env-ff-tradeoffs",
				"FLAGS-AS-CONFIG VERSUS FLAGS-AS-CODE is the trade this section wants you to make deliberately. Flags in config files or environment variables are reviewable in git, free to reason about (the running process's behavior is fixed at boot), and cost a restart to change — the right shape for structural switches (which datastore dialect, whether an entire surface is mounted). Flags as RUNTIME STATE — this store, or a managed service — flip without a restart and per-environment-instance, but they acquire everything state acquires: a write path that must validate (the quote above), a read path that must be race-free (the store's copy-in/copy-out under an RWMutex, race-detector-proven), and — the honest limit this guide states rather than hides — PROCESS LOCALITY: the store is in-memory per process, so a restart resets to defaults and a fleet of replicas would disagree until a shared store replaced it. A managed flag service buys the shared state, the audit trail, and the targeting rules, at the price of a network dependency on every decision and a vendor in your control plane; the demo-scale store buys none of that and pretends to none of it."),
			text("env-ff-live",
				"THE INSPECTOR'S FLAG PANEL closes the loop between this section and the demo: it is not a fixture labeled ``flags`` — it is the ScenarioStore's CURRENT state, snapshotted at request time and rendered, so the flag machinery and the config machinery sit in one payload and the difference between them is visible as BEHAVIOR. The terminal companion at this topic's end walks the exact sequence an operator would: read the panel (fake/none), PATCH the store (``{\"dataMode\":\"live\",\"injectFailure\":\"latency\"}``), reload the panel to watch it follow, and restore the defaults — leaving the guide as it was found, which is flag hygiene in miniature: whoever flips a switch owns putting it back. The aging rule is the last discipline: a flag is a fork in the codebase's behavior, and every fork left open is a path nobody exercises — retire flags when their question is answered, the way the scorecard retires a verdict when the machinery changes."),
		),
		// ── 7. Terminal companion ─────────────────────────────────
		section("terminal-companion", "Terminal companion: the environments tour",
			"The topic over a terminal: the inspector's live table read raw, the redaction proven on the wire, a flag flip reflected and restored, and a scratch binary driven through GDG_PORT/GDG_LOG_LEVEL overrides and the fail-fast refusals.",
			text("term-env-intro",
				"With the stack up (server on :8100), the tour runs in the topic's own order. First the INSPECTOR — this process's resolved config, read raw: the environment name, then every variable as a row (name, source, value, sensitivity), the scorecard's twelve verdicts, and the flag panel. Then the REDACTION PROOF made runnable: the served DSN is checked to carry no password — the row shows ``postgres@postgres:5432`` while the running process's actual DSN carries the credential — and no query string survives either. Then the FLAG FLIP: PATCH the scenario, reload the inspector, watch the panel follow, restore — the operator's sequence in four commands. Finally the SCRATCH BINARY: build the server once to /tmp, refuse it a valid config (a non-numeric port names ``GDG_PORT`` and exits non-zero before serving), then boot it with overrides — a second port, a louder log level — and read the differences out of its own health probe and boot logs before shutting it down:"),
			code("term-env-inspector-bash", "bash", "", "",
				"# the inspector: this process's resolved config, live\n"+
					"curl -s localhost:8100/api/demo/env/inspector | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); \\\n"+
					"   print(\"env:\", p[\"env\"]); \\\n"+
					"   [print(\" \", v[\"envVar\"].ljust(17), v[\"source\"].ljust(12), v[\"value\"]) \\\n"+
					"    for v in p[\"variables\"]]; \\\n"+
					"   print(\"scorecard:\", \", \".join(r[\"verdict\"] for r in p[\"twelveFactor\"])); \\\n"+
					"   print(\"flags:\", p[\"flags\"][\"dataMode\"], \"/\", p[\"flags\"][\"injectFailure\"])'\n"+
					"\n"+
					"# the redaction proof: no password, no query, anywhere in the payload\n"+
					"curl -s localhost:8100/api/demo/env/inspector | \\\n"+
					"  python3 -c 'import json,sys; p=json.load(sys.stdin); \\\n"+
					"   d=[v for v in p[\"variables\"] if v[\"envVar\"]==\"GDG_DATABASE_URL\"][0]; \\\n"+
					"   print(\"served DSN:\", d[\"value\"]); \\\n"+
					"   assert \":guide@\" not in json.dumps(p) and \"?sslmode\" not in json.dumps(p), \"LEAK\"'\n"+
					"\n"+
					"# the flag flip: panel follows the store, then restore the defaults\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'content-type: application/json' \\\n"+
					"  -d '{\"dataMode\":\"live\",\"injectFailure\":\"latency\"}' > /dev/null\n"+
					"curl -s localhost:8100/api/demo/env/inspector | \\\n"+
					"  python3 -c 'import json,sys; print(\"flags now:\", json.load(sys.stdin)[\"flags\"])'\n"+
					"curl -s -X PATCH localhost:8100/api/demo/scenario \\\n"+
					"  -H 'content-type: application/json' \\\n"+
					"  -d '{\"dataMode\":\"fake\",\"injectFailure\":\"none\"}' > /dev/null"),
			code("term-env-scratch-bash", "bash", "", "",
				"# a scratch binary of the same codebase, driven through its config surface\n"+
					"cd /workspaces/go_developers_guide/server && \\\n"+
					"  go build -o /tmp/gdg-env-scratch ./cmd/guide\n"+
					"\n"+
					"# fail-fast: a malformed port names the variable and exits non-zero\n"+
					"GDG_PORT=not-a-port /tmp/gdg-env-scratch; echo \"exit=$?\"\n"+
					"GDG_LOG_LEVEL=bogus /tmp/gdg-env-scratch 2>&1 | head -1; echo \"exit=$?\"\n"+
					"\n"+
					"# overrides: a second instance, louder logs, its own port — then prove it\n"+
					"GDG_PORT=8199 GDG_LOG_LEVEL=debug /tmp/gdg-env-scratch > /tmp/gdg-scratch.log 2>&1 &\n"+
					"sleep 3 && head -3 /tmp/gdg-scratch.log\n"+
					"curl -s localhost:8199/api/health; echo\n"+
					"kill %1   # the scratch instance stands down; the guide on :8100 never noticed"),
			text("term-env-coda",
				"Read the tour as the topic's argument in terminal form. The inspector block is the RESOLVED-CONFIG TABLE as data — every row derived from the live process, its attribution column proving the loader's rule on the real environment. The redaction check is not a courtesy: it asserts, on the wire, that the credential never appears — the policy's test, run by anyone, against the running server, in one line. The flag block is the CONFIG-VERSUS-FLAGS boundary as behavior: the PATCH changes what the next inspector GET reports while the process keeps running — no restart, no rebuild — and the restore is the hygiene rule honored. And the scratch-binary block is the whole one-codebase claim made executable: the SAME bytes refuse bad config with the variable named, boot differently under different environments, and report their own environment back through the probe — one program, many environments, every difference on the outside of the binary. Environments close the platform group; the next group ships: deployment, performance, reliability, scaling."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"environments",
		"Environments in Go: one codebase, many environments",
		"How one Go codebase serves dev, staging, and prod from "+
			"the same bytes while everything that differs becomes "+
			"CONFIG — taught from this repo's own artifacts. One "+
			"codebase: the build's output is immutable (a static "+
			"binary you can checksum between CI and prod); the "+
			"environment is a DECLARATION beside the code — the "+
			"devcontainer and compose files quoted as the dev "+
			"instance, with what varies (backing services, config "+
			"injection, toolchain) and what never does (the module, "+
			"the embedded migrations, identically pinned engine "+
			"versions). The twelve-factor app walked honestly "+
			"against real machinery: ten hits each named to the "+
			"code that earned them, and three DELIBERATE differs "+
			"(the in-process jobs worker, the devcontainer-vs-"+
			"distroless parity gap, boot-time embedded migrations) "+
			"recorded as trade-offs — no dogma, in either "+
			"direction. Typed environment config with core."+
			"LoadConfig as the case study: the GDG_* contract, "+
			"dev-safe secret-free defaults that make dev zero-"+
			"config, validation-at-boot that NAMES the failing "+
			"variable, the fail-fast composition root quoted, and "+
			"the honest boundary where DSN validation belongs to "+
			"the driver's dial. The topic's demo, env:inspector: "+
			"THIS process's resolved config derived at request "+
			"time — every variable with environment-vs-default "+
			"attribution (the loader's own rule, re-applied), "+
			"credentials REDACTED by a table-tested policy (DSN "+
			"password and the ENTIRE query never reach the wire; "+
			"unparseable values hidden outright), the twelve-"+
			"factor scorecard as data, and the ScenarioStore's "+
			"CURRENT state as the live flag panel. Environment-"+
			"specific behavior as the honest list — log levels, "+
			"the pprof fence, error detail, rate limits, "+
			"datastore addresses — beside the bigger fact that "+
			"the code holds no env ifs: the PLATFORM differs, "+
			"not the process. Secrets beyond .env: what never "+
			"goes in code or git, the .gitignore discipline with "+
			"its one public-template exception, compose/devcontainer "+
			"injection as the dev story, secret managers by "+
			"comparison (rotation, leasing, per-service "+
			"authorization — the app still reads getenv), and the "+
			"lifecycle duties (rotate, expose minimally, grep your "+
			"diff, know the blast radius). Feature flags: the "+
			"ScenarioStore as the guide's REAL machinery — "+
			"runtime-flippable, typed enums validated per PATCH, "+
			"race-free copy-in/copy-out — against flags-as-config "+
			"trade-offs and the honest process-locality limit. "+
			"Terminal companion: the inspector read raw, the "+
			"redaction asserted on the wire, a flag flip "+
			"reflected and restored, and a scratch binary driven "+
			"through GDG_PORT/GDG_LOG_LEVEL overrides and the "+
			"fail-fast refusals.",
		"platform",
		36,
		sections,
	)
}
