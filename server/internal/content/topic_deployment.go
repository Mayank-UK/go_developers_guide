package content

import (
	"errors"
	"fmt"
)

// This file is topic 37 deployment (group ship, curriculum order 37):
// what it means to SHIP a Go service — build an immutable artifact,
// declare how it runs, and keep the process honest at the operating-
// system level. Every artifact this topic teaches is REAL and lives in
// the repo: the multi-stage distroless Dockerfile the prod stack has
// built since Phase 0 (.docker/server/Dockerfile.prod), the full prod
// compose declaration with dependency conditions, healthchecks,
// resource hints, and a required-variable secret rail
// (deploy/docker-compose.prod.yml), the nginx reverse proxy the client
// image actually serves (.docker/client/nginx.conf), and the
// Kubernetes manifests under deploy/k8s/ — written as JSON so the
// stdlib encoding/json parses them with no YAML dependency, a choice
// the topic teaches. The graceful-shutdown section quotes the
// composition root's REAL drain order (gRPC first, HTTP bounded, jobs
// worker last); the CI/CD section states the family stance (no hosted
// CI — local verification is the only gate) and shows what a pipeline
// would run; parity/secrets carries the environments topic's config
// parity into the never-in-an-image secret rule, enforced for real by
// the demo's scanner. The topic's demo, deploy:stack: the server
// parses those very files at request time (the compose file with the
// guide's zero-dep YAML-subset reader, the manifests with
// encoding/json) and serves the derived topology with per-artifact
// provenance — no invented values. Linux survival closes the floor:
// signals, exit codes, PID 1, and ports — the facts an operator needs
// at 3 a.m. The terminal companion curls the demo, proves the
// compose rail refuses and then validates, and dry-run-validates the
// manifests the way the repo actually does.

// deploymentTopic builds topic 37.
func deploymentTopic() (Topic, error) {
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
		// ── 1. Ship vs run ────────────────────────────────────────
		section("ship-vs-run", "Ship versus run: the artifact and the process",
			"Deployment is two different jobs that blur together only when done badly: building an immutable artifact, and declaring how that artifact runs. This guide's artifact chain — go build, distroless image, compose/k8s declaration — is the concrete split.",
			text("dep-svr-two-jobs",
				"TWO JOBS, NOT ONE, is the frame everything else in this topic hangs on. SHIPPING is producing an artifact: compiling the server, wrapping it in an image, tagging that image — work that happens ONCE per release, on a build machine, with no secrets and no environment attached. RUNNING is everything after: starting processes of that artifact, wiring their environment, restarting them, replacing them with the next release. When the two smear together — a server compiled by hand on the machine it serves, an image rebuilt slightly differently per host — every environment quietly becomes its own program, and ``it works on my machine`` stops being a joke and becomes a diagnosis you cannot refute. The discipline is the same one-codebase rule the environments topic taught, pushed one step further down the pipeline: one BUILD serves every environment, and every difference between environments attaches at RUN time as config, never by touching the artifact."),
			text("dep-svr-chain",
				"THIS REPO'S ARTIFACT CHAIN is short enough to hold in your head, and every link in it is a real file quoted somewhere in this topic. The compiler emits one static binary (``go build ./cmd/guide`` → a single ``guide`` executable — the next section's whole subject). The Dockerfile wraps that binary in a distroless image — the multi-stage build that ends ``FROM gcr.io/distroless/static-debian12`` — and the image is tagged ``gdg_server_prod``. The declarations then say how it runs: the full prod compose file for a single host, the Kubernetes manifests for a cluster. A RELEASE, in this vocabulary, is an immutable artifact plus a configuration: the same image digest, a different environment. Rollbacks become artifact swaps; audits become image digests; the question ``what exactly is serving prod right now`` becomes answerable by checksum, not by archaeology."),
			list("dep-svr-artifact-chain",
				false,
				"Build — go build ./cmd/guide with CGO disabled: one static binary, reproducible from the module's go.mod + go.sum alone.",
				"Package — the multi-stage Dockerfile copies that binary into distroless: an image with no shell, no package manager, and nothing to attack but your own code.",
				"Declare — deploy/docker-compose.prod.yml (single host) and deploy/k8s/ (cluster) say what runs, what it depends on, and how it is probed.",
				"Release — the image digest plus injected config; environments differ by injection, never by rebuilding the artifact.",
				"Run — the process serves; signals, probes, and resource limits are the operating system's business, negotiated through the declarations above.",
			),
			text("dep-svr-why-it-matters",
				"WHY THE SPLIT EARNS ITS KEEP shows up the first time something goes wrong at scale. A bug report from prod names a version; you pull the exact image by digest and run it locally with the dev compose's datastores — the artifact is identical, so the difference hunt collapses to configuration. A security scan runs against the IMAGE, once, and its verdict covers every environment that will ever run it. A rollout replaces artifact A with artifact B under rules the platform enforces (this topic's rollouts section), instead of a human copying files onto hosts and hoping. None of that is available to a codebase that ships by rebuilding on each target — which is why Go's toolchain meeting this discipline halfway matters so much: the language's static binary makes the artifact side of the split almost free, and the rest of this topic is the run side done honestly."),
		),
		// ── 2. The static-binary advantage ────────────────────────
		section("static-binary", "The static-binary advantage",
			"CGO_ENABLED=0 makes the Go compiler emit one self-contained file — no interpreter, no libc, no runtime to install. That single fact reshapes deployment for Go services.",
			text("dep-sb-what-it-is",
				"THE COMPILE FLAG THAT CHANGES DEPLOYMENT is one environment variable: ``CGO_ENABLED=0 go build`` tells the toolchain to refuse any dependency on C and link everything statically. The output is a single file that contains the program AND the entire runtime it needs — goroutine scheduler, garbage collector, networking stack, crypto — with ``ldd`` reporting ``not a dynamic executable`` because there is nothing dynamic in it. The consequences compound. The file runs on ANY Linux that can execute an ELF binary, from the fattest Ubuntu to a ``FROM scratch`` image with literally nothing in it. There is no interpreter directory to version-match (the Python and Ruby deployment problem), no ``glibc``-versus-``musl`` saga (the C problem), no ``NODE_ENV``-shaped runtime to configure. Version upgrades of the operating system cannot break a binary that does not call into the operating system's libraries beyond the raw syscall interface."),
			code("dep-sb-build-line", "dockerfile",
				"The guide's own build line: static, reproducible, path-stripped, symbol-stripped.",
				".docker/server/Dockerfile.prod",
				"RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags=\"-s -w\" -o /out/guide ./cmd/guide"),
			text("dep-sb-what-it-buys",
				"WHAT THE GUIDE DOES WITH THAT is visible in the flags on the quoted line and in everything the rest of this topic builds on them. ``-trimpath`` strips the build machine's absolute paths out of the binary, so two builds from two machines produce comparable stack traces — the reproducibility habit. ``-s -w`` strips the symbol table and DWARF debug info, trading a smaller debug story (``dlv`` against the stripped binary is limited; the debugging topic's pprof story survives, because Go's runtime profiles carry their own line tables) for a meaningfully smaller artifact. ``GOOS=linux`` pins the target — the same toolchain cross-compiles to any OS/arch pair with a flag, which is the moment you internalize that Go deployment is not PORTING, it is SELECTING a target. And the binary named ``guide`` that falls out is the ENTIRE server: content registry, migrations, every demo this guide serves."),
			text("dep-sb-honest-edge",
				"THE HONEST EDGE of the static stance is ``cgo``: the moment your program imports something that needs C — a database driver built on a C client library, a crypto library with hand-tuned assembly behind a C interface, anything cgo-shaped — static linking stops being a flag and becomes a decision. The guide's stack is deliberately pure Go end to end (``pgx`` speaks PostgreSQL's wire protocol in Go; the mongo and redis drivers likewise), so ``CGO_ENABLED=0`` costs nothing. A cgo-bound service keeps most of the story — multi-stage builds, small runtime images — but its final image needs the C runtime its binary links against, ``scratch`` is off the table, and the distroless family's ``static`` variant gives way to the heavier ones. Rule of thumb worth internalizing now: treat every new cgo dependency as a deployment decision, not just a dependency decision."),
		),
		// ── 3. Distroless multi-stage builds ──────────────────────
		section("distroless-build", "Distroless multi-stage builds",
			"The guide's real prod Dockerfile: a pinned builder stage compiles the binary, a distroless runtime stage carries nothing but the binary and a non-root user. Quoted whole, because it is the artifact this topic ships.",
			text("dep-db-two-stages",
				"A MULTI-STAGE BUILD IS TWO WORLDS in one file, connected by a single ``COPY --from``. The FIRST stage is a build environment: the pinned ``golang:1.27.3-bookworm`` toolchain, the module manifests, the compiler flags. It is fat by design — it has everything a build could need — and it is DISCARDED at the end. The SECOND stage is the runtime: ``gcr.io/distroless/static-debian12``, Google's deliberately minimal image family — no shell, no package manager, no libc even in the static variant, just enough filesystem to run a static binary as a non-root user (plus CA certificates and timezone data so TLS and time zones work). The final image is the guide's entire production attack surface: one static executable. Nothing else to exploit, nothing else to scan, nothing else to patch — the CVE story collapses to ``is my Go version current`` and ``is my own code sound``."),
			code("dep-db-dockerfile", "dockerfile",
				"The real prod Dockerfile, whole: pinned builder, manifest-first layer caching, distroless non-root runtime.",
				".docker/server/Dockerfile.prod",
				"FROM golang:1.27.3-bookworm AS build\n"+
					"\n"+
					"WORKDIR /app\n"+
					"\n"+
					"COPY server/go.mod server/go.sum ./\n"+
					"RUN go mod download\n"+
					"\n"+
					"COPY server/ ./\n"+
					"RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags=\"-s -w\" -o /out/guide ./cmd/guide\n"+
					"\n"+
					"FROM gcr.io/distroless/static-debian12\n"+
					"\n"+
					"WORKDIR /app\n"+
					"COPY --from=build /out/guide /app/guide\n"+
					"\n"+
					"USER nonroot:nonroot\n"+
					"EXPOSE 8100\n"+
					"ENTRYPOINT [\"/app/guide\"]"),
			text("dep-db-why-each-line",
				"EVERY LINE IN THAT FILE EARNS ITS PLACE, and the order is load-bearing. ``COPY server/go.mod server/go.sum`` before ``COPY server/`` is Docker layer caching doing dependency management: the manifests change rarely, the source changes constantly, so ``go mod download`` sits in its own cached layer and daily rebuilds recompile code without re-downloading modules (the dependency topic walked this same caching story from the volume side — the ``gdg-go`` and ``gdg-gocache`` named volumes keep IN-CONTAINER builds warm; this layer ordering keeps IMAGE builds warm). ``USER nonroot:nonroot`` is the distroless image's built-in unprivileged account — the process never holds uid 0, so even a hypothetical code-execution bug has nothing to reach for. ``EXPOSE 8100`` is documentation, not enforcement — the declaration that matters is the compose file's port mapping. And ``ENTRYPOINT`` (not ``CMD``) makes the binary THE process: no shell wrapper to interpret signals badly, which the graceful-shutdown and Linux-survival sections both depend on."),
			list("dep-db-what-you-give-up",
				false,
				"No shell — docker exec into the container gets you nothing; debugging happens through the process's own surfaces (pprof, /metrics, logs), which the observability topic already built.",
				"No package manager — nothing to install, therefore nothing to forget to update; the image is rebuilt, never patched in place.",
				"No healthcheck tooling — a Docker HEALTHCHECK has nothing to exec; probing moves to the platform layer (kubelet HTTP probes, or the client's proxied check), a trade the compose section owns honestly.",
				"No interactive rescue — if the process is wedged, the answer is restart and profile, not SSH-in; disposability replaces hand surgery.",
			),
			text("dep-db-scratch-vs-distroless",
				"SCRATCH VERSUS DISTROLESS is the last refinement worth making explicit, because ``FROM scratch`` is seductive: an image with ZERO bytes of distribution in it, the static-binary story at its logical limit. The guide chose distroless-static for three concrete reasons, all visible in what the server does at boot and in tests. CA CERTIFICATES: the server dials PostgreSQL over TLS-capable DSNs and would verify certificates for any ``https`` call — scratch ships no trust store, and a binary that cannot verify certificates fails in ways that look like network mystery. TIMEZONE DATA: time zone handling walks the IANA database; scratch has none. And an /etc/passwd entry: distroless's ``nonroot`` user gives ``USER nonroot:nonroot`` a name to resolve; scratch makes you manage uid mapping yourself. If you truly need scratch's nothing-at-all, those three files are the checklist you re-implement — and the honest question to ask first is what the last few megabytes of image are buying you."),
		),
		// ── 4. Compose: the full stack [demo deploy:stack] ─────────
		section("compose-stack", "Compose: declaring the full stack",
			"The deployment topic's own artifact: deploy/docker-compose.prod.yml declares server + client + three datastores with dependency conditions, healthchecks, resource hints, and a required-variable secret rail — and the demo parses this very file live.",
			text("dep-cs-what-the-file-is",
				"THE COMPOSE FILE IS THE SINGLE-HOST RUN DECLARATION, and this topic's version is the full one: five services — the distroless server, the nginx client that fronts it, and the three datastores — on one internal network, with only two published front doors (8100 for the API, 8101 for the client). Three disciplines separate it from a naive ``up``. START ORDER BECOMES DEPENDENCY CONDITIONS: the server declares ``condition: service_healthy`` on all three datastores, so compose waits for ``pg_isready`` and friends before starting the process that will immediately migrate PostgreSQL — no more hoping list order is enough. RESOURCE HINTS LAND ON EVERY CONTAINER: cpu and memory limits plus reservations, so a runaway query or a memory leak is bounded by declaration rather than by whatever the host had left. And THE ONE TRUE SECRET rides an interpolation rail — the committed file contains ``${POSTGRES_PASSWORD:?…}``, which makes ``docker compose up`` REFUSE to start until the operator exports the real password; compose splices it into the DSN and the datastore's own environment at start time, and the file that reaches git never held it."),
			code("dep-cs-server-block", "yaml",
				"The server's declaration: env rails, published port, dependency conditions, resource hints — and deliberately no Docker healthcheck.",
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
					"      - app_internal\n"+
					"    depends_on:\n"+
					"      postgres:\n"+
					"        condition: service_healthy\n"+
					"      redis:\n"+
					"        condition: service_healthy\n"+
					"      mongo:\n"+
					"        condition: service_healthy\n"+
					"    deploy:\n"+
					"      resources:\n"+
					"        limits:\n"+
					"          cpus: \"1.0\"\n"+
					"          memory: 256M\n"+
					"        reservations:\n"+
					"          cpus: \"0.25\"\n"+
					"          memory: 64M"),
			text("dep-cs-healthcheck-honesty",
				"THE HEALTHCHECK GAP YOU JUST NOTICED is real and deliberate: the file healthchecks four of five services and gives the server none — not an omission, a consequence. The datastore images ship their own probe tools (``pg_isready``, ``redis-cli``, ``mongosh``), and the nginx client ships busybox ``wget``, so their checks are one line each. The distroless server ships NOTHING but the binary — there is no shell for ``CMD-SHELL`` to invoke, no ``wget`` to spider with — so a Docker healthcheck on it is impossible without breaking the image's whole reason for existing. The honest answer is to move process probing UP a layer, and the stack does it twice: the client's healthcheck walks the proxy path (``wget --spider http://127.0.0.1/api/health`` through nginx's ``/api/`` proxy to the server), proving the serving path end to end, and the Kubernetes manifests later in this topic give the server REAL HTTP probes driven by the kubelet, which needs nothing inside the container at all. The lesson generalizes: WHERE a probe runs is a design decision, and minimal images push it toward the platform."),
			code("dep-cs-client-check", "yaml",
				"The client's check proves the whole serving path: nginx proxies /api/ to the server, so a 200 here is client AND server AND the proxy between them.",
				"deploy/docker-compose.prod.yml",
				"    healthcheck:\n"+
					"      test: [\"CMD\", \"wget\", \"--spider\", \"-q\", \"http://127.0.0.1/api/health\"]\n"+
					"      interval: 30s\n"+
					"      timeout: 5s\n"+
					"      retries: 3\n"+
					"      start_period: 10s"),
			code("dep-cs-postgres-rail", "yaml",
				"The same rail feeds both ends: the datastore's password and the server's DSN interpolate from one exported variable — never committed.",
				"deploy/docker-compose.prod.yml",
				"  postgres:\n"+
					"    image: postgres:17\n"+
					"    container_name: go_developers_guide_postgres_prod\n"+
					"    restart: unless-stopped\n"+
					"    environment:\n"+
					"      POSTGRES_USER: guide\n"+
					"      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?export POSTGRES_PASSWORD before compose up}\n"+
					"      POSTGRES_DB: guide"),
			code("dep-cs-networks", "yaml",
				"The network declaration: everything resolves by service name inside app_internal; only the two front doors publish.",
				"deploy/docker-compose.prod.yml",
				"networks:\n"+
					"  app_internal: {}"),
			text("dep-cs-demo-intro",
				"THE DEMO MAKES THE DECLARATION INSPECTABLE, and its construction is the topic's thesis in miniature: the server parses THESE VERY FILES at request time — the compose bytes through the guide's own zero-dependency YAML-subset reader (the same parser the OpenAPI toolchain uses, from ``internal/apispec``), the Kubernetes manifests through ``encoding/json`` — and derives everything it shows: services, published ports, healthchecks, dependency conditions, resource hints, probe paths, rollout knobs. No value on the wire is invented; every fact names the file it came from. The payloads you see below ARE the artifacts above, parsed — which means the demo can never drift from the declaration it explains, and the suite pins that by parsing the same files in tests and refusing any secret value that is not a ``CHANGE_ME_*`` placeholder:"),
			demo("deploy:stack", "The deployment stack viewer: this repo's real artifacts, parsed live",
				"GET /api/demo/deploy/stack reads deploy/docker-compose.prod.yml and every deploy/k8s/*.json from disk and serves the derived topology: the compose half (five services with their images, published ports, environment rails, dependency conditions, healthchecks, and resource hints) and the Kubernetes half (each manifest's kind, name, and facts — the deployment's probes GETting the REAL /api/health and /api/ready, its rollout knobs, its security context; the service's selector and ports; the configmap's non-secret keys; the secret's placeholder-only values). Per-artifact provenance on every unit: the repo-relative file path it was parsed from. When the repo does not sit beside the process (the distroless prod image ships none), the endpoint degrades to the honest typed 503."),
			code("dep-cs-locate", "go",
				"How the server finds the artifacts: the same walk-up the content package uses for the repo root — anchored on the deployment directory instead.",
				"server/internal/domain/demos/deploy_stack.go",
				"func LocateDeployDir(start string) string {\n"+
					"	// Normalize first: a relative start like \".\" would never ascend,\n"+
					"	// because filepath.Dir(\".\") is \".\" itself.\n"+
					"	dir := start\n"+
					"	if abs, err := filepath.Abs(start); err == nil {\n"+
					"		dir = abs\n"+
					"	}\n"+
					"	for {\n"+
					"		if _, err := os.Stat(filepath.Join(dir, DeployDirName, DeployComposeFile)); err == nil {\n"+
					"			return filepath.Join(dir, DeployDirName)\n"+
					"		}\n"+
					"		parent := filepath.Dir(dir)\n"+
					"		if parent == dir {\n"+
					"			return \"\"\n"+
					"		}\n"+
					"		dir = parent\n"+
					"	}\n"+
					"}"),
			code("dep-cs-handler", "go",
				"The thin handler: constructor-injected artifacts directory, honest 503 when the repo is not beside the process.",
				"server/internal/presentation/demo_deploy.go",
				"	if h.artifactsDir == \"\" {\n"+
					"		core.WriteError(w, r, core.ServiceUnavailable(\n"+
					"			\"The deployment story is parsed live from the repo's deploy/ artifacts, which do not sit beside this process — run the guide from the repository (the dev container) to serve it.\"),\n"+
					"			h.logger)\n"+
					"		return\n"+
					"	}"),
			text("dep-cs-coda",
				"READ THE DEMO'S DEGRADATION AS PART OF ITS TEACHING, not an apology: the distroless image holds only the binary, so when IT serves this endpoint there are no artifacts to parse, and the honest answer is the typed ``service_unavailable`` naming exactly that — the same stance the dependency topic's govulncheck panel takes when the toolchain is absent. In the dev container (and any checkout), the artifacts sit beside the process, the walk-up finds them, and the payload is the declaration you just read, reflected back as data. That symmetry — the file is the source of truth, the wire is a derivation, the test suite re-derives independently — is the deployment topic's version of the guide's whole snippet discipline, applied to infrastructure instead of code."),
		),
		// ── 5. Reverse proxy and TLS ──────────────────────────────
		section("reverse-proxy", "The reverse proxy and TLS",
			"A front door in front of the Go process: TLS termination, static assets, and one address for clients. The client image's real nginx.conf is the artifact; Caddy is the same story with less ceremony.",
			text("dep-rp-why-a-front-door",
				"WHY PUT ANYTHING IN FRONT of a server that is already a complete HTTP listener? Three reasons that survive scrutiny. TLS TERMINATION: certificates need renewing, keys need protecting, and ciphers need tuning — a specialized layer (nginx, Caddy, a cloud load balancer) does that per-host while every backend keeps speaking plain HTTP on an internal network. STATIC CONTENT: the React client is a directory of hashed files a proxy serves from memory without ever touching the Go process — the API server's time goes to API work. And SURFACE CONSOLIDATION: one public address, one place where security headers, compression, rate fences, and routing rules live, with the process behind it unpoked. The Go-side counterpoint is real — ``net/http`` serves TLS natively with ``http.ListenAndServeTLS`` and the standard topic 3 walked the handshake — so the decision is operational, not capability: this guide puts nginx in front (it ships with the client image anyway) and keeps the Go process on the internal network."),
			code("dep-rp-nginx", "nginx",
				"The real proxy block from the client image's nginx.conf: API calls forwarded with WebSocket upgrades kept alive.",
				".docker/client/nginx.conf",
				"    location /api/ {\n"+
					"        proxy_pass http://server:8100;\n"+
					"        proxy_http_version 1.1;\n"+
					"        proxy_set_header Upgrade $http_upgrade;\n"+
					"        proxy_set_header Connection 'upgrade';\n"+
					"        proxy_set_header Host $host;\n"+
					"        proxy_cache_bypass $http_upgrade;\n"+
					"    }"),
			text("dep-rp-reading-the-block",
				"EVERY DIRECTIVE IN THAT BLOCK answers a failure mode you would otherwise discover in production. ``proxy_pass http://server:8100`` resolves the compose service name on the internal network — no host ports, no public datastore surface. The ``Upgrade``/``Connection`` pair with ``proxy_http_version 1.1`` is what keeps the realtime topic's WebSockets alive through the proxy: without them, nginx forwards the upgrade request as a plain HTTP/1.0 transaction and the chat demo dies behind its own front door. ``proxy_set_header Host $host`` preserves the original host so the server's own absolute-URL logic and any host-keyed behavior stay correct. The caching rules on the static side (in the same file: ``/assets/`` immutable for a year because Vite's content-hashed filenames ARE the cache key; ``index.html`` ``no-cache`` so deploys are picked up at once) complete the pattern — hashed-forever, entrypoint-never — that the caching topic taught from the HTTP side."),
			text("dep-rp-headers-and-tls",
				"HEADERS AND TLS SPLIT THEIR HOME, and knowing which home is which prevents double-work bugs. The SECURITY HEADERS the security topic set (``X-Content-Type-Options``, the CSP family, and friends) belong to the APPLICATION — the server sets them on its own responses, and they ride through the proxy untouched; that is why the guide's header demo works identically direct and proxied. The TRANSPORT headers belong to the FRONT DOOR: ``Strict-Transport-Security`` only means something on a TLS response, redirect-to-HTTPS happens before any application logic, and ``X-Forwarded-For``/``X-Forwarded-Proto`` are the proxy telling the truth about the client the process cannot see. For the TLS layer itself, CADDY is the family's favorite contrast: two lines (``example.com { reverse_proxy server:8100 }``) buy you ACME-automated certificates with renewal, which nginx trades for explicit control and an ``http->https`` server block of ceremony. Both are honest; the guide's manifests assume whichever you pick terminates TLS and forwards plain HTTP inside."),
		),
		// ── 6. Graceful shutdown ──────────────────────────────────
		section("graceful-shutdown", "Graceful shutdown: the real drain order",
			"The composition root already drains the whole process on SIGINT/SIGTERM — gRPC first, HTTP inside a bounded timeout, the jobs worker last. Quoted, not paraphrased, because the order is the teaching.",
			text("dep-gs-why-graceful",
				"A DEPLOYMENT'S MOST-VISITED MOMENT is the one where the old process leaves: every rollout, every restart, every scale-down walks through it, and a process that dies mid-request turns routine operations into user-visible errors. The graceful pattern is old and simple to state: catch the termination signal, STOP ACCEPTING new work, FINISH the work in flight within a bounded window, then exit — and Go's standard library makes each step one call. ``signal.NotifyContext`` turns SIGINT/SIGTERM into a context cancellation that the whole program already understands; ``http.Server.Shutdown`` closes listeners and waits for in-flight handlers; the gRPC server's ``GracefulStop`` is its counterpart for the second listener. What separates a real drain from a checkbox is the ORDER and the BOUNDS — which surfaces stop first, how long each may take, and what happens to work that is neither HTTP nor gRPC. This guide's composition root is the case study, verbatim:"),
			code("dep-gs-signal", "go",
				"The signal edge: SIGINT and SIGTERM cancel one context — everything downstream drains on it.",
				"server/cmd/guide/main.go",
				"	// SIGINT/SIGTERM cancel ctx, which triggers the graceful shutdown path.\n"+
					"	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)\n"+
					"	defer stop()"),
			code("dep-gs-drain", "go",
				"The drain order, quoted whole: second-Ctrl-C escape, gRPC first, HTTP bounded, jobs worker last.",
				"server/cmd/guide/main.go",
				"	// Restore default signal behavior: a second Ctrl-C force-exits instead\n"+
					"	// of waiting on the shutdown timeout again.\n"+
					"	stop()\n"+
					"\n"+
					"	logger.Info(\"shutting down\")\n"+
					"	// Both surfaces drain in-flight work: gRPC first (its RPCs are short),\n"+
					"	// then HTTP inside its bounded timeout.\n"+
					"	grpcServer.GracefulStop()\n"+
					"	logger.Info(\"grpc server stopped\")\n"+
					"	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)\n"+
					"	defer cancel()\n"+
					"	if err := srv.Shutdown(shutdownCtx); err != nil {\n"+
					"		return fmt.Errorf(\"graceful shutdown: %w\", err)\n"+
					"	}\n"+
					"	logger.Info(\"server stopped\")\n"+
					"\n"+
					"	// The worker drains last: in-flight demo stories finish, then the\n"+
					"	// asynq server and its redis client stand down (idempotent).\n"+
					"	jobsWorker.Shutdown()\n"+
					"	logger.Info(\"jobs worker stopped\")\n"+
					"	return nil"),
			text("dep-gs-reading-the-order",
				"THE ORDER IS NOT ARBITRARY, and reading it top to bottom is a tour of every long-running thing the process holds. ``stop()`` runs FIRST — restoring default signal behavior so an impatient operator's second Ctrl-C kills the process immediately instead of queuing another graceful pass; bounded patience, enforced twice. gRPC drains before HTTP because its RPCs are short and its ``GracefulStop`` is unbounded by design — the trade works here, and a service with long streaming RPCs would bound it with a watchdog timer instead. HTTP then drains inside ``shutdownTimeout`` (ten seconds, a constant defined beside the server's other bounds) — ``Shutdown`` closes the listeners at once (new requests get connection-refused, which the LOAD BALANCER is supposed to prevent by having already stopped routing here — the readiness probe's job, next sections) and waits out the handlers. The JOBS WORKER drains last and idempotently, because queued work is not request-scoped: an in-flight demo story finishes, and the asynq server's own retry machinery re-queues whatever did not. If any step overruns, the process exits non-zero and the platform restarts it — failure stays loud."),
			text("dep-gs-exit",
				"THE EXIT PATH BOOKENDS THE BOOT PATH: ``run`` returns nil on a clean drain, and any error — a failed boot, an overran shutdown — becomes ``main``'s minimal stderr JSON fatal with exit code 1, quoted below. That non-zero exit is a contract with whatever launched the process: compose's ``restart: unless-stopped``, Kubernetes' restart policy, and any supervision wrapper all read it. The Kubernetes manifests later in this topic set ``terminationGracePeriodSeconds: 30`` — triple the server's own ten-second drain budget — so the platform's SIGKILL deadline never races the process's own patience: the platform always out-waits the program, and the program always out-waits its own timeouts. Graceful shutdown, fully generalized, is exactly that chain of nested patience from handler deadline to kill deadline, with the drain order above as its concrete implementation."),
			code("dep-gs-exit-code", "go",
				"The exit contract: one fatal path, stderr JSON, non-zero exit — before any listener ever bound.",
				"server/cmd/guide/main.go",
				"	if err := run(); err != nil {\n"+
					"		// The configured logger may not exist yet (config load failure),\n"+
					"		// so fatal boot errors land on a minimal stderr JSON logger.\n"+
					"		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error(\"fatal\", \"error\", err)\n"+
					"		os.Exit(1)\n"+
					"	}"),
		),
		// ── 7. CI/CD ──────────────────────────────────────────────
		section("ci-cd", "CI/CD: the family stance, and what a pipeline would run",
			"This repo runs no hosted CI — local verification is the only gate, by family decision. This section states why, shows the exact gate, and maps it onto what a real pipeline would do.",
			text("dep-ci-stance",
				"THE FAMILY STANCE FIRST, because it is a decision rather than a gap: this repo has NO ``.github/``, no hosted CI, no Dependabot — the local verification protocol is the only gate. The reasons are worth restating since deployment is where they bite hardest. SECRET SURFACE: a pipeline needs credentials to check out, to build, and to push — every one of those is a standing attack surface on your release chain, and the blast radius of a leaked pipeline credential is your entire artifact stream. REPRODUCIBILITY: the gate runs on YOUR pinned toolchain (the devcontainer's ``golang:1.27.3-bookworm``, the same image the prod build uses), not on a vendor's mutating runner image — the environment that verifies is the environment that ships. And HONESTY: a local gate the developer runs before every commit creates the habit; a remote gate that runs after creates the queue. None of this argues CI is wrong at team scale — it argues CI is a REPLACEMENT you buy knowingly, not a default you inherit silently."),
			list("dep-ci-the-gate",
				false,
				"Server gate — gofmt -l (formatting is not a style opinion), go build ./..., go vet ./..., go test ./... with the race detector, all inside the long-lived dev container.",
				"Client gate — npm test && npm run build in the client container; type errors and broken renders die before any commit.",
				"Declaration gate — docker compose config on every changed compose file (this topic's own file validates only with its secret rail satisfied) and the Go suite's manifest-structure tests that parse deploy/k8s/*.json for real.",
				"Supply-chain gate — govulncheck over the module (the dependency topic's demo runs it live), plus go.sum as the lock the build verifies.",
			),
			text("dep-ci-what-a-pipeline-would-run",
				"WHAT A PIPELINE WOULD RUN is therefore not a mystery to design — it is the local gate, triggered by push instead of by discipline, plus the steps that genuinely need shared infrastructure. The same commands, in the same pinned containers, on every commit: format check, build, vet, race-enabled tests, the client suite, the compose-config validation, the manifest parse tests. The additions a real pipeline earns its hosting with: ARTIFACT PUBLISHING — build the distroless image once, push it by digest, and promote that digest through environments (the ship-vs-run rule enforced mechanically); SCAN STAGES — govulncheck and image CVE scans on the artifact, gating promotion; and DEPLOYMENT STAGES — ``kubectl apply`` (or ``docker stack deploy``) from the same manifests in this repo, behind approval gates the team chooses. The discipline to keep is that the pipeline NEVER holds special knowledge: it runs the repo's own commands. When the gate and the pipeline share one definition — the Makefile, the script in the repo — ``works locally`` and ``passes CI`` stop being two different claims."),
		),
		// ── 8. Parity and secrets ─────────────────────────────────
		section("parity-secrets", "Parity and secrets: the discipline across stacks",
			"The environments topic's config-parity rule carried into deployment: identical engine versions everywhere, config split by sensitivity, and secrets that never exist in any committed artifact — enforced by a scanner, not by hope.",
			text("dep-ps-parity",
				"PARITY IS THE DEV/PROD SIMILARITY BUDGET from the environments topic, and deployment is where it is spent. The guide holds the line where it is cheap and records where it bends. HELD: the datastore engines pin IDENTICALLY across the dev and prod declarations (``postgres:17``, ``redis:7-alpine``, ``mongo:8`` in both compose files) — the version-specific behaviors the data topics teach are the same behaviors prod runs. HELD: the process is the same bytes — one Dockerfile, one image tag, dev differing only by what is mounted beside it. BENT, DELIBERATELY: the runtimes differ (fat devcontainer toolchain versus distroless nothing) — dev ergonomics bought with a recorded trade-off, not an accident. The practical test of parity is the environments topic's own: anything you can only reproduce in one stack is a parity bug, and its fix is almost always another pin, not another if-branch in code."),
			text("dep-ps-secret-rule",
				"THE SECRET RULE IS ONE SENTENCE with no exceptions: a credential never exists in a committed artifact — not in code, not in an image layer, not in a declaration's literal values. The stacks enforce it at three layers, each with a different mechanism. IN COMPOSE: interpolation rails — the declaration carries ``${POSTGRES_PASSWORD:?…}`` and REFUSES to run until the operator's environment supplies the value; the committed file holds a template, and the running process holds the secret, with nothing in between. IN KUBERNETES: the ConfigMap/Secret SPLIT — non-secret configuration lives in a ConfigMap that may safely appear in logs and ``describe`` output, while the one credential-bearing value lives in a Secret whose committed form is deliberately a ``CHANGE_ME_`` placeholder, replaced at apply time from a real secret store. AND IN THE GUIDE'S OWN TOOLING: the scanner — the demo's builder refuses to serve any Secret value that is not a marked placeholder, so this topic literally cannot display a real credential even by accident:"),
			code("dep-ps-secret-manifest", "json",
				"The committed secret: placeholders only, with the policy stated as a manifest annotation where every reader trips over it.",
				"deploy/k8s/secret.json",
				"  \"type\": \"Opaque\",\n"+
					"  \"data\": {\n"+
					"    \"GDG_DATABASE_URL\": \"cG9zdGdyZXM6Ly9ndWlkZTpDSEFOR0VfTUVfREJfUEFTU1dPUkRAcG9zdGdyZXM6NTQzMi9ndWlkZT9zc2xtb2RlPWRpc2FibGU=\"\n"+
					"  }"),
			code("dep-ps-scanner", "go",
				"The enforcement, as code: the builder refuses to build a payload carrying a non-placeholder secret value.",
				"server/internal/domain/demos/deploy_stack.go",
				"	if offenders := DeployScanSecrets(manifests); len(offenders) > 0 {\n"+
					"		return DeployStackPayload{}, fmt.Errorf(\n"+
					"			\"deploy: secret values are not CHANGE_ME_* placeholders (refusing to serve): %s\",\n"+
					"			strings.Join(offenders, \", \"),\n"+
					"		)\n"+
					"	}"),
			text("dep-ps-lifecycle",
				"THE LIFECYCLE CLOSES THE SECTION because injection is the beginning of a secret's life, not the end — the environments topic stated the duties and deployment inherits them. ROTATION is a restart in both stacks (the compose rail re-reads the environment; the k8s Secret re-mounts on pod replacement), which disposability keeps cheap. SCOPING is structural here: one password on this stack, one DSN consuming it, and the resource-hint declarations bound what a compromised process can consume while exfiltrating. And THE BAD-DAY ARITHMETIC is part of the design: the committed placeholder leaks nothing (that is its entire reason to exist); a leaked running-config snapshot leaks the DSN alone; rotating the one database password re-secures the stack. When the secret count grows past a handful, a real secret manager (Vault, the cloud managers) earns its place exactly as the environments topic graded it — rotation, leasing, per-service authorization — with the application contract UNCHANGED: it still reads an environment variable at boot, which is why the guide's config loader needed no deployment-specific code at all."),
		),
		// ── 9. Kubernetes ─────────────────────────────────────────
		section("kubernetes", "Kubernetes: the real manifests, as JSON",
			"The cluster story of the same server: five small manifests under deploy/k8s/ — namespace, configmap, secret, deployment, service — written as JSON so the stdlib parses them with no YAML dependency. That choice is itself taught.",
			text("dep-k8s-why-json",
				"THE MANIFESTS ARE JSON ON PURPOSE, and the choice is the first thing to teach because it is the kind of decision that looks like heresy until it does not. Kubernetes accepts JSON manifests natively — ``kubectl apply -f thing.json`` is not a workaround, it is the same path YAML takes after its own translation — because JSON IS the API's serialization. What JSON buys THIS repo specifically: the stdlib's ``encoding/json`` parses every manifest with ZERO new dependencies (no ``gopkg.in/yaml.v3``, no YAML-subset gymnastics for structured data), the demo and the test suite share one parser, and syntax errors are impossible to introduce via indentation. What YAML would buy instead: comments inside manifests, marginally terser flow — and the guide's answer to the first is the manifests' ``annotations`` (real, machine-readable, and visible to ``kubectl describe`` — see the secret's placeholder notice) plus this topic's prose. The pattern generalizes: WHEN THE CONSUMER IS A PROGRAM, pick the format the program's standard library already speaks."),
			code("dep-k8s-parse", "go",
				"The stdlib-only choice, as the code that makes it: every manifest decodes with encoding/json and types by kind.",
				"server/internal/domain/demos/deploy_stack.go",
				"// ParseK8sManifests decodes every manifest with encoding/json — the\n"+
					"// deliberate stdlib-only choice the manifests' JSON format makes\n"+
					"// possible (kubectl accepts JSON natively, and no YAML dependency\n"+
					"// enters the module) — and types each by its kind.\n"+
					"func ParseK8sManifests(files []DeployManifestFile) ([]K8sManifest, error) {\n"+
					"	parsed := make([]K8sManifest, 0, len(files))\n"+
					"	for _, file := range files {\n"+
					"		manifest, err := deployParseK8sManifest(file)\n"+
					"		if err != nil {\n"+
					"			return nil, err\n"+
					"		}\n"+
					"		parsed = append(parsed, manifest)\n"+
					"	}\n"+
					"	return parsed, nil\n"+
					"}"),
			text("dep-k8s-the-five-objects",
				"FIVE SMALL OBJECTS TELL THE WHOLE CLUSTER STORY, and each is one real file under ``deploy/k8s/``. The NAMESPACE (``go-developers-guide``) scopes everything: one name, one delete-switch for the whole experiment. The CONFIGMAP carries the non-secret half of the ``GDG_*`` contract — the same names the config loader reads, minus the one credential-bearing DSN. The SECRET carries exactly that DSN, as a placeholder (previous section). The DEPLOYMENT says how pods run: two replicas, the image the compose stack builds, probes against the REAL endpoints, resources, and a locked-down security context. The SERVICE gives those pods one stable cluster-local address with named ports. The set is deliberately complete-but-minimal — a real cluster adds ingress, autoscaling, and network policies on top, but NOTHING here is a placeholder pretending to be real: every field is one the guide's own server satisfies."),
			code("dep-k8s-configmap", "json",
				"The config split's non-secret half: the GDG_* contract the loader already reads, names and all.",
				"deploy/k8s/configmap.json",
				"  \"data\": {\n"+
					"    \"GDG_ENV\": \"prod\",\n"+
					"    \"GDG_PORT\": \"8100\",\n"+
					"    \"GDG_LOG_LEVEL\": \"info\",\n"+
					"    \"GRPC_ADDR\": \":8102\",\n"+
					"    \"GDG_REDIS_ADDR\": \"redis:6379\",\n"+
					"    \"GDG_MONGO_URI\": \"mongodb://mongo:27017\"\n"+
					"  }"),
			code("dep-k8s-deployment-ports", "json",
				"The pod's shape: named ports (http 8100, grpc 8102) and envFrom wiring both config objects in one stroke.",
				"deploy/k8s/deployment.json",
				"            \"ports\": [\n"+
					"              {\n"+
					"                \"name\": \"http\",\n"+
					"                \"containerPort\": 8100\n"+
					"              },\n"+
					"              {\n"+
					"                \"name\": \"grpc\",\n"+
					"                \"containerPort\": 8102\n"+
					"              }\n"+
					"            ],\n"+
					"            \"envFrom\": [\n"+
					"              {\n"+
					"                \"configMapRef\": {\n"+
					"                  \"name\": \"guide-config\"\n"+
					"                }\n"+
					"              },\n"+
					"              {\n"+
					"                \"secretRef\": {\n"+
					"                  \"name\": \"guide-secret\"\n"+
					"                }\n"+
					"              }\n"+
					"            ],"),
			code("dep-k8s-service", "json",
				"The stable address: selector matching the pod labels, ports by NAME so container and service can never drift apart numerically.",
				"deploy/k8s/service.json",
				"    \"selector\": {\n"+
					"      \"app.kubernetes.io/name\": \"go-developers-guide\",\n"+
					"      \"app.kubernetes.io/component\": \"server\"\n"+
					"    },\n"+
					"    \"ports\": [\n"+
					"      {\n"+
					"        \"name\": \"http\",\n"+
					"        \"port\": 8100,\n"+
					"        \"targetPort\": \"http\"\n"+
					"      },\n"+
					"      {\n"+
					"        \"name\": \"grpc\",\n"+
					"        \"port\": 8102,\n"+
					"        \"targetPort\": \"grpc\"\n"+
					"      }\n"+
					"    ]"),
			text("dep-k8s-named-ports",
				"NAMED PORTS ARE THE QUIET DISCIPLINE holding that service quote together, and they are why the guide's manifests never repeat a port number they do not have to. The CONTAINER names its ports (``http``, ``grpc``); the PROBES target the name; the SERVICE's ``targetPort`` references the name — so the number lives in exactly one place (the container declaration, beside the configmap's ``GDG_PORT`` that tells the process to listen on it), and every other reference resolves through it. The demo's test suite pins the chain: configmap ``GDG_PORT`` equals the container's ``http`` port number, the service's port numbers agree with the container ports they target, and the labels thread the service selector through the deployment's matchLabels into the pod template — the consistency a human reviewer would check, held by a test instead. The manifest test also proves what a reader might doubt: THE PROBE PATHS ARE THE REAL ONES (next section), and the whole set parses with the same stdlib call the demo uses at runtime."),
		),
		// ── 10. Rollouts ──────────────────────────────────────────
		section("rollouts", "Rollouts: probes gate the traffic",
			"Replacing artifact A with artifact B without dropping requests is the deployment's final exam. The knobs — maxSurge, maxUnavailable, readiness gates — and the probe contract that makes them meaningful.",
			text("dep-ro-probe-contract",
				"THE PROBE CONTRACT IS THE LOAD-BEARING WALL: every rollout knob presumes the platform can tell a ready pod from a live one, and that distinction is exactly the observability topic's liveness/readiness split, now consumed by infrastructure. LIVENESS answers ``is the process wedged`` — the kubelet GETs ``/api/health``, the endpoint that answers without touching any dependency, and repeated failures mean RESTART the container (a wedged process gets a fresh one; a wedged dependency does not take innocent pods with it). READINESS answers ``can this pod serve RIGHT NOW`` — the kubelet GETs ``/api/ready``, the endpoint that actually pings PostgreSQL and Redis inside bounded timeouts — and failures mean REMOVE the pod from the Service's endpoints (no restart, just no traffic). The manifest wires the real endpoints and nothing else:"),
			code("dep-ro-probes", "json",
				"The probe contract, quoted from the deployment: kubelet-driven HTTP GETs against the server's REAL endpoints.",
				"deploy/k8s/deployment.json",
				"            \"livenessProbe\": {\n"+
					"              \"httpGet\": {\n"+
					"                \"path\": \"/api/health\",\n"+
					"                \"port\": \"http\"\n"+
					"              },\n"+
					"              \"initialDelaySeconds\": 5,\n"+
					"              \"periodSeconds\": 15,\n"+
					"              \"timeoutSeconds\": 2,\n"+
					"              \"failureThreshold\": 3\n"+
					"            },\n"+
					"            \"readinessProbe\": {\n"+
					"              \"httpGet\": {\n"+
					"                \"path\": \"/api/ready\",\n"+
					"                \"port\": \"http\"\n"+
					"              },\n"+
					"              \"initialDelaySeconds\": 10,\n"+
					"              \"periodSeconds\": 10,\n"+
					"              \"timeoutSeconds\": 2,\n"+
					"              \"failureThreshold\": 3\n"+
					"            },"),
			code("dep-ro-readiness-wiring", "go",
				"What readiness actually checks — the real wiring from the composition root: bounded pings of the stack's dependencies.",
				"server/cmd/guide/main.go",
				"	mux.Handle(\"GET /api/ready\", presentation.NewReadinessHandler([]domobs.DependencyCheck{\n"+
					"		{Name: \"postgres\", Pinger: obsdata.NewPgxPinger(pool), Timeout: time.Second},\n"+
					"		{Name: \"redis\", Pinger: obsdata.NewGoRedisPinger(redisClient), Timeout: time.Second},\n"+
					"	}, logger))"),
			text("dep-ro-knobs",
				"THE ROLLOUT KNOBS SET THE REPLACEMENT ARITHMETIC, and the guide's choices are a one-line policy statement. ``maxUnavailable: 0`` — NEVER serve below the declared replica count; capacity is a promise, and a deploy is not an outage. ``maxSurge: 1`` — one extra pod above count may exist during the rollout, which is the headroom that makes zero-unavailable possible (the new pod must start BEFORE an old one stops). ``minReadySeconds: 5`` — a new pod must STAY ready for five seconds before it counts, so a pod that flaps ready-then-crashing cannot advance the rollout. The choreography those knobs buy: new pod scheduled → probes pass → five quiet seconds → an old pod drains (SIGTERM, the graceful-shutdown section's whole story, inside the 30-second grace period) → traffic shifts → repeat. The failure mode the knobs cannot fix is a bad ARTIFACT that passes probes and breaks behavior — for that, the honest toolbox is the CI section's promote-the-digest discipline plus canary-shaped patience, not more YAML."),
			code("dep-ro-strategy", "json",
				"The arithmetic as declaration: never below capacity, one pod of headroom, five seconds of proof.",
				"deploy/k8s/deployment.json",
				"    \"strategy\": {\n"+
					"      \"type\": \"RollingUpdate\",\n"+
					"      \"rollingUpdate\": {\n"+
					"        \"maxSurge\": 1,\n"+
					"        \"maxUnavailable\": 0\n"+
					"      }\n"+
					"    },"),
			text("dep-ro-readiness-gates",
				"READINESS GATES ARE THE HONEST FOOTNOTE the rollout story needs: Kubernetes' built-in readiness decides ready-or-not from INSIDE the pod (probes, conditions), but some gate decisions genuinely belong outside — a load-test soak passed, a migration completed, a canary metric window green. A READINESSGATE adds such an external condition to the pod's readiness formula: the pod is ready only when the gate's status field is set True by an outside controller you run (or a human, for the boldly manual). The guide's manifests deliberately carry none — with no external controller, a gate is a pod that never becomes ready, which is the one deployment artifact worse than no gate. Where the pattern earns its keep in practice: batch-soak gates on risky releases, and database-migration gates that hold new pods until the schema they require exists. The rollout discipline stands whether the gate is a probe, a controller, or a person with a checklist: NOTHING TAKES TRAFFIC UNTIL SOMETHING HONEST SAYS IT CAN."),
		),
		// ── 11. Infrastructure as code ───────────────────────────
		section("infrastructure-as-code", "Infrastructure as code: the honest scope",
			"Manifests-as-code IS infrastructure as code — reviewable, versioned, diffable, tested. This section maps that truth and names what full IaC adds when a fleet earns it.",
			text("dep-iac-what-this-repo-is",
				"THE HONEST SCOPE FIRST: everything this repo declares about infrastructure is CODE in the repo's strongest sense — versioned beside the server it deploys, reviewed in the same pull requests, and EXECUTABLY TESTED (the suite parses the compose file and every manifest and asserts their structure: probe paths, port agreement, label consistency, the placeholder-only secret rule). That last property is the difference between infrastructure-as-code and infrastructure-as- YAML-in-a-repo: a declaration the test suite can break is a declaration that does not rot silently. The blast-radius rule follows naturally — a port change in the configmap without the matching container-port change fails the suite before it fails staging; a real-looking secret value in the manifest fails the suite before it fails the news cycle. The same ratchet thinking the guide applies to CODE SNIPPETS (every quote traceable to a real file) is applied here to DECLARATIONS: every served fact about the stack is parsed from a committed file, never asserted from memory."),
			text("dep-iac-what-full-adds",
				"WHAT FULL-FLEDGED IaC ADDS, when a fleet rather than one stack needs it, is three capabilities this repo deliberately does not fake. PROVISIONING (Terraform, Pulumi, OpenTofu): declaring the CLUSTER and its cloud resources — the database instances, networks, IAM — so the substrate itself is reproducible and drift-detected, where this repo's manifests assume the cluster exists. TEMPLATING (Helm, Kustomize): one chart stamped per environment with small overlays, where this repo's answer is the simpler one-file-per-stack shape the environments topic already graded. And STATE MANAGEMENT: a reconciling control loop (which Kubernetes itself already IS for the manifest layer — apply a Deployment and the controller drives reality toward it) plus the IaC tool's own state for the layers below. The family trap to avoid is adopting the vocabulary without the property: ``Terraform-managed`` infra someone also clicks into being is two sources of truth wearing one name — worse than an honest compose file, because it LIES about being code. When this guide's scope grows, the growth path is clear: the same files, wrapped in the environment-stamping layer, with the suite's parse tests still holding the floor."),
		),
		// ── 12. Linux survival ────────────────────────────────────
		section("linux-survival", "Linux survival: signals, exit codes, PID 1, ports",
			"The operating-system floor under every deployed Go process: what signals do by default, what exit codes mean, why PID 1 is special, and how ports actually bind — the facts an operator needs at 3 a.m.",
			text("dep-lx-signals",
				"SIGNALS ARE THE OPERATING SYSTEM'S management channel, and a deployed process lives or dies by three of them. SIGTERM (15) is the POLITE stop request — ``docker stop``, ``kubectl delete pod``, systemd's default: the guide's whole graceful-shutdown choreography hangs off catching it (the quoted ``signal.NotifyContext`` line). SIGINT (2) is the keyboard's Ctrl-C — same handler, same drain. SIGHUP (1) is the odd one: historically a terminal hangup, by convention a CONFIG RELOAD request for daemons — and Go's DEFAULT for it is to terminate the process, which surprises anyone who assumed reload; if you want reload semantics, catch it and re-read config explicitly (this guide does not: config is boot-time, so a reload IS a restart, by design). THE DEFAULTS WORTH MEMORIZING: a Go program with no signal handling dies on SIGTERM and SIGINT immediately — no defers, no drain — which is precisely why the drain must be wired BEFORE the first deployment, not after the first dropped-request incident."),
			text("dep-lx-exit-codes",
				"EXIT CODES ARE THE PROCESS'S LAST WORD, and the platform reads nothing else so faithfully. ZERO is success — the clean drain returned nil. ONE is this guide's fatal-boot-and-internal-failure code (the quoted ``os.Exit(1)`` after the stderr JSON log). The 128+N family means ``killed by signal N`` — a process that dies of default SIGTERM handling exits 143 (128+15), which is how you distinguish ``the process failed`` from ``the platform stopped the process`` in logs that kept no stdout. TWO is shell convention for misuse, 126/127 mean cannot-execute/not-found — the trio you meet when a container fails to start at all. The deployment-relevant discipline is simply: exit NON-ZERO on every failure path, and let the supervisor (compose restart policy, kubelet restart back-off) do its job with truthful information. A process that swallows its failure and exits 0 will be memorialized as healthy by everything watching it."),
			text("dep-lx-pid1",
				"PID 1 IS A ROLE, NOT JUST A NUMBER: the first process in a container (or a machine) is the one the kernel hands orphaned zombies to and — without an init — the one whose signal-handling quirks apply. Two consequences matter for Go deployments. SIGNALS: as PID 1, default signal dispositions change subtly (a bare PID-1 process ignores signals it has not explicitly handled unless a real init forwards them) — ``docker stop`` sends SIGTERM to PID 1, and an ENTRYPOINT binary that never registers a handler can appear to IGNORE it until Docker's 10-second SIGKILL timeout; the guide is safe twice over, since ``signal.NotifyContext`` registers handlers and the drain is bounded at ten seconds inside a thirty-second platform budget. REAPING: zombie processes (finished children whose exit status nobody collected) accumulate under a PID 1 that never calls ``wait`` — Go's runtime reaps its own children, but shells spawned by the process can orphan grandchildren. The standard fixes are one line: compose's ``init: true`` (visible in the dev stack's devcontainer service) or tini as ENTRYPOINT — the guide's server needs neither (no child processes), and knowing WHY it needs neither is the survival skill."),
			list("dep-lx-port-facts",
				false,
				"Binding — the server listens on :8100 (all interfaces) inside the container; a bind failure (EADDRINUSE) fails the boot honestly, per the composition root's fail-fast stance.",
				"Publishing — the container's 8100 reaches the host only where a declaration maps it (compose ports, k8s Service); the guide's prod datastores publish nothing at all.",
				"Interface scope — 0.0.0.0:8100 (what \":8100\" binds) versus 127.0.0.1:8100: the loopback bind is unreachable from other containers, a classic \"works in dev, dead in compose\" cause.",
				"Inside versus outside — the DSN's postgres:5432 is the IN-NETWORK port; the dev stack additionally publishes it to the host as 5433 — the family's co-run discipline, and a reminder that port numbers are per-network facts.",
			),
			text("dep-lx-coda",
				"THE FLOOR, ASSEMBLED: this section's four facts are the ones the rest of the topic silently relies on. The graceful drain is SIGNALS plus EXIT CODES done deliberately. The compose ``init: true`` line and the Kubernetes ``terminationGracePeriodSeconds: 30`` are PID-1-ADJACENT patience made declarative. The probes are PORTS plus HTTP minus everything the distroless image refused to ship. And the whole guide is runnable on any of it because the static binary asked the operating system for so little: one process, one port, one signal handler, one honest exit. That minimalism is not asceticism — it is the smallest surface an operator must understand at 3 a.m., which is the only audience that matters when deployment is being graded."),
		),
		// ── 13. Terminal companion ────────────────────────────────
		section("terminal-companion", "Terminal companion: the deployment tour",
			"The topic over a terminal: the stack viewer read raw from the demo endpoint, the compose rail proven to refuse and then to validate, and the manifests dry-run-validated exactly the way the repo does it.",
			text("term-dep-intro",
				"With the stack up (server on :8100), the tour runs in the topic's order. First THE STACK VIEWER — the deployment artifacts parsed live: the compose half (five services, their ports and dependency conditions, the healthcheck the distroless server deliberately lacks) and the Kubernetes half (the deployment's probe contract against the REAL endpoints, the rollout arithmetic, the secret scan's verdict). Then THE RAIL PROOF, which is two commands and their two very different outputs: ``docker compose config`` WITHOUT the exported password refuses with the interpolation error — the file doing its job — and WITH a placeholder exported it validates and renders the whole stack, secret never committed anywhere. Finally THE MANIFEST VALIDATION, the repo's own dry-run: ``kubectl apply --dry-run=client`` against the manifest directory when kubectl is present (client-side dry-run parses and schematically checks without touching any cluster), and — because the dev container ships no cluster and no kubectl by stance — the same guarantees from the server suite's structural tests, run right there, which parse every manifest with encoding/json and assert the probe paths, port agreement, and the placeholder-only secret rule:"),
			code("term-dep-stack-bash", "bash", "", "",
				"# the deployment stack, parsed live from the repo's own artifacts\n"+
					"curl -s localhost:8100/api/demo/deploy/stack | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); \\\n"+
					"   print(\"compose:\", p[\"compose\"][\"source\"], \"-\", len(p[\"compose\"][\"services\"]), \"services\"); \\\n"+
					"   [print(\" \", s[\"name\"].ljust(9), \"ports\", s[\"publishedPorts\"], \"depends\", len(s[\"dependsOn\"])) \\\n"+
					"    for s in p[\"compose\"][\"services\"]]; \\\n"+
					"   [print(\"k8s:\", m[\"kind\"].ljust(11), m[\"name\"].ljust(21), \"-\", len(m[\"facts\"]), \"facts\") \\\n"+
					"    for m in p[\"kubernetes\"]]; \\\n"+
					"   print(\"secret scan:\", p[\"secretScan\"][\"status\"], \"-\", p[\"secretScan\"][\"checked\"])'\n"+
					"\n"+
					"# the probe contract, straight off the payload\n"+
					"curl -s localhost:8100/api/demo/deploy/stack | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); \\\n"+
					"   d=[m for m in p[\"kubernetes\"] if m[\"kind\"]==\"Deployment\"][0]; \\\n"+
					"   [print(f[\"label\"].ljust(16), f[\"value\"]) \\\n"+
					"    for f in d[\"facts\"] if \"Probe\" in f[\"label\"]]'"),
			code("term-dep-compose-bash", "bash", "", "",
				"# the rail refuses: no exported password, no rendered stack\n"+
					"docker compose -f deploy/docker-compose.prod.yml config -q; echo \"exit=$?\"\n"+
					"\n"+
					"# the rail satisfied: a placeholder exported for VALIDATION ONLY\n"+
					"# (compose renders it into the config; nothing is committed)\n"+
					"POSTGRES_PASSWORD=validate-only \\\n"+
					"  docker compose -f deploy/docker-compose.prod.yml config | \\\n"+
					"  grep -A3 'GDG_DATABASE_URL' | head -4"),
			code("term-dep-k8s-bash", "bash", "", "",
				"# kubectl's client-side dry-run, when kubectl is present:\n"+
					"# parse + schema-check every manifest, touch no cluster\n"+
					"kubectl apply --dry-run=client -f deploy/k8s/ 2>/dev/null || \\\n"+
					"  echo \"(no kubectl here - the family stance: no cluster spin-up)\"\n"+
					"\n"+
					"# the repo's own dry-run: the suite parses every manifest with\n"+
					"# encoding/json and asserts probe paths, port agreement, labels,\n"+
					"# and the placeholder-only secret rule\n"+
					"cd server && go test ./internal/domain/demos/ -run TestDeployStack -count=1 -v \\\n"+
					"  2>&1 | grep -E '^(=== RUN|--- (PASS|FAIL)|ok|FAIL)'"),
			text("term-dep-coda",
				"Read the tour as the topic's argument in terminal form. The viewer block is FILE-AS-SOURCE-OF-TRUTH made executable — every line rendered was parsed from a committed artifact seconds ago, and the grep for the probes shows the contract (GET /api/health, GET /api/ready) exactly as the kubelet would honor it. The rail block is the SECRETS DISCIPLINE with its teeth out: the same file that refuses without the environment validates with it, and the rendered config shows the interpolation happened at read time — nothing was ever committed. The validation block closes the loop the honest way this guide owns: a cluster is not spun up to prove the manifests (family stance, local verification only), so the dry-run that MATTERS is the one the suite performs on every test run — parse, assert structure, refuse placeholders — which is the same guarantee kubectl's client-side check gives, held by code that ships with the repo. Deployment closes the platform story's setup; what follows sharpens it — performance, reliability, scaling."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"deployment",
		"Deployment in Go: ship the artifact, run the declaration",
		"How a Go service ships: an immutable static artifact plus a declaration of how it runs — taught from this repo's REAL deployment files.",
		"ship",
		37,
		sections,
	)
}
