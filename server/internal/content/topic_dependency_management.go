package content

import (
	"errors"
	"fmt"
)

// This file is topic 31 dependency-management (group platform,
// curriculum order 31): what a dependency IS in Go — a module, pinned
// by a version, resolved to exactly one copy per build — and the whole
// apparatus around that definition, taught from the artifacts this
// repo already runs. The manifest pair first (go.mod states the
// requirements, go.sum proves them, MVS picks one version per module
// deterministically with no lockfile), then the tools that keep the
// manifest honest (tidy, why, graph), then govulncheck — the scanner
// that answers the question the manifest cannot: not what you require,
// but what your code can actually REACH (demo deps:manifest serves the
// live manifest beside the live build info, walks the tools/deps.go
// anchor pattern, names the Docker cache volumes, and runs the real
// govulncheck when the tool and source tree are both present). Upgrades
// are next (bumping a pin is a deliberate act, anchored and verified),
// then the module + build caching that makes Docker verification warm
// (the gdg-go and gdg-gocache named volumes, the manifests-first layer
// ordering), and the bar for choosing dependencies at all. The
// terminal companion tours everything with curl, go version -m, and
// the module subcommands against the running stack.

// dependencyManagementTopic builds topic 31.
func dependencyManagementTopic() (Topic, error) {
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
		// ── 1. What a dependency is ────────────────────────────────
		section("what-a-dependency-is", "What a dependency is: modules, packages, and the one-version rule",
			"Every language answers 'what is a dependency' differently. Go's answer is unusually small: a module, pinned by a version, resolved to exactly one copy per build.",
			text("dm-definition",
				"A DEPENDENCY in Go is a MODULE — a versioned collection of packages that one go.mod file owns. The import path in your source names a PACKAGE inside that module; the module path is the prefix the package's go.mod claims, and the go command resolves between the two by walking the import path up until it finds a module root. The import ``github.com/jackc/pgx/v5/pgxpool`` therefore names the PACKAGE ``pgxpool`` inside the MODULE ``github.com/jackc/pgx/v5`` — the trailing ``/v5`` is not decoration but semantic import versioning: a second major version is, by rule, a DIFFERENT module path, which is how Go dodges the v1/v2 split-brain that plagues package managers without that convention. Versions are semver-shaped tags (``v5.11.0``) or, for untipped commits, pseudo-versions that encode the date and commit hash — ugly on purpose, so nobody is tempted to hand-edit them."),
			code("dm-imports-go", "go",
				"The data layer's import block — the three rings every Go file sorts into: stdlib, external modules, this module's own packages.",
				"server/internal/data/posts/pg.go",
				"import (\n"+
					"	\"context\"\n"+
					"	// …\n"+
					"\n"+
					"	\"github.com/jackc/pgx/v5\"\n"+
					"	\"github.com/jackc/pgx/v5/pgxpool\"\n"+
					"\n"+
					"	\"guide/internal/core\"\n"+
					"	\"guide/internal/domain/posts\"\n"+
					")"),
			text("dm-one-version",
				"The rule that shapes everything else in this topic: ONE VERSION PER MODULE PER BUILD. Where npm nests conflicting copies under nested node_modules and Python confronts you at install time, the go tool refuses the premise — the module graph must converge on a single version of every module, and if it cannot, the build fails with a conflict you must resolve by bumping a require (upgrades, later in this topic). The convergence mechanism is minimal version selection, covered two sections down; its consequence is that once the graph agrees, every package in the build was compiled against the same dependency code. No diamond-dependency hell, no four copies of the same library with four different behaviors — and no lockfile either, because the selection is already deterministic from the manifests alone. The one-version rule is also why the vendor escape hatch (``go mod vendor``) is rarely needed: it exists for hermetic builds and air-gapped networks, not for version reconciliation."),
			text("dm-where-modules-live",
				"Modules are fetched, cached, and verified by machinery this guide will keep pointing at because it is the same machinery the dev container warms. Downloads come from a MODULE PROXY (proxy.golang.org by default; GOPROXY redirects or disables it), and every artifact's hashes are checked against the central CHECKSUM DATABASE (sum.golang.org) the first time the module is seen — the ``h1:`` hashes in go.sum are that verification's local memory. The downloads land in the MODULE CACHE, a content-addressed, read-only store this repo mounts at a named Docker volume (the caching section walks it), where each module version is unpacked exactly once for the whole machine. Nothing about this is configured in this repo beyond the defaults — the defaults ARE the design: proxy plus checksum DB plus cache, all keyed by the manifest pair the next section opens."),
		),
		// ── 2. go.mod, go.sum, MVS ─────────────────────────────────
		section("go-mod-and-mvs", "go.mod and go.sum: the manifest, the integrity record, and minimal version selection",
			"Two small files carry the whole dependency story: one states what the build may use, the other proves what was downloaded — and the resolution algorithm needs neither a lockfile nor a solver's luck.",
			text("dm-gomod",
				"go.mod is the MANIFEST: it names the module (``guide``), the minimum Go version its code assumes (the ``go`` directive — also a real input to language features and module-graph pruning), and the REQUIREMENTS — the module versions this module's code (or its tooling) needs. Requirements come in two flavors that look alike but answer different questions: a DIRECT require names a module your packages import (or your tool anchors import, next section), while an ``// indirect`` require names one that only the transitive graph pulled in — the marker is how tidy tells you WHY a line exists without you running ``go mod why``. The tool directives at the bottom are the newest tier: they say these modules' binaries belong to the development workflow (codegen, migrations), so ``go mod tidy`` and the go command track them as first-class module citizens rather than as ad-hoc ``go install`` invocations drifting outside the manifest:"),
			code("dm-gomod-mod", "mod",
				"This module's own manifest — the stable head plus the tool block; the require blocks churn with every deliberate pin change, so the live demo serves those rows instead of freezing them here.",
				"server/go.mod",
				"module guide\n"+
					"go 1.27.3\n"+
					"\n"+
					"require (\n"+
					"	// … direct requires: pgx, goose, go-redis, asynq, gqlgen, grpc …\n"+
					")\n"+
					"\n"+
					"require (\n"+
					"	// … the // indirect graph MVS selected …\n"+
					")\n"+
					"\n"+
					"tool (\n"+
					"	github.com/99designs/gqlgen\n"+
					"	// …\n"+
					")"),
			text("dm-gosum",
				"go.sum is NOT a lockfile — it is an integrity record. Each row is a module version plus the cryptographic hash of its content (and of its own go.mod), and the go command verifies every download against it before the build touches the code; the first sighting of a new version is additionally attested by the checksum database. The practical consequences are worth internalizing early. You never hand-edit go.sum, never commit a teammate's conflict resolution by guessing, and never ``rm`` it as a fix — ``go mod tidy`` regenerates it from the manifest and the graph when it has genuinely drifted. And because selection is already deterministic from the manifests (the algorithm below), go.sum carries no VERSION information at all: delete it and the same versions resolve, they just have to be re-verified. A lockfile answers ``what version did we agree on''; go.sum answers ``is this the bytes we verified'' — MVS answers the first question by construction."),
			text("dm-mvs",
				"Minimal version selection is the algorithm that makes lockfiles unnecessary, and it is simpler than its reputation: for every module in the graph, walk every manifest that requires it and take the MAXIMUM of the required versions — the highest version any module in the build asked for, and nothing higher. That last clause is the ``minimal'' part and the one that surprises people weaned on eager resolvers: if you require pgx at ``v5.11.0`` and every transitive consumer asks for older versions, the build uses ``v5.11.0`` — your require WON, because it was the highest ask — but nothing silently upgrades to ``v5.12.0`` just because it exists. No solver runs, no backtracking, no possibility of resolution succeeding on Tuesday and failing on Thursday with identical inputs; the graph either converges (it almost always does, since everyone asks for a minimum) or names its conflict plainly. When two modules need incompatible MAJOR versions, semantic import versioning has already made them different module paths — both can coexist, which pushes the real conflict to API design rather than to version arithmetic."),
			text("dm-mvs-why-one-wins",
				"Why ONE require wins is worth one more paragraph, because it inverts a habit from other ecosystems. In npm-land, raising your dependency's version changes only YOUR copy, and downstream packages keep their older ones; in Go, raising YOUR require raises the version EVERYBODY in this build compiles against — MVS takes the maximum across the whole graph. Upgrades are therefore broadcast, not local: bump a widely-shared library and you must verify the consumers you never read. That is the honest cost of the one-version rule, and this repo pays it deliberately — every pinned bump in the state log came with the full verification suite, not with a hope that the semver label was accurate. The broadcast property is also the security story: patch one vulnerable transitive module with a single ``go get module@patchversion``, and every consumer in the build moves to the fixed code in one stroke — no per-consumer whack-a-mole."),
		),
		// ── 3. tidy, why, graph ────────────────────────────────────
		section("tidy-why-graph", "go mod tidy, why, and graph: the tools that keep the manifest honest",
			"The manifest is a claim; these three subcommands are how the claim stays true — or gets explained, line by line, when you stop believing it.",
			text("dm-tidy",
				"``go mod tidy`` reconciles the manifest with reality: it adds requires for every import your packages (including tests, including build-tagged files) actually reference, removes requires nothing references anymore, recalculates which lines deserve the ``// indirect`` marker, and rewrites go.sum to match. It is the routine after adding or deleting an import — and its appetite for build-tagged files is precisely why this repo's anchor file works: tidy scans imports across build configurations, so a blank import behind a ``tools`` tag counts, even though no normal build compiles that file. The anchors exist because this guide pins dependencies BEFORE the topic that imports them lands — a deliberate require with no importing package would be erased by the very next tidy, and the anchor is the import that keeps the pin on the books until real code absorbs it:"),
			code("dm-tools-go", "go",
				"The anchor file, whole: blank imports behind the tools build tag, each one a deliberate pin tidy refuses to forget.",
				"server/tools/deps.go",
				"//go:build tools\n"+
					"\n"+
					"// Package tools pre-lands the Phase 6 (data layer), Phase 7 (auth/\n"+
					"// security), and Phase 8 (realtime/background jobs) library pins: these are\n"+
					"// deliberate go.mod requires with no importing server package yet, so the\n"+
					"// blank imports below keep them anchored through `go mod tidy`. Each one is\n"+
					"// absorbed by its real import when its topic lands (pgx/gorm/go-redis/\n"+
					"// mongo-driver/testcontainers in topics 16-20, x/crypto argon2id in topic\n"+
					"// 23, x/time rate in topic 25, coder/websocket in topic 28, asynq in topic\n"+
					"// 29) — this file shrinks away as that happens and is removed once empty.\n"+
					"//\n"+
					"// The `tools` build tag keeps this package out of every normal build and\n"+
					"// test binary; only `go mod tidy` (and explicit `-tags tools` builds) see it.\n"+
					"package tools\n"+
					"\n"+
					"import (\n"+
					"	_ \"github.com/coder/websocket\"\n"+
					"	// … one blank import per pinned module\n"+
					")"),
			text("dm-anchor-honesty",
				"The honest state of that file today: the topics it pre-landed pins for have all shipped, so every anchor is now also a real import somewhere in the tree — the file has done its job and stays as documentation of intent, harmless because the build tag keeps it out of every binary while tidy still counts it. Two details make the pattern trustworthy rather than superstitious. The anchors name PACKAGE paths, not module paths (``golang.org/x/crypto/argon2`` anchors the module ``golang.org/x/crypto``) — resolution walks up to the module root, exactly as it does for a real import. And the tag is ``tools`` specifically, the community's fossil of the older ``tools.go`` idiom — the name carries no semantics of its own, only the discipline that nothing else ever sets it. Delete the file and the next tidy would demote nothing (the requires are all backed by real imports now) — the demo panel marks each anchor's module linked-in-binary, which is that absorption, visible."),
			text("dm-why-graph",
				"The two read-side subcommands are how you interrogate a manifest you did not write. ``go mod why -m MODULE`` prints the shortest import chain from your packages to that module — the answer to ``WHO is pulling this in'', and the first tool you reach for when an ``// indirect`` line looks expensive. ``go mod graph`` prints every requirement edge in the resolved graph (``A B`` per line: module A requires module B at some version) — huge by design, because the graph IS huge, and therefore best read filtered and counted: pipe it through ``grep``, ``awk``, or ``sort | uniq`` and the questions it answers become concrete (how many modules require x/net; who is the newest ask on pgx). ``go list -m all`` is the flattened companion — the one line per selected module, MVS's final answer, in the same shape ``go version -m`` later stamps into a binary. The tour at the end of this topic runs all three against this very module."),
			list("dm-module-tools-list",
				false,
				"go mod tidy — reconcile: add missing requires, drop unused ones, fix the indirect markers, regenerate go.sum; the routine after any import change.",
				"go mod why -m — explain: the shortest import chain from your code to a module; the first question to ask of a surprising indirect.",
				"go mod graph — expose: every requirement edge in the resolved graph, one `from to` pair per line; read it filtered.",
				"go list -m all — flatten: the selected version of every module in the build, MVS's final answer as one list.",
				"go mod verify — audit: re-check the module cache's bytes against go.sum; cheap insurance after cache oddities.",
			),
		),
		// ── 4. govulncheck [demo] ─────────────────────────────────
		section("govulncheck", "govulncheck: what you require versus what you can reach",
			"The manifest says what you downloaded; the vulnerability database says what is known to be wrong in it; only the call graph says what your binary can actually reach.",
			text("dm-vulncheck-what",
				"``govulncheck`` (``go install golang.org/x/vuln/cmd/govulncheck@latest`` — a standalone tool, deliberately NOT a dependency of this module) cross-references three sources: your module's dependency list, the Go vulnerability database (vuln.go.dev, mirrored from the Go security team's OSV feed), and — the part that makes it worth running over a naive CVE scan — YOUR CODE'S CALL GRAPH. A vulnerability only matters at the symbols your program can actually execute, and govulncheck's default symbol level traces reachability: it reports a finding when a vulnerable function is callable from your entry points, and explicitly downgrades findings your code merely SHIPS (present in a required module, never called) to an informational note. For a graph like this module's — a hundred-plus indirect modules dragged in by gqlgen, grpc, and the data-layer drivers — that distinction is the entire signal: the scan below reports a known vulnerability in a REQUIRED module while reporting ZERO reachable ones, which is a triage decision, not an outage."),
			text("dm-vulncheck-how",
				"The demo runs the real tool, live, under rules that keep it honest in a teaching context. The handler starts ONE scan per process, in the background, bounded by a timeout — the tool needs the network for its database on first run, and a demo endpoint must never hang a response on that — then caches the observation and serves it on every subsequent request. The output is passed through verbatim: the exit code (zero for no reachable findings; three when findings exist — a non-zero exit that is a RESULT, not an error) and the tool's own closing summary block, never a paraphrase that could mistranslate a verdict. When the tool or the source tree is missing — the distroless production image ships neither — the panel degrades to the tool's documented example output, labeled as an example rather than dressed up as a scan:"),
			code("dm-vulncheck-runner-go", "go",
				"The demo's scan runner: bounded context, stdout capture, and the exit-code discipline — a findings exit is a done observation, never an error to swallow.",
				"server/internal/presentation/demo_deps.go",
				"	started := time.Now()\n"+
					"	scanCtx, cancel := context.WithTimeout(ctx, demos.DepsGovulncheckTimeout)\n"+
					"	defer cancel()\n"+
					"	cmd := exec.CommandContext(scanCtx, binary, \"./...\")\n"+
					"	cmd.Dir = moduleDir\n"+
					"	raw, err := cmd.Output()\n"+
					"	elapsed := time.Since(started)\n"+
					"	// …\n"+
					"	if err != nil {\n"+
					"		if exitErr, ok := err.(*exec.ExitError); ok {\n"+
					"			// A non-zero exit is a RESULT (govulncheck exits 3 when\n"+
					"			// reachable findings exist), not an error to hide.\n"+
					"			return demos.DepsVulncheckObservation{\n"+
					"				State:    demos.DepsVulncheckStateDone,\n"+
					"				ExitCode: exitErr.ExitCode(),\n"+
					"				// …\n"+
					"			}\n"+
					"		}\n"+
					"		// …\n"+
					"	}"),
			text("dm-vulncheck-cadence",
				"Cadence is the operational half of the story. A scan is only a fact about the moment it ran — the database grows daily — so the honest patterns are: locally, on demand or before a release (the terminal tour runs it against this module in seconds on a warm cache); in CI, as a scheduled job rather than a per-commit gate (a NEW vulnerability should not fail unrelated builds mid-fix); and always with the reachable-findings exit treated as urgent while required-module findings are triaged through upgrades. This guide's family stance has no hosted CI at all — local verification is the gate — so the demo's one-scan-per-process model mirrors the honest local rhythm: run it when you touch dependencies, read the tool's own words, act deliberately."),
			demo("deps:manifest", "The deps manifest: live go.mod beside live build info, and a real govulncheck",
				"One payload, all runtime-derived: the manifest panel parses this server's real go.mod (module, go directive, direct requires with their pinned versions, indirect count, tool directives), the build panel reads the serving binary's own debug.ReadBuildInfo and states WHICH binary it is, and each direct require is stamped with whether the binary links it — the manifest-versus-binary lesson (test-only requires never link). The anchor panel walks tools/deps.go's blank imports paired with their live versions; the cache panel names the real gdg-go/gdg-gocache/gdg-npm volumes; and the govulncheck panel serves the real tool's exit code and verbatim summary when it can run (reload once — the scan is backgrounded), degrading to a labeled example when it cannot."),
			text("dm-vulncheck-crossref",
				"One cross-reference keeps the demo honest about its own scope: the go-http-fundamentals app-info demo (topic 04) already serves one slice of this story — its ``dependencyCount`` is the linked-module count from the SAME build info this panel reads. This demo derives both sides of that number — go.mod's requirement list on one side, the binary's link set on the other — and the gap between them is the teaching: testcontainers-go is a direct require this serving binary never links, the tool-anchor modules ride the manifest without riding the binary, and the long indirect tail of the graph exists only as versions in a file. Numbers that come from the running process cannot drift from it; that is why the panel derives everything and hardcodes nothing."),
		),
		// ── 5. Upgrades ────────────────────────────────────────────
		section("upgrades", "Upgrades: bumping a pin on purpose",
			"MVS never upgrades on its own — every version bump in Go is somebody's explicit act. Doing that act deliberately is a three-step discipline.",
			text("dm-upgrade-mechanics",
				"The mechanics are one command: ``go get github.com/jackc/pgx/v5@v5.12.0`` (or ``@latest``, ``@patch``, ``@minor`` — the version query resolves, then edits go.mod's require to the chosen version and syncs go.sum; MVS takes the new maximum across the graph for every consumer). What separates an upgrade from a suggestion is everything AFTER the command: ``go mod tidy`` to settle the indirect markers, then the full verification gate this repo runs — build, vet, tests with the race detector — because the broadcast property from the MVS section means every consumer of the bumped module just moved, semver label notwithstanding. A MAJOR upgrade additionally touches code: the new major version's module path differs (``/v6``), so imports change string-by-string — annoying by design, because incompatible API deserves a mandatory diff, not a silent swap."),
			text("dm-upgrade-discipline",
				"This repo's discipline is the state log's pin list: dependency versions are recorded decisions, upgraded on purpose with the verification suite green, never drifted into by a bot — the family's no-Dependabot stance (no hosted CI means no hosted bump PRs either; local verification is the only gate). The Dockerfile below is the same discipline at the image layer: the toolchain is pinned by the base image tag, the manifests are copied BEFORE the source so an upgrade re-downloads dependencies in exactly one cached layer, and nothing about a routine code change can invalidate the dependency layer. Upgrade cadence follows the security story: reachable-vulnerability findings jump the queue (one ``go get module@fixedversion`` moves every consumer at once), everything else batches into deliberate maintenance passes:"),
			code("dm-dockerfile-prod", "dockerfile",
				"The prod builder's dependency layer — manifests first, download second, source last: an upgrade costs one layer rebuild, a code change costs none of the downloads.",
				".docker/server/Dockerfile.prod",
				"WORKDIR /app\n"+
					"\n"+
					"# Manifests first to leverage Docker layer caching (deps change least).\n"+
					"COPY server/go.mod server/go.sum ./\n"+
					"RUN go mod download\n"+
					"\n"+
					"COPY server/ ./"),
			list("dm-upgrade-steps-list",
				true,
				"Pick the target — a specific version, @patch, or @minor for the bump's actual ambition; check the module's changelog for breaking edges.",
				"Run go get — the require moves in go.mod, go.sum syncs, and MVS raises the version for every consumer in the graph at once.",
				"Settle and verify — go mod tidy, then the full gate (build, vet, go test -race); a major bump also rewrites import paths first.",
				"Record it — the pin list in the project state notes the deliberate version, so the next reader knows it was chosen, not inherited.",
			),
		),
		// ── 6. Caching in Docker ───────────────────────────────────
		section("caching-in-docker", "Module and build caching in Docker: the warm-exec story",
			"The verification protocol runs inside a long-lived container; the named volumes are why its go build/vet/test passes take seconds, not minutes.",
			text("dm-two-caches",
				"Go maintains two content-addressed caches, and warming them is the difference between a pleasant container loop and a punishing one. The MODULE cache (``/go/pkg/mod``) holds every downloaded module version, unpacked once, read-only, shared by every build on the machine — a warm module cache means ``go mod download``, fresh testcontainers, and govulncheck's analysis all skip the network. The BUILD cache (``/root/.cache/go-build``) holds compiled package objects keyed by a hash of sources, build flags, and toolchain — an unchanged package is a cache hit no matter which binary is being linked, which is why this repo's standard docker-exec verification pass stays seconds-fast: almost every package is unchanged almost every time. Both live on NAMED VOLUMES in the dev compose stack, so they survive container recreation and are shared with the stack-down fallback runs — never baked into an image (which would bloat every rebuild) and never left inside a throwaway container's writable layer:"),
			code("dm-compose-yaml", "yaml",
				"The dev compose stack's cache wiring: the dev service mounts the named volumes at the cache paths, and the volumes are external — pre-created, unprefixed, shared.",
				".docker/docker-compose.yml",
				"    volumes:\n"+
					"      # Live source for the whole repo.\n"+
					"      - ./../:/workspaces/go_developers_guide\n"+
					"      # Named Go caches: module cache + build cache ride volumes so every\n"+
					"      # docker-exec verification run stays warm (never the image, never the\n"+
					"      # bind-mounted workspace).\n"+
					"      - gdg-go:/go/pkg/mod\n"+
					"      - gdg-gocache:/root/.cache/go-build\n"+
					"      # …\n"+
					"volumes:\n"+
					"  # …\n"+
					"  gdg-go:\n"+
					"    name: gdg-go\n"+
					"    external: true"),
			text("dm-external-volumes",
				"The ``external: true`` block is a scar, and the comment in the compose file says so: without it, compose derives a project-prefixed volume name and helpfully creates an EMPTY twin on first run — silently stranding a warm cache behind a slightly different name and forcing full dependency re-downloads for no visible reason. Pre-creating the volumes once per machine (``docker volume create gdg-go gdg-gocache gdg-npm``) and declaring them external means every consumer — the compose stack, a stack-down ``docker run`` fallback, anything else mounting them — agrees on one name and one warm cache. The npm sibling (``gdg-npm``) completes the picture for the client half of this repo: same pattern, different toolchain, one lesson — caches are machine state, not image state, and worth naming explicitly."),
			text("dm-layer-ordering",
				"The production image plays the same game at the layer level, and the ordering is the whole trick — it is the Dockerfile quote from the upgrades section read as a cache discipline: ``COPY server/go.mod server/go.sum`` first, ``RUN go mod download`` as its own layer, source LAST. Docker invalidates a layer when its inputs change, and everything below it rebuilds; with manifests first, a source-only change re-executes only the compile layer (dependencies already downloaded in the layer beneath), while a dependency change re-runs the download and everything after — exactly the one-layer cost an upgrade should pay. Get the order backwards (source before manifests) and every edit re-downloads the world. The dev image inverts the concern deliberately — its compose file bind-mounts live source over the workspace and shadows the container's node_modules with an anonymous volume, because the DEV loop optimizes for always-fresh code over reproducible layers; the prod loop optimizes the opposite way, and the two Dockerfiles are the two answers side by side."),
		),
		// ── 7. Choosing dependencies ───────────────────────────────
		section("choosing-dependencies", "Choosing dependencies: the bar this repo holds",
			"Every require is a promise to maintain someone else's code alongside yours. The bar for making that promise is mostly knowable in advance.",
			text("dm-bar",
				"The first rule is the cheapest: the STDLIB ALREADY HAS IT. Go's standard library is large and deliberately boring — an HTTP server, a JSON encoder, a testing framework, a structured logger, a template engine, crypto primitives — and this guide's stack decisions read as a tour of that boundary: net/http instead of a router framework, slog instead of a logging library, testing instead of testify, encoding/json instead of a reflection turbo-encoder. Every stdlib choice deletes a require, an upgrade path, and a supply-chain surface; the stdlib never major-versions you (the Go 1 compatibility promise is the strongest in the industry) and never abandons you (its maintenance is funded by the language itself). The honest exceptions are where the stdlib genuinely stops — and this repo's own decisions show how SMALL that set is once the discipline holds:"),
			code("dm-ratelimit-go", "go",
				"One of the rare exceptions, and how small it stays: the stdlib has no token-bucket rate limiter, so the security domain adds exactly one line of one module — nothing else in the file is not stdlib.",
				"server/internal/domain/security/ratelimit.go",
				"import (\n"+
					"	\"context\"\n"+
					"	\"fmt\"\n"+
					"	\"sync\"\n"+
					"	\"time\"\n"+
					"\n"+
					"	\"golang.org/x/time/rate\"\n"+
					")\n"+
					"\n"+
					"// RatePolicy is a token-bucket contract: a sustained RATE of tokens\n"+
					"// per second and a BURST ceiling — the bucket size the client may\n"+
					"// spend instantly.\n"+
					"type RatePolicy struct {\n"+
					"	Rate  rate.Limit // sustained allowance, tokens per second (fractional allowed)\n"+
					"	Burst int        // bucket size — the instant ceiling\n"+
					"}"),
			text("dm-signals",
				"When a require is genuinely needed, the maintenance signals are public and worth reading before committing. WHO maintains it — the Go team's own ``golang.org/x/...`` satellites (x/time, x/crypto, x/net) live nearest the stdlib's guarantees; a vendor with a paid product behind the library (pgx and its database-company stewardship) beats a solo hero project. WHAT it drags along — the transitive weight is one ``go mod graph | grep`` away before you commit, and a database driver that requires an entire cloud SDK is a different decision than the same driver requiring nothing. HOW it fails — issues answered, releases shipped, a SECURITY.md, semver discipline in the tag history. And whether it can LEAVE — an interface at your boundary (this repo's repository ports) means the adapter is the only file that knows the library's name, so the wrong choice is a rewrite of one file, not a strangulation of the codebase. The dependency rule from the architecture story does this job for free: presentation → domain ← data keeps third-party dialects at the edges."),
			list("dm-choosing-list",
				false,
				"Stdlib first — the largest boring standard library in the industry; every stdlib choice deletes a require and an upgrade path.",
				"Transitive weight — count the graph edges a candidate adds before committing; the cost of a dependency is the tree, not the node.",
				"Stewardship — who maintains it, how releases ship, whether issues get answers; x/... satellites sit closest to the stdlib's guarantees.",
				"Containment — an interface at your boundary means the wrong choice is a one-file rewrite, not a codebase strangulation.",
				"Justification on record — the state log's pin list names WHY each require exists, so the next maintainer inherits reasons, not folklore.",
			),
		),
		// ── 8. Terminal companion ──────────────────────────────────
		section("terminal-companion", "Terminal companion: the dependency tour",
			"The manifest demo over curl, the binary's birth certificate over go version -m, the module subcommands against the real graph, and govulncheck's live verdict — all from the dev container.",
			text("term-intro",
				"With the stack up (server on :8100), start where the demo starts: one GET answers the whole payload — go.mod parsed live beside the serving binary's build info, the anchors with their versions, the cache volumes, and the govulncheck panel (give the background scan a beat, then reload for its verdict). The second block moves to the binary itself: build it and read ``go version -m`` — the same manifest the demo parses, but as the go tool stamps it into a compiled artifact, dependency hashes and all. The third block interrogates the graph with the module subcommands and ends with the real scanner. Run them from ``server/`` inside the dev container, where the module cache is warm and the tool is installed."),
			code("term-demo-bash", "bash", "", "",
				"# the demo payload: live manifest + build info + anchors + caches + govulncheck\n"+
					"curl -s localhost:8100/api/demo/deps/manifest | python3 -m json.tool | head -60\n"+
					"curl -s localhost:8100/api/demo/deps/manifest | python3 -c \\\n"+
					"  'import json,sys; p=json.load(sys.stdin); print(p[\"vulncheck\"][\"mode\"], p[\"vulncheck\"][\"exitCode\"])'\n"+
					"\n"+
					"# the binary's own birth certificate — module, toolchain, every linked dep + hash\n"+
					"go build -o /tmp/guide ./cmd/guide\n"+
					"go version -m /tmp/guide | head -12\n"+
					"#   path\tguide/cmd/guide\n"+
					"#   mod\tguide\t(devel)\n"+
					"#   dep\tgithub.com/99designs/gqlgen\tv0.17.98\th1:t7UTlygkXMx1tJmKyR0…\n"+
					"go version -m /tmp/guide | grep -c '^\\sdep'   # the linked count (a subset of go.mod)"),
			code("term-graph-bash", "bash", "", "",
				"# who pulls what in — the shortest import chain to a module\n"+
					"go mod why -m github.com/redis/go-redis/v9\n"+
					"# the full requirement graph, read filtered\n"+
					"go mod graph | wc -l                        # every edge in the resolved graph\n"+
					"go mod graph | grep ' golang.org/x/net' | head -5   # who requires x/net, and at what floor\n"+
					"go list -m all | wc -l                       # MVS's final answer, one line per module\n"+
					"grep -c '// indirect' go.mod                 # the indirect rows in the manifest itself\n"+
					"\n"+
					"# the scanner, live: reachable findings vs required-module noise\n"+
					"govulncheck ./... ; echo \"exit: $?\"\n"+
					"#   === Symbol Results ===\n"+
					"#   No vulnerabilities found.\n"+
					"#   … 1 vulnerability in modules you require, but your code doesn't\n"+
					"#   appear to call these vulnerabilities.\n"+
					"#   exit: 0"),
			text("term-coda",
				"Read the tour as the topic compressed into a session. The demo payload is the whole thesis in one response: two views of one module — what go.mod claims and what the binary links — plus the anchors, the caches, and a scanner verdict, every number derived at request time. ``go version -m`` is the same discipline made portable: a birth certificate baked into the artifact, readable years later on any machine with the go tool, the h1: hashes tying the binary to the exact module bytes go.sum verified. The graph block is the interrogation kit — why for explanations, graph for edges, list for the resolution — and the govulncheck leg is the closing loop between dependency management and security: the manifest says what you took, the database says what is known about it, and the call graph says whether any of it is your problem. Dependency management in Go is not a folder of config; it is two honest files, three subcommands, one deterministic algorithm, and the discipline to bump pins on purpose."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"dependency-management",
		"Dependency management in Go: modules, MVS, and the caches underneath",
		"What a dependency IS in Go — a module pinned at a version, "+
			"resolved to one copy per build — and the machinery around "+
			"that rule, taught from the artifacts this repo runs: go.mod "+
			"and go.sum with minimal version selection (why one require "+
			"wins and why upgrades broadcast), the tidy/why/graph tool "+
			"belt with the tools/deps.go anchor pattern that keeps pins "+
			"honest before their topic lands, govulncheck's reachability "+
			"verdict over the live graph (demo deps:manifest parses the "+
			"real go.mod beside the serving binary's own build info, "+
			"walks the anchors, names the gdg-go/gdg-gocache Docker "+
			"volumes, and runs the real scanner when it can), upgrades "+
			"as deliberate acts with manifests-first layer ordering, and "+
			"the bar for choosing a dependency at all — stdlib first, "+
			"stewardship, transitive weight, containment.",
		"platform",
		31,
		sections,
	)
}
