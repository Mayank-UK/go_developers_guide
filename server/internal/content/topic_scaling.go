package content

import (
	"errors"
	"fmt"
)

// This file is topic 40 scaling (group ship, curriculum order 40 — the
// Ship group's last word): serving more load by adding copies, not by
// re-architecting. Deployment shipped one copy; performance made that
// copy fast; reliability kept it useful while its dependencies
// misbehaved; scaling asks what changes when two, four, forty copies
// run — and the honest answer is: nothing about the code, everything
// about the assumptions. The topic walks the request path's scaling
// surfaces from this repo's real artifacts: the composition root's
// per-process state, the k8s Service and Deployment manifests as the
// in-cluster balancer and replica story, the schema's own index
// doctrine and the process-wide pool as the data tier's rails, the
// client nginx's immutable-asset edge, the rate limiter and the
// realtime hub's ceilings as backpressure, and the metrics pair
// capacity arithmetic feeds on. The topic's demo, scaling:stateless,
// is real machinery: three in-process replicas behind a genuine
// round-robin balancer with an affinity table, one session walked
// through replica-local memory (lost at the first hop, lost again at
// the first rolling update) and through the shared store (found by
// every replica, surviving every swap) — every count on the wire
// measured, nothing invented.

// scalingTopic builds topic 40.
func scalingTopic() (Topic, error) {
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
		// ── 1. What scaling means ───────────────────────────────
		section("what-scaling-means", "What scaling means",
			"More copies, not more archaeology: what actually changes when demand exceeds one copy — and the two questions that organize every decision after.",
			text("scl-wsm-pitch",
				"Topics 37 through 39 built a stack that ships, stays fast, and survives its dependencies failing — one copy of everything. Scaling is the next question: what changes when demand exceeds what one copy can do? The honest answer is that nothing about the APPLICATION changes — the same handlers, the same routes, the same database — but a set of assumptions you never had to examine at one copy suddenly become load-bearing: that state lives where the request lands, that one database connection is always available, that a slow client is YOUR client, that the file the browser cached is the file you are serving. Scaling is the discipline of examining those assumptions BEFORE the traffic examines them for you. It is also the last purely architectural topic in the guide's Ship group for a reason: everything here composes mechanisms the earlier topics built — the deployment artifacts (topic 37), the measured latencies (topic 38), the bulkheads and shedding (topic 39) — and multiplies them."),
			text("scl-wsm-axes-pitch",
				"The word scales in two directions, and conversations go wrong when the two are conflated. VERTICAL scaling makes the one copy bigger — more CPU, more memory, faster disks. HORIZONTAL scaling adds copies of the parts that tolerate copying. Vertical is a purchase order; horizontal is an architecture. The whole topic that follows is essentially the price list for horizontal, axis by axis:"),
			list("scl-wsm-axes",
				false,
				"App tier — more replicas of the server: cheap copies of the same static binary, but every piece of in-process state becomes a bug (the next section is that sentence, taught live).",
				"Balancing — something must choose between the copies for every request: the balancer is where statelessness stops being theory and starts being routing policy.",
				"Data tier — the stores the replicas share: indexes and pooling first, read replicas and sharding only when the single writer genuinely runs out.",
				"Edge — bytes served closer to the client: immutable assets cached for a year, the origin answering only what must be fresh.",
				"Shedding — deciding, on purpose, which work to refuse when the rest is not enough: bounded queues, ceilings, and the 429 envelope.",
			),
			text("scl-wsm-assumption",
				"THE FIRST ASSUMPTION TO EXAMINE is the one this repo states most plainly: state built per process. The composition root constructs exactly one scenario store per boot — the runtime toggle every demo reads — and hands THE SAME POINTER to every handler it wires. At one copy that is an invisible convenience; at two copies it is two stores that agree on nothing. The wiring is worth reading as a scaling contract before the demo breaks it live:"),
			code("scl-wsm-store-quote", "go",
				"One store per process, one pointer per route: invisible at one copy, a lie at two.",
				"server/cmd/guide/main.go",
				"scenarioStore := core.NewScenarioStore()\n"+
					"mux.Handle(\"GET /api/demo/scenario\", presentation.NewScenarioGetHandler(scenarioStore))\n"+
					"mux.Handle(\"PATCH /api/demo/scenario\", presentation.NewScenarioPatchHandler(scenarioStore, logger))"),
			text("scl-wsm-two-questions",
				"Two questions organize everything that follows. First: WHERE DOES STATE LIVE? A replica is a copy of code plus whatever the code keeps in its own heap — if a second replica cannot see a write the first one made, the balancer will route a user's next request to a replica that lies to them. Second: HOW MUCH IS ENOUGH? Copies cost money and shared data tiers have ceilings, so capacity has to be measured, not guessed — which is where topic 33's metrics and topic 38's benchmarks pay out. The rest of this topic answers the first question with a live walk and an inventory, and the second with Little's law and the two numbers the metrics registry already serves."),
		),
		// ── 2. Statelessness ────────────────────────────────────
		section("statelessness", "Statelessness: where state actually lives",
			"This section carries the topic's demo — one session, two placements, a real round-robin fleet.",
			text("scl-st-pitch",
				"A stateless service is not a service without state — every useful application has state somewhere. It is a service where NO request depends on WHICH copy served the previous one: any replica can answer any request correctly, because everything the answer needs either travels with the request, lives outside the process, or is safely derivable. That definition makes statelessness a PLACEMENT question, not a purity test — and this repo answers it honestly rather than ideologically. The posts resource externalizes by design: the switching repository fronts the in-memory fake and the PostgreSQL adapter with the SAME port, so the demo panel's dataMode flag decides placement at runtime. The wiring is the canonical externalization seam:"),
			code("scl-st-switching-quote", "go",
				"The honest seam: one port, two adapters — placement is a wiring decision, not a rewrite.",
				"server/cmd/guide/main.go",
				"postsRepo := postsdata.NewSwitchingRepository(\n"+
					"	scenarioStore,\n"+
					"	postsdata.NewInMemoryRepository(),\n"+
					"	postsdata.NewPostgresRepository(pool),\n"+
					")"),
			text("scl-st-placements-pitch",
				"The three placements, with this repo's real residents in each — the demo inventories exactly these:"),
			list("scl-st-placements",
				false,
				"In-process — the scenario flags, the fake-mode posts substrate, the auth demos' sessions and keys, the rate limiter's per-client buckets, the chat hub's rooms: fast, free, and invisible to every other replica.",
				"Externalized — PostgreSQL's tables (goose-owned schema), Redis's keys and streams, MongoDB's collections: one hop away, shared by every replica, surviving every restart.",
				"Client-side — the session cookie and the JWT (topic 23): state the browser carries, with very different server-side consequences.",
			),
			text("scl-st-demo-pitch",
				"The demo is real machinery, not narration. It builds three in-process replicas — each with its own private session map and served counter — fronts them with a genuine round-robin balancer that keeps an affinity (sticky) table, and stands a shared store beside the fleet as the externalized placement. It then walks ONE session through both placements and measures everything: how the balancer distributes twelve anonymous requests; where the local session lands and who cannot see it; what sticky routing fixes; what the rolling update does to the fix; and how differently the shared session behaves under the identical choreography. The balancer at the center of it is small enough to read in full:"),
			code("scl-st-balancer-quote", "go",
				"The demo's real load balancer: a ring, an affinity table, and a swap that changes the process under a slot without telling the table.",
				"server/internal/domain/demos/scaling_stateless.go",
				"func (b *ScalingBalancer) Next() (*ScalingReplica, int) {\n"+
					"	b.mu.Lock()\n"+
					"	defer b.mu.Unlock()\n"+
					"	slot := b.next % len(b.replicas)\n"+
					"	b.next++\n"+
					"	return b.replicas[slot], slot\n"+
					"}\n"+
					"// …\n"+
					"func (b *ScalingBalancer) Pin(token string) *ScalingReplica {\n"+
					"	b.mu.Lock()\n"+
					"	if slot, ok := b.sticky[token]; ok {\n"+
					"		replica := b.replicas[slot]\n"+
					"		b.mu.Unlock()\n"+
					"		return replica\n"+
					"	}\n"+
					"	b.mu.Unlock()\n"+
					"	replica, _ := b.Next()\n"+
					"	return replica\n"+
					"}\n"+
					"// …\n"+
					"func (b *ScalingBalancer) ReplaceReplica(slot int) (fresh, terminated *ScalingReplica) {\n"+
					"	b.mu.Lock()\n"+
					"	defer b.mu.Unlock()\n"+
					"	terminated = b.replicas[slot]\n"+
					"	fresh = newScalingReplica(slot)\n"+
					"	b.replicas[slot] = fresh\n"+
					"	return fresh, terminated\n"+
					"}"),
			demo("scaling:stateless", "One session, two placements, a real fleet",
				"GET /api/demo/scaling/stateless builds three in-process replicas behind a real "+
					"round-robin balancer with an affinity table, plus the shared store standing "+
					"for externalized state, and walks one session through both placements with "+
					"every number measured: twelve anonymous requests distribute exactly evenly "+
					"(per-replica counts read off the replicas themselves) · the local session "+
					"is stored on one replica, missed by the next hops, rescued by sticky "+
					"routing — then killed by a rolling-update swap of the pinned replica, "+
					"whose dying counters are quoted from the terminated instance · the shared "+
					"session is written through a fresh replacement, found by every replica of "+
					"the fleet, and survives an identical swap untouched, while six increments "+
					"spread across the fleet read back as one accumulated counter. The round is "+
					"deterministic — no clock, no randomness — so the payload replays "+
					"identically on every request."),
			text("scl-st-reading",
				"Read the panel in three acts. The DISTRIBUTION rows are the baseline promise: round-robin spreads evenly when the count divides evenly, and the payload states the spread from the measured rows, never from the algorithm's intentions. The SESSION ROUND is the argument: the local token is stored exactly once and missed everywhere else — process-local state does not follow the request, and the balancer has silently logged the user out; sticky routing is the classic workaround, and it works right up to the swap step, where the affinity entry survives but the memory it pointed at does not, which is the sentence every sticky-session deployment signs at every rollout. The SHARED act is the counter-argument: the same login through the store hits from every replica including the fresh replacements, and the counter beat shows accumulation no replica-local counter could match. The handler that serves all this is deliberately thin:"),
			code("scl-st-handler-quote", "go",
				"The demo's thin handler: build the round, run it, shape the observed facts — nothing else.",
				"server/internal/presentation/demo_scaling.go",
				"func (h *ScalingStatelessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"	round, err := demos.NewScalingStatelessRound(demos.DefaultScalingStatelessConfig())\n"+
					"	if err != nil {\n"+
					"		// …\n"+
					"	}\n"+
					"	writeJSON(w, http.StatusOK, demos.BuildScalingStatelessPayload(round.Run()))\n"+
					"}"),
			text("scl-st-honesty",
				"THE HONESTY NOTE matters as much as the result. The three replicas are state worlds inside one process — an honest single-process demo cannot fork the server, but it can construct exactly what a second process would: the same fresh-memory replica a rolling update boots. The shared store is an in-process analogue of the redis container this stack carries (topics 20 and 21 drove the real client calls); the lesson is PLACEMENT, not hops — the local session died at the first cross-replica request because it lived in a heap, and the shared one survived the swap because it never did. Production reads the same trade as a wiring decision: sessions and rate budgets move to Redis or accept that N replicas enforce N times the limit; sticky sessions avoid the move by renting affinity from the balancer and paying it back at every deploy. Externalizing costs a hop on every request; keeping state in-process costs correctness the moment a second copy exists. There is no third option — every scaling architecture is a position on exactly that trade."),
		),
		// ── 3. Load balancing ───────────────────────────────────
		section("load-balancing", "Load balancing: one address, many replicas",
			"The component that makes N copies look like one service — and the piece that punishes hidden state.",
			text("scl-lb-pitch",
				"Once two replicas exist, something must choose between them on every request: the load balancer. It is the piece that makes N copies look like ONE address — DNS points at the balancer, the balancer points at the replicas, and no client knows how many there are. The choice of replica is where the statelessness question becomes routing policy: with no replica-local state, ANY replica answers ANY request correctly, so the balancer is free to use the dumbest algorithm that spreads load. With replica-local state, every algorithm is wrong sometimes — the right replica is the one holding the user's session, and the demo just measured what routing elsewhere does. In Go's own small deployments the balancer is often not a separate box at all: the client nginx in front of this stack proxies /api/ to the compose service name, and Docker's embedded DNS already round-robins a scaled service name across its containers — one line of config away from a fleet."),
			list("scl-lb-algorithms",
				false,
				"Round-robin — next replica in the ring: trivial and fair over time, blind to the replica that is slow or the request that is expensive.",
				"Least-connections — fewest in-flight requests wins: still simple, and self-correcting when one replica is slow because its count stays high while it catches up.",
				"IP or cookie hash — consistent routing by client identity: sticky behavior by construction, which is a feature for in-process sessions and a liability the moment the hashed replica dies.",
				"Weighted — split by capacity when replicas are uneven (the 8-core box takes twice the 4-core box's share): vertical and horizontal mixed in one fleet.",
				"Health-aware — route away from the replica failing its checks: the balancer becomes the first layer of the reliability story (topic 39's degradation, one level up).",
			),
			text("scl-lb-k8s-pitch",
				"This repo's real in-cluster balancer ships with the deployment topic's manifests: the Kubernetes Service. A Service is a stable virtual address whose selector gathers the pods that match it, and kube-proxy on every node spreads connections across the matching endpoints — the demo's round-robin ring, implemented by the platform instead of the demo. The manifest is worth reading as the whole mechanism in one object:"),
			code("scl-lb-service-quote", "json",
				"The repo's real balancer: a stable name over a selector — endpoints come and go, the address never notices.",
				"deploy/k8s/service.json",
				"{\n"+
					"  \"apiVersion\": \"v1\",\n"+
					"  \"kind\": \"Service\",\n"+
					"  \"metadata\": {\n"+
					"    \"name\": \"guide\",\n"+
					"    \"namespace\": \"go-developers-guide\",\n"+
					"    \"labels\": {\n"+
					"      \"app.kubernetes.io/name\": \"go-developers-guide\",\n"+
					"      \"app.kubernetes.io/component\": \"server\"\n"+
					"    }\n"+
					"  },\n"+
					"  \"spec\": {\n"+
					"    \"type\": \"ClusterIP\",\n"+
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
					"    ]\n"+
					"  }\n"+
					"}"),
			text("scl-lb-service-reading",
				"Notice how much scaling work those few lines do — and what they assume. The Service owns one stable name and port pair over a selector: scale the Deployment's replica count and the endpoint list changes under the address with no client the wiser. But the mechanism is only safe because any pod matching the selector can answer any request: the selector IS the statelessness contract, enforced by routing. Health participates too — a pod that fails its readiness probe (topic 37 wired /api/ready for exactly this) leaves the endpoint list until it recovers, which is the health-aware algorithm running as platform plumbing. And the two ports teach a Go-specific wrinkle: gRPC rides HTTP/2, whose long-lived streams pin a connection to ONE pod — a ClusterIP's per-connection spreading matters less than per-request spreading, which is why gRPC-heavy fleets reach for client-side balancing or a proxy that understands the protocol (Envoy's round robin lives in comparison prose, per the family stance — this stack's balancer is the platform's, and the demo's ring)."),
		),
		// ── 4. Replicas ─────────────────────────────────────────
		section("replicas", "Replicas: copies of the work",
			"The Deployment object is the replica story in one manifest — and graceful shutdown is what makes rollouts safe.",
			text("scl-rp-pitch",
				"A replica is a copy of the stateless tier: same image, same binary, same config, its own heap. The deployment topic's Kubernetes manifest states the whole contract in a handful of fields — how many copies, how they replace each other, and how a copy proves it is ready to serve. Read the strategy block as the rolling update the demo simulated: maxUnavailable 0 means capacity never drops during a rollout, maxSurge 1 means the new copy boots BEFORE the old one dies, and minReadySeconds means the fresh pod must stay healthy for a breath before the next swap — exactly the choreography that killed the sticky session and spared the shared one:"),
			code("scl-rp-deployment-quote", "json",
				"The replica contract: two copies, replaced one at a time, capacity never dropping.",
				"deploy/k8s/deployment.json",
				"  \"spec\": {\n"+
					"    \"replicas\": 2,\n"+
					"    \"minReadySeconds\": 5,\n"+
					"    \"strategy\": {\n"+
					"      \"type\": \"RollingUpdate\",\n"+
					"      \"rollingUpdate\": {\n"+
					"        \"maxSurge\": 1,\n"+
					"        \"maxUnavailable\": 0\n"+
					"      }\n"+
					"    },\n"+
					"    \"selector\": {\n"+
					"      \"matchLabels\": {\n"+
					"        \"app.kubernetes.io/name\": \"go-developers-guide\",\n"+
					"        \"app.kubernetes.io/component\": \"server\"\n"+
					"      }\n"+
					"    },"),
			text("scl-rp-graceful-pitch",
				"What scaling does NOT change is everything the copies share — the database they point at, the network they arrive over, the address that fronts them. What it DOES change is the lifecycle arithmetic: two replicas means rollouts and restarts happen constantly instead of rarely, which makes GRACEFUL SHUTDOWN a scaling property, not a nicety. The terminationGracePeriodSeconds the manifest grants each pod is only honest if the process underneath actually drains — this server's shutdown path does, bounded, on both surfaces:"),
			code("scl-rp-shutdown-quote", "go",
				"The drain every rollout depends on: in-flight work finishes inside a bounded window, then the process exits.",
				"server/cmd/guide/main.go",
				"	// Both surfaces drain in-flight work: gRPC first (its RPCs are short),\n"+
					"	// then HTTP inside its bounded timeout.\n"+
					"	grpcServer.GracefulStop()\n"+
					"	logger.Info(\"grpc server stopped\")\n"+
					"	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)\n"+
					"	defer cancel()\n"+
					"	if err := srv.Shutdown(shutdownCtx); err != nil {\n"+
					"		return fmt.Errorf(\"graceful shutdown: %w\", err)\n"+
					"	}\n"+
					"	logger.Info(\"server stopped\")"),
			text("scl-rp-reading",
				"Put the three artifacts together — Service, Deployment, shutdown — and the replica story is complete: the Deployment keeps two copies alive and replaces them one at a time without dropping capacity; the Service keeps one address over whichever copies exist; the drain keeps each dying copy useful until its replacement is ready. The demo measured the state side of that choreography twice: the local session died at the swap because it lived in the process, and the shared one did not because it lived in the store. The operational side is the same lesson one level up: a rolling update is a stream of small failures, and only stateless replicas make it a non-event. One more Go-specific note: replicas of THIS server are cheap in a way most runtimes envy — the static binary (topic 37's distroless image) means a new replica is a small container start, no runtime to warm, no interpreter to boot. Cheap replicas change the arithmetic of autoscaling: fleets can track load closely instead of provisioning for peaks."),
		),
		// ── 5. Data at scale ────────────────────────────────────
		section("data-at-scale", "Data at scale: the tier that does not copy",
			"Replicas share one database — so the data tier scales by making the one database do less, then helping it share.",
			text("scl-da-pitch",
				"The stateless tier scales by cheap copies; the data tier does not, because every replica points at the SAME database, and a database cannot be copied the way a container can — its copies would disagree. So data scaling is a different discipline with a strict order of operations: first make the one database do less work per query (INDEXES), then make its connections a bounded shared resource (POOLING), then cache the answers (topic 21 did), then read from copies (REPLICAS), then split by key (SHARDING) — each step roughly an order of magnitude more expensive than the one before it, which is why the order matters and why most systems that think they need sharding need an index and a cache. This repo deliberately runs the first two steps for real and teaches the last two as prose; the schema's own migration states the index doctrine in its bones:"),
			code("scl-da-index-quote", "sql",
				"Indexes are per-query decisions, not decoration: the schema's own rationale, in the migration that created them.",
				"server/internal/data/db/migrations/00001_init.sql",
				"CREATE INDEX posts_created_at_idx   ON posts (created_at DESC);\n"+
					"CREATE INDEX post_tags_tag_name_idx ON post_tags (tag_name);"),
			text("scl-da-index-reading",
				"Two lines, two scaling lessons. The first index is QUERY-SHAPED, not table-shaped: the API's default listing is newest-first, so the b-tree is built descending and serves the ORDER BY without a sort node — you index the questions, not the data (the schema-explorer demo introspects both live, and topic 17's EXPLAIN stop shows the plan they buy). The second is the fact teams forget: PostgreSQL does NOT auto-index foreign-key columns, so the reverse direction of the pair table — which posts carry a tag — needs its own btree or every such read scans. At fleet scale the same doctrine compounds: every replica serves the same query mix against the same tables, so one missing index is multiplied by the replica count — the cheapest capacity upgrade in this whole topic is often a single CREATE INDEX. The cost side is real too: every index amplifies writes (each INSERT maintains every tree), which is why the doctrine is per-query decisions, not insurance policies."),
			text("scl-da-pool-pitch",
				"POOLING is the second lever, and it is replicas-times-connections arithmetic: every process holds a small pool of database connections, so twenty replicas with a pool of five are a hundred connections against a PostgreSQL whose default ceiling is a hundred. The pool is the bulkhead that keeps that multiplication honest — this server builds exactly one per process, proven with a ping before boot continues:"),
			code("scl-da-pool-quote", "go",
				"One pool per process, proven at boot: the bounded connection owner every replica carries against the shared database.",
				"server/internal/data/db/db.go",
				"func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {\n"+
					"	pool, err := pgxpool.New(ctx, dsn)\n"+
					"	if err != nil {\n"+
					"		return nil, fmt.Errorf(\"parse database url: %w\", err)\n"+
					"	}\n"+
					"	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)\n"+
					"	defer cancel()\n"+
					"	if err := pool.Ping(pingCtx); err != nil {\n"+
					"		pool.Close()\n"+
					"		return nil, fmt.Errorf(\"ping database: %w\", err)\n"+
					"	}\n"+
					"	return pool, nil\n"+
					"}"),
			text("scl-da-beyond",
				"READ REPLICAS and SHARDING are the levers this repo deliberately does not run, and they deserve honest prose rather than a fabricated exhibit. A read replica is a second PostgreSQL continuously fed the primary's write-ahead log: reads scale horizontally by pointing read-heavy paths at it, writes do not, and the price is replication lag — a read-your-writes violation the app must decide to tolerate (a feed page) or route around (the confirmation page after a write). Sharding splits the data itself by a key (user, tenant, region) across independent databases: it removes the single-writer ceiling entirely and charges the full price — cross-shard queries become application code, transactions stop spanning shards, and resharding a live system is one of the hardest operations in the field. The restraint is the teaching: index, pool, cache, and only then consider copies of the data itself. When connection arithmetic still overflows — hundreds of serverless or edge processes each wanting a pool — a connection pooler such as PgBouncer sits between the fleet and the database (comparison prose, per the family stance; this stack's fleet is small enough that the per-process pool is the honest size)."),
		),
		// ── 6. CDN and the edge ─────────────────────────────────
		section("cdn-edge", "CDN and the edge: the cheapest replica is the one you do not run",
			"Safe caching is a naming discipline — and this stack already ships it.",
			text("scl-cd-pitch",
				"Edge scaling works only on content that can be cached SAFELY, and safe caching is a property of how you NAME things. This stack ships the whole pattern for real: Vite emits content-hashed asset filenames, and the client's nginx answers them with a one-year immutable cache header. The header is safe because the hash IS the cache key: a new build produces a new filename, so a cache entry for an old name can never be wrong — the bytes it holds are the only bytes that filename ever named. Content addressing turns caching from a staleness risk into a correctness guarantee, which is why the caching topic called it the one unconditionally safe cache. A CDN in front of this origin replicates the same guarantee into points of presence around the world: the same immutable header, cached at the edge, users served from the nearby POP instead of the origin:"),
			code("scl-cd-assets-quote", "nginx",
				"Immutable hashed assets: cache forever, because the filename is the fingerprint.",
				".docker/client/nginx.conf",
				"    # Hashed static assets (Vite emits content-hashed filenames under\n"+
					"    # /assets/) — the hash IS the cache key, so cache forever.\n"+
					"    location /assets/ {\n"+
					"        expires 1y;\n"+
					"        add_header Cache-Control \"public, immutable\";\n"+
					"    }"),
			text("scl-cd-index-pitch",
				"The one file that must NOT be cached that way is the one that names all the others: index.html references the hashed bundles by name, so its freshness gates every deploy. Revalidate it always (no-cache means revalidate, not never-store), and a user's next navigation fetches the new HTML, which references the new hashes, which miss the cache exactly once and then cache for a year — deploy visibility in one file, permanence in everything else:"),
			code("scl-cd-index-quote", "nginx",
				"index.html revalidates every time: the one non-hashed file, so a deploy is visible the moment it flips.",
				".docker/client/nginx.conf",
				"    # index.html must always be revalidated so deploys are picked up at once.\n"+
					"    location = /index.html {\n"+
					"        add_header Cache-Control \"no-cache, no-store, must-revalidate\";\n"+
					"        add_header Pragma \"no-cache\";\n"+
					"        expires 0;\n"+
					"    }"),
			text("scl-cd-origin",
				"WHAT STAYS AT THE ORIGIN, deliberately, is everything dynamic: the nginx /api/ location proxies straight through to the Go server, because API answers carry per-request truth — a scenario flag, a request id, a live measured count — that no cache may serve for you. That is the caching topic's what-not-to-cache rule enforced by configuration, and it is also the division of labor this whole topic keeps drawing: the edge scales bytes that never change, replicas scale the stateless work that changes per request, the data tier scales the state that changes for everyone. The gzip block in the same file carries the compression half of the edge story — one gzip on text assets at the proxy — and topic 38 measured what it saves on real payloads; an edge network multiplies that saving by every geographical hop it removes. Nothing about the edge requires a commercial CDN to teach or to run: the browser cache, the nginx cache headers, and the hashed filenames ARE the edge pattern — a CDN is the same pattern with points of presence rented by the hour (Cloudflare and friends live in comparison prose, per the family stance)."),
		),
		// ── 7. Backpressure ─────────────────────────────────────
		section("backpressure", "Backpressure and shedding: refusing on purpose",
			"Scaling's last resort is saying no early — this stack already says it in three places.",
			text("scl-bp-pitch",
				"Scaling's last resort is refusing work, and it deserves first-class design rather than accident. Backpressure is the system telling its producers to slow down — a bounded queue that fills, a connection count that caps, a 429 that says when to come back — instead of absorbing load until memory or latency fails somewhere you did not choose. Topic 39 framed the same mechanisms as reliability (bulkheads against overload); the scaling frame adds the part that matters here: backpressure is how a system at capacity STAYS at capacity instead of past it, degrading the edges of its workload rather than the whole. This repo enforces it in three real places — the HTTP rate limiter, the realtime hub's per-connection ceilings, and the server's own bounded connections — and the first is the clearest:"),
			code("scl-bp-limiter-quote", "go",
				"The per-client token bucket: one map, one mutex, one limiter per client — the shared in-process budget a fleet would externalize.",
				"server/internal/domain/security/ratelimit.go",
				"type ClientRateLimiter struct {\n"+
					"	policy  RatePolicy\n"+
					"	mu      sync.Mutex\n"+
					"	buckets map[string]*rate.Limiter\n"+
					"}\n"+
					"// …\n"+
					"func (l *ClientRateLimiter) bucket(key string) *rate.Limiter {\n"+
					"	l.mu.Lock()\n"+
					"	defer l.mu.Unlock()\n"+
					"	bucket := l.buckets[key]\n"+
					"	if bucket == nil {\n"+
					"		bucket = rate.NewLimiter(l.policy.Rate, l.policy.Burst)\n"+
					"		l.buckets[key] = bucket\n"+
					"	}\n"+
					"	return bucket\n"+
					"}"),
			text("scl-bp-limiter-reading",
				"Read that structure as a scaling document. The buckets map is IN-PROCESS state — behind a load balancer, each replica enforces its OWN budget per client, so N replicas mean N times the effective limit (the statelessness inventory names this; the fix is the one the whole topic teaches: move the buckets to the shared store, paying a hop per request, or set the limit knowing the multiplication). The refusal itself is honest backpressure: the security topic's 429 envelope carries a Retry-After derived from the bucket's refill arithmetic, so the throttled client knows when to come back instead of guessing. That is the essential trade of every shedding mechanism: someone feels the pressure — the throttled client, the retired slow consumer, the refused connection — and the decision of WHO feels it was made in advance, deliberately, instead of by whichever resource happens to exhaust first."),
			code("scl-bp-hub-quote", "go",
				"The realtime ceilings: a bounded buffer and a drop budget per subscriber — named constants, enforced, not advisory.",
				"server/internal/domain/realtime/hub.go",
				"type HubPolicy struct {\n"+
					"	Buffer   int\n"+
					"	MaxDrops int\n"+
					"}\n"+
					"\n"+
					"// DefaultHubPolicy is the buffer/drop pair the live demos run.\n"+
					"func DefaultHubPolicy() HubPolicy {\n"+
					"	return HubPolicy{Buffer: 16, MaxDrops: 8}\n"+
					"}\n"+
					"// …\n"+
					"func NewHub(policy HubPolicy) (*Hub, error) {\n"+
					"	if policy.Buffer <= 0 || policy.MaxDrops <= 0 {\n"+
					"		return nil, fmt.Errorf(\"hub policy requires Buffer > 0 and MaxDrops > 0, got %+v\", policy)\n"+
					"	}\n"+
					"	return &Hub{\n"+
					"		policy:  policy,\n"+
					"		rooms:   make(map[string]map[*Subscriber]struct{}),\n"+
					"		nextSeq: make(map[string]int64),\n"+
					"	}, nil\n"+
					"}"),
			text("scl-bp-hub-reading",
				"The hub is the same doctrine one layer down: a named buffer per subscriber (a ceiling nobody can quote is a ceiling nobody sized), a drop budget after which the slow consumer is retired with the policy close code instead of wedging the room — topic 28 taught the mechanics, and the file topic (26) taught the streaming version: uploads are consumed as streams precisely so a slow disk applies backpressure to a fast network instead of buffering the difference in memory. The third enforcement is the server itself: the bounded read/write/idle timeouts topic 39 quoted are shedding at the connection level — a slowloris client is disconnected by policy, not tolerated until memory dies. Backpressure composes with every other section: the balancer health-checks away a replica that has stopped draining, the pool queues instead of dialing unboundedly, the edge absorbs the traffic that never needed the origin at all. The one thing it never is, in a well-run system, is a surprise."),
		),
		// ── 8. Capacity ─────────────────────────────────────────
		section("capacity", "Capacity: how much is enough, measured",
			"Little's law and the two metrics this stack already serves — sizing the fleet from arithmetic, not vibes.",
			text("scl-cap-pitch",
				"THE CAPACITY QUESTION is the second organizing question and it has a real answer: Little's law, the one equation every scaling conversation runs on. Concurrency equals arrival rate times average latency — the number of requests in flight equals how fast they arrive multiplied by how long each takes. It is a queueing identity, true for any system in any state, and it converts the sizing conversation from vibes to arithmetic: a fleet's throughput ceiling is its replica count times per-replica concurrency (for this server, roughly the goroutines the runtime will schedule well — topic 02 taught GOMAXPROCS; in containers, the CPU limit), and latency is what topic 38 measured per endpoint. When arrival rate approaches the ceiling, queueing theory's least pleasant fact appears: utilization above roughly eighty percent makes latency grow nonlinearly, so a fleet sized at one hundred percent of peak is a fleet that will fall over at peak. Size for headroom, or shed to defend it — those are the options."),
			text("scl-cap-metrics-pitch",
				"The two numbers the law feeds on are already being served by this stack: topic 33's metrics registry declares a request counter and a duration histogram at the composition root — arrivals and latency, the law's two inputs, per route. Everything a capacity decision needs is derivable from those two families over a window: arrival rate from the counter's slope, average and tail latency from the histogram, concurrency from their product, saturation when queues grow (visible as latency rising while throughput flattens):"),
			code("scl-cap-metrics-quote", "go",
				"The law's two inputs, served per route: arrivals (counter) and latency (histogram).",
				"server/internal/presentation/metrics.go",
				"func RegisterHTTPMetrics(registry *obs.MetricsRegistry) (requests *obs.Counter, durations *obs.Histogram) {\n"+
					"	requests = registry.MustCounter(requestsMetricName, requestsMetricHelp,\n"+
					"		[]string{\"method\", \"path\", \"status\"})\n"+
					"	durations = registry.MustHistogramSeconds(durationsMetricName, durationsMetricHelp,\n"+
					"		[]string{\"method\", \"path\"}, obs.DefaultDurationBuckets())\n"+
					"	return requests, durations\n"+
					"}"),
			text("scl-cap-levers-pitch",
				"Capacity work then becomes choosing which lever the arithmetic points at, in the order the whole topic established:"),
			list("scl-cap-levers",
				true,
				"Measure first — arrival rate and latency per route from the metrics pair; a sizing decision without them is a guess wearing a spreadsheet.",
				"Trim the work — the performance topic's levers (allocations, compression, pooling) raise the ceiling without adding a single copy.",
				"Add replicas — the stateless tier's cheap answer, behind the balancer, once the single copy is efficient.",
				"Defend the ceiling — backpressure and shedding keep the system AT capacity instead of past it when the peak exceeds the plan.",
				"Scale the data tier last — indexes, pools, caches first; read replicas and sharding only when the arithmetic says the single writer is the wall.",
			),
			text("scl-cap-autoscale",
				"AUTOSCALING is the same arithmetic executed by a control loop instead of a human: the k8s manifest this repo ships fixes replicas at two, but a production fleet points a HorizontalPodAutoscaler at exactly the metrics above (CPU utilization, or a custom metric like queue depth) and lets the replica count track load. The jobs topic (29) already taught the queue-depth version of this: workers scale by DEPTH, not by request latency, because their capacity question is how long the backlog is — the two observability numbers the jobs substrate serves. The autoscaler's honesty condition is the same as the manual one: it can only add copies of a STATELESS tier. An autoscaler pointed at replica-local state manufactures exactly the bug this topic's demo measured — sessions dying at every scale-out event a Thursday afternoon traffic bump triggers — which is why statelessness is not the first section of this topic by accident. It is the precondition for everything after it: balancing, replicas, autoscaling, and the capacity arithmetic that decides how many of them to run."),
		),
		// ── 9. Terminal companion ───────────────────────────────
		section("terminal-companion", "Terminal companion: the scaling tour",
			"The scaling surfaces over curl, against the running stack: the topic in the curriculum, the demo's measured round, and its determinism proven live.",
			text("scl-tc-pitch",
				"The tour walks the topic's real endpoints on the running stack. The demo's determinism is itself part of the teaching: two GETs replay the identical round, byte for byte, because the fleet reads no clock and no random source — the diff below should print nothing but the echo. Everything else on the panel — the distribution rows, the swap facts, the verdicts — is measured by the machinery that served the round; nothing on the wire was invented to fill a panel."),
			code("scl-tc-tour-1", "bash", "",
				"",
				"# The topic in the curriculum — order 40, the Ship group's last word:\n"+
					"curl -s localhost:8100/api/content | grep -o '\"id\":\"scaling\",\"title\":\"[^\"]*\"'\n"+
					"\n"+
					"# The full shape: nine sections, the demo inline in statelessness:\n"+
					"curl -s localhost:8100/api/content/scaling | grep -o '\"id\":\"[a-z-]*\"' | head -n 12"),
			code("scl-tc-tour-2", "bash", "",
				"",
				"# The statelessness round: measured distribution, 13 steps, counter, verdicts:\n"+
					"curl -s localhost:8100/api/demo/scaling/stateless | grep -o '\"totalServed\":[0-9]*'\n"+
					"curl -s localhost:8100/api/demo/scaling/stateless | grep -o '\"replicaId\":\"[^\"]*\",\"served\":[0-9]*'\n"+
					"\n"+
					"# Determinism, proven against the live endpoint — expect the echo, nothing else:\n"+
					"diff <(curl -s localhost:8100/api/demo/scaling/stateless) \\\n"+
					"     <(curl -s localhost:8100/api/demo/scaling/stateless) && echo deterministic\n"+
					"\n"+
					"# The verdicts the round derived, and the dying replicas' measured counters:\n"+
					"curl -s localhost:8100/api/demo/scaling/stateless | grep -c '\"phase\":\"swap\"'\n"+
					"curl -s localhost:8100/api/demo/scaling/stateless | grep -o 'terminated: [0-9]* request(s) served'\n"+
					"curl -s localhost:8100/api/demo/scaling/stateless | grep -o '\"label\":\"[^\"]*\"'"),
			code("scl-tc-tour-3", "bash", "",
				"",
				"# The balancer's inputs elsewhere on the stack: readiness gates endpoints,\n"+
					"# metrics carry the capacity pair, and unknown topics stay honest:\n"+
					"curl -s localhost:8100/api/ready\n"+
					"curl -s localhost:8100/metrics | grep -c 'guide_http_request'\n"+
					"curl -s -i localhost:8100/api/content/scaling-nope | head -n 6"),
		),
	}

	return Topic{
		ID:       "scaling",
		Title:    "Scaling",
		Subtitle: "Serving more load by adding copies, not re-architecting: what scaling means along the request path, statelessness as a placement question (walked live by a real round-robin fleet — a session that dies in replica-local memory at the first hop and the first rollout, and the same session surviving everything once it lives in the shared store), load balancing from the Kubernetes Service to Docker's DNS, replicas as the Deployment's rolling-update contract, the data tier's honest order (indexes, pooling, then the levers this repo deliberately does not run), the edge this stack already ships (immutable hashed assets, revalidating index.html), backpressure from the token bucket to the realtime ceilings, and capacity as Little's law fed by the metrics pair this server already serves.",
		Group:    "ship",
		Order:    40,
		Sections: sections,
	}, buildErr
}
