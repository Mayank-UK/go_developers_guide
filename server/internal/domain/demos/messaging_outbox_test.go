package demos

import (
	"strings"
	"testing"
)

// The messaging outbox story's payload builder: PURE derivation tests,
// no stores. The proofs must derive from the run's own counts — the
// rollback's absence, the second relay's empty haul, the refused
// duplicate — and honest wording when the stores answer something else.

func messagingOutboxFixtureRun() MessagingOutboxRun {
	return MessagingOutboxRun{
		Namespace: "gdg:demo:msg:0123456789ab",
		Stream:    "gdg:demo:msg:0123456789ab:outbox",
		Recorded: []MessagingRecordedEventRow{
			{ID: "m-aaaaaaaaaaaaaaaa", Kind: "order.created", Payload: "order o-1001 created", Committed: true},
			{ID: "m-bbbbbbbbbbbbbbbb", Kind: "order.paid", Payload: "order o-1001 paid", Committed: true},
		},
		RolledBackEventID: "m-cccccccccccccccc",
		RolledBackPresent: false,
		Relay: MessagingRelayRow{Published: []MessagingPublishedRow{
			{OutboxID: "m-aaaaaaaaaaaaaaaa", StreamEntry: "1780000000000-0"},
			{OutboxID: "m-bbbbbbbbbbbbbbbb", StreamEntry: "1780000000001-0"},
		}},
		RelayAgainCount: 0,
		Consumed: []MessagingDeliveryRow{
			{EntryID: "1780000000000-0", MessageID: "m-aaaaaaaaaaaaaaaa", Payload: "order o-1001 created", FirstTime: true},
			{EntryID: "1780000000001-0", MessageID: "m-bbbbbbbbbbbbbbbb", Payload: "order o-1001 paid", FirstTime: true},
		},
		Redelivered: []MessagingDeliveryRow{
			{EntryID: "1780000000000-0", MessageID: "m-aaaaaaaaaaaaaaaa", Payload: "order o-1001 created", FirstTime: false},
		},
		DuplicatesRefused: 1,
		EventsDeleted:     2,
		OutboxDeleted:     2,
		GroupDestroyed:    true,
		StreamDeleted:     true,
	}
}

func TestMessagingOutboxPayloadDerivesTheThreeProofs(t *testing.T) {
	t.Parallel()
	payload := BuildMessagingOutboxPayload(messagingOutboxFixtureRun())

	if !strings.Contains(payload.Proofs.Atomicity, "count query answered 0") {
		t.Errorf("atomicity proof must derive from the rollback's absence: %q", payload.Proofs.Atomicity)
	}
	if !strings.Contains(payload.Proofs.RelayProof, "second pass drained 0") {
		t.Errorf("relay proof must derive from the second pass: %q", payload.Proofs.RelayProof)
	}
	if !strings.Contains(payload.Proofs.Idempotency, "refused 1 time(s)") {
		t.Errorf("idempotency proof must derive from the refused duplicates: %q", payload.Proofs.Idempotency)
	}
	if payload.Proofs.Duplicates != 1 || payload.Proofs.EventsDeleted != 2 || payload.Proofs.OutboxDeleted != 2 {
		t.Errorf("proof counts drifted: %+v", payload.Proofs)
	}
}

func TestMessagingOutboxPayloadReportsASurvivedRollbackHonestly(t *testing.T) {
	t.Parallel()
	run := messagingOutboxFixtureRun()
	// The nightmare case: the rolled-back pair SURVIVED — the proof must
	// say 1, not claim atomicity.
	run.RolledBackPresent = true

	payload := BuildMessagingOutboxPayload(run)
	if !strings.Contains(payload.Proofs.Atomicity, "answered 1") {
		t.Errorf("a survived rollback must be reported as it happened: %q", payload.Proofs.Atomicity)
	}
}

func TestMessagingOutboxPayloadBeatsCarryThePattern(t *testing.T) {
	t.Parallel()
	payload := BuildMessagingOutboxPayload(messagingOutboxFixtureRun())

	wantBeats := []string{"record (commit)", "record (rollback)", "relay", "consume", "redeliver", "cleanup"}
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
	relay := strings.Join(payload.Beats[2].Commands, "\n")
	if !strings.Contains(relay, "FOR UPDATE SKIP LOCKED") || !strings.Contains(relay, "published_at = now()") {
		t.Errorf("relay commands must carry the drain's real shape: %v", payload.Beats[2].Commands)
	}
	if !strings.Contains(payload.Reasoning, "Exactly-once delivery") {
		t.Errorf("reasoning must carry the topic's thesis: %q", payload.Reasoning)
	}
}
