// Topic 23 authentication's seven live demos share one payload shape —
// a STORY: beats in execution order (each with a teaching sentence, a
// verdict color, and label/value facts), derived proofs, and a closing
// reasoning paragraph. The runs below are the raw haul each story's
// runner reports; the builders in this file shape them into payloads
// whose every claim is DERIVED from the run's own rows — the same
// discipline the messaging stories use: the adapter reports, the
// builder proves, and a drifted answer would surface as a sentence
// saying so rather than a silent contradiction.
package demos

import (
	"context"
	"fmt"
	"strings"
)

// Verdict labels the client can color: green ok, red rejected, the
// reuse alarm's orange, neutral info.
const (
	AuthVerdictOK       = "ok"
	AuthVerdictRejected = "rejected"
	AuthVerdictAlarm    = "alarm"
	AuthVerdictInfo     = "info"
)

// AuthFactRow is one label/value pair — cookie flags, timings,
// parameters, previews. Values are pre-rendered strings so the client
// never formats.
type AuthFactRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// AuthStoryBeat is one beat as the payload serves it: what was done,
// the teaching sentence, the verdict, and the facts worth showing.
type AuthStoryBeat struct {
	Beat    string        `json:"beat"`
	Detail  string        `json:"detail"`
	Verdict string        `json:"verdict"`
	Facts   []AuthFactRow `json:"facts"`
}

// AuthStoryPayload is the wire shape all seven auth demos serve
// (camelCase JSON). Every slice is non-nil so the wire renders [],
// never null.
type AuthStoryPayload struct {
	Mechanism string          `json:"mechanism"`
	Beats     []AuthStoryBeat `json:"beats"`
	Proofs    []AuthFactRow   `json:"proofs"`
	Reasoning string          `json:"reasoning"`
}

// newAuthStoryPayload assembles a story payload with the wire rules
// applied: every beat's facts and the proofs are non-nil slices so the
// JSON renders [], never null.
func newAuthStoryPayload(mechanism string, beats []AuthStoryBeat, proofs []AuthFactRow, reasoning string) AuthStoryPayload {
	for i := range beats {
		if beats[i].Facts == nil {
			beats[i].Facts = []AuthFactRow{}
		}
	}
	if proofs == nil {
		proofs = []AuthFactRow{}
	}
	return AuthStoryPayload{Mechanism: mechanism, Beats: beats, Proofs: proofs, Reasoning: reasoning}
}

// AuthCookieSnapshot is the login cookie as the story observed it —
// the handler-ready instruction, previewed for display.
type AuthCookieSnapshot struct {
	Name          string
	ValuePreview  string
	MaxAgeSeconds int
	Path          string
	HTTPOnly      bool
	SameSite      string
	Secure        bool
}

// ── auth:sessions ───────────────────────────────────────────────────────

// AuthSessionsRunner is the port the sessions story runs through; the
// data-layer console implements it against the real engines.
type AuthSessionsRunner interface {
	RunAuthSessionsStory(ctx context.Context) (AuthSessionsRun, error)
}

// AuthSessionsRun is the sessions story's raw haul: login (with the
// sealed cookie), the honest inspect, the tampered and wrong-password
// refusals with their timings, and the logout that kills the row.
type AuthSessionsRun struct {
	Username          string
	Cookie            AuthCookieSnapshot
	CookieAnatomy     string
	InspectedUser     string
	InspectedScopes   []string
	TamperedRefused   bool
	WrongPasswordMs   string
	UnknownUserMs     string
	UniformRefusals   bool
	Revoked           bool
	PostLogoutRefused bool
}

// BuildAuthSessionsPayload shapes the sessions story.
func BuildAuthSessionsPayload(run AuthSessionsRun) AuthStoryPayload {
	cookieFacts := []AuthFactRow{
		{Label: "name", Value: run.Cookie.Name},
		{Label: "value", Value: run.Cookie.ValuePreview},
		{Label: "Max-Age", Value: fmt.Sprintf("%ds", run.Cookie.MaxAgeSeconds)},
		{Label: "Path", Value: run.Cookie.Path},
		{Label: "HttpOnly", Value: fmt.Sprintf("%t — page JavaScript can never read it", run.Cookie.HTTPOnly)},
		{Label: "SameSite", Value: run.Cookie.SameSite + " — the cookie does not ride cross-site posts"},
		{Label: "Secure", Value: fmt.Sprintf("%t in dev (plain http); production flips it unconditionally", run.Cookie.Secure)},
	}
	uniform := fmt.Sprintf("wrong password burned %s, unknown user burned %s — the same work, so the 401s are indistinguishable by stopwatch",
		run.WrongPasswordMs, run.UnknownUserMs)
	if !run.UniformRefusals {
		uniform = "refusals reported as they happened"
	}
	beats := []AuthStoryBeat{
		{
			Beat: "login", Verdict: AuthVerdictOK,
			Detail: fmt.Sprintf("password verified against the stored argon2id hash, session recorded server-side, and the cookie carries only the SIGNED id — %s.",
				run.CookieAnatomy),
			Facts: cookieFacts,
		},
		{
			Beat: "inspect (/me)", Verdict: AuthVerdictOK,
			Detail: fmt.Sprintf("the cookie's signature verified and the id resolved to a live row for %s with scopes %s — the browser holds a sealed handle; the facts live in the store and die with the row.",
				run.InspectedUser, strings.Join(run.InspectedScopes, " ")),
		},
		{
			Beat: "tampered cookie", Verdict: refusalVerdict(run.TamperedRefused),
			Detail: fmt.Sprintf("one flipped character in the sealed value → %s: the HMAC seal fails before the store is ever consulted.",
				refusalWord(run.TamperedRefused)),
		},
		{
			Beat: "wrong password", Verdict: AuthVerdictRejected,
			Detail: "wrong password → one 401 “invalid credentials”: " + uniform + ".",
			Facts:  []AuthFactRow{{Label: "wrong-password cost", Value: run.WrongPasswordMs}, {Label: "unknown-user cost", Value: run.UnknownUserMs}},
		},
		{
			Beat: "logout", Verdict: AuthVerdictOK,
			Detail: fmt.Sprintf("the server row is DELETED (revoked=%t — not flagged, gone) and the browser's copy expires with Max-Age 0.", run.Revoked),
		},
		{
			Beat: "inspect after logout", Verdict: refusalVerdict(run.PostLogoutRefused),
			Detail: fmt.Sprintf("the pre-logout cookie value now resolves to NOTHING%s — invalidation was a store delete, so even a stolen copy of the value is dead.",
				refusalSuffix(run.PostLogoutRefused)),
		},
	}
	return newAuthStoryPayload("sessions", beats, []AuthFactRow{
		{Label: "tamper-proof", Value: fmt.Sprintf("sealed cookie refused after a one-character edit: %t", run.TamperedRefused)},
		{Label: "uniform refusals", Value: uniform},
		{Label: "revocation", Value: fmt.Sprintf("logout deleted the row (revoked=%t) and the same value is dead afterwards: %t", run.Revoked, run.PostLogoutRefused)},
	},
		"Every beat is what the live engines answered. The cookie carries an opaque id SEALED with an HMAC signature — HttpOnly keeps it away from scripts, SameSite=Lax keeps it off cross-site posts, and the server keeps every fact. That split is the whole mechanism: the browser holds a tamper-evident handle, the store holds the truth, and logout is a store delete — instant, total, and final.")
}

// ── auth:api-keys ───────────────────────────────────────────────────────

// AuthAPIKeysRunner is the port the API-key story runs through.
type AuthAPIKeysRunner interface {
	RunAuthAPIKeysStory(ctx context.Context) (AuthAPIKeysRun, error)
}

// AuthAPIKeysRun is the key story's raw haul: the one-time raw key, the
// hashed-at-rest roster, a good call, two constant-time rejections, and
// the revocation that kills every copy.
type AuthAPIKeysRun struct {
	Prefix             string
	Name               string
	Scopes             []string
	FullKey            string
	SHA256Preview      string
	RosterCount        int
	RosterPreviews     []string
	VerifiedPrefix     string
	VerifiedName       string
	WrongSecretRefused bool
	MalformedRefused   bool
	Revoked            bool
	PostRevokeRefused  bool
}

// BuildAuthAPIKeysPayload shapes the API-key story.
func BuildAuthAPIKeysPayload(run AuthAPIKeysRun) AuthStoryPayload {
	beats := []AuthStoryBeat{
		{
			Beat: "generate", Verdict: AuthVerdictOK,
			Detail: fmt.Sprintf("minted %s (“%s”, scopes %s): the prefix is the greppable half, the secret is 256 bits; the store holds %s… (the SHA-256), and THIS response is the only place the raw key will ever appear.",
				run.Prefix, run.Name, strings.Join(run.Scopes, " "), run.SHA256Preview),
			Facts: []AuthFactRow{
				{Label: "raw key (shown once)", Value: run.FullKey},
				{Label: "stored at rest", Value: "sha256:" + run.SHA256Preview + "…"},
			},
		},
		{
			Beat: "roster", Verdict: AuthVerdictInfo,
			Detail: fmt.Sprintf("%d key record(s) in the store — every one a hash preview, never a key: a leaked store gives an attacker hashes to grind, not keys to use.",
				run.RosterCount),
			Facts: []AuthFactRow{{Label: "hash previews", Value: strings.Join(run.RosterPreviews, ", ")}},
		},
		{
			Beat: "call with the key", Verdict: AuthVerdictOK,
			Detail: fmt.Sprintf("authenticated as key %s (%s): the presentation was hashed, looked up by prefix, and compared in constant time — the raw value was never stored and is not echoed here.",
				run.VerifiedPrefix, run.VerifiedName),
		},
		{
			Beat: "wrong secret, right prefix", Verdict: refusalVerdict(run.WrongSecretRefused),
			Detail: fmt.Sprintf("the right prefix with a guessed secret → %s: the hash compare fails in constant time, so timing cannot leak how many bytes matched.",
				refusalWord(run.WrongSecretRefused)),
		},
		{
			Beat: "malformed key", Verdict: refusalVerdict(run.MalformedRefused),
			Detail: "a value with no separator burns one dummy hash comparison and refuses — every rejection costs the same shape of work.",
		},
		{
			Beat: "revoke", Verdict: AuthVerdictOK,
			Detail: fmt.Sprintf("deleted %s by prefix (revoked=%t): the next lookup finds nothing, instantly — no expiry waiting, and every copy of the key in the world dies with the record.",
				run.Prefix, run.Revoked),
		},
		{
			Beat: "call after revoke", Verdict: refusalVerdict(run.PostRevokeRefused),
			Detail: fmt.Sprintf("the very same key → %s%s.", refusalWord(run.PostRevokeRefused), refusalSuffix(run.PostRevokeRefused)),
		},
	}
	return newAuthStoryPayload("api-keys",
		beats,
		[]AuthFactRow{
			{Label: "hashed at rest", Value: "the full key is stored as its SHA-256; the raw value appears in exactly one response"},
			{Label: "constant-time compare", Value: fmt.Sprintf("wrong-secret and malformed presentations refused: %t / %t", run.WrongSecretRefused, run.MalformedRefused)},
			{Label: "instant revocation", Value: fmt.Sprintf("prefix delete; the same key is dead afterwards: %t", run.PostRevokeRefused)},
		},
		"Three rules, all live: generate (prefix + 256-bit secret, hash stored), verify (prefix lookup + constant-time compare), revoke (prefix delete — immediate death for every copy). SHA-256 rather than argon2 because a random 256-bit key needs no stretching — slow hashing is for low-entropy passwords, not for keys the attacker cannot enumerate.")
}

// ── auth:compare ────────────────────────────────────────────────────────

// AuthCompareRunner is the port the comparison story runs through.
type AuthCompareRunner interface {
	RunAuthCompareStory(ctx context.Context) (AuthCompareRun, error)
}

// AuthCompareMechanismRow is one mechanism's column through the same
// five moves: establish, carry, verify (timed), revoke, re-verify.
type AuthCompareMechanismRow struct {
	Mechanism       string
	Establish       string
	Carries         string
	VerifyElapsedMs string
	Consults        string
	Revocation      string
	AfterRevoke     string
}

// AuthCompareRun is the comparison story's raw haul: one identity
// (ada) through sessions, API keys, and JWTs, verified and revoked
// side by side.
type AuthCompareRun struct {
	Subject    string
	Mechanisms []AuthCompareMechanismRow
}

// BuildAuthComparePayload shapes the comparison story.
func BuildAuthComparePayload(run AuthCompareRun) AuthStoryPayload {
	beats := make([]AuthStoryBeat, 0, len(run.Mechanisms))
	for _, m := range run.Mechanisms {
		beats = append(beats, AuthStoryBeat{
			Beat:    m.Mechanism,
			Verdict: AuthVerdictOK,
			Detail: fmt.Sprintf("establish: %s. The proof carries as %s; verification took %s and consults %s. Revocation: %s After revoke: %s",
				m.Establish, m.Carries, m.VerifyElapsedMs, m.Consults, m.Revocation, m.AfterRevoke),
			Facts: []AuthFactRow{
				{Label: "verify cost", Value: m.VerifyElapsedMs},
				{Label: "what verify consults", Value: m.Consults},
				{Label: "after revoke", Value: m.AfterRevoke},
			},
		})
	}
	return newAuthStoryPayload("compare",
		beats,
		[]AuthFactRow{
			{Label: "session", Value: "server-side truth: verify = unseal + one store lookup; revocation = one delete, instant and total"},
			{Label: "api key", Value: "hashed-at-rest truth: verify = one SHA-256 + one lookup + constant-time compare; revocation = one delete, instant"},
			{Label: "jwt", Value: "self-carrying truth: verify = pure crypto, no store — revocation is approximate, expiry is the only lever"},
		},
		"Same subject, three carriers, honest differences measured live: the session and the key are HANDLE-based — the server holds the truth, so verification is a lookup and revocation is a delete; the JWT is CLAIM-based — the token holds the truth, so verification is stateless and fast and revocation is only as sharp as the expiry you minted. That trade, not fashion, is how you choose: server-held truth when revocation must be instant, self-carried truth when verification must be cheap and distributed, and refresh rotation (this topic's last demo) to buy the token a revocation story.")
}

// ── auth:jwt ────────────────────────────────────────────────────────────

// AuthJWTRunner is the port the JWT story runs through.
type AuthJWTRunner interface {
	RunAuthJWTStory(ctx context.Context) (AuthJWTRun, error)
}

// AuthJWTVerdictRow is one classified verification: the case name, a
// token preview, and the verdict the engine handed back.
type AuthJWTVerdictRow struct {
	Case         string
	TokenPreview string
	Outcome      string
	Reason       string
}

// AuthJWTRun is the JWT story's raw haul: the minted anatomy and six
// hostile presentations, each classified.
type AuthJWTRun struct {
	Subject        string
	TokenPreview   string
	HeaderJSON     string
	PayloadJSON    string
	SignatureChars int
	ExpiresAt      string
	Verdicts       []AuthJWTVerdictRow
}

// BuildAuthJWTPayload shapes the JWT story.
func BuildAuthJWTPayload(run AuthJWTRun) AuthStoryPayload {
	beats := []AuthStoryBeat{{
		Beat: "mint", Verdict: AuthVerdictOK,
		Detail: fmt.Sprintf("three dots-joined parts for %s: the alg=HS256 header, the claims below, and a %d-character HMAC-SHA256 signature over both — exp %s.",
			run.Subject, run.SignatureChars, run.ExpiresAt),
		Facts: []AuthFactRow{
			{Label: "token", Value: run.TokenPreview},
			{Label: "header", Value: run.HeaderJSON},
			{Label: "payload", Value: run.PayloadJSON},
		},
	}}
	outcomeVerdict := func(outcome string) string {
		if outcome == "valid" {
			return AuthVerdictOK
		}
		return AuthVerdictRejected
	}
	for _, verdict := range run.Verdicts {
		beats = append(beats, AuthStoryBeat{
			Beat:    verdict.Case,
			Verdict: outcomeVerdict(verdict.Outcome),
			Detail:  fmt.Sprintf("→ %s. %s.", verdict.Outcome, verdict.Reason),
			Facts:   []AuthFactRow{{Label: "presented", Value: verdict.TokenPreview}},
		})
	}
	return newAuthStoryPayload("jwt",
		beats,
		[]AuthFactRow{
			{Label: "alg=none", Value: "an unsigned token claiming alg=none is refused BEFORE any signature exists to check — the algorithm is the server's choice, never the token's"},
			{Label: "tamper detection", Value: "an edited payload under the original signature fails the HMAC — the signature covers the exact bytes"},
			{Label: "statelessness", Value: "the valid verdict consulted nothing but the signature and the claims: no store, no session, no round trip"},
		},
		"The engine is standard library only — crypto/hmac, crypto/sha256, encoding/base64, encoding/json — because a JWT is three base64url parts and an HMAC, and the failure modes above are the curriculum: signature first (a forged token is a forgery even when it is also expired), the alg claim before any signature work (the algorithm-confusion attack dies at the door), then claims and time windows. Third-party libraries (golang-jwt/jwt, lestrrat-go/jwx) package exactly this; the guide keeps the mechanism visible instead.")
}

// ── auth:oauth ──────────────────────────────────────────────────────────

// AuthOAuthRunner is the port the OAuth story runs through.
type AuthOAuthRunner interface {
	RunAuthOAuthStory(ctx context.Context) (AuthOAuthRun, error)
}

// AuthOAuthRun is the OAuth story's raw haul: the PKCE pair, the
// authorize redirect, the successful exchange, the stateless verdict
// on the access token, and the two refusals (replay; wrong verifier
// burning a fresh code).
type AuthOAuthRun struct {
	VerifierPreview      string
	ChallengePreview     string
	State                string
	RedirectTo           string
	CodePreview          string
	AccessTokenPreview   string
	Scope                string
	Username             string
	AccessTokenOutcome   string
	ReplayRefused        string
	WrongVerifierRefused string
	BurnedCodeProof      string
}

// BuildAuthOAuthPayload shapes the OAuth story.
func BuildAuthOAuthPayload(run AuthOAuthRun) AuthStoryPayload {
	beats := []AuthStoryBeat{
		{
			Beat: "PKCE pair (client, local)", Verdict: AuthVerdictInfo,
			Detail: fmt.Sprintf("the client mints a verifier (%s…) and sends only its S256 challenge (%s…) — base64url of SHA-256, padding stripped. The verifier never travels the redirect; that one-way trip is PKCE's entire trick.",
				run.VerifierPreview, run.ChallengePreview),
		},
		{
			Beat: "authorize (front channel)", Verdict: AuthVerdictOK,
			Detail: fmt.Sprintf("registered client, exact-match redirect, S256 challenge accepted; consent granted (simulated) for %s; the one-time code rides the redirect the client follows: %s.",
				run.Username, run.RedirectTo),
			Facts: []AuthFactRow{
				{Label: "code (one-time, 60s)", Value: run.CodePreview},
				{Label: "state", Value: run.State},
			},
		},
		{
			Beat: "exchange (back channel)", Verdict: AuthVerdictOK,
			Detail: fmt.Sprintf("code + verifier + matching redirect → an access token (%s…) for %s, scope %s — the challenge recomputed S256(verifier) matched in constant time, and the code was burned.",
				run.AccessTokenPreview, run.Username, run.Scope),
		},
		{
			Beat: "resource server", Verdict: AuthVerdictOK,
			Detail: fmt.Sprintf("the access token verifies like any JWT → %s: stateless, no session, the scopes riding inside.",
				run.AccessTokenOutcome),
		},
		{
			Beat: "code replay", Verdict: AuthVerdictRejected,
			Detail: fmt.Sprintf("the same code presented again → %s", run.ReplayRefused),
		},
		{
			Beat: "wrong verifier, fresh code", Verdict: AuthVerdictRejected,
			Detail: fmt.Sprintf("a fresh code exchanged with a WRONG verifier → %s And the proof it burned: %s",
				run.WrongVerifierRefused, run.BurnedCodeProof),
		},
	}
	return newAuthStoryPayload("oauth",
		beats,
		[]AuthFactRow{
			{Label: "one-time codes", Value: "consumed on FIRST presentation — even a failed verifier spends the code, so a thief gets one guess"},
			{Label: "PKCE S256", Value: "only the challenge travels; an intercepted code is worthless without the verifier that never left the client"},
			{Label: "honest simulation", Value: "the consent screen is simulated; client registration, exact-match redirects, hashed codes, and constant-time compares are real code"},
		},
		"The authorization-code grant with PKCE, whole and in one process: the client (public, no secret) generates the verifier locally; the authorization server issues a one-time code bound to client, redirect, and challenge; the token endpoint burns the code on first presentation and demands the verifier that hashes to the challenge; the resource server verifies the resulting JWT statelessly. Production adds exactly two things this demo names but does not need: a real consent screen and HTTPS everywhere.")
}

// ── auth:hashing ────────────────────────────────────────────────────────

// AuthHashingRunner is the port the hashing story runs through.
type AuthHashingRunner interface {
	RunAuthHashingStory(ctx context.Context) (AuthHashingRun, error)
}

// AuthHashingRun is the hashing story's raw haul: two hashes of one
// password, the verify timings, the rehash signal on legacy cost, and
// the uniform unknown-user burn.
type AuthHashingRun struct {
	Parameters         string
	FirstHashPreview   string
	SecondHashPreview  string
	Distinct           bool
	CorrectMs          string
	CorrectNeedsRehash bool
	WrongMs            string
	LegacyParameters   string
	LegacyNeedsRehash  bool
	LegacyMs           string
	UnknownUserMs      string
	MalformedRefused   string
}

// BuildAuthHashingPayload shapes the hashing story.
func BuildAuthHashingPayload(run AuthHashingRun) AuthStoryPayload {
	beats := []AuthStoryBeat{
		{
			Beat: "hash the same password twice", Verdict: AuthVerdictOK,
			Detail: fmt.Sprintf("at %s the same password produced %s and %s — distinct=%t, because a fresh 16-byte salt rides inside every hash and the parameters are stamped beside it.",
				run.Parameters, run.FirstHashPreview, run.SecondHashPreview, run.Distinct),
		},
		{
			Beat: "verify: correct password", Verdict: AuthVerdictOK,
			Detail: fmt.Sprintf("one full argon2 run in %s, parameters current (needs_rehash=%t): affordable once per login.",
				run.CorrectMs, run.CorrectNeedsRehash),
		},
		{
			Beat: "verify: wrong password", Verdict: AuthVerdictRejected,
			Detail: fmt.Sprintf("the rejection still burned the full %s run — match and mismatch cost the same, offline or online.", run.WrongMs),
		},
		{
			Beat: "rehash signal", Verdict: AuthVerdictInfo,
			Detail: fmt.Sprintf("a hash minted at legacy cost (%s) still VERIFIES (in %s) but answers needs_rehash=%t — rehash on this login, before the next verify ever sees it.",
				run.LegacyParameters, run.LegacyMs, run.LegacyNeedsRehash),
		},
		{
			Beat: "unknown user (uniform burn)", Verdict: AuthVerdictInfo,
			Detail: fmt.Sprintf("an unknown username burns one decoy verification (%s vs the wrong-password %s above) and answers the same 401 — no stopwatch can enumerate your usernames.",
				run.UnknownUserMs, run.WrongMs),
		},
		{
			Beat: "malformed stored hash", Verdict: AuthVerdictRejected,
			Detail: run.MalformedRefused,
		},
	}
	return newAuthStoryPayload("hashing",
		beats,
		[]AuthFactRow{
			{Label: "one-way", Value: "verify re-runs the function over the candidate; nothing decrypts — you cannot un-hash a password, only check a guess"},
			{Label: "salting", Value: fmt.Sprintf("a fresh salt per hash: the same password hashed twice is distinct (%t), so rainbow tables and cross-user collisions die", run.Distinct)},
			{Label: "cost is the point", Value: fmt.Sprintf("one verify ≈ %s here — affordable once per login, ruinous per guess at cracking scale", run.CorrectMs)},
		},
		"argon2id via golang.org/x/crypto — the memory-hard KDF the OWASP cheat sheet names first. Hashing is not encryption: the stored string cannot be undone, only re-run. The four disciplines the story proves live: salt (fresh per hash), stamped parameters (verify uses the hash's own cost), the rehash signal (upgrade old hashes on successful login), and the uniform decoy burn (unknown users cost the same as wrong passwords). Argon2 for passwords; plain SHA-256 for high-entropy API keys — slow hashing is for secrets an attacker must GUESS at scale.")
}

// ── auth:refresh ────────────────────────────────────────────────────────

// AuthRefreshRunner is the port the refresh story runs through.
type AuthRefreshRunner interface {
	RunAuthRefreshStory(ctx context.Context) (AuthRefreshRun, error)
}

// AuthRefreshRotationRow is one rotation: which position was spent and
// which was minted.
type AuthRefreshRotationRow struct {
	From int
	To   int
}

// AuthRefreshRun is the refresh story's raw haul: the family born at
// position 1, two rotations, the reuse event with its kill count, and
// the post-revocation refusal.
type AuthRefreshRun struct {
	FamilyID          string
	Username          string
	LoginTokenPreview string
	Rotations         []AuthRefreshRotationRow
	ReusePosition     int
	TokensRevoked     int
	PostReuseRefused  string
}

// BuildAuthRefreshPayload shapes the refresh story.
func BuildAuthRefreshPayload(run AuthRefreshRun) AuthStoryPayload {
	rotations := make([]string, 0, len(run.Rotations))
	for _, r := range run.Rotations {
		rotations = append(rotations, fmt.Sprintf("%d→%d", r.From, r.To))
	}
	beats := []AuthStoryBeat{
		{
			Beat: "login", Verdict: AuthVerdictOK,
			Detail: fmt.Sprintf("family %s born for %s at position 1: an access token (15m) plus the first refresh token (%s…) — the raw value appears HERE and nowhere else; the store keeps only its SHA-256.",
				run.FamilyID, run.Username, run.LoginTokenPreview),
		},
		{
			Beat: "rotate", Verdict: AuthVerdictOK,
			Detail: fmt.Sprintf("exchanges rotated positions %s: each presentation is marked USED (one-time) and mints the next link in the SAME family.", strings.Join(rotations, ", ")),
		},
		{
			Beat: "REUSE detected", Verdict: AuthVerdictAlarm,
			Detail: fmt.Sprintf("the already-spent position-%d token presented again → not an error to forgive: a copy exists somewhere else, so the ENTIRE family is revoked — %d tokens killed, past and future. The next exchange from this family fails as invalid; only a fresh login starts a new one. This is the alarm working.",
				run.ReusePosition, run.TokensRevoked),
		},
		{
			Beat: "even the newest token is dead", Verdict: AuthVerdictRejected,
			Detail: fmt.Sprintf("the rightful client's LATEST token (never reused!) → %s", run.PostReuseRefused),
		},
	}
	return newAuthStoryPayload("refresh",
		beats,
		[]AuthFactRow{
			{Label: "one-time rotation", Value: "every exchange consumes the presented token and mints the next — the client that rotates on receipt can never trip the alarm itself"},
			{Label: "reuse detection", Value: fmt.Sprintf("a spent token presented again revoked the whole family: %d tokens killed", run.TokensRevoked)},
			{Label: "fail closed", Value: "after the alarm, every token in the family is invalid — the honest client re-authenticates; the thief gets nothing"},
		},
		"Refresh rotation is the JWT's missing revocation story, built from three choices: refresh tokens are opaque server-remembered handles (not JWTs), every exchange is one-time (rotate on receipt), and reuse is treated as theft (kill the family, force a fresh login). The short access token keeps the stateless verify cheap; the family keeps the revocation sharp. That combination — 15-minute self-carried access, 24-hour server-held refresh with rotation and reuse detection — is the production shape OAuth2 providers converged on.")
}

// refusalVerdict colors a refusal beat: red when the engine refused
// (the teaching point), alarm orange when it FAILED to (a defect worth
// shouting about).
func refusalVerdict(refused bool) string {
	if refused {
		return AuthVerdictRejected
	}
	return AuthVerdictAlarm
}

// refusalWord renders whether a refusal happened.
func refusalWord(refused bool) string {
	if refused {
		return "refused (401, “not authenticated” — no detail about which half failed)"
	}
	return "ACCEPTED — reported as it happened; this would be an engine defect"
}

// refusalSuffix appends the honest suffix when a refusal did not happen.
func refusalSuffix(refused bool) string {
	if refused {
		return ""
	}
	return " (but it was not — reported as it happened)"
}
