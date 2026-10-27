// Topic 25 security's two demos share one payload shape — the same
// STORY discipline the auth topic established: beats in execution
// order (each with a teaching sentence, a verdict color, and
// label/value facts), derived proofs, and a closing reasoning
// paragraph. This file holds the shared wire shape plus the
// security:lab builder (the header set and a live rate-limit window);
// security_pii.go holds the security:pii builder. Every sentence the
// builders write is DERIVED from the run's own rows — a drifted
// engine answer surfaces as a sentence saying so, never as a silent
// contradiction.
package demos

import (
	"context"
	"fmt"
	"strings"
)

// Verdict labels the security demos' client colors: green ok, red
// rejected, neutral info.
const (
	SecurityVerdictOK       = "ok"
	SecurityVerdictRejected = "rejected"
	SecurityVerdictInfo     = "info"
)

// SecurityFactRow is one label/value pair — header values, retry
// hints, request counts. Values are pre-rendered strings so the client
// never formats.
type SecurityFactRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// SecurityStoryBeat is one beat as the payload serves it: what was
// done, the teaching sentence, the verdict, and the facts worth
// showing.
type SecurityStoryBeat struct {
	Beat    string            `json:"beat"`
	Detail  string            `json:"detail"`
	Verdict string            `json:"verdict"`
	Facts   []SecurityFactRow `json:"facts"`
}

// SecurityStoryPayload is the wire shape both security demos serve
// (camelCase JSON). Every slice is non-nil so the wire renders [],
// never null.
type SecurityStoryPayload struct {
	Mechanism string              `json:"mechanism"`
	Beats     []SecurityStoryBeat `json:"beats"`
	Proofs    []SecurityFactRow   `json:"proofs"`
	Reasoning string              `json:"reasoning"`
}

// newSecurityStoryPayload assembles a story payload with the wire rules
// applied: non-nil facts and proofs so the JSON renders [], never null.
func newSecurityStoryPayload(mechanism string, beats []SecurityStoryBeat, proofs []SecurityFactRow, reasoning string) SecurityStoryPayload {
	for i := range beats {
		if beats[i].Facts == nil {
			beats[i].Facts = []SecurityFactRow{}
		}
	}
	if proofs == nil {
		proofs = []SecurityFactRow{}
	}
	return SecurityStoryPayload{Mechanism: mechanism, Beats: beats, Proofs: proofs, Reasoning: reasoning}
}

// ── security:lab ─────────────────────────────────────────────────────────

// SecurityLabRunner is the port the lab story runs through; the
// data-layer console implements it against the real engines.
type SecurityLabRunner interface {
	RunSecurityLabStory(ctx context.Context) (SecurityLabRun, error)
}

// SecurityHeaderRow is one security header as the run observed it:
// the value applied and the attack it closes.
type SecurityHeaderRow struct {
	Header string
	Value  string
	Why    string
}

// SecurityRateRequestRow is one request in the live rate window: its
// number, its verdict, and — for refusals — the retry hint the
// limiter answered with.
type SecurityRateRequestRow struct {
	N          int
	Allowed    bool
	RetryAfter string
}

// SecurityLabRun is the lab story's raw haul: the header policy, the
// live rate window (burst spent, wall hit, one-token recovery), the
// per-client isolation proof, and the route's own wrap policy.
type SecurityLabRun struct {
	Headers            []SecurityHeaderRow
	Policy             string
	Requests           []SecurityRateRequestRow
	AllowedCount       int
	DeniedCount        int
	FirstRetryAfter    string
	RecoveryWaitMs     string
	SteadyWaitMs       string
	WaitServed         bool
	OtherClientAllowed bool
	Buckets            int
	RoutePolicy        string
}

// BuildSecurityLabPayload shapes the lab story.
func BuildSecurityLabPayload(run SecurityLabRun) SecurityStoryPayload {
	headerFacts := make([]SecurityFactRow, 0, len(run.Headers))
	for _, header := range run.Headers {
		headerFacts = append(headerFacts, SecurityFactRow{Label: header.Header, Value: header.Value})
	}
	window := make([]string, 0, len(run.Requests))
	for _, request := range run.Requests {
		mark := "·"
		if request.Allowed {
			mark = "200"
		} else {
			mark = "429"
		}
		window = append(window, fmt.Sprintf("#%d=%s", request.N, mark))
	}
	beats := []SecurityStoryBeat{
		{
			Beat: "the header set", Verdict: SecurityVerdictOK,
			Detail: fmt.Sprintf("every response this route serves carries the %d-header policy below — set BEFORE the handler runs, so success bodies AND error envelopes carry them. THIS response carries them: curl -i it and read them off the wire.",
				len(run.Headers)),
			Facts: headerFacts,
		},
		{
			Beat: "the burst spends the bucket", Verdict: SecurityVerdictOK,
			Detail: fmt.Sprintf("a fresh limiter at %s starts FULL: the first %d requests all land instantly — the burst ceiling is a budget you may spend at once, not a cap on total requests.",
				run.Policy, run.AllowedCount),
			Facts: []SecurityFactRow{{Label: "the window so far", Value: strings.Join(window[:min(run.AllowedCount, len(window))], " ")}},
		},
		{
			Beat: "the 429 boundary", Verdict: SecurityVerdictRejected,
			Detail: fmt.Sprintf("request #%d finds the bucket empty → 429 rate_limited, with the token bucket's own math as the retry hint: retry after %s (the next token refills in exactly one rate interval). A refused request CONSUMES NOTHING — the reservation was cancelled, so the hint stays honest for the next requester.",
				run.AllowedCount+1, run.FirstRetryAfter),
			Facts: []SecurityFactRow{{Label: "the window", Value: strings.Join(window, " ")}},
		},
		{
			Beat: "recovery", Verdict: SecurityVerdictOK,
			Detail: fmt.Sprintf("the same client now WAITS instead of hammering: the cooperative Wait served its request after one token interval (%s measured — tokens refill CONTINUOUSLY, not at window boundaries; served: %t). A second waiting request cost one MORE interval (%s): over budget, a patient client is served at exactly the sustained rate. Punishment is not permanent; the allowance returns at the stated rate.",
				run.RecoveryWaitMs, run.WaitServed, run.SteadyWaitMs),
		},
		{
			Beat: "per-client isolation", Verdict: SecurityVerdictOK,
			Detail: fmt.Sprintf("a SECOND client arriving while the first is throttled gets its OWN full bucket (allowed: %t; %d buckets live) — one abusive client cannot spend anyone else's allowance. Keying is by identity when the request carries one, else by remote address.",
				run.OtherClientAllowed, run.Buckets),
		},
		{
			Beat: "this very route", Verdict: SecurityVerdictInfo,
			Detail: fmt.Sprintf("the endpoint serving this story is itself wrapped by a second limiter (%s) — hammer it with curl and the 429 envelope you have been reading about arrives on YOUR request, typed, with x-request-id stamped.",
				run.RoutePolicy),
		},
	}
	return newSecurityStoryPayload("security-lab",
		beats,
		[]SecurityFactRow{
			{Label: "burst vs sustained", Value: fmt.Sprintf("%s — burst is the instant budget, rate is the refill speed; both numbers are part of the contract you publish", run.Policy)},
			{Label: "the boundary", Value: fmt.Sprintf("%d allowed then %d refused with retry hints — the transition is smooth, not a window edge", run.AllowedCount, run.DeniedCount)},
			{Label: "recovery", Value: fmt.Sprintf("the over-budget client that WAITS was served after one token interval (%s), then at one interval per request (%s steady): %t", run.RecoveryWaitMs, run.SteadyWaitMs, run.WaitServed)},
			{Label: "isolation", Value: fmt.Sprintf("second client served from its own bucket while the first was throttled: %t", run.OtherClientAllowed)},
		},
		"Every beat is what the live engines answered. The headers are the response's own armor — set before any handler code runs, so even error paths carry them; the rate window is a real golang.org/x/time/rate token bucket spent in front of you: burst first, refusal with an honest retry hint second, continuous refill third. That refill is the property fixed-window counters fake — a window resets at its EDGE, letting twice the budget through around the boundary, while a token bucket charges every request against the same continuous allowance. Per-client keys keep the punishment where the abuse is.")
}
