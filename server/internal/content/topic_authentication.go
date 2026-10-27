package content

import (
	"errors"
	"fmt"
)

// This file is topic 23 authentication (group state-auth, curriculum
// order 23): what authentication is and how it splits from
// authorization, the login flow with its cookie done right (sealed
// value, honest flags, server-side truth — demo auth:sessions), API
// keys hashed at rest with prefix lookup and constant-time compares
// (demo auth:api-keys), the sessions-vs-keys-vs-tokens decision argued
// with live measurements (demo auth:compare), HS256 JWTs minted and
// attacked from the standard library alone (demo auth:jwt), the OAuth2
// code grant with PKCE run honestly in one process (demo auth:oauth),
// the deciding guidance, argon2id password hashing with the rehash
// signal and the uniform decoy burn (demo auth:hashing), and refresh
// rotation whose reuse detection turns a stolen token into an alarm
// (demo auth:refresh). The auth middleware — the piece every protected
// route after this topic stands on — lands here too. The terminal
// companion closes with the curl tour through all seven demos.

// authenticationTopic builds topic 23.
func authenticationTopic() (Topic, error) {
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
		// ── 1. What authentication is ───────────────────────────────
		section("what-authentication-is", "What authentication is",
			"WHO is calling — established by evidence, answered with an identity. Authorization (topic 24) asks what they may do, and confusing the two is how systems check the wrong thing.",
			text("authn-pitch",
				"Authentication is the answer to WHO IS CALLING: a caller makes an identity claim, presents evidence for it, and the server either resolves a PRINCIPAL or refuses. It is deliberately not the answer to WHAT MAY THEY DO — that is authorization, the very next topic, and the split is load-bearing: authentication runs BEFORE the route's work and answers with an identity (a username, a set of scopes); authorization runs AT the work and answers with a verdict. Mix them up and you get endpoints that check a caller's ROLE when they meant to check the caller's IDENTITY, or APIs where any authenticated user may touch every record because someone confused \"logged in\" with \"allowed\". Every mechanism in this topic — session cookie, API key, JWT, OAuth-exchanged token — is only a different way of carrying that principal across requests, and every one of them hands topic 24 the same thing: who, and with which scopes."),
			text("authn-factors-pitch",
				"The EVIDENCE side of the question comes in three kinds, called factors, and knowing them is what makes this topic's mechanism choices rational instead of fashionable:"),
			list("authn-factors", false,
				"Knowledge — something the caller knows: a password, a PIN, the secret half of an API key. Cheap and revocable, but stealable by one phishing page or one leaked database; never sufficient alone for anything that matters.",
				"Possession — something the caller has: a session cookie, a refresh token, a phone for one-time codes. The workhorse factor of this topic: hard to guess, easy to revoke server-side, and the reason logout and rotation exist.",
				"Inherence — something the caller is: biometrics, hardware-bound keys. Never stored as raw data a leak would expose — you cannot re-issue a fingerprint — which is why real systems store derived verifier values, the same discipline this topic teaches for passwords.",
			),
			text("authn-lifecycle",
				"Every mechanism here walks the same lifecycle, and each stage is a place a design can fail. Credentials are ESTABLISHED at signup (a password hashed, a key minted), PRESENTED at login (a form, a header, a redirect), EXCHANGED for proof (a session row, a signed token, an access grant), CARRIED on every call (cookie, ``Authorization`` header), and — the stage beginners forget — RETIRED: expired by TTL, revoked by logout, rotated on suspicion. A mechanism is only as honest as its retirement story: sessions die with a store delete, keys die with a prefix delete, JWTs die only with TIME (their weakness), and refresh families die the moment one of their tokens is presented twice (their whole point). Watch for that pattern as this topic builds it seven times."),
			text("authn-substrate",
				"The substrate is real and in-repo: ``internal/domain/auth`` holds the vocabulary (users, identities, the four store ports) and the six engines — the hasher, the session login, the key service, the token service, the OAuth provider, the refresh rotator — and ``internal/data/auth`` holds the in-memory adapters plus the CONSOLE that wires it all: two seeded users (``ada`` and ``linus``, real argon2id hashes minted at boot), stores for sessions, keys, refresh families, and OAuth codes, and two per-process secrets minted from ``crypto/rand``. The in-memory choice is deliberate and taught where it bites: state lives and dies with the process, and two replicas never see the same sessions — production swaps the SAME ports onto Redis (TTL-evicted rows) or PostgreSQL (a users table) without touching a line of the engines. Seven story endpoints walk the whole surface under ``/api/demo/auth/*``, and the wiring is one composition-root block:"),
			code("authn-wiring-go", "go",
				"The composition root's whole auth wiring: one console, seven story routes, one guarded proof route.",
				"server/cmd/guide/main.go",
				"	authConsole, err := auth.NewConsole()\n"+
					"	if err != nil {\n"+
					"		return fmt.Errorf(\"auth console: %w\", err)\n"+
					"	}\n"+
					"	mux.Handle(\"GET /api/demo/auth/sessions\", presentation.NewAuthSessionsHandler(authConsole, logger))\n"+
					"	// …five more story routes: api-keys, compare, jwt, oauth, hashing\n"+
					"	mux.Handle(\"GET /api/demo/auth/refresh\", presentation.NewAuthRefreshHandler(authConsole, logger))\n"+
					"	mux.Handle(\"GET /api/demo/auth/whoami\",\n"+
					"		presentation.RequireIdentity(authConsole.Resolver, logger)(presentation.NewAuthWhoamiHandler(logger)))"),
		),
		// ── 2. Login flow and sessions ──────────────────────────────
		section("login-flow-and-sessions", "The login flow: cookies done right",
			"Password in, session row out, cookie home — the cookie carrying nothing but a sealed id, the server holding every fact, and the flags making the browser a safe carrier.",
			text("login-flow-pitch",
				"The server-side session is the oldest web answer to HTTP's statelessness, and it is still the best default for browsers. The flow has four moves. ONE: the login form posts credentials over TLS. TWO: the server verifies the password against a stored argon2id hash (the next sections show that machinery) — and on failure answers ONE indistinguishable 401, never \"unknown user\" versus \"wrong password\". THREE: on success it mints a random 256-bit session id, records a row (owner, scopes, expiry) in the session store, and answers a cookie carrying ONLY that id. FOUR: every later request presents the cookie, the server resolves the id to the row, and the identity rides the request context. The browser is a courier, not a vault: everything an attacker would want stays server-side."),
			text("cookie-flags-pitch",
				"The cookie's attributes are not decoration; each flag closes a named attack. The four that matter:"),
			list("cookie-flags", false,
				"HttpOnly — page JavaScript can never read the value, so an injected script cannot exfiltrate the session; without it, one cross-site-scripting bug is one account takeover.",
				"SameSite=Lax — the browser does not attach the cookie to cross-site POSTs, which guts CSRF's classic delivery (topic 25 returns to this with the full defense).",
				"Secure — the value only ever travels over HTTPS; off in this plain-http dev stack, unconditionally on in production.",
				"Max-Age + Path — the cookie expires with the session's TTL and is scoped to the site's own paths instead of every URL the host serves.",
			),
			code("session-login-beat-go", "go",
				"The login's answer in the guide's real engine: the cookie instruction with its flags, and a SEALED value — id plus HMAC signature.",
				"server/internal/domain/auth/sessions.go",
				"	return SessionLoginResult{\n"+
					"		Identity: Identity{\n"+
					"			Username:    user.Username,\n"+
					"			DisplayName: user.DisplayName,\n"+
					"			Scopes:      user.Scopes,\n"+
					"			Via:         \"session\",\n"+
					"		},\n"+
					"		SessionID: id,\n"+
					"		Cookie: CookieInstruction{\n"+
					"			Name:          SessionCookieName,\n"+
					"			Value:         s.seal(id),\n"+
					"			MaxAgeSeconds: int(SessionTTL.Seconds()),\n"+
					"			Path:          SessionCookiePath,\n"+
					"			HTTPOnly:      true,\n"+
					"			SameSite:      \"lax\",\n"+
					"			Secure:        false, // dev stack is plain http; production flips this unconditionally\n"+
					"		},\n"+
					"		IssuedAt:  now,\n"+
					"		ExpiresAt: expiresAt,\n"+
					"	}, nil"),
			text("sealed-cookie-pitch",
				"Why SEAL the id when it is already unguessable? Because a cookie value is attacker-writable input: the browser will happily send back whatever string a crafted request carries, so the server must distinguish \"a value I minted\" from \"a value someone made up\" before spending a store lookup on it. The seal is an HMAC-SHA256 signature over the id, appended after a dot — the same family of construction Starlette's session middleware and itsdangerous use (the fastapi sibling seals the same way), implemented here with the standard library in six lines. The unseal compares signatures with ``hmac.Equal`` (constant time), and — the honest engineering detail — every failure mode collapses into ONE answer, because a response that said \"bad signature\" versus \"unknown id\" versus \"expired\" would teach an attacker exactly how far each guess got:"),
			code("session-seal-beat-go", "go",
				"The seal and its constant-time verification — tamper evidence before the store is ever consulted.",
				"server/internal/domain/auth/sessions.go",
				"func (s *SessionService) seal(id string) string {\n"+
					"	mac := hmac.New(sha256.New, s.secret)\n"+
					"	mac.Write([]byte(id))\n"+
					"	return id + \".\" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))\n"+
					"}\n"+
					"\n"+
					"func (s *SessionService) unseal(value string) (string, bool) {\n"+
					"	cut := strings.LastIndex(value, \".\")\n"+
					"	if cut <= 0 || cut == len(value)-1 {\n"+
					"		return \"\", false\n"+
					"	}\n"+
					"	id, signature := value[:cut], value[cut+1:]\n"+
					"	mac := hmac.New(sha256.New, s.secret)\n"+
					"	mac.Write([]byte(id))\n"+
					"	want := mac.Sum(nil)\n"+
					"	got, err := base64.RawURLEncoding.DecodeString(signature)\n"+
					"	if err != nil || !hmac.Equal(got, want) {\n"+
					"		return \"\", false\n"+
					"	}\n"+
					"	return id, true\n"+
					"}"),
			text("logout-pitch",
				"Logout is where the mechanism proves itself: invalidation is a store DELETE, not a browser promise. The row leaves the session store, so the cookie value — the original or any stolen copy of it — resolves to nothing from that instant; the clearing cookie (same name and flags, empty value, Max-Age 0) just tidies the browser's copy. The demo below logs in, inspects, tampers, fails, and logs out inside one request, and its strongest beat is the LAST one: the pre-logout cookie value, presented again after logout, answers nothing."),
			demo("auth:sessions", "The session story: login, sealed cookie, tamper, logout",
				"One request walks the full cookie lifecycle against the running engines: ada's login verifies her password against the stored argon2id hash and answers the sealed HttpOnly SameSite=Lax cookie (every flag shown as a fact), the /me inspect resolves the sealed value to the live server-side row, a ONE-CHARACTER edit to the value is refused by the seal before any store lookup, a wrong password and an unknown username both burn one full argon2 run and answer the same indistinguishable 401 with their measured costs side by side, logout deletes the row, and the pre-logout value presented once more resolves to NOTHING — invalidation as a store delete, not a promise."),
		),
		// ── 3. API keys ─────────────────────────────────────────────
		section("api-keys", "API keys",
			"For machines, not people: a two-part key — greppable prefix, high-entropy secret — stored only as its SHA-256 and compared in constant time.",
			text("api-keys-pitch",
				"An API key is a pre-shared credential for programs: the CI pipeline's deploy script, a partner's webhook fetcher, the cron job that exports reports. Nobody types it, so it can be long and ugly — and it should be. The design this guide implements lives or dies by three rules. GENERATION mints a two-part key: a short readable PREFIX (``gdg_`` plus eight hex characters) and a 256-bit SECRET, joined by a dot. The prefix is the greppable half — you can find it in logs, type it into a revoke box, and show it in a UI — and the secret is the unguessable half. The STORE holds only ``SHA256(prefix.secret)``; the raw key appears in exactly one response ever, the creation one. LOOKUP splits a presented key at the dot, fetches the record by prefix, re-hashes the presentation, and compares the two hashes in constant time. REVOCATION is a prefix-keyed delete — instant death for every copy of the key in the world."),
			code("api-keys-verify-beat-go", "go",
				"The protected-call check in the real service: prefix lookup plus a constant-time hash compare — and a dummy burn for malformed presentations.",
				"server/internal/domain/auth/apikeys.go",
				"func (s *ApiKeyService) Verify(ctx context.Context, presented string) (StoredAPIKey, error) {\n"+
					"	prefix, _, ok := strings.Cut(presented, APIKeySeparator)\n"+
					"	if !ok {\n"+
					"		_ = subtle.ConstantTimeCompare([]byte(SHA256Hex(presented)), []byte(dummyKeySHA))\n"+
					"		return StoredAPIKey{}, ErrNotAuthenticated\n"+
					"	}\n"+
					"	record, found, err := s.store.FindKeyByPrefix(ctx, prefix)\n"+
					"	if err != nil {\n"+
					"		return StoredAPIKey{}, fmt.Errorf(\"find key by prefix: %w\", err)\n"+
					"	}\n"+
					"	if !found || subtle.ConstantTimeCompare([]byte(SHA256Hex(presented)), []byte(record.KeySHA256)) != 1 {\n"+
					"		return StoredAPIKey{}, ErrNotAuthenticated\n"+
					"	}\n"+
					"	return record, nil\n"+
					"}"),
			text("api-keys-why-constant-time",
				"The constant-time compare (``subtle.ConstantTimeCompare``) is not paranoia; it is the closing of a real oracle. A naive ``==`` on hashes returns at the first differing byte, so a caller who measures response times can learn how many leading bytes it guessed right and grind a hash byte by byte in ~256 tries per position instead of 2^256 overall. The same reasoning puts the DUMMY burn on malformed presentations: without it, \"no dot in the key\" would answer faster than \"wrong secret\", and the response-time histogram would again leak which failure path ran. Every comparison over a secret-shaped value in this topic — cookie seals, JWT signatures, PKCE challenges, key hashes — goes through ``hmac.Equal`` or ``ConstantTimeCompare`` for exactly this reason."),
			text("api-keys-hash-choice",
				"Why SHA-256 here when passwords get argon2id? Entropy. A password is a LOW-entropy secret humans must remember — the space of guesses is small enough to grind, so verification must be deliberately expensive. An API key's secret is 256 bits of ``crypto/rand``: the guess space is 2^256 wide before any hashing, and one plain SHA-256 keeps it there while staying fast enough for every-request verification. Slow-hashing high-entropy secrets buys nothing and costs latency. The rule to carry away: STRETCH what an attacker must guess (passwords); do not stretch what they cannot enumerate (random tokens)."),
			demo("auth:api-keys", "The API-key story: raw once, hashed at rest, dead on revoke",
				"One request walks the key lifecycle: generation mints gdg_<8hex>.<secret> and answers the RAW key exactly once (the store keeps only its SHA-256, shown as a preview), the roster lists every record as hash previews and nothing else, a real call authenticates by prefix lookup plus constant-time compare, the right prefix with a WRONG secret is refused in constant time, a malformed key without a separator burns a dummy comparison and refuses identically, revocation deletes the record by prefix, and the very same key presented once more is dead — instant, total, no expiry waiting."),
		),
		// ── 4. Choosing: sessions vs keys vs tokens ─────────────────
		section("choosing-mechanism", "Sessions, keys, and tokens: choosing",
			"Three carriers, one identity — and one honest decision: who holds the truth? The server (sessions, keys) buys instant revocation; the token buys stateless verification and pays for it with expiry-as-revocation.",
			text("choosing-pitch",
				"Strip the branding and every mechanism in this topic answers one question: WHERE does the truth about the caller live? A SESSION keeps it server-side — the cookie is a meaningless handle, verification is a store lookup, and revocation is a delete. An API KEY also keeps it server-side — the record is the truth, the raw key only unlocks the lookup. A JWT moves the truth INTO the credential: the claims travel with the token, the signature makes them tamper-evident, and verification consults nothing but math. That single difference generates every practical trade-off, and the compare demo measures them live rather than asserting them:"),
			list("choosing-trades", false,
				"Verification cost — a session is one seal check plus one store round trip; a key is one SHA-256 plus one lookup; a JWT is pure crypto with zero I/O, which is why gateways love tokens at high fan-out.",
				"Revocation — sessions and keys die with one delete, instantly and totally; a JWT has no kill switch, so revocation is only as sharp as the expiry you minted (refresh rotation exists to fix this).",
				"Trust distribution — verifying a session or key requires reaching the store that holds it; any service holding the JWT public material (or shared secret) can verify locally, which is what makes tokens the cross-service default.",
				"Carrier ergonomics — browsers carry cookies automatically and safely (HttpOnly); servers and CLIs carry headers comfortably; a JWT inside a browser cookie is a legitimate hybrid this guide's own OAuth demo produces.",
			),
			text("middleware-pitch",
				"Whatever carrier wins, the protected side of the house wants ONE shape: resolve the credential to an identity in the request context, refuse with the standard 401 envelope otherwise, and let handlers stay ignorant of which mechanism answered. That is the auth middleware this topic lands — a real ``presentation.Middleware``, wired per-route at the composition root (the production chain's composition lines are quoted teaching material from topic 08, so the middleware is deliberately NOT spliced into that chain; guarded routes wrap it directly, and any future protected surface does the same). Carrier precedence is explicit: an ``Authorization: Bearer`` header wins over ``x-api-key``, which wins over the session cookie — the header the CLIENT deliberately sent must beat the cookie the BROWSER auto-attached:"),
			code("middleware-beat-go", "go",
				"The auth middleware: three carriers, one resolver port, one uniform 401 — the seam every protected route after this topic stands on.",
				"server/internal/presentation/auth_middleware.go",
				"			switch {\n"+
					"			case r.Header.Get(\"Authorization\") != \"\":\n"+
					"				token, ok := bearerToken(r.Header.Get(\"Authorization\"))\n"+
					"				if !ok {\n"+
					"					core.WriteError(w, r, core.Unauthorized(\n"+
					"						\"not authenticated — the Authorization header must be “Bearer <token>”\"), logger)\n"+
					"					return\n"+
					"				}\n"+
					"				identity, err = resolver.ResolveBearerToken(r.Context(), token, now)\n"+
					"			case r.Header.Get(APIKeyHeader) != \"\":\n"+
					"				identity, err = resolver.ResolveAPIKey(r.Context(), r.Header.Get(APIKeyHeader), now)\n"+
					"			case sessionCookie(r) != \"\":\n"+
					"				identity, err = resolver.ResolveSessionCookie(r.Context(), sessionCookie(r), now)\n"+
					"			default:\n"+
					"				core.WriteError(w, r, core.Unauthorized(\n"+
					"					\"not authenticated — send a session cookie, an x-api-key header, or an Authorization: Bearer token\"), logger)\n"+
					"				return\n"+
					"			}"),
			demo("auth:compare", "The comparison: one identity, three carriers, measured",
				"One request runs ada through ALL THREE mechanisms side by side and reports what each actually costs: the session login, the key generation, and the JWT mint establish the same identity three ways; each verification is TIMED live (store lookup vs hash-plus-lookup vs pure crypto, honest single-shot numbers); then the two revocable mechanisms are revoked — the session row deleted, the key record deleted — and all three proofs are presented AGAIN: the cookie resolves to nothing, the key hashes to nothing, and the JWT is STILL VALID, and will be until exp, because a self-carried credential has no kill switch. The matrix the topic argued, proven by the engines' own answers."),
		),
		// ── 5. JWT ──────────────────────────────────────────────────
		section("jwt", "JWTs: HS256 from the standard library",
			"Three base64url parts and an HMAC — minted, dissected, and attacked six ways, with the algorithm confusion refused at the door and no dependency on the module graph.",
			text("jwt-pitch",
				"A JWT is a signed assertion: a header (``{\"alg\":\"HS256\",\"typ\":\"JWT\"}``), a payload of claims, and a signature over both, each part base64url-encoded and joined by dots. Nothing more. The guide implements HS256 — symmetric: one secret signs and verifies, the natural fit for a single service; RS256/ES256 split the pair so strangers can VERIFY tokens without being able to MINT them, the right shape when other parties must trust your tokens — and it does so with the standard library only (``crypto/hmac``, ``crypto/sha256``, ``encoding/base64``, ``encoding/json``), deliberately: a JWT library is convenient, but the mechanism is small enough that seeing it beats importing it. Third-party options named for real work — ``github.com/golang-jwt/jwt/v5`` is the community default, ``github.com/lestrrat-go/jwx`` the full JOSE toolkit — package exactly what this engine does, plus the algorithm families this guide compares in prose rather than pins in go.mod."),
			code("jwt-mint-beat-go", "go",
				"The mint: the production claim set — sub, iat, exp, nbf, iss, scopes, jti — signed with one HMAC.",
				"server/internal/domain/auth/jwt.go",
				"	header := map[string]string{\"alg\": TokenAlgorithm, \"typ\": \"JWT\"}\n"+
					"	payload := map[string]any{\n"+
					"		\"sub\":    req.Subject,\n"+
					"		\"iat\":    issuedAt,\n"+
					"		\"exp\":    issuedAt + int64(ttl.Seconds()),\n"+
					"		\"nbf\":    issuedAt + int64(req.NotBefore.Seconds()),\n"+
					"		\"iss\":    TokenIssuer,\n"+
					"		\"scopes\": scopes,\n"+
					"		\"jti\":    jti,\n"+
					"	}\n"+
					"	secret := t.secret\n"+
					"	if req.SignWith == \"forged\" {\n"+
					"		secret = []byte(\"attacker-secret-that-the-server-does-not-know\")\n"+
					"	}"),
			text("jwt-verify-order-pitch",
				"Verification is where JWTs get interesting, because the ORDER of checks is the security. This engine's order: shape first (three parts, decodable JSON), then the header's ALG claim — where the two classic attacks die. ``alg=none`` is the 2015 crash: a token whose header claims to be unsigned, which servers that trusted the header's own algorithm field accepted verbatim. The refusal happens BEFORE any signature exists to check. A header claiming some other algorithm (HS512, an RS256 confusion) is refused the same way: the algorithm is the SERVER's choice, never the token's. Only then the signature (``hmac.Equal``, constant time), the required claims, the time windows, the issuer. A forged token is labeled a FORGERY even when its claims would also have expired — classification honesty, pinned in the demo:"),
			code("jwt-verify-beat-go", "go",
				"The verifier's door: alg=none and wrong-alg refused before any signature work — then the constant-time signature check.",
				"server/internal/domain/auth/jwt.go",
				"	var header struct {\n"+
					"		Alg string `json:\"alg\"`\n"+
					"	}\n"+
					"	if err := decodePart(parts[0], &header); err != nil {\n"+
					"		return malformedVerdict(\"the header does not decode as JSON\")\n"+
					"	}\n"+
					"	if header.Alg == \"none\" {\n"+
					"		return TokenVerdict{Outcome: TokenAlgNone,\n"+
					"			Reason: \"the header declares alg=none — an UNSIGNED token. A server that trusts the header's algorithm instead of insisting on its own would accept this; this one refuses before any signature exists to check\"}\n"+
					"	}\n"+
					"	if header.Alg != TokenAlgorithm {\n"+
					"		return TokenVerdict{Outcome: TokenWrongAlg,\n"+
					"			Reason: fmt.Sprintf(\"the header declares alg=%s; this server verifies %s only — the algorithm is the SERVER's choice, never the token's\", header.Alg, TokenAlgorithm)}\n"+
					"	}\n"+
					"	// …decode the presented signature, then:\n"+
					"	want := t.signature(parts[0], parts[1])\n"+
					"	if !hmac.Equal(got, want) {\n"+
					"		return TokenVerdict{Outcome: TokenBadSignature,\n"+
					"			Reason: \"the signature does not match the server's secret over this exact header and payload — a forged or tampered token\"}\n"+
					"	}"),
			text("jwt-trade-honesty",
				"The JWT's virtue and vice are the same property: verification consults NOTHING but the signature and the claims — no store, no session, no round trip. That is stateless, fast, and horizontally scalable, and it is also why a JWT cannot be revoked: the truth travels with the token, so the only levers are the ones baked in at mint time (the exp you chose) or heavyweight additions (deny-lists every verifier must consult — which re-introduces the store round trip and forfeits the point). Production reconciles this with SHORT-LIVED access tokens plus the refresh rotation this topic closes with; the demo's six verdicts — valid, forged, tampered, alg=none, expired, not-yet-valid — are the full curriculum, each classified by the same engine the OAuth demo mints through."),
			demo("auth:jwt", "The JWT lab: mint, dissect, attack six ways",
				"One request mints a real HS256 token for ada and shows its full anatomy (header JSON, claims JSON, signature length, exp), then presents SIX tokens to the verifier: the honest one (valid), the same claims signed with the ATTACKER's secret (bad-signature), the claims edited to say admin under the original signature (bad-signature — the HMAC covers the exact bytes), the alg=none attack with the signature stripped (alg-none-rejected), an expired lease (expired), and a not-yet-valid one (not-yet-valid). Every verdict is the engine's own classification with its reason sentence — none of which ever includes the secret."),
		),
		// ── 6. OAuth2 flows ─────────────────────────────────────────
		section("oauth2-flows", "OAuth2 flows: the code grant with PKCE",
			"When the user is human but the token is for an app: the authorization-code dance with PKCE, run honestly in one process — every party labeled, one step simulated, everything else real.",
			text("oauth-pitch",
				"OAuth2 answers a question the earlier mechanisms cannot: how does an APPLICATION get a token for a USER's resources without ever seeing the user's password? The family's answer (mirrored from the fastapi sibling's topic) is the AUTHORIZATION-CODE grant: the app sends the user to the provider's authorize endpoint; the user logs in THERE and consents; the provider redirects back to the app's registered redirect with a ONE-TIME code (front channel, visible in the URL); the app exchanges that code plus its own credentials at the token endpoint (back channel, private) for an access token. The demo runs this grant with PKCE — Proof of Key for Code Exchange — which a PUBLIC client (a CLI or SPA with no vaultable secret) uses to bind the code to a verifier only it knows: the client sends ``base64url(SHA-256(verifier))`` as the challenge, and must present the verifier itself at exchange. An attacker who intercepts the code in the redirect cannot fabricate the verifier, so the code is worthless to them."),
			text("oauth-honest-simulation",
				"Two things are simulated here, and they are the only two: the resource owner (ada logs in and consents — a real deployment shows a real login page and records the grant) and the HTTPS everywhere (the redirect URIs are https URLs on paper; the loop runs inside one process). Everything else is real protocol code: registered clients, exact-match redirect validation, hashed one-time codes consumed on FIRST presentation, S256 challenge comparison in constant time, and a real JWT minted at exchange. The refusals are the curriculum: an unregistered redirect is REFUSED, never redirected (an open redirect with a code attached is a credential leak); a replayed code is invalid_grant because one-time means one ATTEMPT; and a wrong verifier burns a fresh code — consumed on first presentation even when the exchange fails, so a thief stealing codes gets exactly one guess per code:"),
			code("oauth-authorize-beat-go", "go",
				"The front door: registered client, exact-match redirect, S256 challenge — then the one-time code, stored hashed.",
				"server/internal/domain/auth/oauth.go",
				"	if req.RedirectURI != OAuthRedirectURI {\n"+
					"		return AuthorizeResult{}, fmt.Errorf(\"%w: redirect_uri is not registered for this client — refusing rather than redirecting a code somewhere new\",\n"+
					"			ErrInvalidGrant)\n"+
					"	}\n"+
					"	if len(req.Challenge) != 43 { // an S256 digest is exactly 43 base64url chars\n"+
					"		return AuthorizeResult{}, fmt.Errorf(\"%w: code_challenge must be an S256 digest (43 base64url characters)\",\n"+
					"			ErrInvalidGrant)\n"+
					"	}\n"+
					"	// …mint the one-time code, then:\n"+
					"	if err := p.codes.IssueCode(ctx, StoredAuthorizationCode{\n"+
					"		SHA256:        SHA256Hex(code),\n"+
					"		ClientID:      req.ClientID,\n"+
					"		RedirectURI:   req.RedirectURI,\n"+
					"		CodeChallenge: req.Challenge,\n"+
					"		Username:      owner,\n"+
					"		Scopes:        OAuthClientScopes,"),
			code("oauth-exchange-beat-go", "go",
				"The back door: consume-first, then every binding re-checked — the verifier in constant time.",
				"server/internal/domain/auth/oauth.go",
				"	stored, found, err := p.codes.TakeCode(ctx, SHA256Hex(req.Code))\n"+
					"	if !found {\n"+
					"		return ExchangeResult{}, fmt.Errorf(\"%w: the code is unknown, already used, or expired (one attempt: codes are consumed on first presentation)\",\n"+
					"			ErrInvalidGrant)\n"+
					"	}\n"+
					"	if req.RedirectURI != stored.RedirectURI {\n"+
					"		return ExchangeResult{}, fmt.Errorf(\"%w: redirect_uri does not match the authorize request\", ErrInvalidGrant)\n"+
					"	}\n"+
					"	if !hmac.Equal([]byte(S256Challenge(req.Verifier)), []byte(stored.CodeChallenge)) {\n"+
					"		return ExchangeResult{}, fmt.Errorf(\"%w: S256(verifier) does not match the code_challenge — wrong verifier, and the code is spent\",\n"+
					"			ErrInvalidGrant)\n"+
					"	}"),
			text("oauth-flows-family",
				"The other grants, one line each, because \"which flow\" is a real interview question with a real answer. The CLIENT-CREDENTIALS grant (client id + secret → token, no user) is service-to-service authn and needs no dance. The (deprecated for browsers) IMPLICIT grant handed tokens straight from the redirect and exists now mostly as a cautionary tale. The PASSWORD grant (collect the user's password yourself and post it) is the one this topic's JWT login endpoint models and the one OAuth2 retired for exactly that reason. And DEVICE-CODE is the code grant over a TV. From Go as a CLIENT of someone else's provider, ``golang.org/x/oauth2`` is the maintained standard-library-family client; this guide implements the SERVER side to teach what that client is talking to."),
			demo("auth:oauth", "The OAuth story: PKCE pair, code, exchange, and the refusals",
				"One request runs the whole grant with every party labeled: the client mints its PKCE pair locally (verifier kept, only its S256 challenge sent), the authorize endpoint validates client and redirect and answers the redirect URL with the one-time code and state riding the query, the token endpoint burns the code and verifies the verifier in constant time and answers a real access token, the resource server verifies that token statelessly (valid). Then the refusals: replaying the spent code is invalid_grant, and a FRESH code exchanged with a wrong verifier is invalid_grant AND the code is burned — proven by presenting the right verifier immediately after and being refused again."),
		),
		// ── 7. Deciding ─────────────────────────────────────────────
		section("deciding", "Deciding",
			"A decision procedure, not a favorites list: start from who is calling and who holds the truth, then let the constraints pick the mechanism.",
			text("deciding-pitch",
				"The honest way to choose is to answer three questions in order. WHO is calling: a browser user, a first-party SPA, a partner's server, a CI job? WHAT must verification reach: one service, or every service on the mesh? and HOW SHARP must revocation be: instant and total, or best-effort within minutes? The answers compose into the shape most production systems actually run — several mechanisms side by side behind one middleware, each where it fits:"),
			list("deciding-rules", false,
				"Browser users — server-side sessions behind a sealed HttpOnly cookie: instant logout, no token hygiene asked of the page, the store one delete away from safety. The default, still.",
				"Machines and jobs — API keys hashed at rest: greppable prefixes, per-key revocation, no expiry dance; rotate by minting a new key and deleting the old.",
				"Cross-service and mobile — short-lived JWTs from a login or an OAuth code grant: local verification everywhere, no shared store dependency, revocation delegated to short expiry plus refresh.",
				"Third-party apps acting for your users — OAuth2 code grant with PKCE, always: the user's password never crosses the app, consent is recorded, scopes bound the token.",
				"Everything together — the auth middleware resolves all carriers to one Identity; topic 24's authorization consumes that identity and never cares which mechanism produced it.",
			),
			text("deciding-coda",
				"Two failure modes to name before moving on. CHOOSING BY FASHION — \"we use JWTs\" as an identity rather than an answer — lands teams with unrevocable credentials for browser sessions that a session row would have solved on day one. And CHOOSING BY MECHANISM-ENVY — adopting an external identity provider before the first-party auth works — couples your boot to someone else's uptime before you have anything worth protecting. This guide's own stack answers the three questions like this: the demos share one console; the client shell needs no auth to read; and the guard the middleware provides is deliberately demo-scoped until topic 24 gives it real work. Mechanisms are now CHEAP to add; the decision procedure is the durable part."),
		),
		// ── 8. Password hashing ─────────────────────────────────────
		section("password-hashing", "Password hashing: argon2id",
			"The stored form of a human secret: salted, memory-hard, parameter-stamped — slow by design, one-way by mathematics, and verified in constant time with a uniform decoy burn.",
			text("hashing-pitch",
				"A password must be CHECKABLE without being KNOWABLE — so the server stores a HASH: the output of a function that is cheap to re-run on a guess and ruinous to invert at scale. Argon2id (via ``golang.org/x/crypto/argon2``, pinned in go.mod since the phase's dependency step) is the memory-hard choice the OWASP cheat sheet names first: it costs the attacker not just CPU but RAM per guess, which is exactly the axis GPU farms are strongest on. The guide hashes at 19456 KiB / 2 iterations / 1 lane — the cheat sheet's floor, honestly labeled as such — and teaches the dial: production raises memory and iterations on the same code, because the parameters are stamped INSIDE every hash and verify re-derives with the hash's own cost, never the server's current one."),
			code("hashing-beat-go", "go",
				"The mint: fresh salt, argon2.IDKey, and the PHC string the cost rides inside.",
				"server/internal/domain/auth/password.go",
				"func (h *Hasher) Hash(password string) (string, error) {\n"+
					"	salt := make([]byte, h.Params.SaltBytes)\n"+
					"	if _, err := rand.Read(salt); err != nil {\n"+
					"		return \"\", fmt.Errorf(\"read password salt entropy: %w\", err)\n"+
					"	}\n"+
					"	key := argon2.IDKey([]byte(password), salt,\n"+
					"		h.Params.Iterations, h.Params.MemoryKiB, h.Params.Parallelism, uint32(h.Params.KeyBytes))\n"+
					"	return encodePHC(h.Params, salt, key), nil\n"+
					"}"),
			text("hashing-verify-pitch",
				"Verify re-runs the function over the CANDIDATE — it never decrypts anything, because there is nothing to decrypt: hashing is one-way, and the stored string can only be checked against, not opened. Four disciplines make the mechanism honest, and the demo proves each live. SALT: a fresh 16 bytes per hash, so the same password never hashes to the same string and rainbow tables die. STAMPED PARAMETERS: the PHC string carries ``m``, ``t``, ``p`` beside the salt and key, so a 2015-cost hash still verifies after upgrades — and flags ``NeedsRehash`` so the login that just succeeded can silently upgrade it. CONSTANT-TIME COMPARE: the derived keys are compared with ``subtle.ConstantTimeCompare``, never ``==``. And the UNIFORM BURN: an unknown username verifies against a DECOY hash, so \"no such user\" and \"wrong password\" cost one argon2 run each and answer one 401 — no stopwatch enumerates your usernames:"),
			code("hashing-uniform-beat-go", "go",
				"Verify and the uniform decoy burn: constant-time compare, same work for unknown users.",
				"server/internal/domain/auth/password.go",
				"	got := argon2.IDKey([]byte(password), salt,\n"+
					"		params.Iterations, params.MemoryKiB, params.Parallelism, uint32(len(want)))\n"+
					"	if subtle.ConstantTimeCompare(got, want) != 1 {\n"+
					"		return ErrInvalidCredentials\n"+
					"	}\n"+
					"	return nil\n"+
					"}\n"+
					"\n"+
					"func (h *Hasher) VerifyUniform(password, encoded string) bool {\n"+
					"	if encoded == \"\" {\n"+
					"		encoded = DecoyHash\n"+
					"	}\n"+
					"	return h.Verify(password, encoded) == nil\n"+
					"}"),
			text("hashing-format-pitch",
				"The stored format is the PHC string — self-describing, parseable by every argon2 implementation, and the reason a hash row needs no companion parameter columns: ``$argon2id$v=19$m=19456,t=2,p=1$<salt>$<key>``. A malformed stored hash is a LOUD error in this engine, not a failed match: a corrupt or legacy-format row deserves a 500 and a fix, not a silent 401 that quietly locks a user out while looking exactly like a wrong password. The demo mints two hashes of one password to show the salt working, verifies right and wrong with their real timings side by side, feeds a 2015-cost hash to fire the rehash signal, and shows the decoy burn's cost matching the wrong-password cost to the tenth of a millisecond."),
			demo("auth:hashing", "The hashing lab: salt, cost, rehash signal, uniform burn",
				"One request at the guide's real parameters: the SAME password hashed twice produces two different strings (fresh salt — both verify), a correct verification burns one full argon2 run and reports needs_rehash=false, a WRONG password burns the same-shaped run and reports it (match and mismatch cost the same), a legacy-cost hash (m=8192, t=1 — 2015) still VERIFIES but fires the rehash signal, an unknown-username login burns one decoy verification with its cost shown beside the wrong-password cost, and a malformed stored string is refused as a loud error rather than a silent mismatch."),
		),
		// ── 9. Refresh rotation ─────────────────────────────────────
		section("refresh-rotation", "Refresh rotation and reuse detection",
			"The JWT's missing kill switch, built from three choices: opaque server-remembered handles, one-time rotation, and reuse treated as theft — the family dies the moment one link is presented twice.",
			text("refresh-pitch",
				"The production shape this topic has been building toward reconciles the JWT's best property with its worst: access tokens stay SHORT (15 minutes here — stateless, fast, cheap to lose), and a REFRESH token carries the long lease (24 hours) as an opaque handle the SERVER remembers. The client stores the refresh token durably (never in page JavaScript), presents it when the access token expires, and receives a FRESH pair: new access token, NEW refresh token. That last part is the ROTATION, and it is one-time: the presented token is spent the moment it is exchanged. The client that rotates on receipt can never trip the alarm itself; only someone holding a COPY of an already-spent token can."),
			text("reuse-pitch",
				"Which is exactly the point. Two honest parties never present the same refresh token twice — so the second presentation of a spent token is not an error to forgive, it is THEFT IN PROGRESS: a copy exists in someone else's hands. The only safe response is to kill the ENTIRE FAMILY — every token ever minted in that login's chain, current and future — and force a fresh login. The honest client is briefly inconvenienced; the thief is locked out with the loot. This is the alarm working, and the demo trips it deliberately: it logs in, rotates twice, then replays the SPENT position-2 token and reports the kill count (three tokens — the whole chain), then proves fail-closed by presenting the rightful client's NEWEST token and being refused with it:"),
			code("refresh-reuse-beat-go", "go",
				"The reuse branch in the real engine: revoke the family, answer the kill count — the alarm, as code.",
				"server/internal/domain/auth/refresh.go",
				"	if stored.Status == \"used\" {\n"+
					"		killed, err := s.store.RevokeFamily(ctx, stored.FamilyID)\n"+
					"		if err != nil {\n"+
					"			return RefreshExchangeResult{}, fmt.Errorf(\"revoke refresh family: %w\", err)\n"+
					"		}\n"+
					"		return RefreshExchangeResult{\n"+
					"			Outcome:       RefreshReuseDetected,\n"+
					"			FamilyID:      stored.FamilyID,\n"+
					"			RotatedFrom:   stored.Position,\n"+
					"			FamilyRevoked: killed,\n"+
					"		}, nil\n"+
					"	}"),
			text("refresh-storage-pitch",
				"Storage honesty, the same discipline everywhere in this topic: refresh tokens are stored as SHA-256 hashes (high-entropy handles — no stretching needed, the API-key rule), the RAW value rides exactly one response (the exchange that minted it), and the family bookkeeping — who owns the chain, which positions exist, which are used or revoked — is server-side state the adapter owns. The guide's in-memory store makes the chain inspectable for the demo; production puts the same rows behind Redis or PostgreSQL with the TTL doing eviction. What does NOT change is the protocol shape, because it is the one OAuth2 providers converged on after tokens lived long enough to be stolen: short self-carried access, long server-held refresh, rotation on every exchange, reuse detection as an alarm rather than an inconvenience."),
			demo("auth:refresh", "The refresh story: family born, rotated, killed by reuse",
				"One request walks one family's whole life: login starts family fam_<random> at position 1 (the raw first refresh token shown once, its SHA-256 all the store keeps), two exchanges rotate positions 1→2 and 2→3 (each presentation spent, each answer a fresh access token plus the next link), then the ALREADY-SPENT position-2 token is presented again — reuse detected, the ENTIRE family revoked with the kill count reported (3 tokens, past and future), and the rightful client's newest token — never reused by anyone honest — is refused with the family: fail closed, fresh login required. The alarm works."),
		),
		// ── 10. Terminal companion ───────────────────────────────────
		section("terminal-companion", "Terminal companion: the auth curl tour",
			"All seven demos by curl, the guarded route's honest 401, and the topic count on the wire — every command against the running stack on :8100.",
			text("term-auth-pitch",
				"With the stack up (server on :8100), this tour walks the whole auth surface from the shell. Each demo is ONE request that runs its full story server-side; the interesting reading is the beats array (each with its verdict: ok, rejected, or the reuse alarm) and the proofs. The last stop is the guarded ``whoami`` route with NO credential — the middleware's uniform 401 envelope, which is the topic's thesis in one response: not authenticated, and no hint about which half you would have needed:"),
			code("term-demos", "bash", "", "",
				"# the seven stories — beats, facts, proofs, reasoning per story\n"+
					"curl -s localhost:8100/api/demo/auth/sessions  | python3 -m json.tool | head -30\n"+
					"curl -s localhost:8100/api/demo/auth/api-keys  | python3 -m json.tool | head -30\n"+
					"curl -s localhost:8100/api/demo/auth/compare   | python3 -m json.tool | head -30\n"+
					"curl -s localhost:8100/api/demo/auth/jwt       | python3 -m json.tool | head -30\n"+
					"curl -s localhost:8100/api/demo/auth/oauth     | python3 -m json.tool | head -30\n"+
					"curl -s localhost:8100/api/demo/auth/hashing   | python3 -m json.tool | head -30\n"+
					"curl -s localhost:8100/api/demo/auth/refresh   | python3 -m json.tool | head -30"),
			code("term-guard", "bash", "", "",
				"# the guarded route, no credential — the auth middleware's uniform 401 envelope\n"+
					"curl -si localhost:8100/api/demo/auth/whoami | head -12\n"+
					"# …and with a bogus bearer: same 401, same shape, no detail about which half failed\n"+
					"curl -si -H 'Authorization: Bearer not-a-real-token' localhost:8100/api/demo/auth/whoami | head -8\n"+
					"# the reuse alarm in one line — the verdict the refresh story derives live\n"+
					"curl -s localhost:8100/api/demo/auth/refresh | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d[\"beats\"][2][\"verdict\"], \"—\", d[\"proofs\"][1][\"value\"])'"),
			code("term-content", "bash", "", "",
				"# the topic on the wire — order 23 in the state-auth group\n"+
					"curl -s localhost:8100/api/content | python3 -c 'import json,sys; d=json.load(sys.stdin); print([t[\"id\"] for t in d[\"topics\"] if t[\"group\"]==\"state-auth\"])'\n"+
					"curl -s localhost:8100/api/content/authentication | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d[\"title\"], \"·\", d[\"sectionCount\"], \"sections\")'"),
			text("term-auth-coda",
				"Read the tour as seven proofs you can re-run at will: the sessions story's last beat (the pre-logout cookie resolving to nothing), the key story's one-time raw value, the comparison's STILL-VALID JWT after every revocation, the JWT lab's alg=none refusal, the OAuth story's burned code, the hashing lab's matching millisecond costs, and the refresh story's three-token kill. That is the authentication spine standing: establish, carry, verify, retire — and the identity that comes out the far end is exactly what topic 24 consumes to decide what the caller may DO."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"authentication",
		"Authentication from Go",
		"Who is calling: sessions done right behind a sealed HttpOnly cookie, API keys hashed at rest and compared in "+
			"constant time, HS256 JWTs minted and attacked from the standard library alone, the OAuth2 code grant with "+
			"PKCE run honestly in one process, the sessions-vs-keys-vs-tokens decision measured live, argon2id password "+
			"hashing with the rehash signal and a uniform decoy burn, and refresh rotation whose reuse detection turns a "+
			"stolen token into an alarm — with the auth middleware that resolves every carrier to one identity, and the "+
			"terminal's curl tour.",
		"state-auth",
		23,
		sections,
	)
}
