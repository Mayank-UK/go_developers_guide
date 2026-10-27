package content

import (
	"errors"
	"fmt"
)

// This file is topic 21 caching (group state-auth, curriculum order 21),
// the state group's opening topic: the two worlds a backend cache lives in
// (the HTTP edge and the server's own memory), the conditional-request half
// of HTTP caching (ETag, If-None-Match, the 304) served by the lab
// endpoint's own responses, and the server-side half as a REAL cache-aside
// decorator around the posts port — cache on read, invalidate on write, TTL
// as the safety net, the epoch guard against in-flight refills that straddle
// a write, request coalescing so a herd of misses costs one store read, and
// the partition salt that keeps the scenario's two stores from reading each
// other's entries. The terminal companion closes with the ETag round trip
// by curl.

// cachingTopic builds topic 21.
func cachingTopic() (Topic, error) {
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
		// ── 1. The two worlds ───────────────────────────────────────
		section("two-worlds", "The two worlds of caching",
			"Every cache a backend ships lives in one of two places — the HTTP edge where representations are reused, or the server's own memory where data is derived — and mixing up their rules is where caching goes wrong.",
			text("two-worlds-pitch",
				"\"Should we cache this?\" is two different questions depending on where the "+
					"cache sits. At the EDGE — the browser's HTTP cache, a CDN's PoP, a reverse "+
					"proxy — what gets reused is a REPRESENTATION: the exact bytes of a response, "+
					"replayed under rules the HTTP protocol defines (freshness lifetimes, "+
					"validators, conditional requests). On the SERVER — a map beside your "+
					"handler, a Redis beside your database — what gets reused is DATA: a value "+
					"your code derived once and wants to hand out again without re-deriving "+
					"it. The edge world is governed by headers any client can inspect in "+
					"devtools; the server world is governed by YOUR invariants — what a write "+
					"retires, what a TTL bounds, what a stampede costs. Same word, different "+
					"physics."),
			text("two-worlds-failures",
				"The failure modes differ too, and knowing them tells you which world your "+
					"problem lives in. An edge cache serving something stale is VISIBLE: the "+
					"client asked, the cache answered from its copy, and the response's own "+
					"headers explain the replay — the fix is protocol hygiene (shorter "+
					"freshness, honest validators). A server cache serving something stale is "+
					"SILENT: the handler called the repository, got a value, wrote a 200 — "+
					"nobody between the cache and the wire can tell the value is old — so the "+
					"fix is invalidation discipline and bounded staleness, decided in code "+
					"you own. When someone says \"caching is hard\", they usually mean the "+
					"second world's hardest rule: the cache must never become a second source "+
					"of truth. Topic 20 said it for Redis (everything in it must be "+
					"rebuildable); this topic enforces it for every cache the guide ships."),
			list("two-worlds-where", false,
				"Browser HTTP cache — per-user, per-device; obeys Cache-Control and validators; the only cache your curl tour cannot see from the server side.",
				"CDN edge — shared, global, close to users; caches whole responses or static assets; you control it with headers and URL design, never with code on the request path.",
				"Reverse proxy — nginx or similar in front of the app; can cache, but usually just relays the app's own headers.",
				"In-process cache — a map (or this topic's decorator) inside the Go server; nanosecond hits, per-instance, dies with the process; the world of invalidation and stampedes.",
				"Remote cache store — Redis beside PostgreSQL (topic 20); shared across instances, TTL-native, still derived state.",
			),
			text("two-worlds-map",
				"This topic walks both worlds in the order a request meets them, and the "+
					"guide runs both FOR REAL. The HTTP half is not a slide: the caching lab "+
					"endpoint certifies its own deterministic payload with a strong ETag and "+
					"answers a replayed ``If-None-Match`` with an honest 304 — you will curl "+
					"exactly that round trip in the terminal companion. The server half is "+
					"not a sketch either: a real cache-aside decorator wraps the posts "+
					"repository port, with write invalidation, TTL expiry, and herd "+
					"coalescing — the caching lab runs the whole lifecycle against it and "+
					"reports every counter it moved. One rule travels with you across both "+
					"worlds: cache what you can DERIVE, bound what you cannot invalidate, "+
					"and never let a cache invent truth the store did not write."),
		),
		// ── 2. HTTP caching: validators and the 304 ─────────────────
		section("http-caching", "HTTP caching: validators and the 304",
			"Freshness lets an edge answer without asking; validation lets it ask cheaply — Cache-Control picks the policy, the ETag proves the bytes, and 304 is the round trip that saves the body.",
			text("http-freshness",
				"The edge half of caching is two mechanisms, and confusing them is the "+
					"classic mistake. FRESHNESS is the cache answering alone: "+
					"``Cache-Control: max-age=30`` means \"within thirty seconds, replay "+
					"these bytes without talking to the server\" — fast, free, and blind. "+
					"VALIDATION is the cache asking the server a yes/no question: \"I have "+
					"the representation tagged X — is it still current?\" The tag is the "+
					"VALIDATOR — an ``ETag`` header, an opaque quoted string the server "+
					"guarantees to change whenever the representation's bytes change — and "+
					"the question rides a conditional request: ``If-None-Match: X``. The "+
					"server answers ``304 Not Modified`` with the same headers but NO body "+
					"when nothing changed, or a full ``200`` with new bytes and a new tag "+
					"when something did. Freshness saves the whole round trip; validation "+
					"saves the body, the serialization, and — when the response is "+
					"expensive to produce — the work."),
			text("http-directives",
				"The directive vocabulary is small and worth saying precisely, because "+
					"two of its words do not mean what they sound like. "+
					"``Cache-Control: no-store`` means never keep a copy anywhere — for "+
					"responses that must never be replayed (auth errors, one-time tokens). "+
					"``no-cache`` does NOT mean \"don't cache\": it means \"you may keep a "+
					"copy, but every reuse must revalidate first\" — the 304 path stays "+
					"exercised instead of the entry going silently dark for a max-age. "+
					"``max-age=N`` grants N seconds of freshness. ``private`` restricts "+
					"sharing to the user's own browser cache (a shared CDN must not replay "+
					"one user's personalized response to another); ``public`` explicitly "+
					"permits shared caches. ``s-maxage`` sets a different freshness for "+
					"shared caches than for the browser. A public API that wants edge "+
					"caching pins these deliberately; a teaching API that wants its "+
					"revalidation visible picks ``no-cache`` on purpose — which is exactly "+
					"what the lab endpoint below does."),
			text("http-strong-weak",
				"Validators come strong and weak. A STRONG ETag (no prefix) promises "+
					"byte-for-byte identity: two representations with the same strong tag "+
					"are interchangeable, no asterisks. A WEAK validator — ``W/\"tag\"`` — "+
					"promises only semantic equivalence: the bytes may differ (a rebuilt "+
					"template, a re-ordered JSON map) but the meaning does not. Strong "+
					"tags are what range requests and byte-exact resumption need; weak "+
					"tags are the pragmatic escape when \"equivalent\" is the honest "+
					"claim you can make. The other validator family — ``Last-Modified`` "+
					"and ``If-Modified-Since`` — predates ETags and rides timestamps: "+
					"cheaper to produce, but a timestamp has second granularity and says "+
					"nothing about content, so an ETag computed over the representation "+
					"is the stronger claim whenever you can afford the hash."),
			code("http-etag-go", "go",
				"The lab endpoint is its own exhibit: a strong ETag over the exact response bytes, no-cache so every reuse revalidates, and the honest 304.",
				"server/internal/presentation/demo_caching.go",
				"digest := sha256.Sum256(body)\n"+
					"etag := `\"` + hex.EncodeToString(digest[:8]) + `\"`\n"+
					"\n"+
					"w.Header().Set(\"Content-Type\", \"application/json\")\n"+
					"// no-cache does not mean \"do not store\": it means every reuse must\n"+
					"// REVALIDATE against this ETag first — the 304 path stays exercised\n"+
					"// instead of the entry going dark for max-age.\n"+
					"w.Header().Set(\"Cache-Control\", \"no-cache\")\n"+
					"w.Header().Set(\"ETag\", etag)\n"+
					"\n"+
					"if matchesETag(r.Header.Get(\"If-None-Match\"), etag) {\n"+
					"    w.WriteHeader(http.StatusNotModified)\n"+
					"    return\n"+
					"}"),
			text("http-reading",
				"Read the handler's three honest decisions. First, the ETag is a "+
					"``sha256`` over the exact bytes about to be written — not over some "+
					"upstream value that could drift from the serialization — so the "+
					"validator can never certify a representation it did not hash. "+
					"Second, ``no-cache`` keeps revalidation on the hot path: a max-age "+
					"would make browsers answer from memory and the conditional request "+
					"would never fire, which would teach nothing. Third, the 304 answers "+
					"through the same middleware chain as every other response — request "+
					"id, logging, recovery all still run — because a short-circuit that "+
					"skips the chain is a different endpoint pretending to be this one. "+
					"Go's ``net/http`` gives you all of this as primitives: you set the "+
					"headers, you write the status, the server does no GET-caching magic "+
					"behind your back. The condition itself is one small function:"),
			code("http-match-go", "go",
				"If-None-Match's grammar is a list: any matching member satisfies the condition.",
				"server/internal/presentation/demo_caching.go",
				"func matchesETag(header, etag string) bool {\n"+
					"    if header == \"\" {\n"+
					"        return false\n"+
					"    }\n"+
					"    for _, candidate := range strings.Split(header, \",\") {\n"+
					"        if strings.TrimSpace(candidate) == etag {\n"+
					"            return true\n"+
					"        }\n"+
					"    }\n"+
					"    return false\n"+
					"}"),
			text("http-scope",
				"The helper's comment in the source names its scope plainly: weak "+
					"validators (the ``W/`` prefix) and the ``*`` wildcard are out of this "+
					"demo's path — an honest simplification stated in code, rather than a "+
					"half-implementation that would match sometimes. That is the tone to "+
					"copy: HTTP caching's spec corners are deep, and a real service "+
					"implements the slice it means to serve. The next half of the topic "+
					"moves one layer down — from reusing REPRESENTATIONS at the edge to "+
					"deriving DATA on the server, where the rules are yours to write and "+
					"the mistakes are yours to keep."),
		),
		// ── 3. Cache-aside around the port ──────────────────────────
		section("cache-aside", "Server-side caching: cache-aside around the port",
			"Read the cache first, refill on miss from the store, and invalidate on write — implemented as a decorator over the posts repository port so every use case and handler above the seam stays untouched.",
			text("aside-pattern",
				"Cache-aside (sometimes lazy loading) is the pattern with the best "+
					"effort-to-honesty ratio: the cache sits BESIDE the store, nothing "+
					"writes through it, and every read asks it first. A read that hits "+
					"returns from memory; a read that misses goes to the store, stores "+
					"what it fetched with a deadline, and returns; a WRITE goes to the "+
					"store — always the source of truth — and then retires everything the "+
					"write could have changed. The alternative, write-through (update the "+
					"cache on write), buys fresher caches at the cost of the cache "+
					"becoming a second thing a write must get right; cache-aside keeps "+
					"the write path exactly as it was and accepts a refill on the next "+
					"read. In Go the natural home for the pattern is a DECORATOR over "+
					"the repository port — the seam this guide has used since the "+
					"interfaces topic: the port is the contract, use cases and handlers "+
					"hold it, and a wrapping implementation can add caching without any "+
					"of them knowing:"),
			code("aside-constructor-go", "go",
				"The decorator's constructor: any repository in, the port back out — plus the TTL, the partition salt, and the clock seam.",
				"server/internal/data/posts/caching.go",
				"func NewCachingRepository(inner posts.PostsRepository, ttl time.Duration, partition func() string, now func() time.Time) *CachingRepository {\n"+
					"    if partition == nil {\n"+
					"        partition = func() string { return \"\" }\n"+
					"    }\n"+
					"    if now == nil {\n"+
					"        now = func() time.Time { return time.Now().UTC() }\n"+
					"    }\n"+
					"    return &CachingRepository{\n"+
					"        inner:     inner,\n"+
					"        ttl:       ttl,\n"+
					"        partition: partition,\n"+
					"        now:       now,\n"+
					"        posts:     make(map[string]cachedPost),\n"+
					"        pages:     make(map[string]cachedPage),\n"+
					"        inFlight:  make(map[string]*fillSlot),\n"+
					"    }\n"+
					"}"),
			text("aside-keys",
				"Two maps, two key families. ``GetByID`` answers from a map keyed by the "+
					"post's id; ``List`` answers from a map keyed by the CANONICAL query — "+
					"page, limit, tag, and sort folded into one string — so every distinct "+
					"query caches independently and no two queries can collide. Both keys "+
					"are salted with a PARTITION string (a ``func() string`` the wiring "+
					"chooses) and quote their user-supplied parts, which matters more than "+
					"it looks: the salting means two different backing stores can never "+
					"read each other's entries — the caching lab exploits exactly that "+
					"with the scenario's fake/live modes — and the quoting means an id or "+
					"tag containing separator bytes cannot forge a neighbor's key. The "+
					"read path is one loop with three exits:"),
			code("aside-read-go", "go",
				"The read engine: warm hit, coalesced wait, or leader refill — with the epoch check at store time.",
				"server/internal/data/posts/caching.go",
				"func (c *CachingRepository) GetByID(ctx context.Context, id string) (posts.Post, error) {\n"+
					"    key := c.postKey(id)\n"+
					"    for round := 0; round < 3; round++ {\n"+
					"        c.mu.Lock()\n"+
					"        if entry, ok := c.posts[key]; ok {\n"+
					"            if c.fresh(entry.expiresAt, entry.epoch) {\n"+
					"                c.stats.Hits++\n"+
					"                c.mu.Unlock()\n"+
					"                return copyPost(entry.post), nil\n"+
					"            }\n"+
					"            c.dropStale(key, entry.expiresAt)\n"+
					"        }\n"+
					"        if pending, joined := c.joinFill(key); joined {\n"+
					"            c.mu.Unlock()\n"+
					"            if err := waitFill(ctx, pending); err != nil {\n"+
					"                return posts.Post{}, err\n"+
					"            }\n"+
					"            continue // the leader finished; re-check the cache from the top\n"+
					"        }\n"+
					"        lead := c.leadFill(key)\n"+
					"        c.mu.Unlock()\n"+
					"\n"+
					"        post, err := c.inner.GetByID(ctx, id)\n"+
					"\n"+
					"        c.mu.Lock()\n"+
					"        c.settleFill(key)\n"+
					"        if err == nil && c.ttl > 0 && c.epoch == lead.epoch {\n"+
					"            c.storePostLocked(key, post)\n"+
					"        }\n"+
					"        c.mu.Unlock()\n"+
					"        close(lead.slot.done)\n"+
					"        return post, err\n"+
					"    }\n"+
					"    // …three raced rounds: one uncached passthrough read — slower, never wrong\n"+
					"}"),
			text("aside-read-notes",
				"Four details earn their lines. The hit returns ``copyPost`` — a deep copy, "+
					"so a caller mutating the handed-out post can never corrupt the cached "+
					"entry (the same discipline the in-memory repository applies to its "+
					"store). The refill calls the wrapped repository WITHOUT the lock — "+
					"the whole point of a decorator is that hits stay fast while a miss "+
					"does real work, and holding one mutex across a database round trip "+
					"would serialize every reader behind the slowest store. Errors pass "+
					"through untouched: the typed ``core.NotFound`` of a real miss is the "+
					"caller's to map, and — deliberately — negative results are NEVER "+
					"cached (a later Create of the same shape must not find a cached "+
					"miss; the next section says why that is safe). And the refill only "+
					"stores when ``c.epoch == lead.epoch`` — the guard that keeps a "+
					"refill which straddled a write from caching its pre-write snapshot. "+
					"The demo below runs every one of those lines:"),
			demo("caching:lab", "The caching lab: the whole lifecycle in one payload",
				"One request through the REAL decorator on throwaway stores: a cold read "+
					"and its warm hit, the cached page, the write that retires id and pages "+
					"together, the TTL expiry read as data on a fixture clock, a "+
					"twelve-reader herd coalesced into one store read, and the fake/live "+
					"store swap that stays partition-honest — with a scoreboard whose "+
					"misses and store reads agree, and a payload so deterministic the "+
					"response itself carries the ETag this topic teaches."),
			text("aside-lab-wiring",
				"The lab is not a fixture with printed numbers — it is the real type, "+
					"wired the way a test wires it: two throwaway in-memory stores (the "+
					"\"live\" one seeded with a seventh post the fake never sees), a LOCAL "+
					"scenario store and the same switching facade production uses, a "+
					"counting witness that records every read delegated to a store, and "+
					"the decorator on top with its partition bound to the scenario's data "+
					"mode:"),
			code("aside-lab-wiring-go", "go",
				"The lab's wiring: real facade, real decorator, disposable stores — and the partition bound to the data mode.",
				"server/internal/data/cachinglab/runner.go",
				"scenario := core.NewScenarioStore()\n"+
					"switching := postsdata.NewSwitchingRepository(scenario, fakeStore, liveStore)\n"+
					"store := &readCountingRepository{PostsRepository: switching}\n"+
					"partition := func() string { return string(scenario.Snapshot().DataMode) }\n"+
					"cache := postsdata.NewCachingRepository(store, demos.CachingLabTTL, partition, clock.Now)"),
			text("aside-production",
				"One honest note about where the decorator runs in THIS guide: the lab "+
					"wires it, and the lab is where the production posts path leaves it — "+
					"deliberately. The composition-root lines that build the posts use "+
					"cases over the switching facade are already quoted, byte for byte, "+
					"by the api-design and databases topics (this guide's snippet "+
					"contract: every quoted block matches its source), and re-quoting "+
					"two landed topics to insert a wrap between facade and use cases is "+
					"a cross-topic churn the guide does not need mid-curriculum — the "+
					"seam exists, the type is production-grade, and wrapping the live "+
					"path is one constructor call at that seam the day the guide wants "+
					"it. The judgment is the lesson: a cache is a policy decision with "+
					"a blast radius, not a default. When you DO wrap a production path, "+
					"the next two sections are the checklist that makes it safe."),
		),
		// ── 4. Invalidation ─────────────────────────────────────────
		section("invalidation", "Invalidation: the hard half",
			"A write through the port retires the written id and every cached page, an epoch counter retires anything a racing refill fetched too early, and the TTL bounds what neither can see.",
			text("inv-pitch",
				"There are only two hard things in computer science, the old line goes, "+
					"and cache invalidation is the one this section owns. The decorator's "+
					"rule is blunt on purpose: every SUCCESSFUL write retires the written "+
					"id's entry and EVERY cached page — not just the first page, not just "+
					"the current sort, all of them — because a write moves membership "+
					"(Create adds to some page), totals (every page's metadata), and "+
					"ordering (a new timestamp or title re-sorts every list). Being "+
					"precise about which pages a write touched would need the store to "+
					"answer questions no repository port can ask; flushing pages is "+
					"conservative, correct, and cheap — pages refill on their next read. "+
					"A FAILED write flushes nothing: a Replace that answered 404 changed "+
					"no data, and a cache that flushed on failure would be pessimistic "+
					"for no correctness gain:"),
			code("inv-write-go", "go",
				"The write path: delegate to the store first, retire on success — the cache is never the source of truth.",
				"server/internal/data/posts/caching.go",
				"func (c *CachingRepository) Replace(ctx context.Context, id string, draft posts.Draft) (posts.Post, error) {\n"+
					"    post, err := c.inner.Replace(ctx, id, draft)\n"+
					"    if err == nil {\n"+
					"        c.invalidateWrite(post.ID)\n"+
					"    }\n"+
					"    return post, err\n"+
					"}"),
			code("inv-write-core-go", "go",
				"The retire itself: bump the epoch, drop the id's entry, clear every page, count what left.",
				"server/internal/data/posts/caching.go",
				"func (c *CachingRepository) invalidateWrite(id string) {\n"+
					"    c.mu.Lock()\n"+
					"    defer c.mu.Unlock()\n"+
					"    c.epoch++\n"+
					"    dropped := len(c.pages)\n"+
					"    c.pages = make(map[string]cachedPage, maxPages)\n"+
					"    key := c.postKey(id)\n"+
					"    if _, ok := c.posts[key]; ok {\n"+
					"        delete(c.posts, key)\n"+
					"        dropped++\n"+
					"    }\n"+
					"    c.stats.Invalidations += uint64(dropped)\n"+
					"}"),
			text("inv-epoch",
				"The ``epoch`` counter is the part worth slowing down for, because it "+
					"closes a race that simple delete-missed-key caching gets wrong. "+
					"Picture a read that misses, delegates to the store, and is STILL "+
					"waiting on the database when a write lands and flushes the cache. "+
					"The waiting read then returns with its PRE-write snapshot and "+
					"stores it — the cache now proudly serves yesterday's value with "+
					"tomorrow's deadline, and no future write will retire it, because "+
					"no write knows it arrived. The epoch makes every entry carry the "+
					"generation it was fetched under: writes bump the generation, "+
					"refills only store if the generation is UNCHANGED since their "+
					"fetch began (the ``c.epoch == lead.epoch`` check in the read "+
					"engine), and hits only count if the entry's generation still "+
					"matches — one comparison, checked in the one-liner everything "+
					"else consults:"),
			code("inv-fresh-go", "go",
				"Freshness is two checks: inside the TTL, and fetched before the latest write.",
				"server/internal/data/posts/caching.go",
				"func (c *CachingRepository) fresh(expiresAt time.Time, entryEpoch uint64) bool {\n"+
					"    return c.ttl > 0 && c.now().Before(expiresAt) && entryEpoch == c.epoch\n"+
					"}"),
			text("inv-partition-and-ttl",
				"The same epoch guard answers a question this guide's own architecture "+
					"asks: what happens when the SWITCHING FACADE flips stores under the "+
					"decorator — a cached fake-mode row served in live mode? Two "+
					"defenses stack. The partition salt folds the scenario's data mode "+
					"into every key, so a flip lands on a cold keyspace by construction "+
					"— and because a write in EITHER mode bumps the shared epoch, an old "+
					"entry from the other mode cannot survive its next touch even if "+
					"the flip flips back. The caching lab's swap beat proves it with "+
					"stores that answer differently for the same id: no leak, in either "+
					"direction. And the TTL is the third wall — the one that covers "+
					"writes the process cannot see at all: a psql session inserting a "+
					"row directly into PostgreSQL never passes through the decorator, "+
					"so nothing is invalidated; the entry simply EXPIRES. That is the "+
					"contract to state out loud: invalidation covers every write you "+
					"own, the TTL bounds every write you do not — pick the TTL as the "+
					"longest staleness your users can stomach from a stranger's write."),
			text("inv-negative",
				"Deliberately missing from the write path: invalidating a just-Create-d "+
					"id. Nothing needs to, because misses are never cached — the read "+
					"engine stores only successful fetches. Negative caching (caching "+
					"the 404 for a hot missing id) is a real technique for hot paths "+
					"hammered with unknown ids, but it buys correctness problems this "+
					"guide refuses by default: a Create cannot invalidate an id it did "+
					"not know it would mint, so a cached miss would need its own TTL "+
					"discipline and its own reasoning about when \"not there\" stops "+
					"being true. The rule the guide teaches: never cache the absence of "+
					"something you cannot be told about later. If a path truly needs "+
					"negative caching, it needs it SHORT — seconds, not minutes — and "+
					"stated in the response headers so the client can see the policy."),
		),
		// ── 5. Stampedes ────────────────────────────────────────────
		section("stampedes", "Stampedes: coalescing the herd",
			"When a hot entry expires, every concurrent reader misses at once — and N simultaneous store reads is the store falling over exactly when it is hottest. The decorator coalesces them into one.",
			text("stampede-pitch",
				"The stampede (thundering herd, dog-pile, cache miss storm — the names "+
					"are all the same failure) is what happens at the exact moment a "+
					"cache stops helping: a hot key expires, fifty requests arrive in "+
					"the same millisecond, all fifty miss, and all fifty hit the store "+
					"TOGETHER — the cache was protecting the database from exactly this "+
					"traffic, and its expiry just delivered the whole herd to it at "+
					"once. The standard packaged answer is ``golang.org/x/sync/"+
					"singleflight``: one in-flight call per key, every other caller "+
					"shares its result. This guide takes no new dependency for it, so "+
					"the decorator hand-rolls the same pattern — a per-key map of "+
					"in-flight refills under the same mutex everything else uses — "+
					"which is also the honest way to learn what singleflight actually "+
					"does:"),
			code("stampede-trio-go", "go",
				"The coalescing trio: join a leader if one exists, become the leader if not, park until the leader closes the gate.",
				"server/internal/data/posts/caching.go",
				"func (c *CachingRepository) joinFill(key string) (*fillSlot, bool) {\n"+
					"    if slot, ok := c.inFlight[key]; ok {\n"+
					"        c.stats.Coalesced++\n"+
					"        return slot, true\n"+
					"    }\n"+
					"    return nil, false\n"+
					"}\n"+
					"\n"+
					"func (c *CachingRepository) leadFill(key string) *leadToken {\n"+
					"    slot := &fillSlot{done: make(chan struct{})}\n"+
					"    c.inFlight[key] = slot\n"+
					"    c.stats.Misses++\n"+
					"    c.stats.Underlying++\n"+
					"    return &leadToken{slot: slot, epoch: c.epoch}\n"+
					"}\n"+
					"\n"+
					"func waitFill(ctx context.Context, slot *fillSlot) error {\n"+
					"    select {\n"+
					"    case <-slot.done:\n"+
					"        return nil\n"+
					"    case <-ctx.Done():\n"+
					"        return ctx.Err()\n"+
					"    }\n"+
					"}"),
			text("stampede-semantics",
				"Three semantic choices separate this from a copy-pasted singleflight, "+
					"and each is deliberate. Waiters do NOT share the leader's result "+
					"value: they wake and re-check the cache from the top of the read "+
					"engine — if the leader stored, the waiter hits warm; if the leader "+
					"failed or a write invalidated mid-flight, the waiter becomes the "+
					"next leader itself. That costs a lock cycle per waiter and buys "+
					"simpler reasoning: nobody ever receives a value the cache itself "+
					"would not serve. The wait is context-bound — a caller whose "+
					"request is canceled stops waiting without wedging a goroutine "+
					"(topic 6's law, applied to a cache). And the retry loop is "+
					"BOUNDED: after three rounds of writes-and-refills racing, the read "+
					"falls through to one uncached passthrough — slower, never wrong, "+
					"and honestly counted in the scoreboard. The lab's herd beat pins "+
					"the payoff: twelve concurrent readers of one cold key, exactly one "+
					"store read, every reader holding the same answer — the census the "+
					"payload derives its proof from, not a promise the pattern makes."),
			text("stampede-beyond",
				"Coalescing protects the refill moment, not the whole cache's "+
					"lifecycle, and production systems layer more on top. Jittered or "+
					"staggered TTLs (each entry's deadline varied by a small random "+
					"factor) keep a thousand hot keys from expiring in the same "+
					"instant. Background refresh (a goroutine refreshing a soon-to-"+
					"expire hot entry before any reader misses) moves the cost off the "+
					"request path entirely — at the price of refreshes for entries "+
					"nobody may read again. A \"stale-while-revalidate\" policy serves "+
					"the expired value immediately while one leader refreshes — honest "+
					"only if staleness is bounded and stated. Every one of these is a "+
					"policy dial, not a correctness fix; the correctness floor is what "+
					"this topic already built — invalidate what you own, expire what "+
					"you do not, and never let the herd through the door at once."),
		),
		// ── 6. CDNs and content-hashed filenames ────────────────────
		section("cdn-hashed-assets", "CDNs and content-hashed filenames",
			"You cannot invalidate the world's browsers — so you make asset URLs immutable instead: hash the content into the filename, cache forever, and let the HTML be the only thing that revalidates.",
			text("cdn-pitch",
				"A CDN is a farm of caches you do not control at the edge you do not "+
					"own: it terminates TLS near your users, absorbs your static bytes, "+
					"and replays them without your server ever seeing the request. For "+
					"an API the CDN is a policy surface (which responses may be cached "+
					"at the edge, and for how long — the Cache-Control vocabulary from "+
					"the HTTP section); for STATIC ASSETS the CDN is the whole game, "+
					"and the game has one dominant strategy: make the URL answer for "+
					"its own bytes. The trick is the content-hashed filename — "+
					"``app.a3f9c2.js`` — where the name embeds a digest of the file's "+
					"contents. Change the code, the hash changes, the URL changes, and "+
					"the new URL is a cache miss everywhere in the world by "+
					"construction. No invalidation API, no purge, no TTL arithmetic: "+
					"immutability does the work invalidation would have to."),
			text("cdn-index-html",
				"The one file that must NOT be hashed-and-immortal is the entry point "+
					"— ``index.html`` — because it is the document that NAMES the "+
					"hashed assets. The standard pair is: the HTML served with "+
					"``Cache-Control: no-cache`` (every load revalidates with the "+
					"origin; the 304 from the HTTP section is exactly this round "+
					"trip), and everything under ``/assets/`` served with "+
					"``Cache-Control: public, max-age=31536000, immutable`` — one "+
					"year of freshness plus a promise the bytes will never change, "+
					"which lets browsers skip even the revalidation request. This "+
					"guide's own client ships that shape: Vite — the client's build "+
					"tool — emits content-hashed asset filenames into ``dist/`` on "+
					"every production build, and the deployed HTML is the only file "+
					"whose cache policy stays polite. You can watch the strategy "+
					"work in any browser devtools panel: first load fetches "+
					"everything, reload fetches the HTML (revalidated) and nothing "+
					"else."),
			list("cdn-rules", false,
				"Hash the content into the filename — the URL changes exactly when the bytes do; immutability replaces invalidation.",
				"Cache hashed assets forever — public, max-age=31536000, immutable; a year is a rounding error when the name changes per build.",
				"Keep the entry HTML honest — no-cache on index.html so every deploy is picked up on the next load, via a cheap 304.",
				"Never long-cache an API response by accident — an API's JSON is data, not an asset; its policy is deliberate (usually no-store or short max-age).",
				"Give personalized responses private — a shared edge must never replay one user's response to another; Cache-Control: private says so in one word.",
				"Version APIs in the path, not the headers — /v1/ and /v2/ coexist at the edge forever; header-based versioning makes every cached response a negotiation.",
			),
			text("cdn-coda",
				"Notice what this section did not do: it did not put a CDN in front of "+
					"the guide's API. The stack runs one server and one client in "+
					"containers, and a teaching API wants its requests VISIBLE — every "+
					"topic's curl tour talks to the server that actually did the work. "+
					"But the asset strategy is already real in the client build, and "+
					"the HTTP mechanics are already real in the lab endpoint's ETag "+
					"round trip. When a real deployment adds the CDN (the deployment "+
					"topic's territory), the rules above are the whole checklist — "+
					"the same two worlds from the first section, cooperating: the "+
					"edge caches immutable bytes by name, the server caches derived "+
					"data by policy, and neither ever guesses at truth."),
		),
		// ── 7. What NOT to cache ────────────────────────────────────
		section("what-not-to-cache", "What NOT to cache",
			"The fastest cache is the one you did not build: anything not safely derivable, anything you cannot invalidate or bound, and anything whose cardinality is a stranger — leave those uncached on purpose.",
			text("not-pitch",
				"A cache is a bet: you bet the read savings beat the staleness risk, "+
					"the memory, and the maintenance. Most caching advice is about how "+
					"to win that bet; this section is about which bets not to place. "+
					"The unifying test is the one topic 20 set for Redis, generalized: "+
					"everything in a cache must be REBUILDABLE — derivable from the "+
					"source of truth by a process you control, with a staleness bound "+
					"you chose. The moment a cached value stops being derivable (it "+
					"accumulated state nothing else has) or its bound stops being "+
					"chosen (you cannot say how old it can get), the cache has quietly "+
					"become a database — one with no backups, no migrations, and no "+
					"transaction log. The list below is the guide's do-not-cache "+
					"table, each row a bet refused for a stated reason:"),
			list("not-table", false,
				"Anything you cannot invalidate or bound — if no write path retires it and no TTL expires it, it is not a cache entry, it is an unmanaged database row.",
				"Absence you cannot be told about later — a cached 404 for an id that a later Create can mint; never cache the miss of something whose birth you will not hear about.",
				"Per-user data at a shared edge — personalized responses under shared-cache keys replay one user's data to another; private or no-store, no exceptions.",
				"Authorization decisions in a cache — a permission cached for a TTL outlives the revocation that should have stopped it; deny-by-default (the authorization topic's stance) cannot be stale.",
				"One-shot and soon-mutating values — a value written once and read once (a confirmation number, a just-accepted draft) gains nothing from a cache and pays invalidation for it.",
				"Unbounded key cardinality — a per-user × per-sort × per-page list key space is a memory leak with a map interface; cap the entries and cache the hot queries, or do not cache the cold ones.",
				"Huge blobs with a low hit rate — a 5 MB payload cached for an occasional reader evicts hundreds of small hot entries to save one rare fetch; size caps exist for a reason.",
			),
			text("not-coda",
				"Two corollaries close the topic's server half. First, MEASURE before "+
					"cacheing and after: the decorator's scoreboard (hits, misses, "+
					"expiries, invalidations, underlying reads) is not decoration — it "+
					"is how you know the bet is paying, and the lab's misses-equal-"+"store-reads invariant is the shape of that audit. Second, the cheapest "+
					"cache is the one you deleted: a hot path that got 10x faster by "+
					"fixing the query (an index, topic 16) does not need a cache at "+
					"all, and a cache bolted over a slow query hides the index that "+
					"was missing. Cache the reads you cannot make cheap; fix the ones "+
					"you can."),
		),
		// ── 8. Terminal companion ───────────────────────────────────
		section("terminal-companion", "Terminal companion: the ETag round trip by curl",
			"The topic from the shell: watch the lab's headers, replay the validator for a real 304, prove the payload's determinism with diff, and read the scoreboard the beats left behind.",
			text("term-pitch",
				"With the stack up (server on :8100), this tour exercises the topic's "+
					"two halves with curl alone. The lab endpoint is the star because "+
					"it certifies itself: a deterministic payload hashed into a strong "+
					"ETag, served with ``Cache-Control: no-cache`` so the revalidation "+
					"round trip always fires. Every command below runs against the "+
					"RUNNING server — the same endpoint the browser demo panel "+
					"renders:"),
			code("term-lab-headers", "bash", "", "",
				"# the lab with response headers: find the ETag and the no-cache policy\n"+
					"curl -sD - -o /dev/null localhost:8100/api/demo/caching/lab\n"+
					"\n"+
					"# the payload itself — every beat, the herd census, the swap, the scoreboard\n"+
					"curl -s localhost:8100/api/demo/caching/lab | python3 -m json.tool | head -60"),
			code("term-304", "bash", "", "",
				"# capture the validator, then replay it as If-None-Match: HTTP/1.1 304, no body\n"+
					"curl -sD /tmp/lab-headers -o /tmp/lab-body localhost:8100/api/demo/caching/lab\n"+
					"ETAG=$(grep -i '^etag:' /tmp/lab-headers | tr -d '\\r' | cut -d' ' -f2)\n"+
					"curl -sD - -o /dev/null -H \"If-None-Match: $ETAG\" localhost:8100/api/demo/caching/lab\n"+
					"\n"+
					"# determinism is the premise: a second full fetch is byte-identical\n"+
					"curl -s localhost:8100/api/demo/caching/lab > /tmp/lab-body-2\n"+
					"diff /tmp/lab-body /tmp/lab-body-2 && echo identical — same beats, same bytes, same ETag"),
			code("term-scoreboard", "bash", "", "",
				"# the scoreboard's honesty check, straight from the payload:\n"+
					"# misses must equal storeReads — every miss led exactly one store read\n"+
					"curl -s localhost:8100/api/demo/caching/lab | python3 -c \\\n"+
					"  'import json,sys; s=json.load(sys.stdin)[\"stats\"]; print(s[\"misses\"], \"misses /\", s[\"storeReads\"], \"store reads\")'"),
			text("term-coda",
				"Notice what the tour proves without asking you to trust it: the 304 "+
					"arrives with the same validator and no body — the exact round trip "+
					"the HTTP section taught, served by the endpoint that taught it; "+
					"``diff`` says the two payloads are byte-identical, which is WHY "+
					"the validator could be replayed at all; and the scoreboard's "+
					"misses equal its store reads because coalescing never let a "+
					"second reader through to the store. That closes the state group's "+
					"opening topic: caching in both its worlds, each claim run for "+
					"real. Next, topic 22 keeps the store from topic 20 and builds "+
					"messaging on Redis Streams — queues where caching's \"derive "+
					"safely\" discipline becomes delivery guarantees you can lose "+
					"data by getting wrong."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"caching",
		"Caching",
		"The two worlds of caching — the HTTP edge (ETag, If-None-Match, the honest "+
			"304) and the server's own memory (a real cache-aside decorator around the "+
			"posts port, with write invalidation, TTL expiry, herd coalescing, and the "+
			"partition salt that keeps two stores from sharing entries) — plus CDN and "+
			"content-hashed asset strategy, the do-not-cache table, and a terminal "+
			"companion that replays the validator by curl.",
		"state-auth",
		21,
		sections,
	)
}
