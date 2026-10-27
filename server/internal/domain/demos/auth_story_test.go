package demos

import (
	"encoding/json"
	"strings"
	"testing"
)

// The auth story payload builders: derived-sentence and wire-shape
// pins. The stories themselves run against the real console in
// internal/data/auth; these tests pin the BUILDER contracts — verdict
// mapping, non-nil facts, and the [] never null wire rule.

func authStoryFixturePayloads() []AuthStoryPayload {
	return []AuthStoryPayload{
		BuildAuthSessionsPayload(AuthSessionsRun{
			Username:      "ada",
			Cookie:        AuthCookieSnapshot{Name: "gdg_session", ValuePreview: "abcdefghijk…", MaxAgeSeconds: 1800, Path: "/", HTTPOnly: true, SameSite: "lax"},
			CookieAnatomy: "abcdefgh… . 43 signature chars — the opaque session id, sealed with an HMAC-SHA256 signature; edit one character and the seal is broken",
			InspectedUser: "ada", InspectedScopes: []string{"posts:read"},
			TamperedRefused: true, WrongPasswordMs: "42.1 ms", UnknownUserMs: "42.3 ms",
			UniformRefusals: true, Revoked: true, PostLogoutRefused: true,
		}),
		BuildAuthAPIKeysPayload(AuthAPIKeysRun{
			Prefix: "gdg_deadbeef", Name: "ci pipeline", Scopes: []string{"posts:read"},
			FullKey: "gdg_deadbeef.raw-secret", SHA256Preview: "0123456789ab",
			RosterCount: 1, RosterPreviews: []string{"sha256:0123456789ab"},
			VerifiedPrefix: "gdg_deadbeef", VerifiedName: "ci pipeline",
			WrongSecretRefused: true, MalformedRefused: true, Revoked: true, PostRevokeRefused: true,
		}),
		BuildAuthComparePayload(AuthCompareRun{
			Subject: "ada",
			Mechanisms: []AuthCompareMechanismRow{
				{Mechanism: "session cookie", VerifyElapsedMs: "0.0 ms", Consults: "the session store", AfterRevoke: "the cookie resolves to nothing — dead"},
				{Mechanism: "api key", VerifyElapsedMs: "0.0 ms", Consults: "sha256 + lookup", AfterRevoke: "the key hashes to nothing — dead"},
				{Mechanism: "jwt (HS256)", VerifyElapsedMs: "0.0 ms", Consults: "nothing", AfterRevoke: "STILL VALID (valid) — until exp"},
			},
		}),
		BuildAuthJWTPayload(AuthJWTRun{
			Subject: "ada", TokenPreview: "preview", HeaderJSON: `{"alg":"HS256","typ":"JWT"}`,
			PayloadJSON: `{"sub":"ada"}`, SignatureChars: 43, ExpiresAt: "2026-10-27T12:15:00Z",
			Verdicts: []AuthJWTVerdictRow{
				{Case: "honest verify", Outcome: "valid", Reason: "signature verified"},
				{Case: "alg=none attack", Outcome: "alg-none-rejected", Reason: "refused"},
			},
		}),
		BuildAuthOAuthPayload(AuthOAuthRun{
			VerifierPreview: "abcdefg…", ChallengePreview: "challenge…", State: "xyz",
			RedirectTo: "https://client.example/callback?code=…&state=xyz", CodePreview: "code…",
			AccessTokenPreview: "token…", Scope: "posts:read posts:write", Username: "ada",
			AccessTokenOutcome: "valid", ReplayRefused: "invalid_grant …",
			WrongVerifierRefused: "invalid_grant …", BurnedCodeProof: "invalid_grant …",
		}),
		BuildAuthHashingPayload(AuthHashingRun{
			Parameters:        "argon2id · m=19456 KiB · t=2 · p=1",
			FirstHashPreview:  "$argon2id$v=19$m=19456,t=2,p=1$…",
			SecondHashPreview: "$argon2id$v=19$m=19456,t=2,p=1$…",
			Distinct:          true, CorrectMs: "41.8 ms", WrongMs: "41.9 ms",
			LegacyParameters: "2015-era", LegacyNeedsRehash: true, LegacyMs: "12.0 ms",
			UnknownUserMs: "42.0 ms", MalformedRefused: "refused as a LOUD error",
		}),
		BuildAuthRefreshPayload(AuthRefreshRun{
			FamilyID: "fam_abc", Username: "ada", LoginTokenPreview: "rt_abcdef…",
			Rotations:     []AuthRefreshRotationRow{{From: 1, To: 2}, {From: 2, To: 3}},
			ReusePosition: 2, TokensRevoked: 3, PostReuseRefused: "refused — the family is revoked",
		}),
	}
}

func TestAuthStoryPayloadsMarshalEmptyCollectionsAsArrays(t *testing.T) {
	for i, payload := range authStoryFixturePayloads() {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("payload %d: %v", i, err)
		}
		if strings.Contains(string(encoded), ":null") {
			t.Errorf("payload %d marshals a null collection: %s", i, encoded)
		}
		if len(payload.Beats) == 0 || len(payload.Proofs) == 0 || payload.Reasoning == "" {
			t.Errorf("payload %d lacks beats/proofs/reasoning", i)
		}
		for _, beat := range payload.Beats {
			if beat.Facts == nil {
				t.Errorf("payload %d beat %q has nil facts", i, beat.Beat)
			}
		}
	}
}

func TestAuthStoryRefusalVerdictsMapToRejectedAndAlarm(t *testing.T) {
	if refusalVerdict(true) != AuthVerdictRejected {
		t.Error("a refusal must map to rejected")
	}
	if refusalVerdict(false) != AuthVerdictAlarm {
		t.Error("a failed refusal (engine defect) must map to alarm")
	}
	if refusalWord(true) == refusalWord(false) {
		t.Error("refusal word must distinguish refused from accepted")
	}
}
