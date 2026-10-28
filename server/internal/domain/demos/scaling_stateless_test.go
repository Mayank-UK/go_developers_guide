package demos

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

// The statelessness round, tested against its deterministic
// choreography: every assertion below is a measured fact the machinery
// produced, never a number the test lends it. The full expected trace
// (3 replicas, 12 distribution requests, client ada, 6 increments):
//
//	steps 1-4   local session: stored replica-1, missed replica-2 and
//	            replica-3, rescued by sticky routing on replica-1
//	steps 5-6   swap replica-1 (6 served, 1 session measured off the
//	            dying process), the pinned lookup misses the fresh one
//	steps 7-10  shared session: written through replica-1, found by
//	            replica-2, replica-3, and replica-1 in turn
//	steps 11-12 swap replica-2 (6 served, 0 sessions), the post-swap
//	            lookup still hits on the fresh replica
//	step 13     6 increments, 2 per replica, shared counter reads 6

// runScalingStatelessRound runs the default-config round once.
func runScalingStatelessRound(t *testing.T) ScalingStatelessFacts {
	t.Helper()
	round, err := NewScalingStatelessRound(DefaultScalingStatelessConfig())
	if err != nil {
		t.Fatalf("round: %v", err)
	}
	return round.Run()
}

func TestScalingStatelessDistributionIsMeasuredPerReplica(t *testing.T) {
	facts := runScalingStatelessRound(t)
	if len(facts.Distribution) != 3 {
		t.Fatalf("distribution rows = %d, want 3", len(facts.Distribution))
	}
	total := 0
	for i, want := range []int{4, 4, 4} {
		row := facts.Distribution[i]
		if row.ReplicaID != "replica-1" && i == 0 {
			t.Errorf("row %d replica = %s, want fleet order", i, row.ReplicaID)
		}
		if row.Served != want {
			t.Errorf("%s served %d, want %d (twelve across three)", row.ReplicaID, row.Served, want)
		}
		total += row.Served
	}
	if total != 12 {
		t.Errorf("distribution total = %d, want 12", total)
	}
}

func TestScalingStatelessStepOrderingAndPhases(t *testing.T) {
	facts := runScalingStatelessRound(t)
	wantPhases := []string{
		scalingPhaseLocal, scalingPhaseLocal, scalingPhaseLocal, scalingPhaseSticky,
		scalingPhaseSwap, scalingPhaseSticky, scalingPhaseShared, scalingPhaseShared,
		scalingPhaseShared, scalingPhaseShared, scalingPhaseSwap, scalingPhaseShared,
		scalingPhaseCounter,
	}
	if len(facts.Steps) != len(wantPhases) {
		t.Fatalf("steps = %d, want %d", len(facts.Steps), len(wantPhases))
	}
	for i, step := range facts.Steps {
		if step.Step != i+1 {
			t.Errorf("step %d carries number %d", i, step.Step)
		}
		if step.Phase != wantPhases[i] {
			t.Errorf("step %d phase = %q, want %q", i+1, step.Phase, wantPhases[i])
		}
		if step.Outcome == "" || step.Route == "" || step.ReplicaID == "" || step.Observed == "" {
			t.Errorf("step %d is missing a measured field: %+v", i+1, step)
		}
	}
}

func TestScalingStatelessLocalSessionBreaksOnHopAndSwap(t *testing.T) {
	facts := runScalingStatelessRound(t)
	steps := facts.Steps

	// The login lands on replica-1, the only replica that ever knows
	// the local token.
	if steps[0].ReplicaID != "replica-1" || steps[0].Outcome != scalingOutcomeStored {
		t.Errorf("login step = %s/%s, want replica-1/stored", steps[0].ReplicaID, steps[0].Outcome)
	}
	// The next two hops miss, on the two other replicas.
	for i, wantReplica := range []string{"replica-2", "replica-3"} {
		step := steps[1+i]
		if step.ReplicaID != wantReplica || step.Outcome != scalingOutcomeMiss {
			t.Errorf("hop %d = %s/%s, want %s/miss", i+1, step.ReplicaID, step.Outcome, wantReplica)
		}
	}
	// Sticky routing rescues the lookup on the storing replica…
	if steps[3].ReplicaID != "replica-1" || steps[3].Outcome != scalingOutcomeHit {
		t.Errorf("sticky step = %s/%s, want replica-1/hit", steps[3].ReplicaID, steps[3].Outcome)
	}
	// …until the rolling update replaces it: the swap fact carries the
	// dying replica's measured counters (4 distribution + login +
	// sticky lookup = 6 served, exactly 1 session held).
	if len(facts.Swaps) != 2 {
		t.Fatalf("swaps = %d, want 2", len(facts.Swaps))
	}
	first := facts.Swaps[0]
	if first.Step != 5 || first.ReplicaID != "replica-1" ||
		first.ServedBefore != 6 || first.SessionsBefore != 1 {
		t.Errorf("first swap fact = %+v, want {5 replica-1 6 1}", first)
	}
	// The affinity entry survived; the memory it pointed at did not.
	if steps[5].ReplicaID != "replica-1" || steps[5].Outcome != scalingOutcomeMiss {
		t.Errorf("post-swap sticky step = %s/%s, want replica-1/miss", steps[5].ReplicaID, steps[5].Outcome)
	}
	if !strings.Contains(steps[5].Observed, "0 session(s)") {
		t.Errorf("post-swap observation must quote the fresh replica's empty map: %q", steps[5].Observed)
	}
}

func TestScalingStatelessSharedSessionSurvivesEverything(t *testing.T) {
	facts := runScalingStatelessRound(t)
	steps := facts.Steps

	// The write goes through whichever replica the ring picks — the
	// fresh replica-1 that just lost its local session.
	if steps[6].Outcome != scalingOutcomeStored || steps[6].Phase != scalingPhaseShared {
		t.Errorf("shared store step = %+v", steps[6])
	}
	// Every replica of the fleet answers a round-robin lookup, and all
	// hit — including the fresh replacement.
	wantReplicas := []string{"replica-2", "replica-3", "replica-1"}
	for i, want := range wantReplicas {
		step := steps[7+i]
		if step.ReplicaID != want || step.Outcome != scalingOutcomeHit {
			t.Errorf("shared lookup %d = %s/%s, want %s/hit", i+1, step.ReplicaID, step.Outcome, want)
		}
	}
	// The second swap touches a replica that holds nothing: measured 6
	// served, 0 sessions.
	second := facts.Swaps[1]
	if second.Step != 11 || second.ReplicaID != "replica-2" ||
		second.ServedBefore != 6 || second.SessionsBefore != 0 {
		t.Errorf("second swap fact = %+v, want {11 replica-2 6 0}", second)
	}
	// The lookup after the swap still hits, on the fresh process.
	if steps[11].Outcome != scalingOutcomeHit || steps[11].ReplicaID != "replica-2" {
		t.Errorf("post-swap lookup = %s/%s, want replica-2/hit", steps[11].ReplicaID, steps[11].Outcome)
	}
}

func TestScalingStatelessCounterAccumulatesAcrossReplicas(t *testing.T) {
	facts := runScalingStatelessRound(t)
	if len(facts.Counter.Rows) != 3 {
		t.Fatalf("counter rows = %d, want 3", len(facts.Counter.Rows))
	}
	sum := 0
	for i, want := range []int{2, 2, 2} {
		if facts.Counter.Rows[i].Increments != want {
			t.Errorf("%s increments = %d, want %d", facts.Counter.Rows[i].ReplicaID,
				facts.Counter.Rows[i].Increments, want)
		}
		sum += facts.Counter.Rows[i].Increments
	}
	if sum != facts.Counter.Shared || facts.Counter.Shared != 6 {
		t.Errorf("counter rows sum %d vs shared read %d, want 6/6", sum, facts.Counter.Shared)
	}
}

func TestScalingStatelessFinalTotalsShowTheSwapResets(t *testing.T) {
	facts := runScalingStatelessRound(t)
	// Fleet order rows; replica-1 and replica-2 are the FRESH
	// processes (their predecessors' totals died with them), replica-3
	// is the original. replica-1's fresh count: the post-swap sticky
	// miss + the shared write + the third fleet lookup + 2 counter
	// increments.
	want := map[string]int{"replica-1": 5, "replica-2": 3, "replica-3": 8}
	for _, row := range facts.FinalTotals {
		if row.Served != want[row.ReplicaID] {
			t.Errorf("%s final total = %d, want %d", row.ReplicaID, row.Served, want[row.ReplicaID])
		}
	}
}

func TestScalingStatelessPayloadIsDeterministicByteForByte(t *testing.T) {
	first, err := json.Marshal(BuildScalingStatelessPayload(runScalingStatelessRound(t)))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	second, err := json.Marshal(BuildScalingStatelessPayload(runScalingStatelessRound(t)))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(first) != string(second) {
		t.Error("two rounds must produce byte-identical payloads — the round reads no clock and no random source")
	}
}

// TestScalingStatelessPayloadCarriesOnlyObservedCounts is the demo's
// honesty pin: every figure the payload asserts is derived from the
// facts the machinery observed. The distribution note quotes the
// measured spread; the verdicts quote the measured replicas, miss
// counts, swap counters, lookup totals, and shared counter — nothing
// is a constant dressed up as a result.
func TestScalingStatelessPayloadCarriesOnlyObservedCounts(t *testing.T) {
	facts := runScalingStatelessRound(t)
	payload := BuildScalingStatelessPayload(facts)

	if payload.Distribution.TotalServed != 12 {
		t.Errorf("payload total = %d, want the measured 12", payload.Distribution.TotalServed)
	}
	if !strings.Contains(payload.Distribution.Note, "4/4/4") || !strings.Contains(payload.Distribution.Note, "exactly even") {
		t.Errorf("distribution note must derive from the measured rows: %q", payload.Distribution.Note)
	}

	verdicts := map[string]string{}
	for _, verdict := range payload.Verdicts {
		verdicts[verdict.ID] = verdict.Value
	}
	for id, want := range map[string][]string{
		"distribution":   {"12", "4/4/4"},
		"local-session":  {"replica-1", "2 hop(s)"},
		"sticky":         {"replica-1", "6 request(s)", "1 session(s)"},
		"shared-session": {"4 of 4"},
	} {
		for _, fragment := range want {
			if !strings.Contains(verdicts[id], fragment) {
				t.Errorf("verdict %q must quote %q: %q", id, fragment, verdicts[id])
			}
		}
	}
	if payload.Counter.Total != facts.Counter.Shared {
		t.Errorf("counter total = %d, want the measured read-back %d", payload.Counter.Total, facts.Counter.Shared)
	}
	if !strings.Contains(payload.Counter.Note, "read back as 6") {
		t.Errorf("counter note must quote the measured read-back: %q", payload.Counter.Note)
	}
}

func TestScalingStatelessBuilderSurvivesEmptyFacts(t *testing.T) {
	// Defensive belt: a facts struct with no rows must not panic, and
	// must say so honestly instead of inventing a spread.
	payload := BuildScalingStatelessPayload(ScalingStatelessFacts{
		Config: DefaultScalingStatelessConfig(),
	})
	if payload.Distribution.TotalServed != 0 || !strings.Contains(payload.Distribution.Note, "no distribution rows") {
		t.Errorf("empty-facts distribution = %+v, want an honest zero", payload.Distribution)
	}
	if !strings.Contains(payload.Verdicts[0].Value, "0") {
		t.Errorf("verdicts must not panic or invent on empty facts: %+v", payload.Verdicts[0])
	}
}

func TestScalingStatelessRoundRejectsInvalidConfigs(t *testing.T) {
	base := DefaultScalingStatelessConfig()
	bad := []ScalingStatelessConfig{
		{Replicas: 1, DistributionRequests: 4, ClientID: "ada", CounterIncrements: 2},
		{Replicas: 9, DistributionRequests: 4, ClientID: "ada", CounterIncrements: 2},
		{Replicas: base.Replicas, DistributionRequests: 0, ClientID: "ada", CounterIncrements: 2},
		{Replicas: base.Replicas, DistributionRequests: 4, ClientID: "ada", CounterIncrements: 0},
		{Replicas: base.Replicas, DistributionRequests: 4, ClientID: "", CounterIncrements: 2},
	}
	for i, cfg := range bad {
		if _, err := NewScalingStatelessRound(cfg); err == nil {
			t.Errorf("config %d (%+v) must be refused", i, cfg)
		}
	}
}

// TestScalingStatelessFleetIsRaceCleanUnderParallelTraffic proves the
// machinery types are genuinely shared-state safe: one fleet, one
// balancer, one shared store, hammered by parallel request goroutines
// mixing round-robin routing, sticky pins, both session placements,
// and a mid-storm replica swap — under the race detector this test is
// the proof, and the shared counter must still reconcile exactly.
func TestScalingStatelessFleetIsRaceCleanUnderParallelTraffic(t *testing.T) {
	fleet := newScalingBalancer(3)
	shared := newScalingSharedStore()

	const workers, perWorker = 8, 50
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				if id == 0 && i == 25 {
					// A rolling update mid-storm: slot 1's replica is
					// replaced while the other workers route to it.
					fleet.ReplaceReplica(1)
				}
				replica, slot := fleet.Next()
				switch {
				case i%5 == 0:
					replica.ServeSessionStore("t", "u")
					fleet.Remember("t", slot)
				case i%5 == 1:
					replica.ServeSessionLookup("t")
				case i%5 == 2:
					pinned := fleet.Pin("t")
					pinned.ServeSessionLookup("t")
				case i%5 == 3:
					replica.Serve()
					shared.StoreSession("s", "u")
					shared.Increment()
				default:
					replica.Serve()
					shared.LookupSession("s")
				}
			}
		}(w)
	}
	wg.Wait()

	// The one deterministic reconciliation: every i%5==3 operation
	// incremented the shared store exactly once, swaps or not.
	wantIncrements := workers * perWorker / 5
	if got := shared.Counter(); got != wantIncrements {
		t.Errorf("shared counter = %d, want %d — externalized state accumulates through the storm", got, wantIncrements)
	}
	totalServed := 0
	for _, replica := range fleet.Fleet() {
		totalServed += replica.Served()
	}
	if totalServed <= 0 {
		t.Errorf("fleet served %d, want the storm's requests counted", totalServed)
	}
}
