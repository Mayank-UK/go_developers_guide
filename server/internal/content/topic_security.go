package content

import (
	"errors"
	"fmt"
)

// This file is topic 25 security (group state-auth, curriculum order
// 25): the threat-model habit that orders every other decision, the
// security-header policy a JSON API can carry with the middleware that
// stamps it (demo security:lab), CORS as the browser's read contract,
// rate limiting as a per-client token bucket over golang.org/x/time/rate
// with the typed 429 envelope, the injection family in Go terms
// (parameterized SQL, html/template auto-escaping, path confinement),
// CSRF and SSRF, secrets at boot, the OWASP API Top 10 mapped onto the
// guide's own answers, and the PII/GDPR substrate — classification at
// definition, masked serving, an access-log type with no room for
// values, and erasure with a downstream audit (demo security:pii). The
// terminal companion closes with the curl tour: headers off the wire,
// both stories, and a loop that trips the real 429.

// securityTopic builds topic 25.
func securityTopic() (Topic, error) {
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
		// ── 1. The threat-model habit ───────────────────────────────
		section("threat-model", "The threat-model habit",
			"Security is not a feature you add; it is a set of questions you keep answering. The habit: before every endpoint, ask who can reach it, what they control, and what breaks.",
			text("threat-model-pitch",
				"Every topic before this one built things; this one defends them, and the defense starts with a HABIT rather than a library. Before you write a handler, answer four questions in order — the answers, not the tooling, decide whether the code is defensible. Skipping them is how teams buy a WAF and still ship an endpoint that mutates any row by id, or rate-limit at the edge and still leak PII into the logs behind it. The questions compose into a threat model, and the model — not a checklist — is what tells you which of this topic's pieces your service actually needs."),
			list("threat-model-questions", true,
				"Who can reach this code — the public internet, authenticated users, one partner's egress range, only other services on the mesh? The answer sizes every other control: a route only your own jobs call needs different armor than one curl can hit.",
				"What do they control — the whole request? Route params, query strings, headers, bodies, and the TIMING of requests are all attacker-writable; treat every one as hostile input (the validation topic's stance, now with consequences).",
				"What breaks if it fails — data confidentiality, integrity, availability, or money? Rank the damage honestly; an endpoint that reads public data and cannot corrupt anything deserves far less machinery than one that writes or reveals personal data.",
				"What is the blast radius — one user's session, one tenant, the whole database? Smaller radius means cheaper containment; the answer tells you where the next control (authz checks, rate keys, erasure scope) has to sit.",
			),
			text("threat-model-stride",
				"The classic vocabulary for the third question is STRIDE — Spoofing (claiming to be someone else: topic 23's credentials), Tampering (modifying data or a token in flight or at rest), Repudiation (acting without a trace: the audit-log habit from the data topics), Information disclosure (leaking data to the wrong reader — CORS misconfigurations, over-fetching, logs), Denial of service (exhausting the service: this topic's rate limiting), and Elevation of privilege (topic 24's whole subject). You do not need the acronym on a wall; you need the reflex that every input path gets mapped to the attacks that fit it. This topic walks the reflex around the HTTP edge of a JSON API — the surface every Go service actually ships:"),
			text("threat-model-surface",
				"The substrate is real and in-repo. ``internal/domain/security`` holds the header policy and the per-client token-bucket limiter plus the PII substrate (classification, masked views, erasure, an access-log entry type); ``internal/presentation`` holds the two NEW middlewares that apply the policy — ``security_headers.go`` stamps the header set, ``security_ratelimit.go`` enforces the bucket with the typed 429 envelope; ``internal/domain/teaching`` holds the CORS and web-safety exhibits (the guide's own client is same-origin and its API never renders HTML or fetches caller URLs, so those live as compilable, tested exhibits — the shape to lift the day a real counterpart appears). Two story endpoints walk the whole surface under ``/api/demo/security/*``, and the routes themselves are wrapped in their own policy — the one topic whose demo answers back with a 429 if you love it too much."),
		),
		// ── 2. Security headers ─────────────────────────────────────
		section("security-headers", "Security headers",
			"Five response headers, each closing a named browser-side attack — stamped by middleware before any handler runs, so error envelopes carry them too.",
			text("headers-pitch",
				"A JSON API talks to two audiences: the client that asked, and the BROWSER machinery around it. The browser is an eager interpreter — it sniffs content types, follows frames, attaches referrers — and security headers are the one-sentence instructions that rein it in. Each header closes a named attack, which is why the policy carries its WHY beside every rule: ``Content-Security-Policy`` for an API can refuse EVERY source (an API response renders nothing, so there is nothing to allow), ``Strict-Transport-Security`` commits clients to https for a year, ``X-Content-Type-Options: nosniff`` stops a JSON error body from being reinterpreted as a script, ``X-Frame-Options: DENY`` kills clickjacking, and ``Referrer-Policy: no-referrer`` keeps URLs — query strings and all — from traveling to the next site. The policy is the domain's own type, rule by rule:"),
			code("headers-policy-go", "go",
				"The whole policy as the guide's domain states it: header, value, and the attack each rule closes.",
				"server/internal/domain/security/headers.go",
				"var APISecurityHeaders = []HeaderRule{\n"+
					"	{\n"+
					"		Header: \"Content-Security-Policy\",\n"+
					"		Value:  \"default-src 'none'; frame-ancestors 'none'\",\n"+
					"		Why:    \"an API response renders nothing: refuse every source and every framing, so a content-sniff detour has nothing to execute and the response cannot be framed\",\n"+
					"	},\n"+
					"	{\n"+
					"		Header: \"Strict-Transport-Security\",\n"+
					"		Value:  \"max-age=31536000; includeSubDomains\",\n"+
					"		Why:    \"once a client has seen the site over https, it refuses plain http for a year, subdomains included — a stripped-cookie replay over http dies here\",\n"+
					"	},\n"+
					"	{\n"+
					"		Header: \"X-Content-Type-Options\",\n"+
					"		Value:  \"nosniff\",\n"+
					"		Why:    \"the browser must not second-guess application/json as something executable — content-type sniffing is how a JSON error body becomes a script\",\n"+
					"	},\n"+
					"	// …X-Frame-Options DENY against clickjacking, Referrer-Policy no-referrer\n"+
					"}"),
			text("headers-middleware-pitch",
				"Application is a middleware because the headers must ride EVERY response the route serves — success bodies and error envelopes alike, which means they are set BEFORE the wrapped handler runs, not assembled in each handler by hand. ``Header.Set`` (not Add) makes the middleware idempotent: double-wrapping leaves one clean copy of each header, never a folded list. The honest caveat is placement: wired per-route — this guide's pattern, because the production chain's composition lines are quoted teaching material from topic 08 — the headers ride that route's responses but not the mux's own unmatched-path 404s; wrapping the whole router covers those too, at the cost of one policy for every route including any that needs a looser CSP. The middleware is six lines:"),
			code("headers-middleware-go", "go",
				"The stamping middleware: policy in, headers on every response — set before the handler runs.",
				"server/internal/presentation/security_headers.go",
				"func SecurityHeaders(policy []domsecurity.HeaderRule) Middleware {\n"+
					"	return func(next http.Handler) http.Handler {\n"+
					"		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n"+
					"			for _, rule := range policy {\n"+
					"				w.Header().Set(rule.Header, rule.Value)\n"+
					"			}\n"+
					"			next.ServeHTTP(w, r)\n"+
					"		})\n"+
					"	}\n"+
					"}"),
			text("headers-honesty",
				"Two honesty notes the demo proves live. HSTS over plain http is INERT by specification — browsers only honor the header over TLS — so stating it in the dev stack costs nothing and turns correct the day a TLS terminator sits in front; carrying it from day one is cheaper than remembering it later. And CSP is a BROWSER contract, not a server-side filter: it constrains what a compromised response could make a browser DO, which is why an API's ``default-src 'none'`` is both trivially cheap and exactly right — the strictest policy an API can carry is also the easiest one to justify. What headers never do is protect non-browser clients; curl never sniffed anything. They armor the browser audience, and the demo shows them off the wire:"),
			demo("security:lab", "The security lab: the header set and a live rate-limit window",
				"One request walks the topic's whole edge: the five-header policy this very response carries (curl -i and read them off the wire), then a FRESH token bucket spent in front of you — the burst (5 at 10/s) lands instantly, request #6 hits the 429 boundary with the bucket's own retry hint, the cooperative Wait demonstrates recovery at exactly the sustained rate, a second client proves per-key isolation, and the closing beat names the limiter wrapping the route you just called — hammer it and the typed 429 rate_limited envelope arrives on YOUR request with x-request-id stamped."),
		),
		// ── 3. CORS ─────────────────────────────────────────────────
		section("cors", "CORS: the browser's read contract",
			"Same-origin by default, cross-origin by declaration — CORS governs what a browser may READ, and the server's whole job is to state the policy truthfully.",
			text("cors-pitch",
				"The same-origin policy is the browser's default: a page at ``https://app.example`` may not read a response from ``https://api.other`` — the request can be SENT, but the response is invisible. CORS is the opt-out protocol layered on top: the server DECLARES, in response headers, which origin may read what, and the browser ENFORCES the promise. Two facts follow, and both surprise people. First, CORS is a READ permission, not a send permission — a curl or any non-browser client ignores it entirely, which is why CORS is never an access control; authorization (topic 24) happens server-side regardless. Second, the server still receives and processes cross-origin requests it has not allowed — the BLOCKING happens in the browser after the response arrives, so side-effecting endpoints need CSRF protection (later this topic) rather than a CORS header as their actual defense."),
			text("cors-preflight",
				"The protocol has two request kinds. A SIMPLE request — classic methods, no custom headers — is sent directly, and the browser checks ``Access-Control-Allow-Origin`` on the RESPONSE before handing it to the page. Anything richer (a ``Content-Type: application/json`` POST, custom headers like this guide's ``x-request-id``) triggers a PREFLIGHT: the browser first sends ``OPTIONS`` with ``Access-Control-Request-Method`` and ``Access-Control-Request-Headers``, and the page's JavaScript only gets to send the real request if the preflight ANSWER allows exactly that method and those headers. Preflights carry no credentials and no body, so answering them is cheap — and must happen BEFORE any handler logic, which is middleware-shaped work. The guide's exhibit answers both kinds:"),
			code("cors-middleware-go", "go",
				"The CORS exhibit: preflights answered at the door, the one allowed origin named on every response, unrelated origins get no promise.",
				"server/internal/domain/teaching/cors.go",
				"func CORSMiddleware(config CORSConfig) func(http.Handler) http.Handler {\n"+
					"	return func(next http.Handler) http.Handler {\n"+
					"		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n"+
					"			origin := r.Header.Get(\"Origin\")\n"+
					"			if origin != \"\" && origin != config.Origin && config.Origin != \"*\" {\n"+
					"				// An origin we do not serve: no CORS headers, so the\n"+
					"				// browser blocks the read. The request itself may still\n"+
					"				// proceed for same-origin callers — CORS governs READS\n"+
					"				// by the browser, not who may send.\n"+
					"				next.ServeHTTP(w, r)\n"+
					"				return\n"+
					"			}\n"+
					"			w.Header().Set(\"Access-Control-Allow-Origin\", config.Origin)\n"+
					"			if config.AllowCredentials {\n"+
					"				w.Header().Set(\"Access-Control-Allow-Credentials\", \"true\")\n"+
					"				w.Header().Set(\"Vary\", \"Origin\")\n"+
					"			}\n"+
					"			if r.Method == http.MethodOptions {\n"+
					"				w.Header().Set(\"Access-Control-Allow-Methods\", config.Methods)\n"+
					"				w.Header().Set(\"Access-Control-Allow-Headers\", config.AllowHeaders)\n"+
					"				w.WriteHeader(http.StatusNoContent)\n"+
					"				return\n"+
					"			}\n"+
					"			next.ServeHTTP(w, r)\n"+
					"		})\n"+
					"	}\n"+
					"}"),
			text("cors-honesty",
				"The rules that keep a CORS config honest, each closing a real bug class. Allow ONE origin (or a checked list — exact match, never prefix: ``evil-example.com`` must not ride an ``example.com`` policy). Never combine ``Access-Control-Allow-Origin: *`` with ``Allow-Credentials: true`` — the wildcard-with-credentials combination is the one browsers reject and attackers love, because it would let any page read authenticated responses. Add ``Vary: Origin`` whenever the answer depends on the request's origin, or an intermediary cache will serve one origin's promise to another. And scope the policy to the methods and headers the route actually serves — an allowlist that names everything is a smell, not generosity. The guide's own client needs none of this: it is served through the same-origin Vite proxy in development, so the exhibit exists for the day a route genuinely goes cross-origin."),
		),
		// ── 4. Rate limiting ────────────────────────────────────────
		section("rate-limiting", "Rate limiting: the token bucket",
			"One bucket per client, refilled continuously: burst is the budget you may spend at once, rate is the refill speed — and the 429 that refuses you carries the bucket's own retry math.",
			text("ratelimit-why",
				"Availability is a security property, and rate limiting is its edge control: without one, any client — hostile or just accidentally looping — can exhaust connections, CPU, or your database with request volume alone (OWASP calls it unrestricted resource consumption). The naive design is a fixed-window counter — 100 requests per minute — and it MISLEADS at exactly the boundary: a client that spends 100 at 11:59:59 and another 100 at 12:00:00 pushes 200 through inside one second, because the window RESET at its edge rather than charging elapsed time. The honest alternative is the TOKEN BUCKET, and Go ships it at ``golang.org/x/time/rate`` (pinned in go.mod): a bucket holds up to BURST tokens, every allowed request spends one, and tokens refill CONTINUOUSLY at RATE per second — so an exhausted client recovers one request's worth of allowance every 1/rate seconds, and there is no edge to game."),
			text("ratelimit-semantics",
				"The ``rate.Limiter`` semantics worth knowing cold, because the library is small and every method answers a different question. ``Allow`` is the non-blocking verdict (spend a token if one is available). ``Reserve`` claims a token and answers WHEN it would be usable — the honest way to refuse: a zero delay means allowed now, any other delay means over budget, and ``Cancel`` returns the token so a refused request consumes NOTHING (the hint you publish stays true for the next requester). ``Wait`` is the cooperative counterpart — block until the bucket can afford one token, the right shape for a client that would rather slow down than be refused; it takes a ``context.Context`` first because it can block. One ``rate.Limiter`` is safe for concurrent use, but it is ONE budget — the per-client gate is a limiter per key behind a mutex, created on first sight:"),
			code("ratelimit-allow-go", "go",
				"The per-client gate and its refusal path: Reserve for the verdict, Cancel so refusals consume nothing.",
				"server/internal/domain/security/ratelimit.go",
				"type ClientRateLimiter struct {\n"+
					"	policy  RatePolicy\n"+
					"	mu      sync.Mutex\n"+
					"	buckets map[string]*rate.Limiter\n"+
					"}\n"+
					"\n"+
					"func (l *ClientRateLimiter) Allow(key string) RateDecision {\n"+
					"	reservation := l.bucket(key).Reserve()\n"+
					"	if !reservation.OK() {\n"+
					"		// OK=false means a zero burst — construction forbids it; this\n"+
					"		// branch is defensive, and refuses rather than trusts.\n"+
					"		return RateDecision{Allowed: false}\n"+
					"	}\n"+
					"	delay := reservation.Delay()\n"+
					"	if delay == 0 {\n"+
					"		return RateDecision{Allowed: true}\n"+
					"	}\n"+
					"	reservation.Cancel()\n"+
					"	return RateDecision{Allowed: false, RetryAfter: delay}\n"+
					"}"),
			text("ratelimit-keying",
				"Keying is the design decision with teeth: one shared bucket throttles every client as one abusive mass, while a spoofable key lets any caller MINT a fresh bucket per request and walk straight past the limit. The guide's key derivation is deliberately minimal — the authenticated identity when the auth middleware has run (one bucket per user, whatever device they call from), else the remote address with its port stripped. ``X-Forwarded-For`` is deliberately NOT read: the header is client-writable, so trusting it behind no proxy is the spoofing bug just described; behind a TRUSTED proxy, derive the real client IP at the edge and set ``RemoteAddr``, or agree on an explicit trusted-hop contract — the code trusts only what the transport guarantees:"),
			code("ratelimit-key-go", "go",
				"The per-client key: identity when present, transport address otherwise — forward headers are not trust.",
				"server/internal/presentation/security_ratelimit.go",
				"func ClientKey(r *http.Request) string {\n"+
					"	if identity, ok := IdentityFromContext(r.Context()); ok && identity.Username != \"\" {\n"+
					"		return \"user:\" + identity.Username\n"+
					"	}\n"+
					"	host, _, err := net.SplitHostPort(r.RemoteAddr)\n"+
					"	if err != nil {\n"+
					"		host = r.RemoteAddr\n"+
					"	}\n"+
					"	return \"ip:\" + host\n"+
					"}"),
			text("ratelimit-429-pitch",
				"The refusal itself is part of the contract this guide has been building since topic 04: a TYPED 429, not a bare status. The envelope carries the taxonomy's ``rate_limited`` code and a request id (the log join key), the response carries the standard ``Retry-After`` header (ceil seconds), and the details ride a machine-readable ``retryAfterMs`` plus the policy string — burst AND rate, because a client cannot back off correctly against a contract that states only one number. The message never echoes the client key: which user or IP tripped lives in the server's own logs, never in a response whose audience the triggering client partly controls. State both numbers in your own API docs for the same reason:"),
			code("ratelimit-429-go", "go",
				"The 429 path in the route middleware: typed envelope, Retry-After, and the policy stated in the details.",
				"server/internal/presentation/security_ratelimit.go",
				"			decision := limiter.Allow(ClientKey(r))\n"+
					"			if decision.Allowed {\n"+
					"				next.ServeHTTP(w, r)\n"+
					"				return\n"+
					"			}\n"+
					"			retrySeconds := int(math.Ceil(decision.RetryAfter.Seconds()))\n"+
					"			w.Header().Set(RetryAfterHeader, fmt.Sprintf(\"%d\", retrySeconds))\n"+
					"			refusal := core.RateLimited(fmt.Sprintf(\n"+
					"				\"rate limit exceeded — the sustained allowance refills continuously; retry after %s\",\n"+
					"				decision.RetryAfterText()))\n"+
					"			refusal.Details = map[string]any{\n"+
					"				\"retryAfterMs\": decision.RetryAfter.Milliseconds(),\n"+
					"				\"policy\":       limiter.Policy().String(),\n"+
					"			}\n"+
					"			core.WriteError(w, r, refusal, logger)"),
			text("ratelimit-honesty",
				"The in-memory map is a NAMED simplification, not a hidden one: it grows one bucket per distinct key, which is fine while keys are bounded (authenticated users, a known set of callers) and a slow leak on an open endpoint behind address keying — a production deployment facing unbounded keys pairs it with idle-bucket eviction or moves the buckets into a shared store so every replica enforces the SAME budget (Redis with a TTL per bucket is the usual shape; the messaging topic's consumer machinery would recognize the pattern). What does NOT change across those deployments is the semantics: burst as the instant budget, rate as the refill, refusals that consume nothing, and recovery that is continuous rather than window-shaped — the four properties the lab demo spends in front of you."),
		),
		// ── 5. The injection family ─────────────────────────────────
		section("injection-family", "The injection family",
			"One bug shape, three Go surfaces: untrusted text treated as CODE — SQL spliced into a query, HTML rendered raw, a path resolved outside its base. Go's defaults close all three, when you use them.",
			text("injection-pitch",
				"Injection is the same bug every time: data crosses into a context that INTERPRETS it — a SQL engine, an HTML parser, a filesystem — and the attacker's bytes change what the program does instead of what it stores. The defense is likewise the same every time: keep data as DATA by handing it to the interpreter through a parameter channel, never by splicing it into the instruction text. Go is unusually well-armed here because the standard library's defaults are safe: ``database/sql`` and pgx speak PLACEHOLDERS natively, ``html/template`` ESCAPES by default, and path handling confines with ``filepath.Clean`` — the bugs that remain are the places where someone reaches for ``fmt.Sprintf``, ``template.HTML``, or ``filepath.Join`` with attacker bytes and undoes the default."),
			text("injection-sqli",
				"SQL injection in Go terms: the database driver sends the query TEXT and the parameters SEPARATELY — the engine never parses your ``$1`` as anything but a placeholder, so a tag reading ``'; DROP TABLE posts; --`` arrives as a string VALUE to match, not as SQL to run. Every query in this guide's real repository is placeholdered, including the awkward-looking one below — the ``$1 = '' OR ...`` pattern is the idiom for an OPTIONAL filter, and it stays inject-proof precisely because the tag is a parameter wherever it appears. The one ``fmt.Sprintf`` in the same function interpolates an ORDER BY fragment that an allowlist switch produced — never user bytes — which is the honest rule: interpolation into SQL is legal only when the interpolated value came from YOUR code's fixed vocabulary:"),
			code("sqli-parameterized-go", "go",
				"The real repository's filter query: the user-controlled tag rides $1 — even as an optional filter.",
				"server/internal/data/posts/pg.go",
				"	var total int\n"+
					"	if err := tx.QueryRow(ctx, `\n"+
					"		SELECT count(*) FROM posts p\n"+
					"		WHERE ($1 = '' OR p.id IN (SELECT post_id FROM post_tags WHERE tag_name = $1))`,\n"+
					"		query.Tag).Scan(&total); err != nil {\n"+
					"		return posts.PostPage{}, unavailable(err)\n"+
					"	}"),
			code("sqli-allowlist-go", "go",
				"The only interpolated fragment in the file: an allowlist switch, fixed vocabulary, no user bytes.",
				"server/internal/data/posts/pg.go",
				"func listOrderBy(sort string) string {\n"+
					"	switch sort {\n"+
					"	case posts.SortOldest:\n"+
					"		return \"p.created_at ASC, p.id ASC\"\n"+
					"	case posts.SortTitle:\n"+
					"		return \"lower(p.title) ASC, p.id ASC\"\n"+
					"	default: // SortNewest — the API's default ordering.\n"+
					"		return \"p.created_at DESC, p.id DESC\"\n"+
					"	}\n"+
					"}"),
			text("injection-xss",
				"Cross-site scripting is SQL injection's markup twin: the attacker's bytes must reach the browser as MARKUP instead of text. Go's ``html/template`` closes it by DEFAULT — the engine tracks each value's context (element body, attribute, URL, script) and escapes for exactly that context, so a comment reading ``<script>fetch('/steal')</script>`` renders as inert ``&lt;script&gt;`` text. The escape hatch is deliberate and loud: ``template.HTML(value)`` marks a string as trusted markup, and every real XSS bug in a ``html/template`` codebase traces to someone wrapping attacker-influenced bytes in it. The sibling warning: ``text/template`` does NONE of this — it is for building strings, not pages, and rendering HTML through it is the same as no engine at all. The guide's exhibit renders one hostile comment through the safe engine:"),
			code("xss-template-go", "go",
				"The XSS exhibit: untrusted input through html/template — escaped for its context by default.",
				"server/internal/domain/teaching/websafety.go",
				"func RenderComment(author, body string) (string, error) {\n"+
					"	page, err := template.New(\"comment\").Parse(\n"+
					"		\"<div class=comment><b>{{.Author}}</b><p>{{.Body}}</p></div>\")\n"+
					"	if err != nil {\n"+
					"		return \"\", fmt.Errorf(\"parse comment template: %w\", err)\n"+
					"	}\n"+
					"	var builder strings.Builder\n"+
					"	if err := page.Execute(&builder, map[string]string{\"Author\": author, \"Body\": body}); err != nil {\n"+
					"		return \"\", fmt.Errorf(\"execute comment template: %w\", err)\n"+
					"	}\n"+
					"	return builder.String(), nil\n"+
					"}"),
			text("injection-traversal",
				"Path traversal rounds out the family: a download or upload route joins user input to a base directory, and ``../../etc/passwd`` walks out of it. The guard is two moves — reject outright any request containing ``..`` (a legitimate download URL never needs it; a conservative allowlist beats a clever parser), then ``filepath.Clean`` the joined path and verify it still sits under the base. Cleaning alone is not enough — encodings and doubled separators survive naive checks — and even the two moves confine the STRING: a symlink inside the base pointing outside needs ``filepath.EvalSymlinks`` on top. On input handling generally, the validation topic's layering applies twice over here: hostile input is the assumption, and injection guards are the last line behind it:"),
			code("traversal-guard-go", "go",
				"The traversal guard: '..' refused outright, then Clean plus a base-prefix confinement test.",
				"server/internal/domain/teaching/websafety.go",
				"func ResolveWithin(base, requested string) (string, error) {\n"+
					"	if strings.Contains(requested, \"..\") {\n"+
					"		return \"\", fmt.Errorf(\"download path %q contains '..' — rejected\", requested)\n"+
					"	}\n"+
					"	cleaned := filepath.Clean(filepath.Join(base, requested))\n"+
					"	absoluteBase := filepath.Clean(base)\n"+
					"	if cleaned != absoluteBase && !strings.HasPrefix(cleaned, absoluteBase+string(filepath.Separator)) {\n"+
					"		return \"\", fmt.Errorf(\"download path %q escapes the allowed base\", requested)\n"+
					"	}\n"+
					"	return cleaned, nil\n"+
					"}"),
		),
		// ── 6. CSRF and SSRF ────────────────────────────────────────
		section("csrf-ssrf", "CSRF and SSRF",
			"Two attacks that ride TRUST instead of breaking it: the browser's automatic credentials, and the server's own network position.",
			text("csrf-pitch",
				"CSRF (cross-site request forgery) exploits the browser's habit of ATTACHING credentials automatically: the user is logged in to your API at ``api.example``, opens an attacker's page, and that page fires a cross-site request — the cookie rides along because that is what browsers do — so the request arrives fully authenticated and entirely unwanted. Three defenses stack, in order of weight. ``SameSite=Lax`` cookies (this guide's sessions since topic 23) stop the classic delivery outright: the browser will not attach the cookie to cross-site POSTs. A CSRF TOKEN — a per-session random the page must echo in a header or body — makes the request forgeable-proof even where SameSite does not reach (old browsers, subdomain trust edges). And ``GET`` staying SIDE-EFFECT-FREE is a rule worth its own bullet: no token dance can protect a state-changing GET that a mere image tag can trigger."),
			text("csrf-json-honesty",
				"The honest nuance for a JSON API: a cross-origin ``application/json`` POST triggers a CORS PREFLIGHT, and a server that answers no preflight effectively blocks browser-side CSRF from OTHER origins — which is real protection, but it leans on your CORS config being right (the previous section) and on nothing else ever accepting form-encoded bodies. Treat it as defense-in-depth, not the plan: the plan is SameSite cookies plus a token on every mutating route, plus methods that mean what they say. None of it applies to non-browser callers — an API key in a header is never auto-attached by anything, which is one quiet argument for the API-key shape the authentication topic gave machines."),
			text("ssrf-pitch",
				"SSRF (server-side request forgery) is the mirror image: the attacker does not need your credentials, they need your LOCATION — the ability to make your server fetch a URL of their choosing. Webhook verifiers, URL previewers, and avatar importers are the classic doors, and behind your egress sit things no internet client should ever name: internal admin endpoints on loopback, cloud metadata services at ``169.254.169.254`` that hand out instance credentials, and private-range services that trust network position. The guard is an ALLOWLIST posture at the URL's every layer: scheme (http/https only — ``file://`` and stranger schemes are code-execution shaped), no embedded credentials in the authority, and the host checked against the internal network by name AND by parsed IP. The guide's exhibit is the starter version of that guard:"),
			code("ssrf-guard-go", "go",
				"The SSRF exhibit: scheme allowlist, no credentials, internal hosts refused by name and by IP.",
				"server/internal/domain/teaching/websafety.go",
				"	parsed, err := url.Parse(raw)\n"+
					"	if err != nil {\n"+
					"		return \"\", fmt.Errorf(\"parse outbound url: %w\", err)\n"+
					"	}\n"+
					"	if parsed.Scheme != \"http\" && parsed.Scheme != \"https\" {\n"+
					"		return \"\", fmt.Errorf(\"outbound url scheme %q refused — http/https only\", parsed.Scheme)\n"+
					"	}\n"+
					"	if parsed.User != nil {\n"+
					"		return \"\", fmt.Errorf(\"outbound url carries credentials in its authority — refused\")\n"+
					"	}\n"+
					"	// …host extracted, empty hosts refused, then:\n"+
					"	if blockedHosts[strings.ToLower(host)] {\n"+
					"		return \"\", fmt.Errorf(\"outbound url host %q is an internal address — the server does not fetch on its own network's behalf\", host)\n"+
					"	}\n"+
					"	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {\n"+
					"		return \"\", fmt.Errorf(\"outbound url host %q is an internal address — refused\", host)\n"+
					"	}\n"+
					"	return parsed.String(), nil"),
			text("ssrf-honesty",
				"Where the starter guard ends — named, not hidden. A DNS name can RESOLVE to an internal address (resolve and check the IP, not just the name), redirects can HOP to a refused host after the first hop passed (follow manually and re-check each hop, or refuse redirects), and the full private-CIDR sweep wants ``net/netip`` spans rather than the loopback/link-local checks above. The production shape is a small egress policy module with its own tests — the same discipline as the exhibit, one layer deeper — and, where the platform allows, an egress network rule that makes the whole class of targets UNREACHABLE rather than merely refused. Defense at the network beats defense in the loop."),
		),
		// ── 7. Secrets ──────────────────────────────────────────────
		section("secrets", "Secrets",
			"Signing keys, seal keys, database URLs: minted or loaded at boot, held in memory, never in the repo, never in a log line.",
			text("secrets-pitch",
				"A secret is any value whose disclosure breaks a control you rely on: the JWT signing key (forge any identity), the cookie seal key (mint session ids), the database URL's password, the OAuth client secret. The lifecycle discipline is short and unforgiving. SOURCED from the environment or a secret store at boot — never a literal in code, never a file in the repo, never a build flag; the guide's own demo substrate mints per-process randoms because a teaching stack has nothing worth forging, and says so where it does it. HELD in memory by the one component that uses it — not copied into structs that travel. USED through the narrowest interface the crypto needs (a signer, a seal function). And never LOGGED — which is enforced structurally in this guide by the same trick the PII section uses for access logs: types with no room for the value."),
			code("secrets-mint-go", "go",
				"The auth console's boot-time minting: 32 random bytes per secret, handed straight to the engines.",
				"server/internal/data/auth/console.go",
				"	jwtSecret := make([]byte, 32)\n"+
					"	if _, err := rand.Read(jwtSecret); err != nil {\n"+
					"		return nil, fmt.Errorf(\"mint jwt secret: %w\", err)\n"+
					"	}\n"+
					"	cookieSecret := make([]byte, 32)\n"+
					"	if _, err := rand.Read(cookieSecret); err != nil {\n"+
					"		return nil, fmt.Errorf(\"mint cookie seal secret: %w\", err)\n"+
					"	}\n"+
					"\n"+
					"	directory := NewDirectory(users)\n"+
					"	sessionStore := NewSessionMemory()\n"+
					"	keyStore := NewAPIKeyMemory()\n"+
					"	refreshStore := NewRefreshMemory()\n"+
					"	codeStore := NewCodeMemory()\n"+
					"\n"+
					"	tokens := domauth.NewTokenService(jwtSecret)\n"+
					"	sessions := domauth.NewSessionService(directory, sessionStore, hasher, cookieSecret)"),
			text("secrets-rotation",
				"Rotation and revocation are the two moves that make a leaked secret survivable, and both want DESIGN room from day one. Rotation means a new secret can take over without invalidating every outstanding artifact at once — the standard shape is keyed secrets (a key id rides inside each token or sealed value saying which secret signed it) so the old key verifies while the new one mints, until a cutover moment retires it; this guide's single-secret engines are honest about not having that room, and the JWT topic's ``kid`` claim is where it would go. Revocation for SYMMETRIC secrets is rotation with urgency — there is no kill switch for a leaked HMAC key except replacing it everywhere it is configured, which is why secret stores with per-environment scoping beat environment variables in files at scale. The audit that keeps all of it honest is mechanical: grep the repo for anything high-entropy, and read your own logs for the values you promised never to print."),
		),
		// ── 8. OWASP API Top 10 ─────────────────────────────────────
		section("owasp-api-top-10", "The OWASP API Top 10",
			"The industry's field notes on how APIs actually break — and where each item already has an answer in this guide, or will in a later topic.",
			text("owasp-pitch",
				"The OWASP API Security Top 10 (2023 edition) is not a standard to certify against; it is a FIELD GUIDE — the recurring failure modes found in real API breaches, ordered by how often they hurt. Its most useful property for a Go developer is that it maps almost one-to-one onto decisions this guide has already built, which turns the list from a checklist into a REVIEW RUBRIC: read your service against the ten and every line should land on a control you can name — or on a gap you now know to close. The authorization family first, because it dominates the list:"),
			list("owasp-authz-family", false,
				"API1 BOLA — object-level authorization: user A reads user B's row by id; the defense is the ownership guard topic 24 builds on every id-addressed route, not just an authenticated check.",
				"API2 Broken authentication — the credential machinery itself: topic 23's whole subject, from argon2id to rotation to uniform refusals that leak nothing.",
				"API3 Object property level — the response leaks fields the caller may not see: over-fetching, sensitive fields in the same DTO; this topic's classification-and-masking is the discipline.",
				"API5 Function-level authorization — admin endpoints reachable by mortal users: topic 24's scope middleware at the route, deny by default.",
				"API6 Unrestricted business flows — the ENDPOINT is open by design but the flow is abusable at volume (signup, voting, checkout scalping); rate limiting plus flow-level throttling are the levers.",
			),
			list("owasp-edge-family", false,
				"API4 Unrestricted resource consumption — requests that cost more than they earn: this topic's per-client token buckets, plus payload caps and timeouts from the request-handling topics.",
				"API7 SSRF — the server fetching attacker-chosen URLs from its privileged network position: this topic's outbound guard, allowlist posture at every URL layer.",
				"API8 Security misconfiguration — headers, CORS, TLS, debug endpoints, verbose errors: this topic's header policy and the environments topic's per-env hardening.",
				"API9 Improper inventory — forgotten API versions, shadow endpoints, deprecated-but-live routes: the API-documentation topic's spec-as-truth and the environments topic's inventory discipline.",
				"API10 Unsafe consumption of APIs — trusting THIRD-party responses the way beginners trust user input: validate outbound data like inbound, because an attacker may control the feed.",
			),
			text("owasp-reading",
				"Two reading notes keep the list honest. The ten OVERLAP deliberately — a BOLA finding is usually also property-level leakage, and a misconfiguration is often what made the BOLA reachable — so map findings to ROOT controls rather than ticking items. And the list LAGS by design: it documents what broke recently, not what will break next; the threat-model habit from this topic's first section is what covers the failures the list has not met yet. Used that way — as a rubric with teeth, revisited at every design review — the ten items earn their place on the wall next to STRIDE."),
		),
		// ── 9. PII and GDPR ─────────────────────────────────────────
		section("pii-gdpr", "PII and GDPR",
			"Personal data is an API's heaviest cargo: classify at definition, mask in serving, log access without values, and erase completely — four moves, all real code.",
			text("pii-pitch",
				"GDPR arrives at an API as four engineering obligations, not a legal document. INVENTORY: you cannot protect or delete what you have not enumerated — every field that relates to an identifiable person (email, phone, and the one teams misfile: the signup IP ADDRESS, which is personal data the moment it can single anyone out) must be KNOWN. MINIMIZATION: serve the least data the caller needs — masked views derived per request, never masked copies stored beside the real ones. ACCESS ACCOUNTABILITY: reads and erasures of personal data are themselves events you must be able to show. And ERASURE: the right to be forgotten — delete the values, keep the fact. The guide implements all four as a real substrate in ``internal/domain/security``, and the classification is where it starts, because every other move consults it:"),
			code("pii-classify-go", "go",
				"The classification: a field's sensitivity is stamped at definition — public, internal, or PII.",
				"server/internal/domain/security/security.go",
				"type Sensitivity string\n"+
					"\n"+
					"const (\n"+
					"	// SensitivityPublic is safe for any caller: a display name.\n"+
					"	SensitivityPublic Sensitivity = \"public\"\n"+
					"	// SensitivityInternal is for the service's own use: a plan tier, a\n"+
					"	// feature flag. Not secret, not personal — but not the client's\n"+
					"	// business either.\n"+
					"	SensitivityInternal Sensitivity = \"internal\"\n"+
					"	// SensitivityPII is personal data: masked in every view, tombstoned\n"+
					"	// by erasure, and never allowed into an access-log entry.\n"+
					"	SensitivityPII Sensitivity = \"pii\"\n"+
					")"),
			text("pii-mask-log",
				"Serving and logging read the same stamps. The MASKED view renders public and internal fields verbatim and reduces each PII field to its edges (``ada@example.com`` → ``a*************m``) — a display transform derived per request, explicitly NOT a protection: the output still derives from the value, which is why masking is for serving and erasure is for deletion. The access log goes one better: its entry TYPE carries subject id, action, and an erased marker — and has NO field a value could hide in. That is a compile-time guarantee rather than a convention: a logging call that tried to write the email address would not build, and the type is the policy:"),
			code("pii-access-log-go", "go",
				"The access-log policy as a type: no room for values, so the log cannot leak them.",
				"server/internal/domain/security/security.go",
				"type AccessLogEntry struct {\n"+
					"	SubjectID string\n"+
					"	Action    string // \"read\" or \"erase\"\n"+
					"	Erased    bool\n"+
					"	At        time.Time\n"+
					"}"),
			text("pii-erasure-pitch",
				"Erasure — the right to be forgotten — tombstones every PII field and PERSISTS the record: public and internal fields survive (a plan tier is not personal data), the FACT of erasure survives with a timestamp (the audit trail GDPR expects you to keep), and the VALUES do not. The routine is idempotent (erasing twice answers the first report and touches nothing), and derived views need no separate pass BECAUSE every view renders from the one record — completeness for masks, caches, and logs downstream follows from the source of truth being clean. The demo's closing audit is the discipline that makes erasure real rather than asserted: sweep every downstream copy for the ORIGINAL values and prove the absence:"),
			code("pii-erase-go", "go",
				"The erasure routine: tombstone exactly the PII, stamp the fact, persist.",
				"server/internal/domain/security/security.go",
				"func (r *SubjectRecord) Erase(now time.Time) ErasureReport {\n"+
					"	if r.Erased {\n"+
					"		return ErasureReport{SubjectID: r.SubjectID, ErasedAt: r.ErasedAt, AlreadyErased: true}\n"+
					"	}\n"+
					"	erased := make([]string, 0, len(r.Fields))\n"+
					"	for i := range r.Fields {\n"+
					"		if r.Fields[i].Sensitivity == SensitivityPII {\n"+
					"			r.Fields[i].Value = ErasedValue\n"+
					"			erased = append(erased, r.Fields[i].Name)\n"+
					"		}\n"+
					"	}\n"+
					"	r.Erased = true\n"+
					"	r.ErasedAt = now\n"+
					"	return ErasureReport{SubjectID: r.SubjectID, ErasedFields: erased, ErasedAt: now}\n"+
					"}"),
			text("pii-honesty",
				"The honest boundaries of the in-repo substrate. The store port is backed by an in-memory map for the demo; production puts the SAME port over a table (and then erasure's hardest question becomes the COPIES — analytics extracts, caches, backups, and the log pipeline — which is why the classification travels with the data and why the audit sweep pattern, not just the delete, is the deliverable). Retention limits (delete personal data when its purpose expires, not only on request) and data-processing agreements are process, not code. What IS code, and what this substrate pins: the inventory as a type-checked field list, masked serving derived per request, an access log that structurally cannot hold values, and erasure whose completeness is PROVEN by a sweep, not promised by a comment:"),
			demo("security:pii", "The PII story: classify, mask, log, erase — and audit",
				"One request walks one fixture data subject end to end: the record's five fields with their classifications (the signup IP among the PII — the one most teams misfile), the masked view the API actually serves (PII reduced to its edges), the access-log lines (subject id and action only — nowhere for a value), the erasure (exactly the PII fields tombstoned, persisted, logged), the post-erasure view (tombstones — no mask can resurrect what the record no longer holds), and the closing audit that sweeps the store, every log line, and both views for the ORIGINAL values and reports them absent everywhere."),
		),
		// ── 10. Terminal companion ──────────────────────────────────
		section("terminal-companion", "Terminal companion: the security curl tour",
			"Headers off the wire, both stories, and a loop that trips the real 429 — every command against the running stack on :8100.",
			text("term-security-pitch",
				"With the stack up (server on :8100), this tour reads the topic's armor directly off the wire. First the HEADERS — ``curl -i`` against the lab route shows the five-rule policy on a real response, including the typed envelope's headers. Then the two STORIES (beats, facts, proofs, reasoning per story). And then the tour's signature move: a shell loop that hammers the lab route past its own route limiter until the 429 rate_limited envelope — code, request id, Retry-After, policy in the details — arrives on YOUR request, which is the difference between reading about a rate limit and being refused by one:"),
			code("term-headers", "bash", "", "",
				"# the five-header policy on a real response — read them off the wire\n"+
					"curl -si localhost:8100/api/demo/security/lab | head -14\n"+
					"# HSTS is inert over http by spec (browsers honor it over TLS only) —\n"+
					"# stated anyway: correct the day TLS terminates in front"),
			code("term-demos", "bash", "", "",
				"# the two stories — beats with verdicts, facts, proofs, reasoning\n"+
					"curl -s localhost:8100/api/demo/security/lab | python3 -m json.tool | head -30\n"+
					"curl -s localhost:8100/api/demo/security/pii  | python3 -m json.tool | head -30\n"+
					"# the lab story's boundary beat names the typed refusal it just derived\n"+
					"curl -s localhost:8100/api/demo/security/lab | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d[\"beats\"][2][\"detail\"])'"),
			code("term-ratelimit", "bash", "", "",
				"# trip the route limiter yourself: burst 5 at 2/s — the loop hits the 429s\n"+
					"for i in $(seq 1 8); do curl -s -o /dev/null -w \"%{http_code} \" localhost:8100/api/demo/security/lab; done; echo\n"+
					"# the typed 429: rate_limited code, x-request-id, Retry-After, policy in details\n"+
					"curl -si localhost:8100/api/demo/security/lab | head -16\n"+
					"# wait a few seconds (refill is continuous, not window-shaped) — it serves again\n"+
					"sleep 3; curl -s -o /dev/null -w \"%{http_code}\\n\" localhost:8100/api/demo/security/lab"),
			code("term-content", "bash", "", "",
				"# the topic on the wire — order 25 closes the state-auth group\n"+
					"curl -s localhost:8100/api/content | python3 -c 'import json,sys; d=json.load(sys.stdin); print([t[\"id\"] for t in d[\"topics\"] if t[\"group\"]==\"state-auth\"])'\n"+
					"curl -s localhost:8100/api/content/security | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d[\"title\"], \"·\", len(d[\"sections\"]), \"sections\")'"),
			text("term-security-coda",
				"Read the tour as five proofs you can re-run at will: the headers riding a response you can see, the lab story's honest window (burst spent, 429 with the bucket's own retry math, recovery at exactly the sustained rate), the PII story's clean audit, the route refusing YOU with the same typed envelope it teaches, and the refill a few seconds later — punishment that ends because the allowance returns continuously. That is the security edge standing: headers on everything, one honest budget per client, guards at every interpreter boundary, and personal data that can be accounted for and erased. The platform topics ahead reuse these pieces — observability will lean on the request ids, environments on the secret discipline, and reliability on the fact that every control here degrades loudly rather than silently."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"security",
		"Security from Go",
		"The threat-model habit; the five security headers stamped by middleware (demo security:lab); CORS as the "+
			"browser's read contract; rate limiting as a per-client token bucket over golang.org/x/time/rate with the "+
			"typed 429 envelope; the injection family in Go terms — parameterized SQL, html/template auto-escaping, path "+
			"confinement; CSRF and SSRF; secrets minted at boot and never logged; the OWASP API Top 10 as a review "+
			"rubric; and the PII/GDPR substrate — classification, masked serving, an access log with no room for values, "+
			"and erasure proven by audit (demo security:pii) — closed by the terminal's curl tour, headers and 429s off "+
			"the wire.",
		"state-auth",
		25,
		sections,
	)
}
