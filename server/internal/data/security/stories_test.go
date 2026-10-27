package security

import (
	"context"
	"strings"
	"testing"

	"guide/internal/domain/demos"
)

// The two stories against the REAL console: the lab story's window is
// deterministic (fresh limiter per run), so its counts and verdicts are
// pinned exactly; the PII story's audit must come back clean — and the
// payload builders' derivation is exercised through these same runs.

func TestSecurityLabStoryPlaysTheWholeWindow(t *testing.T) {
	console, err := NewConsole()
	if err != nil {
		t.Fatalf("console: %v", err)
	}
	run, err := console.RunSecurityLabStory(context.Background())
	if err != nil {
		t.Fatalf("lab story: %v", err)
	}

	if len(run.Headers) != 5 {
		t.Errorf("header rows = %d, want the five-rule policy", len(run.Headers))
	}
	if run.AllowedCount != 5 || run.DeniedCount != 3 {
		t.Errorf("window = %d allowed / %d refused, want 5/3 (burst 5, then 3 over budget)", run.AllowedCount, run.DeniedCount)
	}
	for i, request := range run.Requests {
		if request.N != i+1 {
			t.Errorf("request numbering drifted at %d", i)
		}
		if i < 5 && !request.Allowed {
			t.Errorf("request %d must be inside the burst", request.N)
		}
		if i >= 5 && request.Allowed {
			t.Errorf("request %d must be over budget", request.N)
		}
	}
	if run.FirstRetryAfter == "" || run.FirstRetryAfter == "now" {
		t.Errorf("first refusal must carry a retry hint, got %q", run.FirstRetryAfter)
	}
	if !run.WaitServed || run.RecoveryWaitMs == "" || run.SteadyWaitMs == "" {
		t.Errorf("the recovery waits must be measured and served: %+v", run)
	}
	if !run.OtherClientAllowed {
		t.Error("the second client must have its own bucket")
	}
	if run.Buckets != 2 {
		t.Errorf("buckets = %d, want one per distinct story client (2)", run.Buckets)
	}
	if !strings.Contains(run.Policy, "burst 5") || !strings.Contains(run.RoutePolicy, "burst 5") {
		t.Errorf("policies must state burst: story=%q route=%q", run.Policy, run.RoutePolicy)
	}

	payload := demos.BuildSecurityLabPayload(run)
	if payload.Mechanism != "security-lab" || len(payload.Beats) != 6 {
		t.Errorf("payload = %s with %d beats, want security-lab with 6 beats", payload.Mechanism, len(payload.Beats))
	}
	for _, beat := range payload.Beats {
		if beat.Facts == nil {
			t.Errorf("beat %q marshaled nil facts", beat.Beat)
		}
	}
	if !strings.Contains(payload.Beats[2].Detail, "429 rate_limited") {
		t.Errorf("boundary beat must name the typed refusal: %s", payload.Beats[2].Detail)
	}
}

func TestSecurityLabStoryIsDeterministicAcrossRuns(t *testing.T) {
	console, err := NewConsole()
	if err != nil {
		t.Fatalf("console: %v", err)
	}
	first, err := console.RunSecurityLabStory(context.Background())
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := console.RunSecurityLabStory(context.Background())
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if first.AllowedCount != second.AllowedCount || first.DeniedCount != second.DeniedCount {
		t.Errorf("runs drifted: %d/%d then %d/%d — the fresh limiter must reset the window",
			first.AllowedCount, first.DeniedCount, second.AllowedCount, second.DeniedCount)
	}
}

func TestSecurityPIIStoryWalksToACleanAudit(t *testing.T) {
	console, err := NewConsole()
	if err != nil {
		t.Fatalf("console: %v", err)
	}
	run, err := console.RunSecurityPIIStory(context.Background())
	if err != nil {
		t.Fatalf("pii story: %v", err)
	}

	if run.SubjectID != "sub_ada" {
		t.Errorf("subject = %s, want the seeded fixture", run.SubjectID)
	}
	if len(run.Classification) != 5 {
		t.Errorf("classification rows = %d, want 5 fields", len(run.Classification))
	}
	if strings.Join(run.ErasedFields, ",") != "email,phone,signup_ip" {
		t.Errorf("erased fields = %v, want exactly the PII ones", run.ErasedFields)
	}
	if len(run.LogBefore) != 1 || len(run.LogAfter) != 3 {
		t.Errorf("log rows = %d before / %d after, want 1 read / 1 read + 1 erase + 1 read", len(run.LogBefore), len(run.LogAfter))
	}
	for _, row := range run.LogAfter {
		if strings.Contains(row.Line, "@example.com") || strings.Contains(row.Line, "555") {
			t.Errorf("log line carries a field value: %s", row.Line)
		}
	}
	for _, field := range run.MaskedView {
		if field.Name == "email" && !strings.HasPrefix(field.Value, "a") {
			t.Errorf("masked email = %q, want the edges preserved", field.Value)
		}
	}
	for _, field := range run.ViewAfterErase {
		if field.Name == "email" && field.Value != "[erased per subject request]" {
			t.Errorf("post-erasure email = %q, want the tombstone", field.Value)
		}
	}
	if !run.PIIAbsentEverywhere {
		t.Error("the audit must come back clean — original PII found downstream")
	}

	payload := demos.BuildSecurityPIIPayload(run)
	if payload.Mechanism != "security-pii" || len(payload.Beats) != 6 {
		t.Errorf("payload = %s with %d beats, want security-pii with 6 beats", payload.Mechanism, len(payload.Beats))
	}
	if payload.Beats[5].Verdict != demos.SecurityVerdictOK {
		t.Errorf("audit beat verdict = %s, want ok", payload.Beats[5].Verdict)
	}
	if !strings.Contains(payload.Beats[0].Detail, "signup_ip") {
		t.Errorf("classification beat must name the IP-is-PII lesson: %s", payload.Beats[0].Detail)
	}
}
