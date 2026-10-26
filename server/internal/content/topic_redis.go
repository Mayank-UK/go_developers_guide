package content

import (
	"errors"
	"fmt"
)

// This file is topic 20 redis (group data, curriculum order 20), the
// data group's last store: what Redis actually is (a data structures
// server, not just a cache), the optional-service discipline that keeps
// a down Redis a typed 503 instead of a crash, the five core structures
// walked live through go-redis v9 with one honest beat each, TTL/expiry
// semantics read as data (SET EX, TTL, SET NX), INCR atomicity proven
// with a pipelined batch, one REAL pub/sub round-trip, and the judgment
// of when a second store earns its keep beside PostgreSQL. The terminal
// companion closes with redis-cli through docker exec and the curl tour.

// redisTopic builds topic 20.
func redisTopic() (Topic, error) {
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
		// ── 1. What Redis is ────────────────────────────────────────
		section("what-redis-is", "What Redis actually is",
			"A data structures server — the store itself owns the sorting, the uniqueness, the counting, and the ordering your code would otherwise have to coordinate.",
			text("redis-pitch",
				"Redis — REmote DIctionary Server — is best understood by refusing its "+
					"most famous job first: it is not \"a cache\" that happens to have some "+
					"commands. It is a SERVER over data structures. You send a command "+
					"named after the operation you want (``LPUSH``, ``HSET``, ``ZADD``, "+
					"``INCR``), the store executes it against the structure the key holds, "+
					"and the answer comes back. The comparison that makes it click is with "+
					"PostgreSQL from topic 17: there you send a STATEMENT in a language and "+
					"the planner decides how to walk rows; here you send an OPERATION and "+
					"the structure IS the index. Because the structures live in memory "+
					"(with optional persistence behind them), each operation costs "+
					"microseconds, and because a single thread executes commands one at a "+
					"time, each command is ATOMIC without any lock in your code — a "+
					"property this topic leans on twice, for counters and for SET NX."),
			text("redis-structures-not-tables",
				"The mental shift from SQL is that data has SHAPE before it has schema. "+
					"A key in Redis holds exactly one structure, and the structure decides "+
					"what the store can do for you: a string is one binary-safe value up to "+
					"512 MB; a hash is field→value pairs under one key (the object-shaped "+
					"value, addressed per field); a list is an ordered sequence you push to "+
					"and pop from at EITHER end; a set is unique membership with set "+
					"algebra; a sorted set carries a score per member and stays ordered BY "+
					"that score. Choosing the structure IS the schema design — a "+
					"leaderboard is a sorted set in the same direct way a many-to-many "+
					"edge is a join table, and once you see that mapping, the command "+
					"names read as what they do. There is no query planner to negotiate "+
					"with, no indexes to reason about per statement; there is only \"which "+
					"structure holds this, and which operation answers this question?\""),
			text("redis-in-this-stack",
				"This guide runs Redis as the compose service ``redis:7-alpine`` — "+
					"container ``go_developers_guide_redis_dev``, reachable inside the "+
					"stack network as ``redis:6379`` (the host's published slot is 6380), "+
					"no password in dev as the compose file states plainly. It shares the "+
					"optional-service deal with MongoDB from topic 19: the stack is "+
					"USEFUL with it down, so its absence must degrade, never crash — the "+
					"next section makes that a discipline rather than a hope. The Go side "+
					"is ``go-redis/v9`` (``github.com/redis/go-redis/v9``, pinned at "+
					"v9.23.0): the de-facto standard client, context-first on every "+
					"command, one client per process with a connection pool inside it — "+
					"the same one-pool-per-process shape topic 17 established for pgx."),
			list("redis-structure-map", false,
				"string — one binary-safe value up to 512 MB; caches, counters, whole serialized payloads; the workhorse everything else composes from.",
				"hash — field→value pairs under one key; the object-shaped value, readable per field (HGET) or whole (HGETALL).",
				"list — an ordered sequence addressed from both ends (LPUSH/RPUSH/LPOP/RPOPLPUSH); feeds, timelines, cheap queues.",
				"set — unordered unique membership with set algebra (SINTER, SUNION, SDIFF); tags, presence, deduplication.",
				"sorted set — members carrying float scores, kept ordered by them; leaderboards, ranking, rate-limit windows, time-series-ish access.",
				"extras beyond this topic — bitmaps, HyperLogLog, geospatial indexes, streams (topic 22 uses Redis Streams for messaging)."),
		),
		// ── 2. Optional-service discipline ──────────────────────────
		section("optional-service-discipline", "The optional-service discipline",
			"Redis down is a typed 503, never a crash: the client is lazy, the timeouts are short, and the failure rides the same error taxonomy as every other failure.",
			text("optional-pitch",
				"Compose states the contract on the redis service itself: \"the app "+
					"degrades gracefully if it is down, so demos report off rather than "+
					"failing the stack.\" PostgreSQL in this guide is a boot dependency — "+
					"topic 17 fails fast at startup when it is unreachable, because a "+
					"guide without its database has nothing to serve. Redis is the other "+
					"kind of dependency: every job it does here (caching, hot counters, "+
					"pub/sub for demos) has a slower or quieter fallback, so the honest "+
					"behavior when it is unreachable is a typed ``service_unavailable`` "+
					"on exactly the surfaces that need it, and normal service everywhere "+
					"else. The discipline has three parts, and all three live in the "+
					"adapter, not scattered through callers:"),
			list("optional-laws", false,
				"Construct lazily — the client dials nothing at build time; the app boots with Redis down and the first command pays the connect.",
				"Fail fast — dial/read/write timeouts of two seconds turn a dead address into a quick error, not a hanging worker.",
				"Map, never panic — the adapter returns the error wrapped with %w; the handler maps it to the 503 envelope through the one central writer.",
			),
			code("client-factory-go", "go",
				"The process's one client: lazy construction, short timeouts — the whole optional-service posture in one function.",
				"server/internal/data/redis/playground.go",
				"func NewClient(addr string) *goredis.Client {\n"+
					"    return goredis.NewClient(&goredis.Options{\n"+
					"        Addr:         addr,\n"+
					"        DialTimeout:  dialTimeout,\n"+
					"        ReadTimeout:  readTimeout,\n"+
					"        WriteTimeout: writeTimeout,\n"+
					"    })\n"+
					"}"),
			text("optional-mapping",
				"The handler's half of the deal is one mapping: any failure from the "+
					"runner port becomes the typed 503 with a plain-words message, "+
					"through the same ``core.WriteError`` every other failure in the "+
					"guide rides. Nothing is swallowed (the cause is wrapped with "+
					"``%w`` and logged with the request id), nothing becomes a panic, "+
					"and — just as important — nothing serves a STALE answer to avoid "+
					"the question: this endpoint's whole value is that its rows are "+
					"what the live store just said, so \"store unreachable\" is the "+
					"truthful response, not an excuse to replay an old payload. The "+
					"client degrades the same way in the browser: the error panel "+
					"shows the envelope's code and message with a retry, because the "+
					"503 arrives through the identical error pipeline as a 404."),
			code("handler-degrade-go", "go",
				"The whole degradation: run, map any failure to service_unavailable, write the payload only on success.",
				"server/internal/presentation/demo_redis.go",
				"run, err := h.runner.Run(r.Context(), namespace)\n"+
					"if err != nil {\n"+
					"    core.WriteError(w, r, core.Wrapf(err, core.KindServiceUnavailable,\n"+
					"        \"The redis playground runs real commands against the guide's Redis, which is not reachable right now.\"), h.logger)\n"+
					"    return\n"+
					"}\n"+
					"writeJSON(w, http.StatusOK, demos.BuildRedisPlaygroundPayload(run))"),
		),
		// ── 3. The five structures, walked live ─────────────────────
		section("go-redis-structures", "go-redis v9: the five structures, walked live",
			"One honest beat per structure against the running Redis — real commands, real read-backs, and the sorting and uniqueness happening inside the store.",
			text("structures-pitch",
				"The demo at the end of this section runs every beat you are about to "+
					"read against the RUNNING stack Redis — not fixtures, not a fake. "+
					"Each request mints a throwaway namespace (``gdg:demo:redis:`` plus "+
					"twelve random hex characters), so concurrent requests never share "+
					"keys and the walk's arithmetic needs no coordination; every key "+
					"the run creates is removed at the end. The namespace mint is "+
					"itself worth a look, because namespacing keys by colon-separated "+
					"convention (``area:entity:field``) is how Redis projects do "+
					"\"tables\" — the store has no key hierarchies, only names, and a "+
					"prefix you chose is the closest thing to a schema:"),
			code("namespace-mint-go", "go",
				"The demo's throwaway namespace: random per request, so runs are independent.",
				"server/internal/domain/demos/redis_playground.go",
				"func NewRedisDemoNamespace() (string, error) {\n"+
					"    var entropy [6]byte\n"+
					"    if _, err := rand.Read(entropy[:]); err != nil {\n"+
					"        return \"\", fmt.Errorf(\"read demo namespace entropy: %w\", err)\n"+
					"    }\n"+
					"    return RedisDemoKeyPrefix + \":\" + hex.EncodeToString(entropy[:]), nil\n"+
					"}"),
			text("string-hash-pitch",
				"The string beat is deliberately two commands: ``SET`` carrying its "+
					"expiration INLINE (the ``EX`` option — value and deadline are one "+
					"atomic command, not a SET followed by a racy EXPIRE), and ``GET`` "+
					"reading the value straight back. In go-redis that is "+
					"``client.Set(ctx, key, value, ttl)`` returning a status command "+
					"whose ``.Err()`` you check, and ``client.Get(ctx, key).Result()`` "+
					"returning ``(string, error)`` — the context is the FIRST argument "+
					"on every command, so a canceled request frees its connection "+
					"immediately (topic 6's law, applied to a network store). The hash "+
					"beat then writes a whole small object in one ``HSET`` (go-redis "+
					"accepts a map directly) and reads it back whole with ``HGETALL`` — "+
					"which returns a Go ``map[string]string``, an unordered type, so "+
					"the adapter sorts the fields before displaying them:"),
			code("string-hash-beat-go", "go",
				"The first two beats: SET with its inline EX + GET, then HSET of a whole map and HGETALL of the whole object.",
				"server/internal/data/redis/playground.go",
				"if err := r.client.Set(ctx, key, demos.RedisDemoGreeting,\n"+
					"    time.Duration(demos.RedisDemoTTLSeconds)*time.Second).Err(); err != nil {\n"+
					"    return demos.RedisStructureRun{}, fmt.Errorf(\"SET %s: %w\", key, err)\n"+
					"}\n"+
					"value, err := r.client.Get(ctx, key).Result()\n"+
					"// …the row carries the two commands and the value GET answered\n"+
					"\n"+
					"added, err := r.client.HSet(ctx, key, demos.RedisDemoHashFields).Result()\n"+
					"// …HSET accepts the map directly and answers how many fields were NEW\n"+
					"whole, err := r.client.HGetAll(ctx, key).Result()\n"+
					"// …a map[string]string: fields sorted before display, order is not a promise"),
			text("collection-pitch",
				"The three collection structures earn the demo's best tricks. The LIST "+
					"beat pushes ``first``, ``second``, ``third`` with one ``LPUSH`` and "+
					"reads ``third``, ``second``, ``first`` back with ``LRANGE 0 -1`` — "+
					"the push-order inversion is not a bug to work around, it is the "+
					"structure's design: LPUSH lands at the head, so a timeline is "+
					"\"newest first\" for free. The SET beat pushes FOUR arguments of "+
					"which one is a duplicate and ``SADD`` answers 3 — uniqueness is "+
					"enforced by the structure, so deduplicating tags is not application "+
					"code. The SORTED SET beat writes members in descending score order "+
					"on purpose (100, 85, 97) and ``ZRANGE … WITHSCORES`` hands them back "+
					"ascending (85, 97, 100) — the ordering lives in the structure, "+
					"updated on every ZADD, which is why a leaderboard is a sorted set "+
					"and not a ``SELECT … ORDER BY score DESC`` you re-run on every read:"),
			code("collection-beats-go", "go",
				"Three collection beats: LPUSH/LRANGE inversion, SADD refusing the duplicate, ZAdd/ZRangeWithScores re-sorting.",
				"server/internal/data/redis/playground.go",
				"length, err := r.client.LPush(ctx, key, demos.RedisDemoListPushes).Result()\n"+
					"elements, err := r.client.LRange(ctx, key, 0, -1).Result()\n"+
					"// …elements = third, second, first — the push order, inverted\n"+
					"\n"+
					"added, err := r.client.SAdd(ctx, key, demos.RedisDemoSetMembers).Result()\n"+
					"members, err := r.client.SMembers(ctx, key).Result()\n"+
					"sort.Strings(members)\n"+
					"// …SADD answered 3 of 4: the duplicate never became a member\n"+
					"\n"+
					"added, err := r.client.ZAdd(ctx, key, scored...).Result()\n"+
					"ascending, err := r.client.ZRangeWithScores(ctx, key, 0, -1).Result()\n"+
					"// …written 100, 85, 97; returned 85, 97, 100 — the sort is the structure's"),
			demo("redis:playground", "The playground: five structures, real commands",
				"One request against the guide's RUNNING Redis: the five structures each "+
					"with their real commands and read-backs (SET with inline EX, HSET/"+
					"HGETALL, the list's push-order inversion, SADD refusing a duplicate, "+
					"ZRANGE WITHSCORES re-sorting), a pipelined batch of 25 INCRs proving "+
					"atomicity with an exact total, the TTL countdown read as data with "+
					"the SET NX lock pair, one REAL pub/sub round-trip reported from both "+
					"sides, and the cleanup that removes every key the walk created."),
			text("structures-read",
				"Read the demo's rows as contracts, not output. Every structure's "+
					"``outcome`` is derived from what the store's replies actually added "+
					"up to — the count SADD answered, the length LPUSH answered — and "+
					"the payload builder in the domain derives every proof sentence from "+
					"the run's own numbers, so a claim can never drift from the store. "+
					"And the run ends the way every demo in this guide ends: it cleans "+
					"up after itself. The cleanup beat scans the namespace with ``SCAN`` "+
					"(batched, non-blocking — ``KEYS`` would freeze the store's single "+
					"command thread on a big keyspace, the one scalability sin this "+
					"guide refuses to teach by accident) and deletes the whole batch in "+
					"one ``DEL``; the payload reports exactly how many keys left, and "+
					"the adapter's own tests verify the keyspace is provably empty "+
					"afterwards:"),
			code("cleanup-beat-go", "go",
				"The no-residue beat: SCAN under the namespace pattern, then one DEL for the whole batch.",
				"server/internal/data/redis/playground.go",
				"match := namespace + \":*\"\n"+
					"var keys []string\n"+
					"iterator := r.client.Scan(ctx, 0, match, 100).Iterator()\n"+
					"for iterator.Next(ctx) {\n"+
					"    keys = append(keys, iterator.Val())\n"+
					"}\n"+
					"// …one DEL removes the whole batch; the channel is not a key"),
		),
		// ── 4. TTL and expiry ───────────────────────────────────────
		section("ttl-and-expiry", "TTL and expiry: SET EX, TTL, NX",
			"Expiration is part of the write, the countdown belongs to the store, and SET NX is the lock primitive — read as data, never slept out.",
			text("ttl-pitch",
				"Everything in Redis is ephemeral by design: keys live until they are "+
					"deleted OR until their deadline passes, and the deadline is first "+
					"class. The idiomatic form is the one the demo uses — ``SET key "+
					"value EX 30`` — because it makes the value and its expiry ONE "+
					"atomic command; the alternative (``SET`` then ``EXPIRE``) is two "+
					"commands with a window between them where a crash leaves the key "+
					"immortal, and on a cache that window is a slow leak. Once the "+
					"deadline is set, the store owns the countdown: ``TTL key`` reports "+
					"the seconds remaining (``-1`` for no expiry, ``-2`` for no key — "+
					"two answers worth distinguishing), ``EXPIRE``/``PERSIST`` move the "+
					"deadline later or remove it, and eviction happens lazily when an "+
					"expired key is touched plus a background sweep — which is why a "+
					"demo that WANTED to prove expiry by sleeping would be proving "+
					"patience instead."),
			text("ttl-as-data",
				"This guide's TTL beat therefore reads the countdown as DATA: SET with "+
					"``EX 30``, then ``TTL`` immediately after, and the payload reports "+
					"both numbers side by side — the configured deadline and the live "+
					"countdown the store answered. A test pins the honest interval (the "+
					"read must be positive and at most the configured value) rather "+
					"than pretending wall-clock precision the store never promised. "+
					"The same beat carries the other half of SET's option grammar: "+
					"``NX`` — set ONLY if the key does not exist. Two commands tell the "+
					"whole story: the first ``SET lock \"owner-1\" NX`` answers OK "+
					"(acquired), the second ``SET lock \"owner-2\" NX`` answers nil "+
					"(refused) — and that pair is the lock primitive, the seed of every "+
					"distributed lock, before the hard parts (leases, fencing tokens, "+
					"the Redlock debate) that a real lock adds on top:"),
			code("ttl-beat-go", "go",
				"The expiry beat: SET carries its EX inline, TTL reports the countdown, and the NX pair acquires then refuses.",
				"server/internal/data/redis/playground.go",
				"flash := \"here for 30 seconds — the store's clock evicts me\"\n"+
					"if err := r.client.Set(ctx, flashKey, flash,\n"+
					"    time.Duration(demos.RedisDemoTTLSeconds)*time.Second).Err(); err != nil {\n"+
					"    return demos.RedisTTLRun{}, fmt.Errorf(\"SET %s: %w\", flashKey, err)\n"+
					"}\n"+
					"firstRead, err := r.client.TTL(ctx, flashKey).Result()\n"+
					"// …-1 would mean no expiry, -2 no key — the beat refuses to lie about either\n"+
					"\n"+
					"nxFirst, err := r.client.SetNX(ctx, lockKey, \"owner-1\", 0).Result()\n"+
					"nxSecond, err := r.client.SetNX(ctx, lockKey, \"owner-2\", 0).Result()\n"+
					"// …OK then nil: acquired, then refused — one holder"),
		),
		// ── 5. Atomic operations ────────────────────────────────────
		section("atomic-operations", "Atomic operations: INCR and pipelines",
			"The single command thread makes INCR atomic for free — and pipelines turn N round trips into one without pretending to be a transaction.",
			text("atomic-pitch",
				"\"Increment this value and tell me the new total\" is the classic "+
					"read-modify-write race — in PostgreSQL you would wrap it in a "+
					"transaction or use ``UPDATE … SET n = n + 1 RETURNING n``; in "+
					"application memory it is exactly the race topic 6's race detector "+
					"exists to catch. In Redis it is one command, ``INCR``, and it is "+
					"atomic BECAUSE the store executes commands one at a time on a "+
					"single thread: there is no interleaving point, no lock to forget, "+
					"no compare-and-swap loop. Two concurrent INCRs on the same key "+
					"cannot lose an increment by construction. The demo's counter beat "+
					"leans on this deliberately: it deletes the key for a fresh start, "+
					"fires twenty-five INCRs, and requires the final GET to answer "+
					"exactly 25 — the total is the store's arithmetic, reported, not "+
					"the code's expectation, asserted."),
			code("counter-beat-go", "go",
				"The atomicity beat: DEL for a fresh key, 25 INCRs queued on one pipeline, one Exec, an exact total.",
				"server/internal/data/redis/playground.go",
				"if err := r.client.Del(ctx, key).Err(); err != nil {\n"+
					"    return demos.RedisCounterRun{}, fmt.Errorf(\"DEL %s: %w\", key, err)\n"+
					"}\n"+
					"pipe := r.client.Pipeline()\n"+
					"for range demos.RedisDemoCounterPipeline {\n"+
					"    pipe.Incr(ctx, key)\n"+
					"}\n"+
					"if _, err := pipe.Exec(ctx); err != nil {\n"+
					"    return demos.RedisCounterRun{}, fmt.Errorf(\"pipeline %d INCRs on %s: %w\", demos.RedisDemoCounterPipeline, key, err)\n"+
					"}\n"+
					"total, err := r.client.Get(ctx, key).Int64()"),
			text("pipeline-vs-multi",
				"The pipeline deserves its own paragraph because it is the most "+
					"misunderstood word in Redis. A PIPELINE is a network optimization "+
					"and NOTHING more: the client queues commands and ships them in one "+
					"round trip, saving per-command latency — twenty-five INCRs over a "+
					"network with one millisecond of round-trip cost go from "+
					"twenty-five milliseconds to roughly one. The commands still "+
					"execute one at a time, and OTHER clients' commands may interleave "+
					"between them. When you need \"all of these as one unit\", that is "+
					"``MULTI``/``EXEC`` — a transaction that queues server-side and "+
					"applies atomically — and when you need \"fail if anybody changed "+
					"what I read\", that is ``WATCH``: optimistic concurrency that aborts "+
					"the transaction if a watched key moved. The honest hierarchy: "+
					"INCR for a single atomic operation, pipeline for batch latency, "+
					"MULTI for all-or-nothing, WATCH for read-decide-write. The demo "+
					"needs only the first two — each INCR is atomic alone, and the total "+
					"is exact no matter how the pipeline interleaves — and the beat's "+
					"proof sentence derives from the GET's answer, so if the arithmetic "+
					"ever disagreed, the payload would say so."),
		),
		// ── 6. Pub/sub ──────────────────────────────────────────────
		section("pubsub", "Pub/sub: one real round-trip",
			"SUBSCRIBE dedicates a connection; PUBLISH fans out to whoever is listening — and nobody else, with no persistence and no replay.",
			text("pubsub-pitch",
				"Redis pub/sub is the textbook model with the sharp edges left visible: "+
					"``PUBLISH channel payload`` delivers to every current subscriber of "+
					"the channel and answers with how many saw it; ``SUBSCRIBE`` puts a "+
					"connection into subscriber mode, after which it only receives. Two "+
					"properties shape every use. Delivery is FIRE-AND-FORGET: no "+
					"persistence, no replay — a subscriber that was not there, or a "+
					"message published before the SUBSCRIBE landed, is simply gone. "+
					"And the subscriber's connection is DEDICATED: it cannot carry "+
					"regular commands once subscribed, which is why go-redis's "+
					"``Subscribe`` returns a separate ``*PubSub`` object with its own "+
					"connection and its own ``Receive`` loop. When your needs outgrow "+
					"fire-and-forget — consumer groups, pending lists, replay — the "+
					"answer inside the same store is Streams, which topic 22 builds on; "+
					"pub/sub is for presence, invalidation hints, and demo wiring."),
			text("pubsub-choreography",
				"The demo's round-trip is choreographed rather than hoped for, and the "+
					"order is the lesson: subscribe first, then WAIT for the store's "+
					"subscription confirmation (``Receive`` returns it as a "+
					"``*Subscription`` with kind ``subscribe``), and only then publish — "+
					"a publish that races ahead of the subscribe reaches nobody, and "+
					"the demo would report zero receivers while claiming a round-trip. "+
					"The wait for the delivered message is bounded by a context "+
					"deadline, and the goroutine that receives is released by the SAME "+
					"cancel whether the message arrived, the deadline hit, or the "+
					"publish failed — the leak discipline topic 6 taught, applied to a "+
					"network wait. On success the payload reports both sides: what "+
					"``PUBLISH`` answered (one receiver) and the exact string the "+
					"subscriber collected, matched in the open where a mismatch cannot "+
					"hide:"),
			code("pubsub-beat-go", "go",
				"The round-trip's core: confirmation before publish, bounded wait, goroutine always released.",
				"server/internal/data/redis/playground.go",
				"subscriber := r.client.Subscribe(ctx, channel)\n"+
					"defer subscriber.Close()\n"+
					"\n"+
					"ack, err := subscriber.Receive(ctx)\n"+
					"// …the *Subscription confirmation arrives BEFORE any publish may count\n"+
					"\n"+
					"msgCtx, cancel := context.WithTimeout(ctx, pubsubWait)\n"+
					"defer cancel()\n"+
					"// …a goroutine collects ReceiveMessage into a channel; cancel releases it on every path\n"+
					"\n"+
					"receivers, err := r.client.Publish(ctx, channel, payload).Result()\n"+
					"// …then a select waits for the goroutine OR msgCtx.Done() — never forever"),
		),
		// ── 7. When a second store ──────────────────────────────────
		section("when-a-second-store", "When a second store earns its keep",
			"PostgreSQL stays the source of truth; Redis earns its place only where memory-speed structure beats disk-speed queries — and the boundary is a decision, not an accident.",
			text("second-store-pitch",
				"This guide carries two stores, and the split is deliberate. PostgreSQL "+
					"from topics 16–17 owns TRUTH: the posts, the audit log, the "+
					"constraints that make wrong data unwritable, the transactions that "+
					"make multi-step writes all-or-nothing. Redis owns DERIVED state — "+
					"values that are computable from the truth but expensive to "+
					"recompute per request, or coordination artifacts that are not data "+
					"at all. The discipline that keeps the split honest is one "+
					"sentence: anything in Redis must be REBUILDABLE — a cache entry "+
					"from its source row, a counter from a recount, a session from a "+
					"re-login. When that sentence stops being true of a key, the key "+
					"has quietly become truth in the wrong store, and the system now "+
					"has two sources of truth agreeing by luck. Topic 21 makes this "+
					"concrete with cache-aside and invalidation; this topic's job is "+
					"the decision itself:"),
			list("second-store-jobs", false,
				"Hot-path caching — read-heavy rows served from memory with a TTL; the source of truth stays PostgreSQL, and topic 21 builds the whole pattern.",
				"Sessions and presence — small, per-request, naturally ephemeral state that TTL expires for you; no migration ever owns it.",
				"Atomic counters and rate limits — INCR with EXPIRE is the sliding-window rate limiter in two commands (topic 25 uses it).",
				"Leaderboards and ranking — sorted sets keep the order updated on write, where SQL re-sorts on every read.",
				"Coordination — SET NX locks, pub/sub invalidation hints, presence channels; coordination is not data and never needs durability.",
			),
			text("second-store-caution",
				"The honest counter-list is shorter but heavier. Redis does not earn "+
					"its keep as a system of record: persistence exists (RDB snapshots, "+
					"AOF journals) but the store's contract is memory-first — "+
					"durability is a tuned trade-off, not the paranoid default "+
					"PostgreSQL ships. It does not earn ad-hoc querying: there is no "+
					"planner, no join, no secondary index over arbitrary predicates — "+
					"you query by the structure's own operations, which is exactly why "+
					"structure choice is schema design. And it does not earn "+
					"\"we'll figure out consistency later\": a cache beside a database "+
					"is a distributed system the moment you write to both, and topic "+
					"21's invalidation patterns are the down payment on that fact. "+
					"Start with one store, measure the specific pain (a slow query, a "+
					"hot counter, a presence feature), and add Redis for THAT — the "+
					"second store that arrives with a reason is infrastructure; the one "+
					"that arrives with the stack template is a liability wearing a "+
					"caching label."),
		),
		// ── 8. Terminal companion ───────────────────────────────────
		section("terminal-companion", "Terminal companion: redis-cli and the curl tour",
			"The topic from the shell: PING, a manual SET/TTL, the playground by curl, a live MONITOR window, and a hand-rolled pub/sub round-trip.",
			text("term-pitch",
				"With the stack up (server on :8100, Redis in-network), this tour runs "+
					"the topic by hand. ``redis-cli`` ships inside the redis image, so "+
					"every command below is ``docker exec`` against the running "+
					"container — the same store the demo just walked. The MONITOR "+
					"window is the best teaching moment in the whole topic: run it in "+
					"one terminal, re-run the demo in another, and WATCH the commands "+
					"scroll by — SET, HSET, LPUSH, SADD, ZADD, the pipelined INCRs, "+
					"SUBSCRIBE and PUBLISH, then the SCAN and DEL of cleanup:"),
			code("term-ping-set", "bash", "", "",
				"# the handshake, then a manual SET/GET/TTL — note TTL counting down on re-read\n"+
					"docker exec go_developers_guide_redis_dev redis-cli PING\n"+
					"docker exec go_developers_guide_redis_dev redis-cli SET tour:flash \"watch me\" EX 30\n"+
					"docker exec go_developers_guide_redis_dev redis-cli TTL tour:flash\n"+
					"docker exec go_developers_guide_redis_dev redis-cli DEL tour:flash"),
			code("term-demo", "bash", "", "",
				"# the playground, straight from the shell — the whole walk in one payload\n"+
					"curl -s localhost:8100/api/demo/redis/playground | python3 -m json.tool | head -40\n"+
					"# …and the no-residue proof: the walk's namespace is gone\n"+
					"docker exec go_developers_guide_redis_dev redis-cli KEYS 'gdg:demo:redis:*'"),
			code("term-monitor", "bash", "", "",
				"# terminal one: watch the store's own stream of commands…\n"+
					"docker exec -it go_developers_guide_redis_dev redis-cli MONITOR\n"+
					"# terminal two: …while the demo walks again\n"+
					"curl -s localhost:8100/api/demo/redis/playground > /dev/null && echo walked"),
			code("term-pubsub", "bash", "", "",
				"# a hand-rolled round-trip: subscribe in one exec (it blocks, as subscribed connections do)…\n"+
					"docker exec go_developers_guide_redis_dev redis-cli SUBSCRIBE tour:events\n"+
					"# …then publish from another — the subscriber prints the message live\n"+
					"docker exec go_developers_guide_redis_dev redis-cli PUBLISH tour:events \"hello from the shell\""),
			text("term-coda",
				"Notice what the tour proves without asking you to trust it: the demo "+
					"payload is the same one the browser renders; the KEYS check comes "+
					"back EMPTY because the walk cleans up after itself; MONITOR shows "+
					"the store's own log of every command the adapter ran — the honest "+
					"witness. That closes the data group's tour of stores: PostgreSQL "+
					"for truth (16–17), code generation compared on top of it (18), "+
					"documents where shape beats relation (19), and structures where "+
					"memory-speed coordination beats both (20). The next group starts "+
					"the payoff: topic 21 puts this store to work as a CACHE with the "+
					"invalidation discipline that keeps it derived, not divine."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"redis",
		"Redis from Go",
		"What Redis actually is (a data structures server, not just a cache), the optional-service "+
			"discipline that keeps a down store a typed 503, the five core structures walked live "+
			"through go-redis v9, TTL and SET NX semantics read as data, INCR atomicity proven with "+
			"a pipelined batch, one real pub/sub round-trip, and the judgment of when a second store "+
			"earns its keep beside PostgreSQL — with the terminal's redis-cli and curl tour.",
		"data",
		20,
		sections,
	)
}
