package demos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"guide/internal/core"
)

// The shared demo reset, pinned from both sides: the use case's
// orchestration (registration order, per-area isolation under typed
// errors, propagation of non-taxonomy bugs, wiring validation) and the
// one Reset() hook the production wiring rides — the reliability
// breaker's return to its constructed state, driven through the real
// console the endpoint serves.

func TestDemoResetRunsEveryParticipantInRegistrationOrder(t *testing.T) {
	var order []string
	participant := func(area string) DemoResetParticipant {
		return DemoResetParticipant{
			Area:  area,
			Label: "Label " + area,
			Action: func() (string, error) {
				order = append(order, area)
				return area + " back to its constructed state", nil
			},
		}
	}
	resetter, err := NewDemoResetter(participant("alpha"), participant("beta"))
	if err != nil {
		t.Fatalf("resetter: %v", err)
	}

	report, err := resetter.Run()
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(order) != 2 || order[0] != "alpha" || order[1] != "beta" {
		t.Errorf("execution order = %v, want [alpha beta]", order)
	}
	if len(report.Reset) != 2 {
		t.Fatalf("rows = %d, want 2", len(report.Reset))
	}
	for i, area := range []string{"alpha", "beta"} {
		row := report.Reset[i]
		if row.Area != area || row.Outcome != DemoOutcomeReset {
			t.Errorf("row %d = %+v, want area %q reset", i, row, area)
		}
		if row.Detail == "" {
			t.Errorf("row %d detail is empty — the row must report what really happened", i)
		}
	}
	if want := []string{"alpha", "beta"}; fmt.Sprint(resetter.RegisteredAreas()) != fmt.Sprint(want) {
		t.Errorf("registered areas = %v, want %v", resetter.RegisteredAreas(), want)
	}
}

func TestDemoResetDegradesTypedErrorsAndStillResetsTheRest(t *testing.T) {
	resetter, err := NewDemoResetter(
		DemoResetParticipant{Area: "healthy", Label: "Healthy", Action: func() (string, error) {
			return "reset for real", nil
		}},
		DemoResetParticipant{Area: "unreachable", Label: "Unreachable", Action: func() (string, error) {
			return "", core.ServiceUnavailable("the optional store is down")
		}},
	)
	if err != nil {
		t.Fatalf("resetter: %v", err)
	}

	report, err := resetter.Run()
	if err != nil {
		t.Fatalf("a typed failure degrades its row, never the sweep: %v", err)
	}
	if report.Reset[0].Outcome != DemoOutcomeReset {
		t.Errorf("healthy row outcome = %q, want reset", report.Reset[0].Outcome)
	}
	degraded := report.Reset[1]
	if degraded.Outcome != DemoOutcomeDegraded {
		t.Errorf("unreachable row outcome = %q, want degraded", degraded.Outcome)
	}
	if degraded.Detail == "" || !strings.Contains(degraded.Detail, string(core.KindServiceUnavailable)) {
		t.Errorf("degraded detail %q must name the failure kind %q", degraded.Detail, core.KindServiceUnavailable)
	}
}

func TestDemoResetPropagatesNonTaxonomyErrors(t *testing.T) {
	resetter, err := NewDemoResetter(
		DemoResetParticipant{Area: "buggy", Label: "Buggy", Action: func() (string, error) {
			return "", errors.New("a plain bug, not a degraded demo")
		}},
	)
	if err != nil {
		t.Fatalf("resetter: %v", err)
	}
	if _, err := resetter.Run(); err == nil {
		t.Error("a non-taxonomy error must propagate — nothing is swallowed")
	}
}

func TestDemoResetterRejectsBrokenWiring(t *testing.T) {
	action := func() (string, error) { return "", nil }
	cases := []struct {
		name         string
		participants []DemoResetParticipant
	}{
		{"blank area", []DemoResetParticipant{{Area: "", Label: "L", Action: action}}},
		{"blank label", []DemoResetParticipant{{Area: "a", Label: "", Action: action}}},
		{"nil action", []DemoResetParticipant{{Area: "a", Label: "L"}}},
		{
			"duplicate area",
			[]DemoResetParticipant{
				{Area: "a", Label: "L1", Action: action},
				{Area: "a", Label: "L2", Action: action},
			},
		},
	}
	for _, tc := range cases {
		if _, err := NewDemoResetter(tc.participants...); err == nil {
			t.Errorf("%s: NewDemoResetter must reject the wiring", tc.name)
		}
	}
}

func TestDemoResetReportNamesItsDeliberateScope(t *testing.T) {
	if len(demoNotResetAreas) == 0 {
		t.Fatal("the report must say what it deliberately does not touch")
	}
	for _, row := range demoNotResetAreas {
		if row.Area == "" || row.Label == "" || row.Reason == "" {
			t.Errorf("not-reset row %+v must carry area, label, and reason", row)
		}
	}

	resetter, err := NewDemoResetter(DemoResetParticipant{
		Area: "solo", Label: "Solo", Action: func() (string, error) { return "done", nil },
	})
	if err != nil {
		t.Fatalf("resetter: %v", err)
	}
	report, err := resetter.Run()
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// Wire keys are camelCase and the rows never marshal as null.
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := wire["notReset"]; !ok {
		t.Error("the not-reset list must ride the wire under notReset")
	}
	if wire["reset"] == nil {
		t.Error("reset rows must marshal as an array, never null")
	}
}

func TestReliabilityBreakerResetRestoresTheConstructedState(t *testing.T) {
	store := core.NewScenarioStore()
	if _, err := store.Update(core.ScenarioPatch{
		InjectFailure: ptrInjection(core.FailureError),
	}); err != nil {
		t.Fatalf("arm error injection: %v", err)
	}
	console, err := NewReliabilityConsole(ReliabilityConfig{
		FailureThreshold: 2,
		Cooldown:         time.Hour, // keep the trip open for the test's lifetime
		CallTimeout:      10 * time.Millisecond,
		WalkAttempts:     4,
		RetryAttempts:    2,
		RetryBaseDelay:   time.Millisecond,
	}, store)
	if err != nil {
		t.Fatalf("console: %v", err)
	}

	// Trip the breaker for real through the console's own walk.
	console.Walk(context.Background())
	snapshot := console.BreakerSnapshot(time.Now())
	if snapshot.State != ReliabilityBreakerOpen {
		t.Fatalf("state after the walk = %q, want open (the test must trip it first)", snapshot.State)
	}
	if snapshot.Counters.Failed == 0 || len(snapshot.Transitions) == 0 {
		t.Fatal("the tripped breaker must carry failures and history to reset")
	}

	// Reset — the observable must be the constructed state again.
	console.Reset()
	snapshot = console.BreakerSnapshot(time.Now())
	if snapshot.State != ReliabilityBreakerClosed {
		t.Errorf("state after reset = %q, want closed", snapshot.State)
	}
	if snapshot.Counters != (ReliabilityBreakerCounters{}) {
		t.Errorf("counters after reset = %+v, want all zero", snapshot.Counters)
	}
	if len(snapshot.Transitions) != 0 {
		t.Errorf("transitions after reset = %d rows, want the history cleared", len(snapshot.Transitions))
	}
	if snapshot.ProbeInFlight {
		t.Error("probe flag must clear on reset")
	}

	// Idempotent: a second reset changes nothing observable.
	console.Reset()
	again := console.BreakerSnapshot(time.Now())
	if again.State != ReliabilityBreakerClosed || again.Counters != (ReliabilityBreakerCounters{}) {
		t.Errorf("second reset changed observable state: %+v", again)
	}

	// The breaker still WORKS after a reset — the next failure path
	// trips it again from the constructed state.
	store.Update(core.ScenarioPatch{InjectFailure: ptrInjection(core.FailureNone)})
	if _, err := store.Update(core.ScenarioPatch{InjectFailure: ptrInjection(core.FailureError)}); err != nil {
		t.Fatalf("re-arm error injection: %v", err)
	}
	console.Walk(context.Background())
	if state := console.BreakerSnapshot(time.Now()).State; state != ReliabilityBreakerOpen {
		t.Errorf("state after the post-reset walk = %q, want open — a reset must not break the machinery", state)
	}
}

// ptrInjection adapts a failure-injection constant into the patch's
// pointer shape (constants are not addressable).
func ptrInjection(mode core.FailureInjection) *core.FailureInjection { return &mode }
