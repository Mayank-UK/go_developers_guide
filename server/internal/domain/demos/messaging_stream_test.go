package demos

import (
	"strings"
	"testing"
)

// The messaging stream story's payload builder: PURE derivation tests,
// no store. The proofs must derive from the run's own rows — the happy
// story proves ordering/queue/recovery from the ids and reads it
// carries, and a drifted run (a read out of order, a second consumer
// that received work, a claim that missed the stalled entry) must say
// so in the open instead of papering over it.

func messagingStreamFixtureRun() MessagingStreamRun {
	return MessagingStreamRun{
		Namespace: "gdg:demo:msg:0123456789ab",
		Stream:    "gdg:demo:msg:0123456789ab:orders",
		Group:     MessagingDemoGroup,
		AppendedIDs: []string{
			"1780000000000-0",
			"1780000000001-0",
			"1780000000002-0",
		},
		ConsumerARead: []MessagingEntryRow{
			{ID: "1780000000000-0", Step: "created"},
			{ID: "1780000000001-0", Step: "paid"},
			{ID: "1780000000002-0", Step: "shipped"},
		},
		ConsumerBRead: nil,
		AckedByA:      []string{"1780000000000-0", "1780000000002-0"},
		StalledID:     "1780000000001-0",
		PendingBefore: []MessagingPendingRow{
			{ID: "1780000000001-0", Consumer: "consumer-a", Deliveries: 1},
		},
		ClaimedByB: []MessagingEntryRow{
			{ID: "1780000000001-0", Step: "paid"},
		},
		PendingAfterCount: 0,
		GroupDestroyed:    true,
		StreamDeleted:     true,
	}
}

func TestMessagingStreamPayloadDerivesTheThreeProofs(t *testing.T) {
	t.Parallel()
	payload := BuildMessagingStreamPayload(messagingStreamFixtureRun())

	if payload.Namespace != "gdg:demo:msg:0123456789ab" || payload.Stream != "gdg:demo:msg:0123456789ab:orders" {
		t.Errorf("namespace/stream drifted: %q/%q", payload.Namespace, payload.Stream)
	}
	if !strings.Contains(payload.Proofs.Ordering, "same ids in the same order") {
		t.Errorf("ordering proof must derive from the matched ids: %q", payload.Proofs.Ordering)
	}
	if !strings.Contains(payload.Proofs.Ordering, "created → paid → shipped") {
		t.Errorf("ordering proof must carry the steps in sequence: %q", payload.Proofs.Ordering)
	}
	if !strings.Contains(payload.Proofs.QueueProof, "answered 0 entries") {
		t.Errorf("queue proof must derive from consumer-b's empty read: %q", payload.Proofs.QueueProof)
	}
	if !strings.Contains(payload.Proofs.Recovery, "exactly the stalled entry") {
		t.Errorf("recovery proof must match the claim against the stalled id: %q", payload.Proofs.Recovery)
	}
	if payload.Proofs.PendingLeft != 0 {
		t.Errorf("pendingLeft = %d, want 0", payload.Proofs.PendingLeft)
	}
}

func TestMessagingStreamPayloadReportsADriftedRunHonestly(t *testing.T) {
	t.Parallel()
	run := messagingStreamFixtureRun()
	// Drift 1: consumer-a read the entries out of append order.
	run.ConsumerARead = []MessagingEntryRow{
		{ID: "1780000000001-0", Step: "paid"},
		{ID: "1780000000000-0", Step: "created"},
		{ID: "1780000000002-0", Step: "shipped"},
	}
	// Drift 2: consumer-b somehow received new work.
	run.ConsumerBRead = []MessagingEntryRow{{ID: "1780000000003-0", Step: "refunded"}}
	// Drift 3: the claim missed the stalled entry.
	run.ClaimedByB = []MessagingEntryRow{{ID: "1780000000002-0", Step: "shipped"}}

	payload := BuildMessagingStreamPayload(run)
	if !strings.Contains(payload.Proofs.Ordering, "a different order") {
		t.Errorf("a drifted read must not claim the ordering guarantee: %q", payload.Proofs.Ordering)
	}
	if !strings.Contains(payload.Proofs.QueueProof, "read 1 entries") {
		t.Errorf("a non-empty second read must be reported as it happened: %q", payload.Proofs.QueueProof)
	}
	if !strings.Contains(payload.Proofs.Recovery, "a different set than the stalled entry") {
		t.Errorf("a missed claim must be reported as it happened: %q", payload.Proofs.Recovery)
	}
}

func TestMessagingStreamPayloadBeatsCarryTheCommands(t *testing.T) {
	t.Parallel()
	payload := BuildMessagingStreamPayload(messagingStreamFixtureRun())

	wantBeats := []string{"append", "group", "consume (a)", "consume (b)", "ack + crash", "pending", "autoclaim", "drained"}
	got := make([]string, 0, len(payload.Beats))
	for _, beat := range payload.Beats {
		got = append(got, beat.Beat)
		for _, command := range beat.Commands {
			if command == "" {
				t.Errorf("beat %q carries an empty command", beat.Beat)
			}
		}
	}
	if strings.Join(got, ",") != strings.Join(wantBeats, ",") {
		t.Errorf("beats = %v, want %v", got, wantBeats)
	}
	joined := strings.Join(payload.Beats[0].Commands, "\n")
	if !strings.Contains(joined, "step created") || !strings.Contains(joined, "step shipped") {
		t.Errorf("append commands must derive from the appended steps: %v", payload.Beats[0].Commands)
	}
}

func TestMessagingStreamNamespaceIsNamespacedAndRandom(t *testing.T) {
	t.Parallel()
	first, err := NewMessagingDemoNamespace()
	if err != nil {
		t.Fatalf("mint namespace: %v", err)
	}
	second, err := NewMessagingDemoNamespace()
	if err != nil {
		t.Fatalf("mint namespace: %v", err)
	}
	if !strings.HasPrefix(first, MessagingDemoKeyPrefix+":") {
		t.Errorf("namespace %q must live under %q", first, MessagingDemoKeyPrefix)
	}
	if first == second {
		t.Errorf("two mints must differ: %q twice", first)
	}
}
