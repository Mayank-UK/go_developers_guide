package content

import (
	"errors"
	"fmt"
)

// This file is topic 12 api-documentation (group apis, curriculum order
// 12), authored on the exemplar pattern: prose is the default block,
// every must-source code block quotes a real repo file (the snippet
// ratchet verifies it), the live demo sits INLINE in the section that
// teaches serving the spec, and the topic closes with the bash terminal
// companion — curling the spec URL and eyeballing a live response against
// it. The whole topic stands on ONE real artifact: server/api/openapi.yaml,
// the machine-readable description of topic 11's posts API.

// apiDocumentationTopic builds topic 12 — documentation as a
// machine-readable contract: why prose docs rot, OpenAPI's anatomy, the
// spec-first loop with oapi-codegen (Go's idiomatic path) held against
// annotation generation, serving the spec and the Swagger UI story,
// drift tests that fail when the spec lies about the wire, and the
// document-first move beyond REST.
func apiDocumentationTopic() (Topic, error) {
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
		// ── 1. Why machine-readable documentation ────────────────────
		section("why-machine-readable", "Why machine-readable documentation",
			"Prose docs describe an aspiration; a machine-readable spec is a claim the server can be tested against.",
			text("wmr-pitch",
				"Every API has documentation, written or not — the truth is whatever the handlers answer, and "+
					"the only question is whether a written description of it exists, and whether it stays "+
					"true. Prose documentation starts honest and rots by increments: a field is added in a "+
					"hurry, the docs page is updated \"later\", and six months later the page describes an "+
					"API that never existed. The failure is structural, not moral — prose has no machine "+
					"checking it, so nothing fails when code and copy disagree. The fix is to describe the "+
					"contract in a FORM machines can read: a structured document that names every path, "+
					"parameter, status, and schema, so tooling can validate, generate, and render from the "+
					"same source. That is what OpenAPI is: not a standardization of your API's design, but "+
					"a standardization of its DESCRIPTION."),
			text("wmr-what-unlocks",
				"A machine-readable contract pays four times. Tooling renders it: Swagger UI, Redoc, and "+
					"Postman turn the document into interactive documentation with zero hand-written HTML. "+
					"Generators consume it: client SDKs and server stubs in a dozen languages fall out of "+
					"the same file, and — the direction this topic takes — Go types can be GENERATED from "+
					"it, so the compiler enforces the contract at every call site. Tests pin it: a spec "+
					"describes exact response shapes, which means a test can drive the live routes and "+
					"refuse any response the document does not allow — the drift test this topic builds. "+
					"And reviews anchor on it: a \"rename this field\" pull request shows the spec line "+
					"changing beside the handler, and a spec-only change with no code change is instantly "+
					"suspicious. None of that is available to a Markdown page."),
			text("wmr-when-it-hurts",
				"The honest costs, so the trade is made with open eyes. A spec is a second artifact to "+
					"maintain — the contract now lives in two places (the code that implements it and the "+
					"document that describes it), and every discipline in this topic exists to keep those "+
					"two from diverging. Tooling has opinions: linters, validators, and generators each "+
					"support a slightly different slice of OpenAPI 3.0 and 3.1, and part of the skill is "+
					"knowing which corner of the vocabulary your tools actually honor. And ceremony "+
					"multiplies: a five-route CRUD surface like the posts API carries a spec comfortably; "+
					"two hundred endpoints want an owner, a review process, and probably code generation "+
					"on a CI timer. The family stance: machine-readable documentation earns its keep the "+
					"moment a SECOND consumer — a client team, a code generator, a contract test — reads "+
					"the contract, and before that moment it is optional weight."),
			text("wmr-this-topic",
				"This topic ships the whole loop against the real posts API from the api-design topic — "+
					"no toy example, the actual five routes the workbench drives. The spec lives at "+
					"``server/api/openapi.yaml``; oapi-codegen turns its schemas into committed Go types "+
					"under ``server/internal/apispec/``; the running server serves the document verbatim "+
					"at ``/api/openapi.yaml``; and a drift test drives the live handlers and validates "+
					"their JSON against the spec's schemas, so the document cannot lie. One contract, "+
					"four consumers — renderer, compiler, HTTP route, and test — all reading the same file."),
		),
		// ── 2. OpenAPI anatomy ───────────────────────────────────────
		section("openapi-anatomy", "OpenAPI anatomy",
			"One YAML document: version, info, servers, paths full of operations, and components where schemas live once and are referenced everywhere.",
			text("oa-pitch",
				"OpenAPI (the specification; \"Swagger\" is the tooling family that spawned it) describes an "+
					"API as a single document, today at 3.1. The skeleton is small: ``openapi`` names the "+
					"spec version the file targets; ``info`` is the title, version, and description humans "+
					"see at the top of every rendered docs page; ``servers`` lists the base URLs the "+
					"described API answers at — the entries tooling prepends to every path; and the body "+
					"is ``paths`` (the operations) plus ``components`` (the reusable parts, schemas chief "+
					"among them). The guide's own spec opens exactly so, describing the devcontainer "+
					"server the terminal companion curls:"),
			code("oa-head-code", "yaml",
				"The document's head: which spec version, which API, which servers.",
				"server/api/openapi.yaml",
				"openapi: 3.1.0\n"+
					"info:\n"+
					"  title: Go Developers Guide — posts API\n"+
					"  version: 1.0.0\n"+
					"  summary: \"The teaching posts resource: list, create, read, replace, delete.\"\n"+
					"servers:\n"+
					"  - url: http://localhost:8100\n"+
					"    description: The devcontainer server (docker compose stack)"),
			text("oa-paths",
				"Under ``paths``, each URL maps to a PATH ITEM holding one OPERATION per HTTP method — "+
					"lowercase ``get``, ``post``, ``put``, ``delete`` keys. An operation carries an "+
					"``operationId`` (the unique, tooling-friendly name generators derive function names "+
					"from), a ``summary``, optional ``parameters`` (path, query, and header inputs, each "+
					"with a schema of its own), an optional ``requestBody``, and ``responses`` keyed by "+
					"status string. The discipline that makes specs readable: every response status gets "+
					"named, including the failures — a spec that documents only 200s is lying by "+
					"omission. The list operation shows the shape, including the closed sort vocabulary "+
					"the domain actually enforces:"),
			code("oa-operation-code", "yaml",
				"One operation: identity, one query parameter with its enum, and both statuses it can answer.",
				"server/api/openapi.yaml",
				"paths:\n"+
					"  /api/posts:\n"+
					"    get:\n"+
					"      operationId: listPosts\n"+
					"      summary: One page of the posts collection\n"+
					"      parameters:\n"+
					"        - name: sort\n"+
					"          in: query\n"+
					"          description: Closed sort vocabulary; omit for newest first.\n"+
					"          schema:\n"+
					"            type: string\n"+
					"            enum: [newest, oldest, title]\n"+
					"            default: newest\n"+
					"      responses:\n"+
					"        \"200\":\n"+
					"          description: The requested page with honest pagination metadata.\n"+
					"          content:\n"+
					"            application/json:\n"+
					"              schema:\n"+
					"                $ref: \"#/components/schemas/PostPage\"\n"+
					"        \"422\":\n"+
					"          description: page, limit, or sort outside the rules.\n"+
					"          content:\n"+
					"            application/json:\n"+
					"              schema:\n"+
					"                $ref: \"#/components/schemas/ErrorEnvelope\""),
			text("oa-components",
				"``components/schemas`` is where wire shapes are defined ONCE and referenced everywhere — "+
					"the DRY move that keeps a 200 body and the response of three different operations "+
					"consistent by construction. A schema is JSON Schema (3.1 makes that explicit): "+
					"``type: object`` with ``properties``, ``required`` naming which keys must ride "+
					"every response, ``additionalProperties: false`` closing the object so unknown keys "+
					"are violations rather than surprises, ``enum`` for closed vocabularies, "+
					"``format: date-time`` promising RFC 3339 strings, and array/item constraints like "+
					"``maxItems``. Operations point at schemas with ``$ref`` pointers — ``#/components/"+
					"schemas/PostPage`` means \"the PostPage schema in this document's components\" — "+
					"and schemas reference each other the same way, which is how a page of items says "+
					"\"each item is a Post\" without repeating it:"),
			code("oa-schemas-code", "yaml",
				"Schemas defined once, referenced everywhere: the post and its page envelope.",
				"server/api/openapi.yaml",
				"components:\n"+
					"  schemas:\n"+
					"    Post:\n"+
					"      type: object\n"+
					"      description: One post on the wire — the response shape of read, create, and replace.\n"+
					"      required: [id, title, body, tags, createdAt, updatedAt]\n"+
					"      additionalProperties: false\n"+
					"      properties:\n"+
					"        id:\n"+
					"          type: string\n"+
					"          description: Server-assigned identity, for example p-1.\n"+
					"        tags:\n"+
					"          type: array\n"+
					"          description: Write-normalized labels (trimmed, lowercase); [] when none — never null.\n"+
					"          items:\n"+
					"            type: string\n"+
					"        createdAt:\n"+
					"          type: string\n"+
					"          format: date-time\n"+
					"          description: When the post was stored, RFC 3339 UTC.\n"+
					"    PostPage:\n"+
					"      type: object\n"+
					"      description: The list envelope — one page's items plus the honest pagination metadata.\n"+
					"      required: [items, page, limit, totalItems, totalPages, hasNextPage, hasPrevPage]\n"+
					"      additionalProperties: false\n"+
					"      properties:\n"+
					"        items:\n"+
					"          type: array\n"+
					"          description: This page's posts, sorted; [] for an empty page — never null.\n"+
					"          items:\n"+
					"            $ref: \"#/components/schemas/Post\""),
			text("oa-envelope-schema",
				"The error envelope gets the same treatment, and its schema is where the spec earns real "+
					"precision: ``code`` is an ``enum`` of the taxonomy's eight buckets, and "+
					"``details`` genuinely varies — the issue ARRAY for validation failures, an object "+
					"keyed by ``body`` for strict-decode refusals — so it is documented as a ``oneOf`` "+
					"union. One deliberate extension rides along: ``x-go-type: any``, an "+
					"``x-``-prefixed vendor extension OpenAPI ignores but code generators may honor — "+
					"it tells oapi-codegen to map the union to Go's ``any`` instead of synthesizing a "+
					"union helper type (a decision the next section justifies). Optional members like "+
					"``details`` are described but left out of ``required``: this API's rule — optionals "+
					"omitted, never null — is exactly what a schema without ``required`` membership and "+
					"without nullable types states:"),
			code("oa-error-schema-code", "yaml",
				"The recorded envelope as schemas: closed code enum, and details as an honest union.",
				"server/api/openapi.yaml",
				"    ErrorBody:\n"+
					"      type: object\n"+
					"      description: \"The envelope's payload: taxonomy bucket, human sentence, join key, structured context.\"\n"+
					"      required: [code, message, requestId]\n"+
					"      additionalProperties: false\n"+
					"      properties:\n"+
					"        code:\n"+
					"          type: string\n"+
					"          description: The closed taxonomy bucket clients branch on.\n"+
					"          enum: [validation_failed, unauthorized, forbidden, not_found, conflict, rate_limited, service_unavailable, internal]\n"+
					"        details:\n"+
					"          description: Structured context — omitted when absent, never null.\n"+
					"          x-go-type: any\n"+
					"          oneOf:\n"+
					"            - type: array\n"+
					"              description: The collected validation issues (field, rule, got).\n"+
					"              items:\n"+
					"                $ref: \"#/components/schemas/ValidationIssue\"\n"+
					"            - type: object\n"+
					"              description: The decoder's refusal reason keyed by body.\n"+
					"              additionalProperties:\n"+
					"                type: string"),
			text("oa-31-vs-30",
				"A word on versions, because files in the wild carry both. OpenAPI 3.0 is the broadly "+
					"supported baseline — nearly every tool reads it, and its schemas are a JSON-Schema-"+
					"\"like\" dialect with special rules (``nullable: true`` instead of type unions, for "+
					"example). 3.1 aligned schemas WITH JSON Schema 2020-12 proper: ``type`` accepts "+
					"arrays for unions, ``examples`` is a real array, and annoying edges like "+
					"``exclusiveMinimum`` booleans were fixed. This guide writes 3.1 because the spec "+
					"uses none of the exotic corners — the vocabulary above is identical in both — and "+
					"because the walker that validates drift reads plain JSON Schema semantics. When "+
					"YOU choose: 3.0 if your toolchain (an old gateway, a legacy generator) demands it, "+
					"3.1 otherwise; what actually matters is that the file parses and tells the truth."),
		),
		// ── 3. Spec-first with oapi-codegen ──────────────────────────
		section("spec-first-with-oapi-codegen", "Spec-first with oapi-codegen",
			"Author the YAML, generate the Go types, commit the output — the compiler walks contract changes into every caller.",
			text("sf-pitch",
				"There are two directions documentation can flow, and Go has a clear idiomatic answer. "+
					"SPEC-FIRST: the OpenAPI document is the source of truth, code is generated from it, "+
					"and edits start in the YAML. CODE-FIRST: the handlers are the truth, and tooling "+
					"DERIVES documentation from them — by parsing annotations in comments, or by "+
					"registering routes into a spec builder. For Go the community's center of gravity is "+
					"spec-first with oapi-codegen (and its siblings likeogen, and goa for full design-"+
					"first generation): the ecosystem never grew a de-facto annotation standard the way "+
					"Java's Springdoc or Python's FastAPI did, partly because Go's stdlib HTTP handlers "+
					"carry no metadata for a scanner to find, and partly because Go programmers already "+
					"love generated code with committed output — protobuf and gRPC taught the pattern. "+
					"So this topic takes the idiomatic path and teaches the other by comparison."),
			text("sf-loop",
				"The loop, concretely. Edit ``server/api/openapi.yaml`` — add a field, tighten an enum, "+
					"name a new response. Regenerate: one command renders ``components/schemas`` into Go "+
					"structs with JSON tags, enums into named string types, ``format: date-time`` into "+
					"``time.Time``. The generated file is COMMITTED — the repository diff shows exactly "+
					"what the spec change cost in types, reviews see it, and builds never depend on a "+
					"generator being installed (the tool is a pinned ``go.mod`` tool dependency, "+
					"``go tool oapi-codegen``, so even that is reproducible). Then the compiler goes to "+
					"work: every handler or client constructing those types now fails to build until it "+
					"matches the new contract. The contract change propagates the way Go propagates "+
					"everything — at compile time, loudly, before the tests even run."),
			text("sf-config",
				"oapi-codegen is configured by a small YAML file that decides WHAT to generate: models "+
					"only, a server skeleton for one of the router ecosystems, a client, an embedded "+
					"copy of the spec. The guide's config generates models ONLY — the handlers already "+
					"exist and stay hand-written (thin, on purpose); what the spec contributes is the "+
					"TYPE LAYER, the shapes that cross the boundary. That choice also keeps the output "+
					"stdlib-only: no server stub means no dependency on the runtime helper package the "+
					"full stubs import:"),
			code("sf-config-code", "yaml",
				"The whole codegen configuration: types only, one output file.",
				"server/internal/apispec/cfg.yaml",
				"package: apispec\n"+
					"generate:\n"+
					"  models: true\n"+
					"output: types.gen.go"),
			text("sf-generate",
				"The regeneration command lives in the package as a ``go:generate`` directive — "+
					"``go generate ./internal/apispec`` runs it, and nothing else in the repository "+
					"remembers the flags:",
			),
			code("sf-generate-code", "go",
				"The directive that regenerates the types after a spec edit.",
				"server/internal/apispec/gen.go",
				"package apispec\n\n//go:generate go tool oapi-codegen -config cfg.yaml ../../api/openapi.yaml"),
			text("sf-output",
				"The output is deliberately boring Go — structs whose fields carry the spec's JSON "+
					"names as tags, ``time.Time`` for timestamps, enums as named string types with "+
					"constants and a ``Valid`` method. Boring is the achievement: these types are the "+
					"wire, stated in the language the rest of the server speaks, and any code that "+
					"builds against them cannot misname a field without the compiler saying so:",
			),
			code("sf-output-code", "go",
				"Committed generated types: the spec's Post, verbatim Go.",
				"server/internal/apispec/types.gen.go",
				"// Post One post on the wire — the response shape of read, create, and replace.\n"+
					"type Post struct {\n"+
					"	// Body Trimmed, 1–5000 characters.\n"+
					"	Body string `json:\"body\"`\n"+
					"	// CreatedAt When the post was stored, RFC 3339 UTC.\n"+
					"	CreatedAt time.Time `json:\"createdAt\"`\n"+
					"	// Id Server-assigned identity, for example p-1.\n"+
					"	Id string `json:\"id\"`\n"+
					"	// Tags Write-normalized labels (trimmed, lowercase); [] when none — never null.\n"+
					"	Tags []string `json:\"tags\"`\n"+
					"	// Title Trimmed, 1–120 characters.\n"+
					"	Title string `json:\"title\"`\n"+
					"	// UpdatedAt When the post last changed (equals createdAt until then).\n"+
					"	UpdatedAt time.Time `json:\"updatedAt\"`\n"+
					"}"),
			text("sf-x-go-type",
				"One generated-type decision deserves its own paragraph, because it is the kind of "+
					"judgment codegen config forces on you. The error envelope's ``details`` is a union "+
					"— issue array OR decoder map — and oapi-codegen's default rendering of a union is a "+
					"wrapper type with ``As…``/``From…``/``Merge…`` methods whose ``Merge`` arms pull in "+
					"``github.com/oapi-codegen/runtime``. A types-only package suddenly carrying a "+
					"runtime dependency is a real cost for one field, and the escape hatch is the "+
					"``x-go-type: any`` extension from the previous section: the SPEC keeps the honest "+
					"``oneOf`` for every tool that reads it (Swagger UI renders the two branches; the "+
					"drift walker validates both), while codegen maps the property to plain ``any``. "+
					"The general lesson: vendor extensions are the sanctioned way to say generator-"+
					"specific things without lying to the ecosystem — and when the generated output "+
					"would cost more than it earns, say less in Go and keep the truth in the spec."),
			text("sf-where-types-live",
				"A layering note for this repository specifically. The generated types sit in "+
					"``internal/apispec`` and nothing else imports them yet — deliberately. The "+
					"presentation package's hand-written ``postResponse`` and ``postPageResponse`` "+
					"remain the shapes the handlers marshal; the generated types are the CONTRACT "+
					"statement, the drift test's reference point, and the seam a future client or "+
					"refactor would adopt (the day the handlers marshal ``apispec.Post`` directly, "+
					"the spec and the wire become one artifact). Keeping generation and adoption "+
					"separate is honest about what codegen bought TODAY — types and a test oracle — "+
					"without rewriting working handlers for the sake of purity."),
		),
		// ── 4. Annotation generation, by comparison ──────────────────
		section("annotations-by-comparison", "The other direction: annotation generation",
			"Code-first tooling derives the spec from the code — convenient, and the mirror-image trade.",
			text("ann-pitch",
				"The code-first direction deserves a fair hearing, because on a small Go API it feels "+
					"lighter: no YAML to maintain, documentation that cannot forget a route because it "+
					"is discovered FROM the routes. The Go tools in this family work by ANNOTATION: "+
					"the dominant one, swag (go-swagger's swaggo fork), reads comment blocks above "+
					"handlers — a ``@Summary`` line, ``@Param`` entries describing types inline, "+
					"``@Success``/``@Failure`` naming statuses and schema shapes — and a CLI walks the "+
					"package, stitches the annotations into an OpenAPI document, and writes it out; "+
					"fanciertools and huma go further, building the spec programmatically from the "+
					"router registrations or generic request types. The generated document then feeds "+
					"the same renderers and SDK generators as a hand-written one, so the CONSUMER side "+
					"is identical — the difference is entirely in where the truth lives and who "+
					"enforces it."),
			text("ann-trade",
				"Run the comparison on the axes that matter. Drift: annotations sit next to the code "+
					"they describe, so a handler edit at least has the docs comment RIGHT THERE to "+
					"update — but nothing fails when it is not; a spec-first repo can go further and "+
					"make the drift a test failure, which is exactly the drift test below. Review: "+
					"annotations bury the contract inside comment prose where it is hard to read as a "+
					"whole; a spec file is the entire contract in one reviewable document. Language: "+
					"annotations duplicate a DSL inside comments — types written as strings a typo "+
					"away from compiling fine and rendering wrong; the spec writes schemas in a "+
					"schema language. Tooling: swag historically targets OpenAPI 2.0/3.0 with "+
					"partial 3.x coverage, while spec-first generators track the document versions "+
					"they parse. Coupling: code-first ties the contract to one framework's handler "+
					"shapes — migrate routers and the annotations migrate badly; the YAML is "+
					"framework-agnostic by construction. Client generation: both directions feed "+
					"generators, but only spec-first lets the SERVER consume its own contract as "+
					"types-first input rather than after-the-fact output."),
			list("ann-decision", false,
				"Spec-first (this guide's choice) — the document is the source of truth; generate types, drift-test the wire against it; strongest when the contract outlives any one implementation.",
				"Code-first annotations — handlers are the truth, comments feed a scanner; cheapest on a small stdlib API you control end to end.",
				"Code-first programmatic — build the spec from route registrations in code (huma-style); type-safe and refactor-friendly, at the price of the document never being readable as one artifact.",
				"Hybrid — spec-first for schemas (the shapes), annotations for per-handler prose; defensible, but two sources of truth need one to be declared the master.",
				"Not choosing — the worst row on the table: an aging spec, an annotation drift, and docs nobody trusts."),
			text("ann-verdict",
				"The verdict this guide records: spec-first wins when the API is a product — multiple "+
					"consumers, a versioned contract, design review before implementation — and "+
					"annotation generation wins when the API is an implementation detail of one team's "+
					"service, where the handlers genuinely are the whole truth. The posts API here is "+
					"deliberately in the first camp: it is the TEACHING surface, its contract is the "+
					"curriculum, and the drift test below is only possible because the document came "+
					"first and independent of the code."),
		),
		// ── 5. Serving the spec + Swagger UI ─────────────────────────
		section("serving-the-spec", "Serving the spec, and the Swagger UI story",
			"The document belongs at a stable URL on the running server — verbatim, from the same embedded bytes everything else reads.",
			text("srv-pitch",
				"A spec in the repository is half deployed; the other half is the running server "+
					"answering for it. The family rule this guide settles on: serve the document at a "+
					"STABLE, well-known URL — ``/api/openapi.yaml`` — byte for byte, with an honest "+
					"content type. The \"byte for byte\" part is load-bearing: the bytes ride the binary "+
					"via ``go:embed``, so the route, the drift tests, and the demo's parsed facts all "+
					"read ONE copy of the file; there is no build step that could transform the "+
					"document between repository and wire, and no way for the served copy to be stale "+
					"relative to the binary that serves it. The handler is four lines and says so:"),
			code("srv-handler-code", "go",
				"The spec route: embedded bytes, YAML content type, no transformation.",
				"server/internal/presentation/demo_api_docs.go",
				"func (h *OpenAPISpecHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {\n"+
					"	w.Header().Set(\"Content-Type\", \"application/yaml\")\n"+
					"	w.Header().Set(\"Cache-Control\", \"no-cache\")\n"+
					"	_, _ = w.Write(api.OpenAPIYAML)\n"+
					"}"),
			text("srv-content-type",
				"The content type deserves its own sentence because it is a real decision: "+
					"``application/yaml`` states the payload IS YAML (the registered type from RFC "+
					"9512), where ``text/plain`` would render nicely in a browser and lie about the "+
					"bytes, and ``application/openapi+yaml`` — the media type some tooling asks for — "+
					"is still vying for registration and reads as unknown to many clients. \"Serve "+
					"the document with the type of the document\" survives every tool: browsers show "+
					"it, curl fetches it, spec validators accept it. If your consumers want JSON, "+
					"the honest move is a second route that CONVERTS at request time (or a "+
					"build-time conversion committed beside the YAML) — never silently rewriting the "+
					"canonical URL's payload based on an ``Accept`` header the document's own "+
					"consumers never sent."),
			text("srv-swagger",
				"Now the Swagger UI story, told honestly rather than as a step this guide takes. "+
					"Swagger UI is a static web application that fetches an OpenAPI document and "+
					"renders it as interactive documentation — the \"try it out\" page most developers "+
					"picture when they hear \"API docs\". Because it only needs the document at a "+
					"URL, the guide's ``/api/openapi.yaml`` is already compatible: point a hosted "+
					"Swagger UI (or Redoc, or Postman, or Insomnia) at the route and the interactive "+
					"docs render from the same bytes the drift test validates — there is nothing "+
					"else to keep true. What the guide deliberately does NOT ship is a bundled UI "+
					"page inside the Go binary: the client app is this guide's presentation layer "+
					"by design, a second docs UI would be a second surface to maintain and theme, "+
					"and \"bring the viewer you already trust\" is the deployment story that "+
					"scales. When a team DOES want self-hosted docs, the move is one static file "+
					"mount — serve swagger-ui's ``dist`` and set its spec URL — and it composes "+
					"with everything here without changing a line of the contract."),
			text("srv-demo-intro",
				"The demo below puts the whole story on one screen — and it is built the way it "+
					"teaches: the payload is PARSED from the actual spec file at request time (the "+
					"same embedded bytes), so the title, version, server list, path and schema "+
					"inventories, and the raw spec text cannot drift from the document even if "+
					"someone edits the YAML and forgets the demo exists. The drift rows summarize "+
					"exactly what the next section's test enforces. Everything on the card is "+
					"derived, nothing hand-copied:"),
			demo("api:docs-link", "The docs-link card",
				"The OpenAPI contract as data, parsed live from server/api/openapi.yaml: the spec's "+
					"own facts (title, version, servers, path/operation/schema counts), the five "+
					"operations, the six schemas, the docs-serving story with the spec URL, the "+
					"drift guarantees the test enforces — and the raw spec text, the very bytes "+
					"/api/openapi.yaml serves."),
			text("srv-demo-read",
				"Read the card beside the previous sections and notice the closure: the operation "+
					"table is the spec's ``paths`` walked; the schema inventory is what oapi-codegen "+
					"turned into Go types; the drift rows are the test file's function names; the "+
					"raw text pane is the route's payload. One document, every consumer on one "+
					"screen — the state this whole topic argues for, running live."),
		),
		// ── 6. Drift tests ───────────────────────────────────────────
		section("drift-tests", "Drift tests: the spec versus the live wire",
			"A spec that cannot fail is a rumor — drive the real handlers and validate their JSON against the document.",
			text("dt-pitch",
				"Everything before this section built trust by construction: one file, embedded once, "+
					"parsed by every consumer. But construction cannot prove the file is TRUE — the "+
					"spec can still disagree with what the handlers actually answer, and that "+
					"disagreement is DRIFT, the quiet failure that kills documentation's credibility. "+
					"The fix is the same move this guide makes everywhere else a claim needs "+
					"enforcing: turn the claim into a TEST. A drift test drives the real routes "+
					"(the same constructors ``main.go`` wires, behind the same middleware chain), "+
					"decodes each response as generic JSON, and validates it against the spec's "+
					"schemas — so a renamed key, a missing required field, a wrong type, a value "+
					"outside an enum, or a timestamp that is not RFC 3339 fails the suite with the "+
					"JSON path in the message."),
			text("dt-walker",
				"The validator is a hand-rolled WALKER — deliberately no schema library, because "+
					"the point is to teach what schema validation IS, and because the OpenAPI "+
					"subset this spec uses (objects, arrays, scalars, ``required``, "+
					"``additionalProperties``, ``enum``, ``format: date-time``, ``oneOf``, "+
					"numeric bounds) is a few dozen lines of recursion. The walker parses the "+
					"embedded YAML into plain maps, indexes ``components/schemas`` by name, and "+
					"resolves ``$ref`` pointers locally; entry is one method per value. The object "+
					"branch shows the style — every violation names its JSON path:"),
			code("dt-walker-code", "go",
				"The walker's object branch: required keys must ride, closed objects refuse strangers.",
				"server/internal/apispec/walker.go",
				"func (w *Walker) walkObject(value any, node map[string]any, path string) []string {\n"+
					"	object, ok := value.(map[string]any)\n"+
					"	if !ok {\n"+
					"		return []string{fmt.Sprintf(\"%s: want object, got %s\", path, typeOf(value))}\n"+
					"	}\n"+
					"	var violations []string\n"+
					"	for _, required := range stringSlice(node[\"required\"]) {\n"+
					"		if _, present := object[required]; !present {\n"+
					"			violations = append(violations, fmt.Sprintf(\"%s: required property %q missing\", path, required))\n"+
					"		}\n"+
					"	}\n"+
					"	// …unknown keys are refused when additionalProperties is false,\n"+
					"	// then every documented property recurses through w.walk\n"+
					"	return violations\n"+
					"}"),
			text("dt-walker-alignment",
				"One subtlety the walker's existence teaches: YAML and JSON decode differently into "+
					"Go, and a validator that forgets that lies confidently. The spec's integers "+
					"decode as ``int``; the response's integers decode as ``float64`` — so enum "+
					"and bound comparisons normalize across both before judging. The ``oneOf`` "+
					"discipline is equally strict: a value matches a union only by fully matching "+
					"one branch, no partial credit — which is exactly the rule that lets the 422 "+
					"envelope's ``details`` be an issue array on one route and a decoder map on "+
					"another without the spec hedging into vagueness."),
			text("dt-test",
				"The test file reads like a contract checklist — one function per guarantee, each "+
					"driving the real stack and handing the body to the walker:"),
			code("dt-test-code", "go",
				"One drift guarantee, one test: the single-post wire against the Post schema.",
				"server/internal/presentation/spec_drift_test.go",
				"func assertConforms(t *testing.T, walker *apispec.Walker, body string, schema string) {\n"+
					"	t.Helper()\n"+
					"	if violations := walker.Validate(decodeWireJSON(t, body), schema); len(violations) > 0 {\n"+
					"		t.Errorf(\"%s conformance violations against the spec:\\n  %s\",\n"+
					"			schema, strings.Join(violations, \"\\n  \"))\n"+
					"	}\n"+
					"}"),
			code("dt-test-case-code", "go",
				"The guarantee itself: real handler, real middleware, spec-validated body.",
				"server/internal/presentation/spec_drift_test.go",
				"func TestSpecDriftSinglePostMatchesPost(t *testing.T) {\n"+
					"	var buf strings.Builder\n"+
					"	server := newDriftServer(newTestLogger(&buf))\n"+
					"	walker := driftWalker(t)\n"+
					"\n"+
					"	rec := doRequest(server, http.MethodGet, \"/api/posts/p-1\", \"\")\n"+
					"	if rec.Code != http.StatusOK {\n"+
					"		t.Fatalf(\"status = %d, want 200\", rec.Code)\n"+
					"	}\n"+
					"	assertConforms(t, walker, rec.Body.String(), \"Post\")\n"+
					"}"),
			text("dt-coverage",
				"The covered guarantees are the four the contract actually rests on, plus the two "+
					"error branches: the LIST response (including the honest empty page past the "+
					"end) validates against ``PostPage``; a SINGLE post and a freshly CREATED one "+
					"(201, with its ``Location`` header checked) against ``Post``; the 404 MISS "+
					"against ``ErrorEnvelope`` with ``details`` correctly absent; and the two 422 "+
					"shapes — broken draft (issue-array details) and strict-decode refusal (the "+
					"map branch) — exercise BOTH arms of the ``oneOf``. The suite also carries a "+
					"CANARY, the snippet-ratchet trick applied to schemas: it tampers a copy of the "+
					"spec (renames ``createdAt`` to ``created_at``), walks a REAL response against "+
					"the lie, and demands violations naming the honest key — proving that green "+
					"means \"wire matches document\", never \"walker approves everything\"."),
			text("dt-scope",
				"Scope discipline, so the tool earns its keep without becoming a second test "+
					"framework. The walker validates RESPONSES against schemas — the shapes this "+
					"API's clients parse — and deliberately does not re-implement request "+
					"validation (the domain already does that, better, with better error "+
					"messages); it checks structure, not business truth (a page envelope with "+
					"wrong totals still conforms — the repository's own tests own that claim); "+
					"and it runs in the ordinary test suite, no services, because the handlers "+
					"under test construct their in-memory repository directly. When the API "+
					"grows, the same walker grows: new schema names are validated by name, and "+
					"a nightly variant could even fetch ``/api/openapi.yaml`` from a deployed "+
					"environment and drift-test PRODUCTION — the contract following the code "+
					"all the way out the door."),
		),
		// ── 7. Beyond REST ───────────────────────────────────────────
		section("beyond-rest", "Beyond REST: the document-first move generalizes",
			"GraphQL, gRPC, and webhooks each carry their own contract format — and the same spec-first disciplines transfer.",
			text("br-graphql",
				"GraphQL moves the contract INTO the protocol: the schema — types, queries, "+
					"mutations, and their exact field graphs — IS the API, served over HTTP and "+
					"introspectable at runtime, so there is no separate description document to "+
					"drift: every client discovers the shape by asking. The Go-idiomatic tool is "+
					"gqlgen, and its loop is the oapi-codegen loop transposed: author the schema "+
					"first (this guide's GraphQL topic builds it over the same posts use cases "+
					"with gqlgen generating resolvers and types), and a resolver that drifts "+
					"from the schema fails the generator, not just the docs. What transfers "+
					"unchanged from this topic: contract-first editing, generated types, and "+
					"tests that drive the real resolver wiring — the drift test wearing a "+
					"GraphQL hat."),
			text("br-grpc",
				"gRPC's contract is the ``.proto`` file, and Go's ecosystem is the most "+
					"spec-first in the industry because code generation is not optional there: "+
					"the interface definition language IS the source of truth, ``buf`` and "+
					"``protoc`` generate the stubs and clients, and a breaking change is "+
					"DETECTABLE as a diff against the old descriptor (buf's breaking-rule "+
					"checker is the drift test, run before the code exists). The grpc topic "+
					"builds exactly that loop with ``buf`` codegen and in-process ``bufconn`` "+
					"tests. What transfers from this topic: the committed-output discipline "+
					"(generated stubs live in the repository), and the stance that the "+
					"document — not any implementation — is what compatibility means."),
			text("br-webhooks",
				"Webhooks REVERSE the arrow — your API becomes the caller, and the contract "+
					"that matters describes what you SEND — and the description format for "+
					"that world is AsyncAPI, OpenAPI's sibling for event-driven interfaces: "+
					"channels instead of paths, publish/subscribe operations instead of "+
					"methods, message payload schemas instead of response schemas. The "+
					"discipline this topic built maps one to one: document the payload "+
					"schemas once (the webhook topic's event shapes), generate or validate "+
					"against them, sign what you send (HMAC), and drift-test the EMITTER — "+
					"fire a delivery in a test and validate the payload against the "+
					"documented schema, exactly as this topic validated the 404 envelope. "+
					"The formats differ; the habit — one honest document, every consumer "+
					"reading it, a test that fails when reality disagrees — is the same one."),
		),
		// ── 8. Terminal companion ────────────────────────────────────
		section("terminal-companion", "Terminal companion: curl the spec",
			"Fetch the document the server serves, then eyeball a live response against it — the manual drift test.",
			text("tc-pitch",
				"With the stack up (server on :8100), this tour is the manual version of the "+
					"drift test: fetch the spec from the route that serves it, read the "+
					"operation inventory straight out of the document, then pull a live response "+
					"and compare it against the schema block you just read — field by field, "+
					"with your own eyes. ``grep -n`` finds the schema blocks; "+
					"``python3 -m json.tool`` pretty-prints payloads; no extra tooling:",
			),
			code("tc-spec-head", "bash", "", "",
				"# the document itself, from the route that serves it (verbatim, application/yaml)\n"+
					"curl -s localhost:8100/api/openapi.yaml | head -24"),
			code("tc-operations", "bash", "", "",
				"# the operation inventory, straight out of the served file\n"+
					"curl -s localhost:8100/api/openapi.yaml | grep -n 'operationId'"),
			code("tc-live-list", "bash", "", "",
				"# a live list response — the page envelope you are about to check against PostPage\n"+
					"curl -s 'localhost:8100/api/posts?tag=go&sort=oldest&limit=2' | python3 -m json.tool"),
			code("tc-schema-block", "bash", "", "",
				"# the schema for what you just saw: items, the seven metadata keys, no strangers\n"+
					"curl -s localhost:8100/api/openapi.yaml | sed -n '/    PostPage:/,/hasPrevPage/p'"),
			code("tc-404", "bash", "", "",
				"# the 404 envelope on a miss — then find ErrorEnvelope in the spec and compare\n"+
					"curl -si localhost:8100/api/posts/p-404\n"+
					"curl -s localhost:8100/api/openapi.yaml | sed -n '/    ErrorBody:/,/oneOf/p'"),
			code("tc-demo-card", "bash", "", "",
				"# the docs-link card — every fact parsed from the same file at request time\n"+
					"curl -s localhost:8100/api/demo/api/docs-link | python3 -m json.tool | head -30"),
			text("tc-coda",
				"Notice what the tour proves without running a single test: the served document "+
					"and the live wire AGREE — the keys the spec requires are the keys the "+
					"response carries, the enum the spec closes is the vocabulary the server "+
					"enforces, the envelope's optionals are absent rather than null. That "+
					"agreement is what the drift test automates and what the next topics "+
					"inherit: GraphQL will schema-first the same posts domain, gRPC will "+
					"proto-first it, and both will lean on the habit this topic built — the "+
					"document is the contract, and the contract is tested."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"api-documentation",
		"API documentation",
		"OpenAPI as a machine-readable contract for the real posts API: spec-first with oapi-codegen, "+
			"annotation generation by comparison, serving the document with the Swagger UI story, and "+
			"drift tests that fail the moment the spec lies about the wire.",
		"apis",
		12,
		sections,
	)
}
