package demos

import (
	"strings"
	"testing"
)

// The security:lab payload builder: every sentence derives from the
// run's own rows — counts, retry hints, and policy strings appear only
// as the run reported them, and the beats are pinned in order.

func TestSecurityLabPayloadDerivesFromTheRun(t *testing.T) {
	run := SecurityLabRun{
		Headers: []SecurityHeaderRow{
			{Header: "X-Frame-Options", Value: "DENY", Why: "no framing"},
			{Header: "X-Content-Type-Options", Value: "nosniff", Why: "no sniffing"},
		},
		Policy: "10/s sustained, burst 5",
		Requests: []SecurityRateRequestRow{
			{N: 1, Allowed: true}, {N: 2, Allowed: true}, {N: 3, Allowed: true},
			{N: 4, Allowed: true}, {N: 5, Allowed: true},
			{N: 6, Allowed: false, RetryAfter: "90ms"},
			{N: 7, Allowed: false, RetryAfter: "100ms"},
			{N: 8, Allowed: false, RetryAfter: "90ms"},
		},
		AllowedCount:       5,
		DeniedCount:        3,
		FirstRetryAfter:    "90ms",
		RecoveryWaitMs:     "97.3 ms",
		SteadyWaitMs:       "99.1 ms",
		WaitServed:         true,
		OtherClientAllowed: true,
		Buckets:            2,
		RoutePolicy:        "2/s sustained, burst 5",
	}
	payload := BuildSecurityLabPayload(run)

	if payload.Mechanism != "security-lab" {
		t.Errorf("mechanism = %q", payload.Mechanism)
	}
	if len(payload.Beats) != 6 {
		t.Fatalf("beats = %d, want 6 (headers, burst, boundary, recovery, isolation, route)", len(payload.Beats))
	}
	wants := []struct{ beat, fragment string }{
		{"the header set", "2-header policy"},
		{"the burst spends the bucket", "first 5 requests"},
		{"the 429 boundary", "request #6"},
		{"recovery", "97.3 ms"},
		{"per-client isolation", "its OWN full bucket"},
		{"this very route", "2/s sustained, burst 5"},
	}
	for i, want := range wants {
		if payload.Beats[i].Beat != want.beat {
			t.Errorf("beat %d = %q, want %q", i, payload.Beats[i].Beat, want.beat)
		}
		if !strings.Contains(payload.Beats[i].Detail, want.fragment) {
			t.Errorf("beat %q must derive %q from the run: %s", want.beat, want.fragment, payload.Beats[i].Detail)
		}
	}
	if !strings.Contains(payload.Beats[2].Detail, "retry after 90ms") {
		t.Errorf("boundary beat must carry the run's retry hint: %s", payload.Beats[2].Detail)
	}
	if len(payload.Beats[0].Facts) != 2 {
		t.Errorf("header facts = %d, want one per header rule", len(payload.Beats[0].Facts))
	}
	if payload.Beats[1].Facts[0].Value != "#1=200 #2=200 #3=200 #4=200 #5=200" {
		t.Errorf("burst window rendering = %q", payload.Beats[1].Facts[0].Value)
	}
	for _, proof := range payload.Proofs {
		if proof.Label == "" || proof.Value == "" {
			t.Errorf("empty proof row: %+v", proof)
		}
	}
	if payload.Reasoning == "" {
		t.Error("payload needs its reasoning paragraph")
	}
}

func TestSecurityLabPayloadNeverMarshalsNilSlices(t *testing.T) {
	payload := BuildSecurityLabPayload(SecurityLabRun{
		Headers:            []SecurityHeaderRow{{Header: "h", Value: "v", Why: "w"}},
		Policy:             "p",
		Requests:           []SecurityRateRequestRow{{N: 1, Allowed: true}},
		WaitServed:         true,
		OtherClientAllowed: true,
	})
	for i, beat := range payload.Beats {
		if beat.Facts == nil {
			t.Errorf("beat %d (%q) marshaled nil facts", i, beat.Beat)
		}
	}
	if payload.Proofs == nil {
		t.Error("proofs marshaled nil")
	}
}
