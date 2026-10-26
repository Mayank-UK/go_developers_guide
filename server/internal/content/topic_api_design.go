package content

import (
	"errors"
	"fmt"
)

// This file is topic 11 api-design (group apis, curriculum order 11),
// authored on the exemplar pattern: prose is the default block, every
// must-source code block quotes a real repo file (the snippet ratchet
// verifies it), the live demo sits INLINE in the section that teaches the
// whole pipeline, and the topic closes with the bash terminal companion —
// a full CRUD round-trip against the live /api/posts surface this topic
// ships.

// apiDesignTopic builds topic 11 — resource-oriented HTTP API design on
// the real posts substrate: what REST is, designing resources, methods
// and statuses, URL design with pagination and filtering, the recorded
// error envelope (with RFC 9457 held up for comparison), versioning, and
// the whole request pipeline from route to repository.
func apiDesignTopic() (Topic, error) {
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
		// ── 1. What REST actually is ──────────────────────────────────
		section("what-rest-is", "What REST actually is",
			"REST is a set of constraints borrowed from HTTP's grain — not a protocol, and definitely not a URL aesthetic.",
			text("rest-pitch",
				"REST is the most used and most misused word in API design, so start by pinning it down. REST — "+
					"Representational State Transfer — is Fielding's 2000 description of an architectural STYLE: "+
					"a set of constraints (resources identified by URLs, a uniform interface of methods and media "+
					"types, stateless requests, layered caching) that together make a distributed system behave "+
					"predictably at internet scale. It is not a protocol, not a library, and not the act of putting "+
					"nouns in your URLs. An API \"is REST\" to the degree it honors those constraints, and the practical "+
					"value is not purity — it is that every consumer who knows HTTP already knows most of your "+
					"contract: GET reads, POST creates, status codes mean what they always mean, and the same "+
					"request can be retried, cached, or proxied because the semantics are uniform."),
			text("rest-misuse",
				"The misuse arrives as two caricatures. The first is RPC-in-URLs: endpoints like "+
					"``/api/createPost`` or ``/api/posts/delete?id=7`` that use HTTP as a transport for function "+
					"calls — one URL per verb-shaped action, methods ignored, status codes set to 200 with the real "+
					"outcome buried in the body. The second is REST-as-decorum: collections that are plural and "+
					"URLs that are pretty while PUT means \"partial update\" and DELETE returns 200 with "+
					"``{deleted: true}``. Both miss the point. The uniform interface is the product: when methods "+
					"carry verb semantics and statuses carry outcomes, generic tooling — proxies, caches, retry "+
					"logic, load balancers, curl — can reason about your API without reading a page of your docs."),
			text("rest-pragmatic",
				"This guide takes the pragmatic middle the family guides share: design resources honestly, use "+
					"the uniform interface strictly where it is strict (methods, statuses, one error envelope), and "+
					"never invent ceremony the constraints do not demand. You will not find HATEOAS hypermedia "+
					"controls or content-type negotiation machinery here — those are real REST ideas that most "+
					"production JSON APIs deliberately skip. What you WILL find is the discipline that pays rent "+
					"on day one, summarized here and unpacked across the topic:"),
			list("rest-constraints", false,
				"Resources, not actions — the URL names a THING (a post, the posts collection); the method names the verb; /api/posts/delete?id=7 is a smell.",
				"Uniform interface — GET/POST/PUT/DELETE mean what RFC 9110 says they mean on every route; no endpoint redefines them.",
				"Stateless requests — every request carries everything it needs; no server-side session affinity hiding in the API's semantics.",
				"Representations — clients exchange JSON representations of resources; the resource is the idea, the JSON is its current shape.",
				"Honest statuses — 2xx success, 4xx the caller's fault, 5xx the server's; the status is the contract's first word.",
				"One error envelope — every failure wears the same machine-parseable shape, so error handling is written once."),
		),
		// ── 2. Designing resources ────────────────────────────────────
		section("designing-resources", "Designing resources",
			"A resource is anything you can name and give a lifecycle — and naming it well is half the API.",
			text("res-pitch",
				"Resource design starts with a question that sounds soft and is not: what are the THINGS in this "+
					"domain? A blog-like guide has posts; a shop has carts and products and orders; a build system "+
					"has pipelines and runs. A resource is one of those things, identified by a URL, exchanged as a "+
					"representation (here, JSON). Getting the inventory right matters more than any URL detail, "+
					"because the resources become your nouns, your storage shape, and eventually your team's "+
					"vocabulary — engineers who say \"the posts collection\" are speaking the API's model of "+
					"reality. This topic ships a real one: the guide now has an actual posts resource, "+
					"``/api/posts``, and every claim from here on is checked against its code."),
			text("res-entity",
				"The entity in the domain layer is the resource's honest definition — everything a post IS, with "+
					"no wire keys, no HTML, no storage schema. It is deliberately boring, and boring is the "+
					"achievement: identity (``ID``), content (``Title``, ``Body``), classification (``Tags``), and "+
					"lifecycle stamps (``CreatedAt``, ``UpdatedAt``) that say when it came to be and when it last "+
					"changed:"),
			code("res-entity-code", "go",
				"The resource entity — domain types only, no wire concerns.",
				"server/internal/domain/posts/post.go",
				"type Post struct {\n"+
					"    ID        string\n"+
					"    Title     string\n"+
					"    Body      string\n"+
					"    Tags      []string\n"+
					"    CreatedAt time.Time\n"+
					"    UpdatedAt time.Time\n"+
					"}"),
			text("res-entity-read",
				"Three decisions in that shape deserve words. ``Tags`` is a slice of strings, not a Tag entity — "+
					"a tag here is a label, has no lifecycle of its own, and promoting it to a resource would be "+
					"design by ceremony (when tags gain state — counters, owners, descriptions — they earn their "+
					"own resource and endpoints). The timestamps are ``time.Time``, the domain's type for an "+
					"instant; the RFC 3339 string on the wire is a representation detail the JSON encoder handles "+
					"for free. And the struct carries no JSON tags at all: the WIRE shape lives in the "+
					"presentation layer's response types, so changing a JSON key is a presentation change, never "+
					"a domain rewrite — the seam that keeps storage, domain, and contract independently evolvable."),
			text("res-naming",
				"Naming has one hard rule and a pile of habits. The hard rule: a resource URL names a thing, so it "+
					"reads as a noun — ``/api/posts`` (the collection) and ``/api/posts/{id}`` (one item). The "+
					"habits: plural collections (the family convention — the collection at ``/api/posts`` holds "+
					"many posts, and ``POST /api/posts`` reads \"add one to the collection\"); lowercase kebab "+
					"where a name needs words (``/api/posts/{id}/revisions`` if revisions ever ship); no verbs, "+
					"no file extensions, no query strings doing the router's job. When an action is genuinely not "+
					"CRUD — \"publish this post\", \"resend the webhook\" — it is honest to design it as either a "+
					"state change (``PUT`` the post with ``published: true`` once that field exists) or a "+
					"controller sub-resource named with a verb (``POST /api/posts/{id}/publish``), and to choose "+
					"deliberately rather than by accident."),
		),
		// ── 3. Methods and status codes ──────────────────────────────
		section("methods-and-status", "Methods and status codes",
			"The uniform interface: five methods, five statuses that matter, and one DELETE decision made in the open.",
			text("mns-pitch",
				"HTTP methods come with CONTRACTS, not conventions. GET is safe — it reads, changes nothing, and "+
					"may be retried, pre-fetched, or cached without consequence. PUT is a full-replace and "+
					"idempotent — send it twice and the resource is in the same state. DELETE removes and is "+
					"idempotent in the sense that the END state (gone) survives repetition, though what you ANSWER "+
					"on the second call is a design decision this section makes explicitly. POST is neither safe "+
					"nor idempotent — it creates, and each call may create again. Those contracts are load-bearing: "+
					"middleware, proxies, and clients branch on them. The posts surface honors them with exactly "+
					"five routes:"),
			list("mns-route-map", false,
				"GET /api/posts — read one page of the collection; safe; 200 with the page envelope, 422 if the query breaks a rule.",
				"POST /api/posts — create from a full draft body; 201 with the stored post and a Location header naming it.",
				"GET /api/posts/{id} — read one post; 200, or the 404 envelope when no such post exists.",
				"PUT /api/posts/{id} — full replacement (the body is the whole next state); 200, 404 on a miss, 422 on a broken draft.",
				"DELETE /api/posts/{id} — remove; 204 with no body, or 404 under this API's strict stance."),
			text("mns-status-pitch",
				"Status codes are the contract's vocabulary, and the mistake to avoid is the two-code API "+
					"(everything is 200, details in the body). The family discipline maps outcomes onto statuses "+
					"mechanically: success is 200 (or 201 with Location on create, 204 on a bodyless delete), "+
					"the caller's mistake is a 4xx the caller can fix (422 a broken body or query, 404 a missing "+
					"resource, 405 a wrong verb — with an ``Allow`` header the router already sends), and the "+
					"server's failure is a 5xx the caller can only retry or report. Every one of those answers "+
					"rides the same JSON error envelope, so the status and the body never disagree about what "+
					"happened. What this API deliberately does NOT have yet is 409 conflict — the taxonomy "+
					"carries it for the day a unique key or optimistic-concurrency check exists, and inventing a "+
					"conflict now, with nothing to conflict over, would be decoration."),
			text("mns-delete-decision",
				"One decision deserves to be made in the open because real APIs disagree on it: what does "+
					"``DELETE`` answer the SECOND time, when the post is already gone? The idempotent camp "+
					"answers 204 forever — the end state is what matters, retries are free, and a monitor "+
					"re-running an old playbook meets no errors. The strict camp — this API — answers 404: the "+
					"response reports the world as it is NOW, a caller deleting something that is not there has "+
					"a stale id or a bug, and a silent 204 swallows exactly the signal that would surface it. "+
					"The repository says so in code, and the workbench card records the alternative so the "+
					"trade is taught, not hidden:"),
			code("mns-delete-code", "go",
				"The strict DELETE stance, in the adapter that owns the storage truth.",
				"server/internal/data/posts/inmemory.go",
				"func (r *Repository) Delete(_ context.Context, id string) error {\n"+
					"    r.mu.Lock()\n"+
					"    defer r.mu.Unlock()\n"+
					"    if _, ok := r.store[id]; !ok {\n"+
					"        return core.Newf(core.KindNotFound, \"Post %q not found.\", id)\n"+
					"    }\n"+
					"    delete(r.store, id)\n"+
					"    return nil\n"+
					"}"),
			text("mns-put-semantics",
				"PUT's full-replace semantics deserve the same explicitness because \"partial PUT\" is a "+
					"spreading habit. Here, the request body IS the next state of the post: every field you send "+
					"is written, every field you omit is gone — omit ``tags`` and the post ends with none. That "+
					"is what makes PUT idempotent and cacheable: the same body yields the same end state, no "+
					"merging rules to document. When partial update is genuinely needed it gets its own method — "+
					"PATCH with present-only semantics, exactly like the scenario store's patch handler from the "+
					"validation topic — rather than a PUT that sometimes means \"replace\" and sometimes "+
					"\"merge\". One verb, one meaning; if you need the other meaning, that is a different verb."),
		),
		// ── 4. URLs, pagination, and filtering ───────────────────────
		section("urls-pagination-filtering", "URLs, pagination, and filtering",
			"Identity lives in the path; everything that selects a view of the collection lives in the query string.",
			text("url-pitch",
				"The split between path and query is a real design rule, not formatting taste. The PATH carries "+
					"identity — which resource, out here in the world, this request is about: "+
					"``/api/posts/p-3``. The QUERY STRING carries the parameters of a view — everything that "+
					"selects, sorts, or slices without changing WHICH collection is being asked about: page, "+
					"limit, sort, and filters like ``tag``. You can see the rule in a single honest URL: "+
					"``/api/posts?tag=go&sort=oldest&page=2&limit=4`` asks one question (the posts collection) "+
					"with four dials. The rule also explains what does NOT belong in URLs: verbs (the method owns "+
					"the verb), session identity (a header's job), and API keys (a header's job for a reason — "+
					"URLs end up in logs, browser history, and referrers, and secrets must not ride in them)."),
			text("page-pitch",
				"Any collection that can grow needs pagination designed in from the first commit, and the "+
					"design has three parts: the parameters, the envelope, and the math. The parameters here are "+
					"``page`` (1-based) and ``limit`` (1–50, default 10), with ``sort`` from a closed vocabulary "+
					"— ``newest`` (the default), ``oldest``, or ``title``. Closed vocabularies are deliberate: "+
					"an unknown sort is a 422 whose details teach the legal set, exactly like the scenario "+
					"store's enums, instead of a silent fallback to a default the caller never asked for. The "+
					"boundary rules live in the domain, where the list use case normalizes whatever strings the "+
					"query string carried:"),
			code("page-normalize-code", "go",
				"Query-string strings become a normalized, validated ListQuery — or one 422 naming every violation.",
				"server/internal/domain/posts/usecases.go",
				"if page == \"\" {\n"+
					"    normalized.Page = 1\n"+
					"} else if parsed, err := strconv.Atoi(page); err != nil || parsed < 1 {\n"+
					"    issues = append(issues, Issue{\n"+
					"        Field: \"page\",\n"+
					"        Rule:  \"an integer ≥ 1 when sent; omit for the first page\",\n"+
					"        Got:   strconv.Quote(page),\n"+
					"    })\n"+
					"} else {\n"+
					"    normalized.Page = parsed\n"+
					"}"),
			text("page-math",
				"The math is small enough to hold in your head and still easy to get subtly wrong, so say it "+
					"out loud. ``totalItems`` is the count of everything matching the filter — 6 seed posts, or "+
					"4 when ``tag=go`` matches four of them. ``totalPages`` is the ceiling of the division: "+
					"``(total + limit - 1) / limit`` — 6 items at limit 4 is 2 pages, with page 2 holding the "+
					"partial remainder of 2. The window for page p is the half-open slice from "+
					"``(p-1) * limit`` to that plus ``limit``, clamped to the collection:"),
			code("page-math-code", "go",
				"The whole page arithmetic: totals, ceiling division, a clamped half-open window.",
				"server/internal/data/posts/inmemory.go",
				"total := len(filtered)\n"+
					"totalPages := (total + query.Limit - 1) / query.Limit\n"+
					"start := (query.Page - 1) * query.Limit\n"+
					"end := start + query.Limit\n"+
					"// …start and end clamp to the collection before slicing\n"+
					"items := make([]posts.Post, 0, end-start)\n"+
					"items = append(items, filtered[start:end]...)"),
			text("page-envelope",
				"The list response is an ENVELOPE, not a bare array — the habit that separates paginated APIs "+
					"from ones that paint themselves into a corner. A bare ``[]`` cannot grow metadata without "+
					"breaking every client's parser; the envelope can add a field tomorrow. This API's page "+
					"carries ``items`` (never null — an empty page renders ``[]``) beside ``page``, ``limit``, "+
					"``totalItems``, ``totalPages``, ``hasNextPage``, and ``hasPrevPage`` — everything a UI "+
					"needs to render honest navigation without a second round trip. And the awkward case is "+
					"specified rather than left to chance: a page PAST the end (page 9 of 2) is a valid 200 "+
					"whose ``items`` is empty and whose totals still tell the truth about where the data "+
					"ended. It is not a 404 — the collection exists; the window is simply empty — and it is "+
					"not an error, because \"you scrolled past the end\" is a normal race in any list being "+
					"actively deleted from."),
			text("filter-pitch",
				"Filtering follows the same honesty rules. ``tag=go`` is an EXACT membership test against the "+
					"post's tags — not a search, not a substring, and case-sensitive with confidence because "+
					"tags are normalized on write (trimmed, lowercased) by the draft rules from the validation "+
					"topic. Exact-match filters compose with pagination cleanly (``totalItems`` reflects the "+
					"filter, so the page numbers are pages OF THE FILTERED SET), and a filter that matches "+
					"nothing is an empty 200 page, not a 404 — nothing was missed; everything was correctly "+
					"excluded. Search — ranked, tokenized, fuzzy — is a different feature with different "+
					"machinery (the databases topics build one with PostgreSQL's full-text search), and "+
					"pretending ``?q=`` over a filtered scan is search is how APIs accumulate lie-shaped "+
					"query parameters that must never be removed again."),
		),
		// ── 5. Error responses ───────────────────────────────────────
		section("error-responses", "Error responses: one envelope, and RFC 9457 beside it",
			"Every failure wears the same machine-parseable shape — recorded once, compared to the standard honestly.",
			text("err-pitch",
				"An API's error responses are its most-read documentation, because they are what developers "+
					"meet while integrating at 2 a.m. The family rule this repo records in its wire contract: "+
					"ONE envelope for every failure, produced by ONE central writer, fed by a typed error "+
					"taxonomy. A handler never formats an error body; it returns a typed error and the writer "+
					"shapes it. The recorded shape on the wire:"),
			code("err-body-code", "go",
				"The recorded envelope body: camelCase, details omitted when absent — never null.",
				"server/internal/core/envelope.go",
				"type errorBody struct {\n"+
					"    Code      string `json:\"code\"`\n"+
					"    Message   string `json:\"message\"`\n"+
					"    RequestID string `json:\"requestId\"`\n"+
					"    Details   any    `json:\"details,omitempty\"`\n"+
					"}"),
			text("err-anatomy",
				"Each member earns its place. ``code`` is the taxonomy bucket — a CLOSED set ("+
					"``not_found``, ``validation_failed``, ``conflict``, and five more) that clients branch "+
					"on, never a free-form word. ``message`` is one human sentence, safe to show any user; "+
					"internals ride the logs and never the body. ``requestId`` matches the "+
					"``x-request-id`` response header and the server's warn-level log line — the join key "+
					"that turns \"it failed\" into a debugging session with one grep. ``details`` is "+
					"structured context — for the posts API, either the decoder's reason under ``body`` or "+
					"the issue list (field, rule, got) naming every broken field — and it is omitted when "+
					"absent, never null, so clients test presence once and trust it. On the posts routes "+
					"the 422s carry the draft's collected issues:"),
			code("err-issues-code", "go",
				"Validation failures collect every issue — one 422 teaches the whole contract.",
				"server/internal/domain/posts/post.go",
				"func (d Draft) ValidateDraft() []Issue {\n"+
					"    var issues []Issue\n"+
					"    switch {\n"+
					"    case d.Title == \"\":\n"+
					"        issues = append(issues, Issue{\n"+
					"            Field: \"title\",\n"+
					"            Rule:  \"required — present-but-empty is data, and data can be wrong\",\n"+
					"            Got:   strconv.Quote(d.Title),\n"+
					"        })\n"+
					"    // …the length, body, and tag branches follow the same shape\n"+
					"    }\n"+
					"    return issues\n"+
					"}"),
			text("err-rfc9457",
				"Standards people will now reasonably ask: why not RFC 9457 — \"Problem Details for HTTP "+
					"APIs\", the ``application/problem+json`` media type? The honest answer: this envelope "+
					"and problem+json agree on the bones, and differ on three deliberate choices. RFC 9457 "+
					"specifies members ``type`` (a URI identifying the problem class), ``title`` (a short "+
					"human summary), ``status`` (the status again, for the proxy-rewrites-the-status case), "+
					"``detail`` (this occurrence's explanation), and ``instance`` (the specific occurrence's "+
					"URI), plus extension members. The recorded envelope maps onto it easily — ``code`` "+
					"plays ``type``, ``message`` plays ``title``-plus-``detail``, ``details`` plays "+
					"extensions — and if you must interoperate with problem+json consumers, a translation "+
					"at the writer is a day's work, which is exactly WHY one central writer exists. The "+
					"differences that matter:"),
			list("err-9457-comparison", false,
				"requestId — the recorded envelope's join key for logs and support conversations; problem+json has no such member, and a distributed system needs one.",
				"Closure — code is a small closed set the whole app shares, where type is an open URI space that drifts toward a-thousand-strings-that-all-mean-the-same-thing.",
				"One shape everywhere — 401s, 422s, and 500s all wear the envelope; problem+json is usually deployed per-endpoint and unevenly.",
				"What it borrows anyway — machine-readable class first, human sentence second, status aligned with the real status code; the bones ARE 9457's bones.",
				"What it gives up — no per-problem type URIs to dereference, and no instance member; for this API's scale, the request id covers the occurrence-identity need."),
			text("err-stance",
				"The teachable stance is neither \"our envelope is better\" nor \"adopt the RFC\" — it is that "+
					"CONSISTENCY beats provenance. An internally-consistent envelope every endpoint honors "+
					"serves clients better than a standard applied sometimes; a standard adopted fully serves "+
					"better than either when interoperation demands it. What is indefensible is the default "+
					"state of many APIs: each route inventing its error shape, statuses and bodies "+
					"disagreeing, and the \"errors\" documentation section describing an aspiration. This "+
					"repo records its envelope once, in the wire contract, and the snippet ratchet keeps "+
					"every quoted error body honest against the one writer that produces them."),
		),
		// ── 6. Versioning ────────────────────────────────────────────
		section("versioning", "Versioning",
			"Version the day you break a client you do not control — and not one day before.",
			text("ver-pitch",
				"Versioning is change management for the contract, and the first honest observation is that "+
					"most APIs version too early and too hard. A version is a PROMISE: as long as v1 lives, "+
					"its clients keep working — which means every version you maintain is a branch of your "+
					"behavior you must test, document, and eventually retire. The trigger for taking on that "+
					"cost is specific: you are about to ship a change that breaks a client you do not "+
					"control, and you cannot coordinate the migration. Additive changes — a new optional "+
					"field, a new route, a new error code in a set clients are told to handle as a set — "+
					"break nobody and version nothing. The family's optionals-omitted-never-null rule is "+
					"quietly a versioning policy: it is what lets the server ADD fields without a bump."),
			text("ver-strategies",
				"When the moment does come, the strategies are few and the tradeoffs are known. Path major "+
					"(``/api/v2/posts``) is the loudest and the most debuggable — the version is visible in "+
					"every log line and every curl — at the cost of doubling your route surface per version. "+
					"A request header (``Accept: application/vnd.gdg+json;version=2`` or a custom "+
					"``X-Api-Version``) keeps URLs stable but hides the version from logs and makes "+
					"reproducing a customer's call harder. A media-type or content negotiation scheme is "+
					"the most REST-correct and the most invisible. A ``version`` FIELD in the body is the "+
					"gentlest — no routing changes, the client reads what it got — which is exactly what "+
					"this guide's own content API does, and it is a real choice with a real limitation: it "+
					"can describe what came back, but it cannot route two different behaviors for the same "+
					"URL:"),
			code("ver-content-code", "go",
				"This repo's gentle version: a field on every content response, not a URL segment.",
				"server/internal/content/model.go",
				"// ContentAPIVersion is served on both content endpoints so a client can\n"+
					"// detect a wire-shape bump and migrate deliberately. Bump only with a\n"+
					"// deliberate contract change.\n"+
					"const ContentAPIVersion = 1"),
			text("ver-stance",
				"The posts API's recorded stance: an unversioned ``/api`` prefix today, and the day a "+
					"breaking change must ship, path major — ``/api/v2/posts`` — with v1 kept alive for a "+
					"deprecation window and sunset dates communicated through documentation (and, when the "+
					"deployment topic adds headers, the ``Deprecation`` and ``Sunset`` headers the platform "+
					"world is converging on). That stance is on the workbench's contract card beside the "+
					"routes it governs, because an API's versioning policy is part of its contract, not a "+
					"footnote: clients decide how much to invest in tracking your changes by how honestly "+
					"you tell them the policy up front."),
		),
		// ── 7. The whole pipeline ────────────────────────────────────
		section("the-whole-pipeline", "The whole pipeline",
			"One request, every layer: middleware → route → thin handler → use case → port → adapter — wired at the composition root.",
			text("pipe-pitch",
				"Everything this topic teaches composes into one request path, and the posts API is the "+
					"first surface in the guide where you can watch ALL of it at once. A ``POST /api/posts`` "+
					"arrives; the middleware stack stamps a request id, logs, and guards against panics; "+
					"the ServeMux pattern matches the method and path to exactly one handler; the handler "+
					"decodes and shape-checks the body (the validation topic's discipline) and calls ONE "+
					"use case; the use case normalizes and validates the draft, then calls the repository "+
					"PORT — an interface it owns; the adapter behind it — the in-memory fake or the live "+
					"PostgreSQL, chosen per call by the databases topic's switching facade and flippable "+
					"at runtime through the scenario endpoint — stores the post and stamps its identity "+
					"and timestamps; the result flows back up and the handler writes 201, a ``Location`` "+
					"header, and the stored post. The wiring that makes this real is one block in the "+
					"composition root — Go's answer to dependency injection, constructor by constructor:"),
			code("pipe-main-code", "go",
				"The composition root: one port behind the switching facade, five use cases, five thin handlers, five routes.",
				"server/cmd/guide/main.go",
				"postsRepo := postsdata.NewSwitchingRepository(\n"+
					"    scenarioStore,\n"+
					"    postsdata.NewInMemoryRepository(),\n"+
					"    postsdata.NewPostgresRepository(pool),\n"+
					")\n"+
					"postsList := posts.NewListPostsUseCase(postsRepo)\n"+
					"postsCreate := posts.NewCreatePostUseCase(postsRepo)\n"+
					"// …the get, replace, and delete use cases wire identically over the same port\n"+
					"mux.Handle(\"GET /api/posts\", presentation.NewPostsListHandler(postsList, logger))\n"+
					"mux.Handle(\"POST /api/posts\", presentation.NewPostsCreateHandler(postsCreate, logger))\n"+
					"mux.Handle(\"GET /api/posts/{id}\", presentation.NewPostDetailHandler(postsGet, logger))\n"+
					"mux.Handle(\"PUT /api/posts/{id}\", presentation.NewPostReplaceHandler(postsReplace, logger))\n"+
					"mux.Handle(\"DELETE /api/posts/{id}\", presentation.NewPostDeleteHandler(postsDelete, logger))"),
			text("pipe-thin",
				"Read the create handler and count the jobs it does NOT have: no rule checking, no storage "+
					"knowledge, no status arithmetic beyond its own two answers, no error formatting. It "+
					"parses, delegates, and shapes — the THIN handler discipline the architecture topic "+
					"will generalize. Every failure path is one line, ``core.WriteError``, and that one "+
					"line is why every error on every route wears the same envelope. The success path is "+
					"four lines, and one of them is the contract detail beginners forget — the "+
					"``Location`` header that turns a 201 from \"it worked\" into \"here is where it "+
					"lives\":"),
			code("pipe-create-code", "go",
				"The whole create handler: decode, delegate, 201 + Location. Nothing else.",
				"server/internal/presentation/posts.go",
				"func (h *PostsCreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"    draft, err := decodePostDraft(w, r)\n"+
					"    if err != nil {\n"+
					"        core.WriteError(w, r, err, h.logger)\n"+
					"        return\n"+
					"    }\n"+
					"    post, err := h.create.Execute(r.Context(), draft)\n"+
					"    if err != nil {\n"+
					"        core.WriteError(w, r, err, h.logger)\n"+
					"        return\n"+
					"    }\n"+
					"    w.Header().Set(\"Location\", \"/api/posts/\"+post.ID)\n"+
					"    writeJSON(w, http.StatusCreated, newPostResponse(post))\n"+
					"}"),
			text("pipe-demo-intro",
				"The demo below puts the whole pipeline in your hands. The workbench is not a mock-up: "+
					"every button drives the LIVE ``/api/posts`` routes through the client's own domain "+
					"pipeline (repository port, use cases, container wiring — the same layering, mirrored "+
					"in TypeScript). List, paginate, filter, and sort the collection; create a post and "+
					"watch the 201 land; edit one with a full PUT replacement; delete one and watch the "+
					"next read answer the 404 envelope. Errors render honestly — break a rule on purpose "+
					"(empty title, a 50-item limit) and the workbench shows the server's own envelope, "+
					"code and message and all. The contract card beside it is served from the same "+
					"server, generated from the real route table:"),
			demo("api:posts-workbench", "The posts workbench",
				"A live CRUD workbench over the real /api/posts resource — list/paginate/filter/sort, "+
					"create, full-replace edit, delete — plus the API's contract card: routes with their "+
					"success and error cases, pagination rules, versioning stance, delete semantics, and "+
					"the error-envelope anatomy with RFC 9457 held up for comparison."),
			text("pipe-demo-read",
				"Read the card and the workbench together and you have this topic in one screen. The "+
					"route table row you are exercising is highlighted by what you do: the list controls "+
					"demonstrate the page envelope's honest metadata (empty page past the end included), "+
					"the create and edit forms demonstrate strict decoding and the issue-carrying 422s, "+
					"and delete demonstrates the strict 404 stance the card records with its rationale. "+
					"Nothing on the screen is staged — the same binary serves the card, the routes, and "+
					"this page's prose, which is the guide's whole trick: the teaching IS the product."),
		),
		// ── 8. Terminal companion ────────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl tour",
			"A full CRUD round-trip from the shell — create, read, list, replace, delete, and the honest errors between.",
			text("term-pitch",
				"With the stack up (server on :8100), this tour walks the entire contract with ``curl`` — "+
					"the same round-trip the workbench drives, one status code at a time. You will see the "+
					"201 and its ``Location`` header, the page envelope under filtering and pagination, a "+
					"full-replace PUT, the strict-stance DELETE, and the two error shapes every client "+
					"must handle: the 404 envelope on a miss and the issue-carrying 422 on a broken body. "+
					"``python3 -m json.tool`` pretty-prints with zero extra tooling; ``curl -si`` shows "+
					"status AND headers:"),
			code("term-create", "bash", "", "",
				"# create: 201 + Location naming the new post (first created id is p-7)\n"+
					"curl -si -X POST localhost:8100/api/posts \\\n"+
					"  -H 'Content-Type: application/json' \\\n"+
					"  -d '{\"title\":\"A post from the shell\",\"body\":\"Written by curl, stored by Go.\",\"tags\":[\"api\",\"demo\"]}'"),
			code("term-read", "bash", "", "",
				"# read it back at exactly the URL the Location header promised\n"+
					"curl -s localhost:8100/api/posts/p-7 | python3 -m json.tool"),
			code("term-list", "bash", "", "",
				"# list with filtering + pagination + sort: the page envelope, honest metadata\n"+
					"curl -s 'localhost:8100/api/posts?tag=api&sort=oldest&page=1&limit=2' | python3 -m json.tool"),
			code("term-empty-page", "bash", "", "",
				"# page 9 of 2: a valid EMPTY page (items: []) — totals still tell the truth\n"+
					"curl -s 'localhost:8100/api/posts?page=9&limit=4' | python3 -m json.tool"),
			code("term-put", "bash", "", "",
				"# full replace: the body IS the next state — old tags are gone, not merged\n"+
					"curl -si -X PUT localhost:8100/api/posts/p-7 \\\n"+
					"  -H 'Content-Type: application/json' \\\n"+
					"  -d '{\"title\":\"Replaced from the shell\",\"body\":\"The whole next state.\",\"tags\":[\"edited\"]}'"),
			code("term-422", "bash", "", "",
				"# the issue-carrying 422: empty title AND empty body, both collected, both named\n"+
					"curl -si -X POST localhost:8100/api/posts \\\n"+
					"  -H 'Content-Type: application/json' -d '{\"title\":\"\",\"body\":\"\"}'"),
			code("term-delete", "bash", "", "",
				"# delete: 204, no body — then the strict stance on the SECOND call\n"+
					"curl -si -X DELETE localhost:8100/api/posts/p-7\n"+
					"curl -si -X DELETE localhost:8100/api/posts/p-7"),
			code("term-404", "bash", "", "",
				"# the 404 envelope after deletion: same shape as every other failure\n"+
					"curl -si localhost:8100/api/posts/p-7"),
			code("term-card", "bash", "", "",
				"# the workbench's contract card, served from the same binary as the routes\n"+
					"curl -s localhost:8100/api/demo/api/posts-workbench | python3 -m json.tool | head -40"),
			text("term-coda",
				"Notice what the tour never shows: a 200 with an error in the body, an unexplained 500, or "+
					"two different error shapes. Every answer had one envelope, every status told the "+
					"truth, and every miss named its request id — the contract this topic built, held by "+
					"convention and by the central writer rather than by hope. The next topics build "+
					"upward from this substrate: the documentation topic will describe these exact routes "+
					"from a machine-readable spec, and GraphQL will pose the first honest alternative to "+
					"resource URLs. The posts API is the ground they stand on."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"api-design",
		"API design",
		"Resource-oriented HTTP on a real posts API: resources, methods, statuses, pagination, the recorded "+
			"error envelope beside RFC 9457, versioning, and the whole pipeline — live at /api/posts.",
		"apis",
		11,
		sections,
	)
}
