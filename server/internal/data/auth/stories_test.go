package auth

import (
	"strings"
	"testing"

	"guide/internal/domain/demos"
)

// The seven stories, run against a REAL console (fresh per test — the
// seeded directory hashes two argon2id credentials at construction, the
// same cost production pays once per process). Each test pins the
// story's decisive outcomes: refusals that must happen, kills that must
// count, verdicts that must classify.

func newTestConsole(t *testing.T) *Console {
	t.Helper()
	console, err := NewConsole()
	if err != nil {
		t.Fatalf("console: %v", err)
	}
	return console
}

func TestAuthSessionsStoryWalksTheFullLifecycle(t *testing.T) {
	run, err := newTestConsole(t).RunAuthSessionsStory(t.Context())
	if err != nil {
		t.Fatalf("story: %v", err)
	}
	if run.Username != "ada" || run.InspectedUser != "ada" {
		t.Errorf("story users = %q/%q", run.Username, run.InspectedUser)
	}
	if !run.TamperedRefused || !run.UniformRefusals || !run.Revoked || !run.PostLogoutRefused {
		t.Errorf("story outcomes missing: tampered=%t uniform=%t revoked=%t postLogout=%t",
			run.TamperedRefused, run.UniformRefusals, run.Revoked, run.PostLogoutRefused)
	}
	if !strings.Contains(run.WrongPasswordMs, "ms") || !strings.Contains(run.UnknownUserMs, "ms") {
		t.Errorf("timings not rendered: %q %q", run.WrongPasswordMs, run.UnknownUserMs)
	}
	payload := demos.BuildAuthSessionsPayload(run)
	if payload.Mechanism != "sessions" || len(payload.Beats) != 6 {
		t.Errorf("payload = %q with %d beats", payload.Mechanism, len(payload.Beats))
	}
}

func TestAuthAPIKeysStoryShowsRawKeyOnceAndRefusesHostileUses(t *testing.T) {
	run, err := newTestConsole(t).RunAuthAPIKeysStory(t.Context())
	if err != nil {
		t.Fatalf("story: %v", err)
	}
	if !strings.HasPrefix(run.Prefix, "gdg_") {
		t.Errorf("prefix = %q", run.Prefix)
	}
	if !strings.Contains(run.FullKey, run.Prefix+".") {
		t.Errorf("raw key %q lacks prefix", run.FullKey)
	}
	if !run.WrongSecretRefused || !run.MalformedRefused || !run.Revoked || !run.PostRevokeRefused {
		t.Errorf("refusals missing: wrongSecret=%t malformed=%t revoked=%t postRevoke=%t",
			run.WrongSecretRefused, run.MalformedRefused, run.Revoked, run.PostRevokeRefused)
	}
	if run.RosterCount < 1 || len(run.RosterPreviews) != run.RosterCount {
		t.Errorf("roster = %d previews %d", run.RosterCount, len(run.RosterPreviews))
	}
}

func TestAuthCompareStoryTimesAllThreeAndKillsTheRevocable(t *testing.T) {
	run, err := newTestConsole(t).RunAuthCompareStory(t.Context())
	if err != nil {
		t.Fatalf("story: %v", err)
	}
	if len(run.Mechanisms) != 3 {
		t.Fatalf("mechanisms = %d, want 3", len(run.Mechanisms))
	}
	seen := map[string]bool{}
	for _, m := range run.Mechanisms {
		seen[m.Mechanism] = true
		if !strings.Contains(m.VerifyElapsedMs, "ms") {
			t.Errorf("%s verify time %q", m.Mechanism, m.VerifyElapsedMs)
		}
		if m.Mechanism != "jwt (HS256)" && strings.Contains(m.AfterRevoke, "STILL VALID") {
			t.Errorf("%s should be dead after revoke: %q", m.Mechanism, m.AfterRevoke)
		}
	}
	if !seen["session cookie"] || !seen["api key"] || !seen["jwt (HS256)"] {
		t.Errorf("mechanism set = %v", seen)
	}
	// THE lesson: the JWT alone survives every revocation.
	jwtRow := run.Mechanisms[2]
	if !strings.Contains(jwtRow.AfterRevoke, "STILL VALID") {
		t.Errorf("jwt after revoke = %q, want STILL VALID", jwtRow.AfterRevoke)
	}
}

func TestAuthJWTStoryClassifiesAllSixAttacks(t *testing.T) {
	run, err := newTestConsole(t).RunAuthJWTStory(t.Context())
	if err != nil {
		t.Fatalf("story: %v", err)
	}
	want := map[string]string{
		"honest verify":    "valid",
		"forged signature": "bad-signature",
		"tampered payload (claims edited, signature kept)": "bad-signature",
		"alg=none attack":     "alg-none-rejected",
		"expired token":       "expired",
		"not-yet-valid token": "not-yet-valid",
	}
	if len(run.Verdicts) != len(want) {
		t.Fatalf("verdicts = %d, want %d", len(run.Verdicts), len(want))
	}
	for _, verdict := range run.Verdicts {
		expected, ok := want[verdict.Case]
		if !ok {
			t.Errorf("unexpected case %q", verdict.Case)
			continue
		}
		if verdict.Outcome != expected {
			t.Errorf("%s → %s (%s), want %s", verdict.Case, verdict.Outcome, verdict.Reason, expected)
		}
	}
	if !strings.Contains(run.HeaderJSON, "HS256") || !strings.Contains(run.PayloadJSON, "\"sub\":\"ada\"") {
		t.Errorf("anatomy = %s / %s", run.HeaderJSON, run.PayloadJSON)
	}
}

func TestAuthOAuthStoryEndsInBurnedCodes(t *testing.T) {
	run, err := newTestConsole(t).RunAuthOAuthStory(t.Context())
	if err != nil {
		t.Fatalf("story: %v", err)
	}
	if run.AccessTokenOutcome != "valid" {
		t.Errorf("access token outcome = %q, want valid", run.AccessTokenOutcome)
	}
	if !strings.Contains(run.ReplayRefused, "invalid_grant") {
		t.Errorf("replay = %q, want invalid_grant", run.ReplayRefused)
	}
	if !strings.Contains(run.WrongVerifierRefused, "invalid_grant") {
		t.Errorf("wrong verifier = %q, want invalid_grant", run.WrongVerifierRefused)
	}
	if !strings.Contains(run.BurnedCodeProof, "invalid_grant") {
		t.Errorf("burned code proof = %q, want the second (right-verifier) attempt refused too", run.BurnedCodeProof)
	}
	if !strings.Contains(run.RedirectTo, "code=") || !strings.Contains(run.RedirectTo, "state="+run.State) {
		t.Errorf("redirect = %q", run.RedirectTo)
	}
}

func TestAuthHashingStoryProvesSaltRehashAndUniformity(t *testing.T) {
	run, err := newTestConsole(t).RunAuthHashingStory(t.Context())
	if err != nil {
		t.Fatalf("story: %v", err)
	}
	if !run.Distinct {
		t.Error("the same password must hash to distinct strings")
	}
	if run.CorrectNeedsRehash {
		t.Error("a fresh default-cost hash must not need rehash")
	}
	if !run.LegacyNeedsRehash {
		t.Error("a legacy-cost hash must fire the rehash signal")
	}
	if !strings.Contains(run.MalformedRefused, "decode stored hash") {
		t.Errorf("malformed refusal = %q", run.MalformedRefused)
	}
}

func TestAuthRefreshStoryRotatesThenDiesOnReuse(t *testing.T) {
	run, err := newTestConsole(t).RunAuthRefreshStory(t.Context())
	if err != nil {
		t.Fatalf("story: %v", err)
	}
	if len(run.Rotations) != 2 || run.Rotations[0].From != 1 || run.Rotations[1].From != 2 {
		t.Errorf("rotations = %+v, want 1→2 and 2→3", run.Rotations)
	}
	if run.ReusePosition != 2 || run.TokensRevoked != 3 {
		t.Errorf("reuse = position %d, %d killed; want 2 and 3", run.ReusePosition, run.TokensRevoked)
	}
	if !strings.Contains(run.PostReuseRefused, "refused") {
		t.Errorf("post-reuse = %q, want refused", run.PostReuseRefused)
	}
}

// TestAuthStoriesAreConcurrencySafe pins -race honesty: the console is
// SHARED process state, so two stories in flight must not trip the
// detector (mutexes in the adapters, no shared mutable engine state).
func TestAuthStoriesAreConcurrencySafe(t *testing.T) {
	console := newTestConsole(t)
	done := make(chan error, 3)
	for _, story := range []func() error{
		func() error { _, err := console.RunAuthSessionsStory(t.Context()); return err },
		func() error { _, err := console.RunAuthAPIKeysStory(t.Context()); return err },
		func() error { _, err := console.RunAuthRefreshStory(t.Context()); return err },
	} {
		story := story
		go func() { done <- story() }()
	}
	for range 3 {
		if err := <-done; err != nil {
			t.Fatalf("concurrent story: %v", err)
		}
	}
}
