package content

import (
	"errors"
	"fmt"
)

// This file is topic 13 graphql (group apis, curriculum order 13),
// authored on the exemplar pattern: prose is the default block, every
// must-source code block quotes a real repo file (the snippet ratchet
// verifies it), the live demo sits INLINE in the section that teaches
// introspection and tooling, and the topic closes with the bash terminal
// companion — a curl tour of POST /api/graphql with -d '{"query":"…"}'.

// graphqlTopic builds topic 13 — the posts resource served through one
// endpoint and a query language: schema-first thinking, gqlgen codegen,
// queries and mutations over the same use cases the REST surface calls,
// GraphQL errors carrying the typed taxonomy as extensions codes, the N+1
// problem measured by an instrumented port and beaten by a hand-rolled
// batch loader, and introspection as the contract's own documentation.
func graphqlTopic() (Topic, error) {
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
		// ── 1. Schema-first thinking ────────────────────────────────
		section("schema-first-thinking", "Schema-first thinking",
			"GraphQL turns the API contract into a typed, queryable type system — and the schema, not the handler, becomes the thing you author first.",
			text("gql-pitch",
				"GraphQL is three things stacked: a query language clients write, a type system that "+
					"describes everything the API can return, and an execution engine that walks a query "+
					"against that type system and produces exactly the shape asked for. Instead of five "+
					"routes with five fixed shapes, there is one endpoint — here ``POST /api/graphql`` — "+
					"that accepts a document naming the fields you want, and the response mirrors the "+
					"request's shape. Ask for ``{ posts(limit: 2) { items { title } } }`` and you receive "+
					"two titles; ask for ``tags`` too and they appear; ask a phone-sized view for a "+
					"slow connection and a desktop view from the same schema. The type system is the "+
					"entire contract: if a field exists in the schema, a client may select it, and if "+
					"the server changes the schema, every client learns at the next introspection."),
			text("gql-vs-rest",
				"The honest framing against topic 11's REST surface: GraphQL is not a replacement, it "+
					"is a different contract SHAPE for the same domain — and this guide runs both "+
					"against the same posts use cases to prove it. REST gives resources stable URLs, "+
					"HTTP caching, and a verb vocabulary every proxy understands; GraphQL gives "+
					"client-shaped reads, one round trip instead of waterfall fetches, and a typed "+
					"contract tooling can read at runtime. The costs are just as real: queries are "+
					"harder to cache (the URL is always the same), per-field resolution invites the "+
					"N+1 problem this topic measures and fixes, and the single endpoint concentrates "+
					"complexity that REST spreads across routes. Teams that ship both usually gate "+
					"GraphQL behind a genuine need — heterogeneous clients, deep view trees, or "+
					"federated graphs — and let plain REST carry the long tail."),
			text("gql-when",
				"So when does the graph earn its keep? The pattern that repeats across the industry "+
					"breaks into a few crisp cases, and the decision is usually visible in the "+
					"CLIENT population long before it is visible in the server:"),
			list("gql-when-list", false,
				"Client-shaped reads — many consumers want different slices of the same entity (mobile trims fields, dashboards widen them), and fixed response shapes serve none of them well.",
				"One round trip — a screen needs the page, each item's relations, and a lookup table; REST answers in a waterfall, the graph answers in one document.",
				"Typed contract — the schema IS the documentation, and codegen on both ends derives client types from it; drift becomes a compile error.",
				"Rapid client iteration — new screens select fields that already exist; no server release is needed for a new view of old data.",
				"Federated surface — many teams own one graph stitched behind one endpoint (Apollo Federation's whole pitch); REST composes poorly at the URL level.",
				"The costs you accept — response caching by URL is gone, queries need depth/complexity limits, and per-field resolution reintroduces the N+1 problem unless you batch."),
			text("gql-schema-first",
				"The Go-idiomatic path through this is SCHEMA-FIRST, and it inverts the habit REST "+
					"development builds. You author the contract as schema language first — types, "+
					"fields, nullability, arguments — and a code generator derives the server's "+
					"skeleton: the typed models, the resolver interfaces your code must satisfy, and "+
					"the executor that parses, validates, and walks operations. The alternative "+
					"approach, schema-LAST (graphql-go's style: build Go structs, derive the schema "+
					"from them via annotations), couples the contract to the implementation's shape "+
					"and makes reviewing the API a code-reading exercise. This guide uses gqlgen — "+
					"the schema-first generator the Go community converged on — pinned at "+
					"v0.17.98, and the schema for the posts resource mirrors the REST wire "+
					"field-for-field on purpose: same entity, same envelope, second contract."),
		),
		// ── 2. Schema and resolvers ─────────────────────────────────
		section("schema-and-resolvers", "Schema and resolvers: gqlgen codegen",
			"Author the schema, run the generator, satisfy the interfaces — the contract is a file, the server is a checklist.",
			text("schema-types",
				"Start with the domain types. The schema language writes them as ``type`` blocks "+
					"with one field per line, an exclamation mark meaning non-null, and square "+
					"brackets meaning lists. Two decisions below are load-bearing. ``tags: "+
					"[String!]!`` — the LIST is non-null AND its elements are non-null — mirrors "+
					"the REST wire's materialized ``[]``: present-but-empty is data, and null is a "+
					"state this API never produces. And ``PostPage`` is not an afterthought: it is "+
					"the REST list envelope transcribed field-for-field, so a client that paginates "+
					"the graph paginates identically to one that paginates REST — ``totalItems`` "+
					"and ``hasNextPage`` mean exactly what topic 11 promised:"),
			code("schema-types-code", "graphql",
				"The posts wire, transcribed into schema language.",
				"server/internal/graph/schema/posts.graphqls",
				"type Post {\n"+
					"  id: ID!\n"+
					"  title: String!\n"+
					"  body: String!\n"+
					"  tags: [String!]!\n"+
					"  createdAt: Time!\n"+
					"  updatedAt: Time!\n"+
					"}\n"+
					"\n"+
					"type PostPage {\n"+
					"  items: [Post!]!\n"+
					"  page: Int!\n"+
					"  limit: Int!\n"+
					"  totalItems: Int!\n"+
					"  totalPages: Int!\n"+
					"  hasNextPage: Boolean!\n"+
					"  hasPrevPage: Boolean!\n"+
					"}"),
			text("schema-roots",
				"Then the roots. Every GraphQL schema has a ``Query`` type (the read surface) and "+
					"may have a ``Mutation`` type (the write surface); each field on a root is an "+
					"operation. The arguments below deserve a close read because they encode this "+
					"API's rules in TYPE, before any code runs: ``page``, ``limit``, ``tag``, and "+
					"``sort`` are all OPTIONAL (omission means the REST defaults), and the "+
					"nullability of RETURN types is a teaching decision — ``posts`` is "+
					"``PostPage!`` (a list always answers), but ``post(id)`` is nullable because "+
					"a miss is data about the world, answered as null plus an errors entry. The "+
					"mutations follow the substrate exactly: full-replacement ``updatePost`` with "+
					"PUT semantics, and the strict ``deletePost`` that reports a vanished post "+
					"as an error, never a silent success:"),
			code("schema-roots-code", "graphql",
				"The operation vocabulary: two queries, three mutations.",
				"server/internal/graph/schema/posts.graphqls",
				"type Query {\n"+
					"  posts(page: Int, limit: Int, tag: String, sort: String): PostPage!\n"+
					"  post(id: ID!): Post\n"+
					"}\n"+
					"\n"+
					"type Mutation {\n"+
					"  createPost(input: PostInput!): Post!\n"+
					"  updatePost(id: ID!, input: PostInput!): Post\n"+
					"  deletePost(id: ID!): ID\n"+
					"}"),
			text("codegen-config",
				"The generator is driven by one YAML file — where the schema lives, where the "+
					"generated executor and models go, and where the hand-written resolvers sit. "+
					"The layout keeps every artifact inside ``internal/graph/``: the schema in its "+
					"own directory, generated code clearly separated (``generated/``, ``model/``) "+
					"and COMMITTED — generated files are build inputs like any other here, so a "+
					"clean checkout builds without the tool — and resolvers in their own package "+
					"where gqlgen drops stubs and never overwrites what you implement. "+
					"Regeneration is one recorded command from ``server/``: ``go tool gqlgen "+
					"generate --config internal/graph/gqlgen.yml`` — the tool directive in the "+
					"frozen go.mod pins v0.17.98, so every machine generates identical code:"),
			code("codegen-config-code", "yaml",
				"The whole codegen contract: inputs, outputs, package layout.",
				"server/internal/graph/gqlgen.yml",
				"schema:\n"+
					"  - internal/graph/schema/posts.graphqls\n"+
					"\n"+
					"exec:\n"+
					"  filename: internal/graph/generated/execution.go\n"+
					"  package: generated\n"+
					"\n"+
					"model:\n"+
					"  filename: internal/graph/model/models_gen.go\n"+
					"  package: model\n"+
					"\n"+
					"resolver:\n"+
					"  layout: follow-schema\n"+
					"  dir: internal/graph/resolver\n"+
					"  package: resolver\n"+
					"  filename_template: \"{name}.resolvers.go\""),
			text("generated-interfaces",
				"Run the generator and it hands back the part schema-first development lives "+
					"on: typed INTERFACES your code must satisfy, derived from the schema you "+
					"authored. This is the generated executor demanding its implementations — "+
					"the same shape a ``PostsRepository`` port demands of its adapters, which is "+
					"why the layering feels familiar. Note how the schema's nullability became "+
					"Go types: optional arguments arrive as POINTERS (``nil`` = not sent), a "+
					"nullable field returns a pointer, a non-null one returns the value. The "+
					"generated models carry the camelCase JSON tags, and ``Time`` — declared a "+
					"scalar in the schema — bound to ``time.Time`` automatically from gqlgen's "+
					"built-in scalar table, marshaling as RFC 3339 exactly like the REST wire:"),
			code("generated-interfaces-code", "go",
				"What the codegen demands: one method per root field, types derived from the schema.",
				"server/internal/graph/generated/execution.go",
				"type MutationResolver interface {\n"+
					"\tCreatePost(ctx context.Context, input model.PostInput) (*model.Post, error)\n"+
					"\tUpdatePost(ctx context.Context, id string, input model.PostInput) (*model.Post, error)\n"+
					"\tDeletePost(ctx context.Context, id string) (*string, error)\n"+
					"}\n"+
					"type QueryResolver interface {\n"+
					"\tPosts(ctx context.Context, page *int, limit *int, tag *string, sort *string) (*model.PostPage, error)\n"+
					"\tPost(ctx context.Context, id string) (*model.Post, error)\n"+
					"}"),
			text("resolver-root",
				"The resolver root is the one file humans own from the start: the dependency "+
					"object every generated method hangs off, holding exactly what the thin "+
					"handlers hold — use cases, never storage. The graph is one more "+
					"PRESENTATION of the same domain, so the dependency rule holds graph-side "+
					"too: resolvers call verbs, adapters call stores, and the composition root "+
					"is the only place the two meet. Pass the resolver the same use case "+
					"instances the REST handlers share and one in-memory store answers both "+
					"surfaces; pass it fresh ones over a scratch repository and it owns a "+
					"private world — which is precisely how this topic's demo runs its "+
					"mutations without touching the live posts substrate:"),
			code("resolver-root-code", "go",
				"The root dependency object: use cases in, generated calls out.",
				"server/internal/graph/resolver/resolver.go",
				"// Resolver is the root dependency object. It holds USE CASES, not storage:\n"+
					"// the graph is one more presentation of the same domain the REST handlers\n"+
					"// serve, so it asks the domain's verbs and never the repository — the\n"+
					"// dependency rule (presentation → domain ← data) holds graph-side too.\n"+
					"type Resolver struct {\n"+
					"\tlist    *posts.ListPostsUseCase\n"+
					"\tget     *posts.GetPostUseCase\n"+
					"\tcreate  *posts.CreatePostUseCase\n"+
					"\treplace *posts.ReplacePostUseCase\n"+
					"\tdelete  *posts.DeletePostUseCase\n"+
					"}"),
			text("resolver-methods",
				"Filling a stub is the thin-handler discipline again, one graph-shaped edition: "+
					"translate the typed GraphQL arguments, delegate to ONE use case, map the "+
					"result. The interesting seam below is the argument bridge — GraphQL hands "+
					"over ``*int`` (nil = not sent) while the domain's ``NormalizeListQuery`` "+
					"speaks query-string strings (\"\" = not sent), because it was written for "+
					"the REST surface. Rather than duplicate the rules, the resolver converts "+
					"representations and REUSES the one rulebook: page and limit bounds, the "+
					"closed sort vocabulary, and the issue-collecting rejection all stay in the "+
					"domain, shared by both surfaces — a ``limit: 500`` in a query and a "+
					"``?limit=500`` on the URL fail identically:"),
			code("resolver-methods-code", "go",
				"Two queries, implemented: bridge typed args, delegate, translate, map errors.",
				"server/internal/graph/resolver/posts.resolvers.go",
				"func (r *queryResolver) Posts(ctx context.Context, page *int, limit *int, tag *string, sort *string) (*model.PostPage, error) {\n"+
					"\t// GraphQL hands over TYPED optional arguments (nil = not sent); the\n"+
					"\t// domain's list normalizer speaks query-string strings (\"\" = not\n"+
					"\t// sent). The two bridges below keep ONE rulebook: page/limit bounds,\n"+
					"\t// the closed sort vocabulary, and the issue-collecting 422-equivalent\n"+
					"\t// all stay in NormalizeListQuery, shared with the REST surface.\n"+
					"\tquery, err := posts.NormalizeListQuery(\n"+
					"\t\targInt(page),\n"+
					"\t\targInt(limit),\n"+
					"\t\tstrings.TrimSpace(argString(tag)),\n"+
					"\t\targString(sort),\n"+
					"\t)\n"+
					"\tif err != nil {\n"+
					"\t\treturn nil, graph.MapError(err)\n"+
					"\t}\n"+
					"\tresult, err := r.list.Execute(ctx, query)\n"+
					"\tif err != nil {\n"+
					"\t\treturn nil, graph.MapError(err)\n"+
					"\t}\n"+
					"\treturn modelPage(result), nil\n"+
					"}\n"+
					"\n"+
					"func (r *queryResolver) Post(ctx context.Context, id string) (*model.Post, error) {\n"+
					"\tpost, err := r.get.Execute(ctx, id)\n"+
					"\tif err != nil {\n"+
					"\t\t// The field is nullable: the miss answers null for post(...) plus\n"+
					"\t\t// an errors entry carrying extensions.code \"not_found\" — GraphQL's\n"+
					"\t\t// way of saying 404 without a status code.\n"+
					"\t\treturn nil, graph.MapError(err)\n"+
					"\t}\n"+
					"\treturn modelPost(post), nil\n"+
					"}"),
		),
		// ── 3. Queries and mutations over the posts substrate ────────
		section("queries-and-mutations", "Queries and mutations over the posts substrate",
			"One execution path, two transports: the demo executes operations in-process, and POST /api/graphql serves the same path over HTTP.",
			text("exec-pitch",
				"Everything above composes into an execution path you can hold in one hand. "+
					"An operation arrives as text (plus optional variables); the executor PARSES "+
					"it into a document, VALIDATES it against the schema (unknown fields, wrong "+
					"argument types, malformed selections — rejected before any resolver runs), "+
					"then DISPATCHES it: each root field calls its resolver, each nested field "+
					"marshals through its generated accessor, and the response document is "+
					"assembled in the shape the query asked for. The helper below is that path "+
					"as one function — written once in ``internal/graph`` and used by BOTH "+
					"transports: the playground demo calls it directly (in-process, against a "+
					"scratch world), and the HTTP endpoint calls it per request. One behavior, "+
					"no drift between what the demo shows and what the endpoint serves:"),
			code("exec-code", "go",
				"The single execution path both transports share.",
				"server/internal/graph/execute.go",
				"func Execute(ctx context.Context, schema graphql.ExecutableSchema, request Request) *graphql.Response {\n"+
					"\t// The executor's stats want a trace start in the context (the library's\n"+
					"\t// HTTP server seeds it per request); ours is per operation.\n"+
					"\tctx = graphql.StartOperationTrace(ctx)\n"+
					"\tex := executor.New(schema)\n"+
					"\topCtx, list := ex.CreateOperationContext(ctx, &graphql.RawParams{\n"+
					"\t\tQuery:         request.Query,\n"+
					"\t\tOperationName: request.OperationName,\n"+
					"\t\tVariables:     request.Variables,\n"+
					"\t})\n"+
					"\tif len(list) != 0 {\n"+
					"\t\treturn ex.DispatchError(ctx, list)\n"+
					"\t}\n"+
					"\t// …introspection is re-enabled here, deliberately, per operation\n"+
					"\thandler, execCtx := ex.DispatchOperation(ctx, opCtx)\n"+
					"\tresponse := handler(execCtx)\n"+
					"\t// …a nil response degrades to a shaped error, never an empty body\n"+
					"\treturn response\n"+
					"}"),
			text("endpoint-pitch",
				"The HTTP endpoint is then a genuinely thin handler — but its thinness is worth "+
					"reading because GraphQL-over-HTTP has one decision REST never faces: WHEN "+
					"is a failure a status code? This endpoint's stance, taught in code: "+
					"transport-level failures (unreadable body, not one JSON object, no query) "+
					"answer 400 with a GraphQL-shaped errors body — the request was never an "+
					"operation; everything DURING execution rides a 200 with the ``{data, "+
					"errors}`` document, because GraphQL owns exactly one status and every "+
					"client is built to read the errors list, not the status line. The body "+
					"decodes straight into the library's own ``RawParams`` — ``{query, "+
					"operationName?, variables?}`` is the request contract — and the read is "+
					"bounded like every other body in this guide:"),
			code("endpoint-code", "go",
				"POST /api/graphql: bounded read, one document, the 400-vs-200 line drawn in code.",
				"server/internal/presentation/demo_graphql.go",
				"func (h *GraphQLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"\tbody, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGraphQLBodyBytes))\n"+
					"\tif err != nil {\n"+
					"\t\th.reject(w, r, http.StatusBadRequest, \"could not read request body\")\n"+
					"\t\treturn\n"+
					"\t}\n"+
					"\n"+
					"\t// The request body IS the library's own params shape: {query,\n"+
					"\t// operationName?, variables?} decode straight into RawParams.\n"+
					"\tvar params graphql.RawParams\n"+
					"\tif err := json.Unmarshal(body, &params); err != nil {\n"+
					"\t\th.reject(w, r, http.StatusBadRequest,\n"+
					"\t\t\t`request body must be one JSON object: {\"query\":\"…\",\"variables\":{…}}`)\n"+
					"\t\treturn\n"+
					"\t}\n"+
					"\tif strings.TrimSpace(params.Query) == \"\" {\n"+
					"\t\th.reject(w, r, http.StatusBadRequest, \"query is required\")\n"+
					"\t\treturn\n"+
					"\t}\n"+
					"\tparams.Headers = r.Header\n"+
					"\n"+
					"\tresponse := graph.Execute(r.Context(), h.schema, graph.Request{\n"+
					"\t\tQuery:         params.Query,\n"+
					"\t\tOperationName: params.OperationName,\n"+
					"\t\tVariables:     params.Variables,\n"+
					"\t})\n"+
					"\twriteJSON(w, http.StatusOK, response)\n"+
					"}"),
			text("wire-shape",
				"On the wire, a request is one POST whose body names the operation, and the "+
					"response mirrors the query's braces. A client asks for the first page of "+
					"two, oldest first, with titles only; the server answers the exact shape — "+
					"note ``data`` carries the selection verbatim and there are no unused "+
					"fields anywhere in the response. That shape-mirroring is the property "+
					"every GraphQL client is built around, and it is why response CACHING is "+
					"the hard part: the URL never changes, so the document is the only thing "+
					"that varies."),
			code("wire-request-code", "bash", "", "",
				"# one POST, one document: the fields you named are the fields you get\n"+
					"curl -s localhost:8100/api/graphql -H 'Content-Type: application/json' \\\n"+
					"  -d '{\"query\":\"{ posts(page:1, limit:2, sort:\\\"oldest\\\") { items { id title } totalItems hasNextPage } }\"}'\n"+
					"# → {\"data\":{\"posts\":{\"items\":[{\"id\":\"p-1\",\"title\":\"Why Go for backends\"},…],\n"+
					"#      \"totalItems\":6,\"hasNextPage\":true}}}"),
			text("mutations-pitch",
				"Mutations run through the same document shape with a ``mutation`` keyword, and "+
					"the substrate's semantics carry over untouched because the resolvers "+
					"delegate to the same use cases. ``updatePost`` is full replacement — the "+
					"input IS the whole next state, so omitting ``tags`` in the input removes "+
					"them, exactly like PUT on the REST surface; one rule set, two verbs, "+
					"shared because both contracts call the one ``ReplacePostUseCase``. "+
					"``deletePost`` keeps the strict stance: deleting a vanished post is a "+
					"``not_found`` error entry, never a silent success. And mutation ordering "+
					"is a spec guarantee worth knowing: top-level mutation fields run ONE AT A "+
					"A TIME, in document order — the create-then-update-then-delete round "+
					"trip in the demo below is sequential by construction, while top-level "+
					"QUERY fields may resolve concurrently, which is precisely the property "+
					"the N+1 section exploits."),
		),
		// ── 4. GraphQL errors ───────────────────────────────────────
		section("graphql-errors", "Errors: extension codes from typed Go errors",
			"GraphQL spends one status code — the taxonomy rides extensions instead, and the SAME mapping serves both surfaces.",
			text("err-pitch",
				"GraphQL's error model inverts REST's central convention: the status line is "+
					"always 200 (transport aside), and failures travel in the ``errors`` list "+
					"beside whatever ``data`` survived. Each entry carries a ``message`` for "+
					"humans, optional ``path`` naming the field that failed, and — the part "+
					"worth designing — ``extensions``: a free-form object the SERVER defines. "+
					"This guide puts the typed taxonomy there. The domain already returns "+
					"``*core.Error`` values (kind + message + structured details) — the same "+
					"values the REST error writer turns into 404 and 422 envelopes — so the "+
					"graph's mapping is one function translating kind to ``extensions.code`` "+
					"and carrying ``details`` verbatim. One taxonomy, two presentations:"),
			code("err-map-code", "go",
				"The whole translation: typed taxonomy error → GraphQL error with extensions.",
				"server/internal/graph/errors.go",
				"func MapError(err error) *gqlerror.Error {\n"+
					"\tvar typed *core.Error\n"+
					"\tif !errors.As(err, &typed) {\n"+
					"\t\treturn &gqlerror.Error{\n"+
					"\t\t\tMessage:    \"Internal error.\",\n"+
					"\t\t\tExtensions: map[string]any{\"code\": string(core.KindInternal)},\n"+
					"\t\t}\n"+
					"\t}\n"+
					"\textensions := map[string]any{\"code\": string(typed.Kind)}\n"+
					"\tif typed.Details != nil {\n"+
					"\t\textensions[\"details\"] = typed.Details\n"+
					"\t}\n"+
					"\treturn &gqlerror.Error{\n"+
					"\t\tMessage:    typed.Message,\n"+
					"\t\tExtensions: extensions,\n"+
					"\t}\n"+
					"}"),
			text("err-codes",
				"Read the two error shapes below as a client would. The miss answers ``null`` "+
					"for the nullable field plus one entry whose ``extensions.code`` is "+
					"``not_found`` — the same bucket the REST 404 envelope serves, so a "+
					"shared client-side switch handles both surfaces. The rule violation "+
					"carries ``validation_failed`` with the issue list riding ``details`` — "+
					"field, rule, and what the server actually got, the identical payload "+
					"topic 11's 422 serves. Nullability decides how far the null travels: a "+
					"miss on NULLABLE ``post`` nulls one field; the same error under a "+
					"NON-NULL field would propagate upward and null its parent — which is "+
					"why this schema made ``createPost`` non-null (creation either returns a "+
					"post or the whole mutation failed) while ``updatePost`` and ``post`` "+
					"stay nullable (a miss is field-local information):"),
			code("err-wire-code", "bash", "", "",
				"# the miss: 200, data carries null, the taxonomy rides extensions\n"+
					"curl -s localhost:8100/api/graphql -H 'Content-Type: application/json' \\\n"+
					"  -d '{\"query\":\"{ post(id:\\\"p-999\\\") { id } }\"}'\n"+
					"# → {\"errors\":[{\"message\":\"Post \\\"p-999\\\" not found.\",\n"+
					"#      \"path\":[\"post\"],\"extensions\":{\"code\":\"not_found\"}}],\n"+
					"#    \"data\":{\"post\":null}}\n"+
					"\n"+
					"# the rule violation: the issue list rides extensions.details\n"+
					"curl -s localhost:8100/api/graphql -H 'Content-Type: application/json' \\\n"+
					"  -d '{\"query\":\"{ posts(limit:500) { totalItems } }\"}'\n"+
					"# → {\"errors\":[{\"message\":\"Invalid post list query.\",\"extensions\":\n"+
					"#      {\"code\":\"validation_failed\",\"details\":[{\"field\":\"limit\",…}]}}],\n"+
					"#    \"data\":null}"),
			text("err-stance",
				"The stance to take away: CONSISTENCY of the error vocabulary across surfaces "+
					"beats either surface's native style. A client of this guide's API learns "+
					"one closed set of codes — ``not_found``, ``validation_failed``, and the "+
					"rest of the taxonomy — and branches identically whether the envelope "+
					"came from an HTTP status or an extensions object. That is only possible "+
					"because the mapping lives at the graph's BOUNDARY, translating the typed "+
					"errors the domain already returns; the moment resolvers start formatting "+
					"ad-hoc error strings, two vocabularies exist and every client pays for "+
					"both. The untyped fallback in ``MapError`` matters for the same reason — "+
					"an unexpected error degrades to a generic internal entry instead of "+
					"leaking a stack trace onto the graph."),
		),
		// ── 5. The N+1 problem and dataloaders ──────────────────────
		section("n-plus-one-and-dataloaders", "The N+1 problem and a hand-rolled dataloader",
			"Measure the N+1 with an instrumented port, then collapse it to one batched call — no new dependency, all real code.",
			text("n1-pitch",
				"The N+1 problem is GraphQL's oldest tax: resolving a LIST of items where each "+
					"item needs its own lookup. One query — \"the page, and for each post its "+
					"fresh detail\" — becomes one call for the page plus one call PER item: "+
					"``N+1`` round trips, where REST's fixed envelope needed exactly one. "+
					"The shape appears the moment a schema has any per-item relation or "+
					"per-item field backed by its own fetch; the fix is always the same "+
					"idea — BATCH the per-item lookups — and the standard packaging is a "+
					"DATALOADER: a request-scoped object that collects ids as sibling "+
					"resolvers ask for them, waits a tiny coalescing window, and resolves "+
					"every collected id in ONE batch call. GraphQL's concurrency model is "+
					"what makes it work: sibling fields resolve concurrently, so a page's "+
					"worth of asks lands inside one window naturally."),
			text("n1-instrument",
				"Before fixing it, MEASURE it — \"is this an N+1?\" should be answered with a "+
					"number, not a vibe. The instrument is a counting wrapper around the "+
					"posts port: every method call crosses it and moves its own counter, and "+
					"the wrapper satisfies the same interface so it injects anywhere the port "+
					"fits. Wrap the real adapter, run the workload, read the snapshot — the "+
					"naive hydration of the six-post seed shows one ``List`` plus six "+
					"``GetByID`` calls, seven port calls where one batched call would do:"),
			code("n1-counting-code", "go",
				"The instrument: one counter per port method, injectable anywhere the port fits.",
				"server/internal/graph/loader/counting.go",
				"type PortCounts struct {\n"+
					"\tList    int `json:\"list\"`\n"+
					"\tGetByID int `json:\"getById\"`\n"+
					"\tCreate  int `json:\"create\"`\n"+
					"\tReplace int `json:\"replace\"`\n"+
					"\tDelete  int `json:\"delete\"`\n"+
					"}\n"+
					"\n"+
					"// Total sums the counters — the port-call count the experiment reports.\n"+
					"func (p PortCounts) Total() int {\n"+
					"\treturn p.List + p.GetByID + p.Create + p.Replace + p.Delete\n"+
					"}"),
			text("loader-pitch",
				"The loader itself is small enough to own, so this guide hand-rolls it instead "+
					"of importing one — the moving parts are worth holding. A ``Load`` "+
					"registers the id under a mutex (duplicates share one entry) and blocks "+
					"on a one-slot channel — a future, collapsed into the call. One "+
					"dispatcher goroutine owns a coalescing window: every nudge from a "+
					"``Load`` extends the window; when the window closes with work pending, "+
					"the dispatcher snapshots the ids, makes ONE batch call, and delivers "+
					"every answer; when it closes quiet, the goroutine retires — no leak, "+
					"no shutdown API to forget. Context cancellation propagates to the batch "+
					"and fails every pending load with the typed cause:"),
			code("loader-load-code", "go",
				"The future: register, maybe spawn the dispatcher, wait for your batch.",
				"server/internal/graph/loader/loader.go",
				"func (l *PostsLoader) Load(ctx context.Context, id string) (posts.Post, error) {\n"+
					"\t// A buffered channel of one: the dispatcher delivers without holding\n"+
					"\t// a goroutine hostage, and the receiver may arrive late or never\n"+
					"\t// (context canceled) without blocking delivery.\n"+
					"\tch := make(chan result, 1)\n"+
					"\n"+
					"\tl.mu.Lock()\n"+
					"\tif _, seen := l.waiters[id]; !seen {\n"+
					"\t\tl.order = append(l.order, id)\n"+
					"\t}\n"+
					"\tl.waiters[id] = append(l.waiters[id], ch)\n"+
					"\t// …the dispatcher spawns once, under the same lock\n"+
					"\tl.mu.Unlock()\n"+
					"\n"+
					"\tselect {\n"+
					"\tcase got := <-ch:\n"+
					"\t\treturn got.post, got.err\n"+
					"\tcase <-ctx.Done():\n"+
					"\t\treturn posts.Post{}, core.Wrapf(ctx.Err(), core.KindInternal,\n"+
					"\t\t\t\"Load of post %q canceled before its batch resolved.\", id)\n"+
					"\t}\n"+
					"}"),
			code("loader-loop-code", "go",
				"The window: extend on stragglers, fire on quiet, retire when done.",
				"server/internal/graph/loader/loader.go",
				"func (l *PostsLoader) dispatchLoop(ctx context.Context) {\n"+
					"\tidle := time.NewTimer(l.window)\n"+
					"\tdefer idle.Stop()\n"+
					"\n"+
					"\tfor {\n"+
					"\t\tselect {\n"+
					"\t\tcase <-l.wake:\n"+
					"\t\t\t// Straggler arrived: extend the window so it joins this batch.\n"+
					"\t\t\tif !idle.Stop() {\n"+
					"\t\t\t\tdrainTimer(idle)\n"+
					"\t\t\t}\n"+
					"\t\t\tidle.Reset(l.window)\n"+
					"\t\tcase <-idle.C:\n"+
					"\t\t\tif l.retireIfQuiet() {\n"+
					"\t\t\t\treturn\n"+
					"\t\t\t}\n"+
					"\t\t\tl.fire(ctx)\n"+
					"\t\t\tidle.Reset(l.window)\n"+
					"\t\tcase <-ctx.Done():\n"+
					"\t\t\tl.retireIfQuiet()\n"+
					"\t\t\tl.failAll(ctx.Err())\n"+
					"\t\t\treturn\n"+
					"\t\t}\n"+
					"\t}\n"+
					"}"),
			text("loader-batch",
				"The batch seam is a function over the port — one call resolving many ids. "+
					"Against a real database this is the adapter's ``WHERE id = ANY($1)`` "+
					"query; against the in-memory substrate it is one unfiltered page read, "+
					"indexed by id — the same shape, honestly stated. What matters is the "+
					"CONTRACT at the seam: the loader does not care how the batch is served, "+
					"only that one call serves every id, and absent ids come back absent so "+
					"the loader can answer the typed miss:"),
			code("loader-batch-code", "go",
				"The batch seam over the posts port — one call, many ids.",
				"server/internal/graph/loader/loader.go",
				"func PortBatch(repo posts.PostsRepository) BatchFetch {\n"+
					"\treturn func(ctx context.Context, ids []string) (map[string]posts.Post, error) {\n"+
					"\t\tpage, err := repo.List(ctx, posts.ListQuery{\n"+
					"\t\t\tPage:  1,\n"+
					"\t\t\tLimit: posts.MaxPageLimit,\n"+
					"\t\t\tSort:  posts.SortOldest,\n"+
					"\t\t})\n"+
					"\t\tif err != nil {\n"+
					"\t\t\treturn nil, err\n"+
					"\t\t}\n"+
					"\t\tbyID := make(map[string]posts.Post, len(page.Items))\n"+
					"\t\tfor _, post := range page.Items {\n"+
					"\t\t\tbyID[post.ID] = post\n"+
					"\t\t}\n"+
					"\t\treturn byID, nil\n"+
					"\t}\n"+
					"}"),
			text("n1-verdict",
				"Run the experiment with the instrument in place and the numbers are the "+
					"lesson. The naive hydration of the six-post seed: one ``List`` plus six "+
					"``GetByID`` — SEVEN port calls, and behind a database, seven round "+
					"trips. The same page through the loader, with every id requested twice "+
					"concurrently to make deduplication visible: TWELVE loads, six "+
					"duplicates, ONE batch fetch — a single ``List`` crossing the port. "+
					"Those counts are pinned by tests (``internal/graph/loader``) and "+
					"reported by the demo payload as data, because a claim like \"the "+
					"loader fixes N+1\" is exactly the kind of sentence this guide prefers "+
					"to prove. The cost side is real too: the coalescing window adds "+
					"latency to every batched read (milliseconds here), per-request loaders "+
					"add allocation churn under load, and a loader wired to a WEAK batch "+
					"seam just moves the N+1 one layer down — measure before and after, "+
					"always."),
		),
		// ── 6. Introspection and tooling ───────────────────────────
		section("introspection-and-tooling", "Introspection and tooling",
			"The schema serves itself: __schema queries, generated docs, playgrounds — and this topic's demo inline.",
			text("intro-pitch",
				"Every GraphQL schema answers questions about itself. The spec reserves two "+
					"meta-fields — ``__schema`` and ``__type`` — that expose the whole type "+
					"system at runtime: every type, every field, every argument, every "+
					"deprecation. That single feature is why the GraphQL tooling ecosystem "+
					"is so uniform: GraphiQL and GraphQL Playground build autocomplete and "+
					"schema browsers from an introspection query; code generators derive "+
					"TypeScript types from the served schema; linters catch deprecated "+
					"fields before you ship them. The guide's executor ships introspection "+
					"DISABLED by default (the safe posture for graphs exposed to strangers) "+
					"and opts in deliberately per operation — a two-line decision you can "+
					"read in ``internal/graph/execute.go`` — because a teaching graph "+
					"that hides its own schema would be a strange lesson:"),
			code("intro-code", "bash", "", "",
				"# ask the graph about itself: the roots, then any type's fields\n"+
					"curl -s localhost:8100/api/graphql -H 'Content-Type: application/json' \\\n"+
					"  -d '{\"query\":\"{ __schema { queryType { name } mutationType { name } } }\"}'\n"+
					"# → {\"data\":{\"__schema\":{\"queryType\":{\"name\":\"Query\"},\n"+
					"#      \"mutationType\":{\"name\":\"Mutation\"}}}}"),
			text("demo-intro",
				"The demo below is a playground built on everything above. It is NOT a mock: "+
					"the payload executes REAL operations through the same in-process "+
					"execution path the endpoint serves — a paginated query, a read, a full "+
					"create→update→delete mutation round-trip, both error shapes with their "+
					"extension codes, an introspection query, and the loader experiment's "+
					"port-call counts (naive 7 versus batched 1). The sample list beside "+
					"the results is selectable: pick one and RUN it against the live "+
					"``POST /api/graphql`` endpoint — the console POSTs the document and "+
					"renders the response beside the reference run, so the in-process "+
					"results and the served results sit close enough to compare. Mutations "+
					"run for real against the live substrate — create, watch the id, "+
					"update, delete — the same round trip the terminal companion walks:"),
			demo("api:graphql-playground", "The GraphQL playground",
				"A live console over the posts graph: executed sample operations (pagination, "+
					"reads, a create→update→delete round-trip, not_found and validation_failed "+
					"errors with their extensions codes, introspection), the schema card read "+
					"off the real parsed schema, the N+1-versus-dataloader port-call counts, "+
					"and a RUN button that POSTs any sample to the live /api/graphql."),
			text("demo-read",
				"Read the runs and the card together and the topic compresses into one "+
					"screen: the schema card's signatures are the operations you can RUN; "+
					"the error runs show the taxonomy riding extensions; the loader "+
					"experiment turns the N+1 story into two numbers and a verdict. The "+
					"payload is deterministic — fixed seeds, fixed ids, timestamps kept "+
					"out of every selected field — while the RUN button hits the live "+
					"endpoint and shows the world as it is RIGHT NOW, ids and all. The "+
					"same binary serves the payload, the endpoint, and this page's prose: "+
					"the graph is not described here, it is RUNNING here."),
		),
		// ── 7. Terminal companion ──────────────────────────────────
		section("terminal-companion", "Terminal companion: the curl tour",
			"The whole graph from the shell: one endpoint, -d '{\"query\":\"…\"}' documents, every error shape along the way.",
			text("term-pitch",
				"With the stack up (server on :8100), this tour walks the graph with ``curl`` "+
					"— the same operations the playground runs, one document at a time. The "+
					"shape never changes: one POST, a JSON body whose ``query`` member holds "+
					"the operation. You will see the paginated read, the one-post read, a "+
					"real mutation round-trip, both error shapes with their extensions "+
					"codes, introspection asking the schema about itself, and the demo "+
					"payload's loader verdict. Before the endpoint was wired, the same "+
					"operations ran in-process — the final stop shows that test suite, "+
					"which is exactly what the demo payload executes:"),
			code("term-list", "bash", "", "",
				"# the first page, oldest first — ask for exactly the fields you want\n"+
					"curl -s localhost:8100/api/graphql -H 'Content-Type: application/json' \\\n"+
					"  -d '{\"query\":\"{ posts(page:1, limit:2, sort:\\\"oldest\\\") { items { id title } totalItems totalPages hasNextPage } }\"}' \\\n"+
					"  | python3 -m json.tool"),
			code("term-one", "bash", "", "",
				"# one post by id — nullable field, so a miss is data, not a status\n"+
					"curl -s localhost:8100/api/graphql -H 'Content-Type: application/json' \\\n"+
					"  -d '{\"query\":\"{ post(id:\\\"p-3\\\") { id title tags } }\"}' | python3 -m json.tool"),
			code("term-mutation", "bash", "", "",
				"# a real mutation round-trip: create (first created id is p-7), replace, delete\n"+
					"curl -s localhost:8100/api/graphql -H 'Content-Type: application/json' \\\n"+
					"  -d '{\"query\":\"mutation { createPost(input: { title: \\\"From the shell\\\", body: \\\"Written by curl.\\\", tags: [\\\"graphql\\\"] }) { id title } }\"}'\n"+
					"curl -s localhost:8100/api/graphql -H 'Content-Type: application/json' \\\n"+
					"  -d '{\"query\":\"mutation { updatePost(id: \\\"p-7\\\", input: { title: \\\"Replaced\\\", body: \\\"Whole next state.\\\", tags: [\\\"edited\\\"] }) { id title } }\"}'\n"+
					"curl -s localhost:8100/api/graphql -H 'Content-Type: application/json' \\\n"+
					"  -d '{\"query\":\"mutation { deletePost(id: \\\"p-7\\\") }\"}'"),
			code("term-not-found", "bash", "", "",
				"# the miss: 200, data nulls the field, the taxonomy rides extensions.code\n"+
					"curl -s localhost:8100/api/graphql -H 'Content-Type: application/json' \\\n"+
					"  -d '{\"query\":\"{ post(id:\\\"p-999\\\") { id } }\"}' | python3 -m json.tool"),
			code("term-validation", "bash", "", "",
				"# the rule violation: same vocabulary the REST 422 serves, issue list and all\n"+
					"curl -s localhost:8100/api/graphql -H 'Content-Type: application/json' \\\n"+
					"  -d '{\"query\":\"{ posts(limit:500) { totalItems } }\"}' | python3 -m json.tool"),
			code("term-introspection", "bash", "", "",
				"# introspection: the schema documents itself — this is what every tool runs\n"+
					"curl -s localhost:8100/api/graphql -H 'Content-Type: application/json' \\\n"+
					"  -d '{\"query\":\"{ __schema { queryType { name } mutationType { name } } }\"}'"),
			code("term-payload", "bash", "", "",
				"# the playground's companion payload: executed runs + the loader verdict\n"+
					"curl -s localhost:8100/api/demo/api/graphql-playground | python3 -m json.tool | head -40"),
			code("term-inprocess", "bash", "", "",
				"# before (and after) wiring: the same operations, executed in-process\n"+
					"cd server && go test ./internal/graph/... -run 'Test' -count=1"),
			text("term-coda",
				"Notice what the tour never shows: a second endpoint for a second shape, a "+
					"status code that disagrees with the body, or an error you must parse "+
					"prose to understand. One URL, one document grammar, one error list "+
					"carrying the same codes the REST surface serves — and behind it, the "+
					"same use cases, the same repository port, the same swap seam the "+
					"databases topics will exercise. The next topic steps sideways to gRPC "+
					"— contracts compiled instead of introspected — and the one after "+
					"reverses the arrow entirely with webhooks. Three contract shapes, one "+
					"domain: that is the lesson this group keeps teaching."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"graphql",
		"GraphQL",
		"The posts resource through one endpoint and a query language: gqlgen schema-first codegen, "+
			"resolvers over the same use cases, taxonomy-carrying errors, and the N+1 problem measured "+
			"then beaten by a hand-rolled dataloader.",
		"apis",
		13,
		sections,
	)
}
