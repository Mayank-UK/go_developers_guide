package content

import (
	"errors"
	"fmt"
)

// This file is topic 24 authorization (group state-auth, curriculum
// order 24): the question authentication deliberately did not answer —
// WHAT MAY the caller do — answered by a policy that is pure enough to
// table-test every cell of its matrix. Sections: the authn/authz split
// and the 401-vs-403 wire contract · RBAC as one registry (demo
// authz:matrix, the full roles × scopes × ownership decision run) ·
// enforcement middleware building on topic 23's RequireIdentity ·
// scopes as the narrowing half (a role can never widen a narrowed
// credential) · object-level ownership over the posts substrate
// (owner, editor, admin, stranger) · deny by default as a stance, not
// an accident · how the wild does it (ABAC, ReBAC, policy engines) ·
// the terminal companion's curl tour through the live matrix and the
// guarded routes' honest 401s.

// authorizationTopic builds topic 24.
func authorizationTopic() (Topic, error) {
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
		// ── 1. What authorization is ────────────────────────────────
		section("what-authorization-is", "What authorization is",
			"WHAT MAY the caller do — the question authentication deliberately left open, answered here by a policy pure enough to table-test every verdict it gives.",
			text("authz-pitch",
				"Authentication established WHO is calling and answered with an identity; authorization takes that identity and answers a different question: WHAT MAY they do? The split is not academic bookkeeping — it is the line between two different failure modes. A system that only authenticates gives every logged-in user the run of every resource, because \"logged in\" quietly became \"allowed\". A system that muddles the two into one check cannot answer \"is this user allowed to edit THIS post\" at all, because it never separated \"who\" from \"what\" from \"which object\". Topic 23 ended by placing an identity into the request context; this topic is everything that happens next: the identity's roles looked up, its credential's scopes consulted, the object's owner compared, and a verdict returned with a reason."),
			text("authz-identity-pitch",
				"The input everything here consumes is exactly the shape topic 23's middleware resolved — one struct carrying the username, the credential's scopes, and which carrier answered. The doc comment already promised this hand-off: scopes are VOCABULAR there and become VERDICTS here. Nothing in the identity is trusted beyond its source (the middleware only places it there after a verified credential), and nothing here re-verifies it — authorization trusts authentication's output and gets on with its own question:"),
			code("authz-identity-go", "go",
				"The hand-off: every carrier in topic 23 resolved to this one shape, and every rule in topic 24 consumes it.",
				"server/internal/domain/auth/auth.go",
				"// Identity is what every mechanism resolves to and what topic 24's\n"+
					"// authorization consumes: WHO is calling, what they may address (scopes\n"+
					"// are vocabulary, not verdicts), and WHICH mechanism carried the proof.\n"+
					"type Identity struct {\n"+
					"	Username    string\n"+
					"	DisplayName string\n"+
					"	Scopes      []string\n"+
					"	// Via names the carrier: \"session\", \"api-key\", or \"jwt\".\n"+
					"	Via string\n"+
					"}"),
			text("authz-wire-contract",
				"On the wire the split is one status code each, and the distinction is a contract with your callers. ``401 unauthorized`` means I DO NOT KNOW WHO YOU ARE — no credential, a malformed one, an expired one; the caller's only move is to authenticate again (topic 23's middleware owns this answer and deliberately says nothing more). ``403 forbidden`` means I KNOW EXACTLY WHO YOU ARE, AND THE ANSWER IS NO — the credential verified, the identity resolved, and the policy refused; re-authenticating changes nothing. Three consequences follow, and the whole topic obeys them:"),
			list("authz-401-403-rules", false,
				"Never 401 for an authenticated-but-forbidden request — answering 401 there sends the client into a re-login loop that cannot fix a policy refusal; the envelope's code field is `forbidden`, status 403.",
				"Refusals may teach — a 403 can name which gate said no (scope, role, object), because the caller already proved who they are; topic 23's uniform 401 stayed silent to avoid leaking which half of a credential pair failed.",
				"404 and 403 have an order — this guide answers 404 first for unknown objects (existence is not a secret in an open content API); a private system inverts it deliberately, returning 404 to hide existence from strangers — a stance to choose, not to drift into.",
			),
			text("authz-model-map",
				"The model this topic builds has three moving parts, and each gets its own section because each answers a different question. ROLES answer \"what does this kind of user do\" — bundles of permissions assigned in bulk. SCOPES answer \"what was this credential minted to do\" — the narrowing a token or key carries, checked before any role so it can only ever shrink access. OWNERSHIP answers \"may the caller touch THIS object\" — the rule that needs the resource in hand, where owners, editors, and strangers finally get different answers. Stack them in that order and each gate only narrows the one before it; the verdict that survives all three is allowed, and everything else — by construction — denies."),
		),
		// ── 2. RBAC: roles and permissions ──────────────────────────
		section("rbac-roles-permissions", "RBAC: roles and permissions",
			"Role-based access control: permissions named once, bundled into roles, assigned to users — one registry table standing behind every grant.",
			text("rbac-pitch",
				"Role-based access control is the workhorse model because it is the cheapest one that still audits. PERMISSIONS are the service's closed vocabulary of capabilities — this guide uses ``posts:read``, ``posts:write``, and ``users:manage``, the same ``namespace:verb`` string shape a credential's scopes use, so the two worlds compare without a translation layer. ROLES are named bundles of those permissions — reader, author, editor, admin — defined once in a registry and assigned to users in bulk. The wins are structural: a new permission is added to one role and every holder gets it; an audit asks \"who can delete posts\" and gets the admin roster instead of a grep through every handler; and a developer reads the registry top to bottom and knows the whole policy in ten lines."),
			code("rbac-registry-go", "go",
				"The whole RBAC policy as one table — and the lookup that makes unknown roles worthless.",
				"server/internal/domain/authorization/authorization.go",
				"var roleRegistry = map[Role][]Permission{\n"+
					"	RoleReader: {PermPostsRead},\n"+
					"	RoleAuthor: {PermPostsRead, PermPostsWrite},\n"+
					"	RoleEditor: {PermPostsRead, PermPostsWrite},\n"+
					"	RoleAdmin:  {PermPostsRead, PermPostsWrite, PermUsersManage},\n"+
					"}\n"+
					"\n"+
					"// RoleGrants reports whether one role grants a permission. Unknown\n"+
					"// roles grant nothing: the map miss answers false, which is the whole\n"+
					"// deny-by-default stance in one lookup.\n"+
					"func RoleGrants(role Role, perm Permission) bool {\n"+
					"	for _, granted := range roleRegistry[role] {\n"+
					"		if granted == perm {\n"+
					"			return true\n"+
					"		}\n"+
					"	}\n"+
					"	return false\n"+
					"}"),
			text("rbac-roles-vs-permissions",
				"Why not assign permissions to users directly? Because direct assignment does not survive contact with reality. Five hundred users times eleven permissions is five thousand five hundred rows nobody can read, every new capability means touching every affected user, and \"what can the interns do\" stops having an answer. Roles add exactly one level of indirection and buy bulk-assignment, one-place audits, and a vocabulary the whole team shares. The cost is equally real and worth naming: role explosion — when \"author-of-announcements-but-not-pinned-posts\" appears, the ladder is being asked to model attributes (the in-the-wild section shows where that road leads). The discipline that keeps RBAC healthy is refusing to mint a role per exception; exceptions belong in scopes or object rules, both of which compose WITH roles here."),
			text("rbac-permissions-are-not-http",
				"A permission is not an HTTP method, and conflating them is the classic first bug. ``posts:write`` covers POST, PUT, and DELETE on the posts resource — three routes, one capability, because the business question is \"may this caller change posts\", not \"may this caller send a PUT\". The mapping from routes to permissions happens once, in the enforcement wiring (the next section), where each route declares the capability it demands; the vocabulary itself stays a domain concept the storage layer never knows about. Two namespaces exist here on purpose — ``posts`` and ``users`` — so namespace isolation is real from day one: a role holding every posts permission still answers no to ``users:manage``, because the grant never crosses the colon."),
			text("rbac-matrix-intro",
				"The demo below runs the entire policy honestly: the seeded directory assigns roles to six principals (two of them the authentication topic's own ada and linus, so a real login of either meets exactly these verdicts), three seeded posts carry owners, and every subject × action × object cell is evaluated through the real evaluator — the same pure functions the middleware will call. Watch for the rows the section argued: the roster table itself, reader/author/editor/admin laddering over the same objects, and the first appearances of the narrowed credential and the role-less control that the next two sections unpack:"),
			demo("authz:matrix", "The decision matrix: roles × scopes × ownership, every cell live",
				"One request runs the full matrix against the real evaluator over the seeded directory and owned posts: the role roster with its exact grants, six principals including ada and linus (topic 23's own seeded users), the narrowed author kim, the editor grace, the admin root, and the role-less guest control — then every decision cell for every action on every post, each with the evaluator's own reason sentence. The beats walk the teaching moments (collection-level reads, owner vs editor vs stranger, scopes narrowing roles, deny by default, namespace isolation) and the proofs are derived from the cells, so a drifted answer renders as a red beat rather than a green lie."),
		),
		// ── 3. Middleware-based enforcement ─────────────────────────
		section("middleware-enforcement", "Enforcement middleware",
			"Policy no handler repeats: RequireIdentity resolves who, RequirePermission and RequireOwnership answer may — composed per-route at the composition root, 403 never 401.",
			text("enforcement-pitch",
				"A policy only matters where it is enforced, and the honest place is ONE wrapper per route, not a guard clause pasted into every handler. Topic 23 landed ``RequireIdentity`` — resolve any carrier to the identity in the request context, or answer the uniform 401. This topic's two middlewares build directly ON that, importing its context helpers rather than reinventing them: ``RequirePermission`` is the coarse gate (one capability demanded of the whole route), ``RequireOwnership`` the fine gate (the object's owner fetched and the object-level rule run). Handlers stay what the family architecture demands — parse, call a use case, map the result — because the verdict was already given before the handler ran, and the middleware records its decision in the request context so a guarded handler can ECHO why it opened without re-running policy."),
			code("enforcement-permission-go", "go",
				"The coarse gate: roles resolved through the port, one evaluator call, 403 with the evaluator's own reason — the decision recorded for the handler.",
				"server/internal/presentation/authz_middleware.go",
				"			roles, err := directory.RolesFor(r.Context(), identity.Username)\n"+
					"			if err != nil {\n"+
					"				core.WriteError(w, r, core.Wrap(err, core.KindInternal,\n"+
					"					\"The authorization directory could not answer.\"), logger)\n"+
					"				return\n"+
					"			}\n"+
					"			decision := domauthz.Authorize(identity, roles, permission)\n"+
					"			if !decision.Allowed {\n"+
					"				core.WriteError(w, r, core.Forbidden(\n"+
					"					\"forbidden — \"+decision.Reason), logger)\n"+
					"				return\n"+
					"			}\n"+
					"			next.ServeHTTP(w, r.WithContext(WithDecision(r.Context(), decision)))"),
			text("enforcement-composition",
				"The gates compose AFTER the identity middleware, and the composition is one wrapping at the composition root — the same shape topic 23's guarded whoami used, now with a policy layer inside: ``RequireIdentity(resolver, logger)(RequirePermission(directory, posts:write, logger)(handler))``. Order is load-bearing and the code makes it visible: identity first (401 for the unknown), policy second (403 for the known-but-refused), handler last (only for the allowed). Swap the order and the policy middleware would read an identity that is not there — it answers 401 defensively in that case, but the wiring should never arrange it. The routes that demonstrate this live are part of this topic's demo surface: an unauthenticated matrix story plus two guarded proofs, so the curl tour can show the 401-vs-403 boundary with real responses."),
			text("enforcement-vs-usecase",
				"When is a check middleware and when is it use-case logic? The test this guide applies: does the rule need the OBJECT? ``RequirePermission`` demands a capability of the route — answerable before any fetch, so middleware. \"May this caller edit THIS post\" needs the post's owner — still fine as middleware here because the ownership port fetches exactly that one fact by the route's ``{id}``. But rules that need the request BODY, the object's state transitions, or multi-object relationships (\"may I merge these two accounts\") belong in the use case, where the whole domain is in hand. The mistake to avoid is enforcement scattered across BOTH layers for the same question — one gate per question, at the layer that has the inputs."),
		),
		// ── 4. Scopes ───────────────────────────────────────────────
		section("scopes", "Scopes: narrowing the credential",
			"The other half of the model: what the CREDENTIAL was minted to do — checked before roles, so it can only ever shrink access, never widen it.",
			text("scopes-pitch",
				"Roles describe the USER; scopes describe the CREDENTIAL. The distinction earns its keep the first time a user needs one read-only token: a CI job that only reposts content, a partner integration that only reads, a phone app that should not hold full write. OAuth2 minted this vocabulary — ``posts:read`` riding an access token is a scope — and topic 23's identities already carry the field. The rule that makes scopes safe is structural: the scope check runs BEFORE the role check, and BOTH must say yes, so the effective access is the INTERSECTION of what the user may do and what the credential may ask. A role can never widen a narrowed credential; a credential can never grant what no role gives."),
			code("scopes-grammar-go", "go",
				"The whole grammar: exact permission, whole namespace, or everything — and an empty scope list permits nothing.",
				"server/internal/domain/authorization/policy.go",
				"func ScopePermits(scopes []string, perm Permission) bool {\n"+
					"	namespace, _, _ := strings.Cut(string(perm), \":\")\n"+
					"	for _, scope := range scopes {\n"+
					"		switch trimmed := strings.TrimSpace(scope); trimmed {\n"+
					"		case \"*\", namespace, string(perm):\n"+
					"			return true\n"+
					"		}\n"+
					"	}\n"+
					"	return false\n"+
					"}"),
			text("scopes-empty-deny",
				"Read the loop's empty case carefully, because it is the fail-closed property: a credential with NO scopes permits NOTHING — not \"everything\", nothing. That reading is a choice, and it is the right default for the same reason deny-by-default is: scope fields are added to systems late, half the credentials in the wild have empty ones, and the permissive reading (\"no scopes = full access\") turns every forgotten field into an over-privileged token. The narrowing also composes with ownership in the strict direction — the capability gate runs before the object rule, so the matrix's star row holds: kim the author cannot write even her OWN post through a read-only credential. \"But it's mine\" is an ownership argument, and the credential already said no."),
			text("scopes-vs-roles-decision",
				"Choosing between a scope restriction and a new role, honestly: if the limit describes WHAT KIND OF USER someone is, it is a role (a viewer role for the support team, minted once, assigned in bulk). If it describes WHAT THIS PARTICULAR CREDENTIAL MAY DO — one token, one key, one session — it is a scope (ada's full login versus ada's read-only API key; same user, same roles, different credentials). The test travels well: roles answer \"who is this person\", scopes answer \"what was this thing minted for\", and the matrix's kim row exists precisely because the two questions have different answers there."),
		),
		// ── 5. Ownership ────────────────────────────────────────────
		section("ownership", "Ownership: object-level rules",
			"The gate that needs the object in hand: owners, editors, and strangers get different answers on the same post — the rule RBAC alone cannot express.",
			text("ownership-pitch",
				"Every model so far answers questions about the CALLER. The last question needs the OBJECT: may ada edit grace's draft? RBAC alone cannot express \"authors may write their OWN posts\" — the permission says posts:write, the role says the holder has it, and nothing in either knows whose post is in hand. Object-level (or row-level) authorization closes the gap with one addition to the resource: an owner. The posts substrate from the earlier topics never needed one — public content, no writes guarded — so this topic extends the REAL posts.Post entity by embedding, adding exactly the column a production migration would add:"),
			code("ownership-entity-go", "go",
				"The posts substrate with its one new column — the real entity embedded, not forked.",
				"server/internal/domain/authorization/ownership.go",
				"type OwnedPost struct {\n"+
					"	posts.Post\n"+
					"	// Owner is the username of the account that created this post.\n"+
					"	Owner string\n"+
					"}"),
			text("ownership-ladder-pitch",
				"The decision function takes (identity, roles, action, post) and runs the gates in the fixed order the topic has been building — capability first (anonymous denied, scopes narrowed, roles consulted), then the OBJECT ladder, where each rung is a rule you can say in one sentence:"),
			list("ownership-ladder", false,
				"Reads are collection-level — any caller whose capability passed may read any post; private resources would add a visibility field, not a new gate.",
				"Admins override — the break-glass row: incident response must be able to fix anything, audited after, not blocked mid-fire.",
				"Editors write any post — the role's object-level MEANING, and the row that makes editor differ from author even though their permission lists are identical.",
				"Owners write their own — the object-level pass that answers the original question: yes, ada may edit her post because she made it.",
				"Strangers deny — capability held is not permission granted; ada the author is refused on grace's draft at the object gate, with a reason that says so.",
			),
			code("ownership-decide-go", "go",
				"The ladder as code: capability first, then one rung per rule, the refusal naming all three ways to have passed.",
				"server/internal/domain/authorization/ownership.go",
				"	capability := Authorize(identity, roles, action)\n"+
					"	if !capability.Allowed {\n"+
					"		return capability\n"+
					"	}\n"+
					"	switch {\n"+
					"	case action == PermPostsRead:\n"+
					"		return allow(\"reading is collection-level — any caller holding posts:read may read any post\")\n"+
					"	case hasRole(roles, RoleAdmin):\n"+
					"		return allow(\"admin overrides object ownership — the break-glass row\")\n"+
					"	case hasRole(roles, RoleEditor):\n"+
					"		return allow(\"editor writes span every post — the role's object-level meaning\")\n"+
					"	case post.Owner == identity.Username:\n"+
					"		return allow(fmt.Sprintf(\"object-level pass: %s owns %s\", post.Owner, post.ID))\n"+
					"	default:\n"+
					"		return deny(fmt.Sprintf(\"object-level refusal: %s owns %s and %s is neither its owner, an editor, nor an admin\",\n"+
					"			post.Owner, post.ID, identity.Username))\n"+
					"	}"),
			text("ownership-enforcement-shape",
				"Enforcement for object rules is the ``RequireOwnership`` middleware from the enforcement section: it reads the route's ``{id}``, asks the ownership port who made that post, runs the ladder, and answers 404 (unknown object), 403 (refused, with the rung named), or hands the handler a context carrying the decision. The handler does not re-ask — it echoes. One subtlety worth teaching while it is small: the comparison is by USERNAME, which works while owners are local accounts and becomes a mapping problem the day owners arrive from an external identity provider (their \"sub\" claim will not equal your username) — the ownership port is the seam where that translation belongs, which is exactly why ownership is a PORT here and not a field read."),
		),
		// ── 6. Deny by default ──────────────────────────────────────
		section("deny-by-default", "Deny by default",
			"The stance that makes the rest safe: absent rules refuse — unknown roles grant nothing, empty scopes permit nothing, unassigned users hold nothing, anonymous denies first.",
			text("deny-pitch",
				"Every model in this topic fails toward NO, and that is a design decision restated at each layer rather than one global flag. The registry map-misses unknown roles to an empty grant. The scope loop falls through an empty list to false. An authenticated user the directory has never seen answers an empty role list. The zero identity — anonymous — refuses before any rule runs. None of these is an error, all of them are DENIALS, and the reason sentences say which gate said no. The code is almost too small to quote, which is rather the point:"),
			code("deny-order-go", "go",
				"Authorize's fixed order: anonymous, then the credential's scopes, then the roles — each refusal naming its gate.",
				"server/internal/domain/authorization/policy.go",
				"	if Anonymous(identity) {\n"+
					"		return deny(\"not authenticated — authorization runs after authentication, and an anonymous caller holds nothing\")\n"+
					"	}\n"+
					"	if !ScopePermits(identity.Scopes, perm) {\n"+
					"		return deny(fmt.Sprintf(\"the credential's scopes [%s] do not permit %q — a role cannot widen what the credential narrowed\",\n"+
					"			strings.Join(identity.Scopes, \" \"), perm))\n"+
					"	}\n"+
					"	if !RolesGrant(roles, perm) {\n"+
					"		return deny(fmt.Sprintf(\"no role in [%s] grants %q — unassigned and unknown roles grant nothing\",\n"+
					"			joinRoles(roles), perm))\n"+
					"	}"),
			text("deny-why",
				"Why default-deny instead of default-allow with exceptions? Because the two stances fail in opposite directions and only one of them fails safely. A default-allow system drifts open: every new route is public until someone remembers to guard it, every refactor that drops a check is invisible, and the audit question \"what is public\" requires proving a negative across the whole codebase. A default-deny system drifts shut: an unguarded route answers 403, a forgotten rule surfaces as a user who cannot do their job — a loud, recoverable failure — and the audit reads the registry and the wiring, which is a bounded, reviewable surface. The matrix demo carries a control row for exactly this: guest, an authenticated identity with full-star scopes and NO role, denied on every action — the walking proof that holding a verified credential and an everything-scope still means nothing without a grant."),
			text("deny-boundary-honesty",
				"The stance has a boundary, and honesty demands naming it: deny-by-default is a property of the ENFORCEMENT path, not of the data. Routes wired without a gate are as public as ever — the stance does not reach them until the wiring adds it, which is why this guide's protected surfaces are guarded at the composition root where the wrapping is visible in one place. And the public-by-design surface (the content API itself, every demo story) is a DECISION recorded in wiring, not an oversight the model prevented. Default-deny is discipline at the seams, and the seams are the route table."),
		),
		// ── 7. In the wild ──────────────────────────────────────────
		section("in-the-wild", "In the wild",
			"What production actually runs: the same three gates wearing different names — plus where RBAC stops, and what the Go ecosystem offers when it does.",
			text("wild-pitch",
				"The model here — roles, scopes, ownership, deny by default — is the shape most production systems run, wearing different names. The splits and joins to recognize: OAuth2 scopes ARE the narrowing half (every access token carries them); GitHub's collaborator/organization roles ARE RBAC with object-level rules per repository; Google Workspace's admin console is RBAC plus attribute conditions; and database row-level security (PostgreSQL's RLS, topic 16's territory) is the ownership ladder pushed into the storage engine, where the WHERE owner = current_user clause replaces the middleware. When you meet a system's access model, look for the three questions: who is the user (roles), what was minted (scopes), and which object (ownership) — the naming varies, the questions do not."),
			text("wild-beyond-rbac",
				"Where RBAC stops, two families take over, and knowing their names prevents reinventing them badly. ABAC (attribute-based) decides from attributes of the subject, object, and environment — \"editors may publish DURING business hours\" is one attribute too many for a role ladder; the policy becomes rules over attributes instead of registry rows. ReBAC (relationship-based, Google's Zanzibar being the famous lineage) decides from the object GRAPH — \"the owner of a repository may delete issues in it\" — which is the ownership ladder generalized to arbitrary relations. Both earn their cost only when role explosion has already hurt; most systems run happily on RBAC plus object rules plus scopes, which is precisely this guide's stack. In Go, when the registry outgrows a map:"),
			list("wild-go-options", false,
				"Casbin — the de-facto Go authorization library: models as text (RBAC, ABAC, RESTful) plus adapters to many stores; powerful, with the config-is-code review burden that implies.",
				"OPA (Open Policy Agent) — policies in Rego evaluated as a service or sidecar; the choice when multiple services must share ONE policy source and answer the same question identically.",
				"Authzed/spiceDB and friends — Zanzibar-lineage ReBAC engines when relations, not attributes, are the shape of the rules.",
				"Hand-rolled like this guide — a registry, an evaluator, table tests; exactly right while the policy fits on one screen, and the migrations to the heavier tools start from the same ports.",
			),
			text("wild-audit",
				"One habit travels from this guide to any system you build: refusals are LOGGED with their reason. The evaluator returns a reason with every deny, the 403 envelope carries it, and the middleware logs it with the request id — so \"why can't I edit this\" is answered by grep, not by archaeology in a debugger. In production the same sentences become the audit trail's atoms: every refusal recorded as who, what, which object, which gate — the difference between an access-review checklist that takes an afternoon and one that takes a quarter. Authorization is where security becomes legible; keep it legible."),
		),
		// ── 8. Terminal companion ───────────────────────────────────
		section("terminal-companion", "Terminal companion: the authz curl tour",
			"The matrix by curl, the guarded routes' honest 401s, and the 401-versus-403 boundary as live responses — every command against the running stack on :8100.",
			text("term-authz-pitch",
				"With the stack up (server on :8100), the tour walks topic 24's surface from the shell. The matrix endpoint is one unauthenticated request — the family's story shape — and the interesting reading is the beats array: each names a teaching moment (owner vs editor vs stranger, scopes narrowing roles, deny by default) with the evaluator's own verdicts inline. The guarded routes answer 401 without a credential, exactly as topic 23's whoami did — the policy layer underneath is pinned by this topic's Go tests, which compose the real gates and assert the 403-for-authenticated-refusal mapping the wire contract demands:"),
			code("term-matrix", "bash", "", "",
				"# the full matrix — roster, principals, every cell with its reason\n"+
					"curl -s localhost:8100/api/demo/authz/matrix | python3 -m json.tool | head -30\n"+
					"# one beat in one line — the stranger's refusal, derived live\n"+
					"curl -s localhost:8100/api/demo/authz/matrix | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d[\"beats\"][4][\"detail\"])'"),
			code("term-guarded", "bash", "", "",
				"# the guarded route with no credential — the identity middleware's 401\n"+
					"curl -si localhost:8100/api/demo/authz/guarded | head -12\n"+
					"# a bogus bearer changes nothing — still the uniform 401, no policy consulted\n"+
					"curl -si -H 'Authorization: Bearer not-a-real-token' localhost:8100/api/demo/authz/guarded | head -8\n"+
					"# the object-level gate on a seeded post — unknown ids answer 404 first\n"+
					"curl -si localhost:8100/api/demo/authz/guarded/post/p-sessions | head -8"),
			code("term-content", "bash", "", "",
				"# the topic on the wire — order 24 in the state-auth group\n"+
					"curl -s localhost:8100/api/content | python3 -c 'import json,sys; d=json.load(sys.stdin); print([t[\"id\"] for t in d[\"topics\"] if t[\"group\"]==\"state-auth\"])'\n"+
					"curl -s localhost:8100/api/content/authorization | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d[\"title\"], \"·\", d[\"sectionCount\"], \"sections\")'"),
			text("term-authz-coda",
				"Read the tour as three proofs. The matrix beat at index four is the owner-editor-stranger triple — ada allowed on her own post, grace the editor allowed on ada's, ada the stranger refused on grace's, each verdict the evaluator's own sentence. The guarded route's 401 (with and without a bogus token) is the boundary held: no identity, no policy, one uniform envelope. And the content check is topic 24 standing in the registry — the last state-auth topic before security hardens everything this one left deliberate. Authentication said who; authorization said what; the rest of the guide can now guard things without re-deriving either."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"authorization",
		"Authorization from Go",
		"What the caller may do: RBAC as one registry table, enforcement middleware that builds on the auth "+
			"identity (403 for the known-but-refused, never 401), scopes as the narrowing half a role can never "+
			"widen, object-level ownership over the posts substrate where owners, editors, and strangers get "+
			"different answers, deny by default at every gate — with the full roles × scopes × ownership matrix "+
			"run live over the seeded directory, and the terminal's curl tour.",
		"state-auth",
		24,
		sections,
	)
}
