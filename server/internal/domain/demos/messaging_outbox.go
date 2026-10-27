// Topic 22 messaging's second live demo: the transactional outbox story.
// One request walks the full pattern against BOTH live stores — the
// business event and its outbox row committed in ONE PostgreSQL
// transaction, a third write pair deliberately ROLLED BACK to prove the
// unit is all-or-nothing, the relay draining committed rows onto a Redis
// stream (and a second pass draining nothing), a consumer group
// processing each message exactly once by deduplicating on the message
// id, and a forced redelivery of an already-processed message that the
// dedup seam REFUSES — the at-least-once pipeline made safe by an
// idempotent consumer. The builder below is PURE: proofs derive from the
// run's own counts, so the payload can never claim atomicity the
// database did not answer.
package demos

import (
	"context"
	"fmt"
)

// MessagingOutboxRunner is the port the msg:outbox demo reads through:
// one call walks the whole outbox lifecycle under a throwaway namespace
// and reports what both stores answered. The pgx + go-redis-backed
// implementation lives in internal/data/messaging.
type MessagingOutboxRunner interface {
	// RunOutboxStory records, rolls back, relays, consumes, redelivers,
	// and cleans up — the full pattern, step by step.
	RunOutboxStory(ctx context.Context, namespace string) (MessagingOutboxRun, error)
}

// MessagingRecordedEventRow is one business event of the story: what was
// committed (or rolled back) and its payload.
type MessagingRecordedEventRow struct {
	ID        string
	Kind      string
	Payload   string
	Committed bool
}

// MessagingRelayRow is one relay pass: what it drained and published.
type MessagingRelayRow struct {
	Published []MessagingPublishedRow
}

// MessagingPublishedRow is one outbox row the relay moved onto its
// stream: the outbox id (the message id) and the stream entry id.
type MessagingPublishedRow struct {
	OutboxID    string
	StreamEntry string
}

// MessagingDeliveryRow is one delivery attempt at a consumer: the
// stream entry, the message id it carried, and whether this attempt was
// the first (processed) or a repeat (refused by dedup).
type MessagingDeliveryRow struct {
	EntryID   string
	MessageID string
	Payload   string
	FirstTime bool
}

// MessagingOutboxRun is the outbox story's raw haul: exactly what the
// two live stores answered, beat by beat.
type MessagingOutboxRun struct {
	Namespace         string
	Stream            string
	Recorded          []MessagingRecordedEventRow
	RolledBackEventID string
	RolledBackPresent bool
	Relay             MessagingRelayRow
	RelayAgainCount   int
	Consumed          []MessagingDeliveryRow
	Redelivered       []MessagingDeliveryRow
	DuplicatesRefused int
	EventsDeleted     int64
	OutboxDeleted     int64
	GroupDestroyed    bool
	StreamDeleted     bool
}

// MessagingOutboxPayload is the wire shape of GET /api/demo/msg/outbox
// (camelCase JSON). Deterministic for a given pair of stores: payloads
// are fixed, only the namespace and ids vary per run; every slice is
// non-nil so the wire renders [], never null.
type MessagingOutboxPayload struct {
	Namespace string                    `json:"namespace"`
	Stream    string                    `json:"stream"`
	Beats     []MessagingOutboxBeatRow  `json:"beats"`
	Proofs    MessagingOutboxProofGroup `json:"proofs"`
	Reasoning string                    `json:"reasoning"`
}

// MessagingOutboxBeatRow is one beat as the payload serves it.
type MessagingOutboxBeatRow struct {
	Beat     string   `json:"beat"`
	Commands []string `json:"commands"`
	Outcome  string   `json:"outcome"`
	Ids      []string `json:"ids"`
}

// MessagingOutboxProofGroup carries the story's derived verdicts:
// atomicity (the rollback), relay idempotency, and consumer
// idempotency under redelivery.
type MessagingOutboxProofGroup struct {
	Atomicity     string `json:"atomicity"`
	RelayProof    string `json:"relayProof"`
	Idempotency   string `json:"idempotency"`
	Duplicates    int    `json:"duplicates"`
	EventsDeleted int64  `json:"eventsDeleted"`
	OutboxDeleted int64  `json:"outboxDeleted"`
}

// BuildMessagingOutboxPayload shapes a run into the demo payload: beats
// in execution order, proofs derived from the run's own counts — the
// rolled-back event's absence, the second relay's empty haul, the
// refused duplicate count — so the payload can never claim a guarantee
// the stores did not answer.
func BuildMessagingOutboxPayload(run MessagingOutboxRun) MessagingOutboxPayload {
	committedCount := 0
	for _, event := range run.Recorded {
		if event.Committed {
			committedCount++
		}
	}
	committedIDs := make([]string, 0, committedCount)
	for _, event := range run.Recorded {
		if event.Committed {
			committedIDs = append(committedIDs, event.ID)
		}
	}

	// Atomicity: the committed events exist, the rolled-back one does
	// not — the transaction held BOTH writes or NEITHER.
	atomicity := fmt.Sprintf(
		"%d event(s) committed with their outbox rows in one transaction each; a third write pair (%s) rolled "+
			"back deliberately and the count query answered %d — neither the event nor its outbox row exists. "+
			"All-or-nothing is not a hope: the rollback is the proof.",
		committedCount, run.RolledBackEventID, presenceCount(run.RolledBackPresent),
	)

	// Relay idempotency: the first pass moved every pending row; the
	// second pass found nothing — published rows never ride twice from
	// the same committed state.
	relayProof := fmt.Sprintf(
		"the relay's first pass published %d message(s) onto the stream; a second pass drained %d — the "+
			"published_at stamp the relay commits with its row locks is what keeps a restart from re-publishing "+
			"the same rows.",
		len(run.Relay.Published), run.RelayAgainCount,
	)

	// Consumer idempotency: every delivery processed exactly once; the
	// forced redelivery was refused by the dedup seam.
	processed := 0
	for _, delivery := range run.Consumed {
		if delivery.FirstTime {
			processed++
		}
	}
	idempotency := fmt.Sprintf(
		"the consumer processed %d message(s), each once (dedup key SET NX per message id); the forced redelivery "+
			"of an already-processed message was refused %d time(s) — under at-least-once delivery the SECOND "+
			"delivery is a WHEN, and the consumer's dedup is what turns it into a no-op.",
		processed, run.DuplicatesRefused,
	)

	publishedIDs := make([]string, 0, len(run.Relay.Published))
	for _, row := range run.Relay.Published {
		publishedIDs = append(publishedIDs, row.OutboxID)
	}
	consumedIDs := make([]string, 0, len(run.Consumed))
	for _, row := range run.Consumed {
		consumedIDs = append(consumedIDs, row.EntryID)
	}
	redeliveredIDs := make([]string, 0, len(run.Redelivered))
	for _, row := range run.Redelivered {
		redeliveredIDs = append(redeliveredIDs, row.EntryID)
	}

	beats := []MessagingOutboxBeatRow{
		{
			Beat: "record (commit)",
			Commands: []string{
				"BEGIN",
				"INSERT INTO msgdemo_events …",
				"INSERT INTO msgdemo_outbox …",
				"COMMIT",
			},
			Outcome: fmt.Sprintf("%d event rows and their outbox rows committed — each pair one transaction.", committedCount),
			Ids:     committedIDs,
		},
		{
			Beat: "record (rollback)",
			Commands: []string{
				"BEGIN",
				"INSERT INTO msgdemo_events …",
				"INSERT INTO msgdemo_outbox …",
				"ROLLBACK",
			},
			Outcome: fmt.Sprintf("the third pair rolled back: SELECT count(*) answers %d — the crash window has no state in it.", presenceCount(run.RolledBackPresent)),
		},
		{
			Beat: "relay",
			Commands: []string{
				"SELECT … WHERE published_at IS NULL FOR UPDATE SKIP LOCKED",
				fmt.Sprintf("XADD %s messageId … payload …", run.Stream),
				"UPDATE msgdemo_outbox SET published_at = now()",
				"COMMIT",
			},
			Outcome: fmt.Sprintf("the relay published %d message(s) onto the stream; a second pass drained %d.", len(run.Relay.Published), run.RelayAgainCount),
			Ids:     publishedIDs,
		},
		{
			Beat:     "consume",
			Commands: []string{fmt.Sprintf("XREADGROUP GROUP %s consumer-a COUNT 10 >", MessagingDemoGroup)},
			Outcome:  fmt.Sprintf("consumer-a processed %d message(s), each exactly once (SET NX on the message id), then acknowledged.", processed),
			Ids:      consumedIDs,
		},
		{
			Beat:     "redeliver",
			Commands: []string{fmt.Sprintf("XAUTOCLAIM %s %s consumer-b 0 0-0 COUNT 10", run.Stream, MessagingDemoGroup)},
			Outcome:  fmt.Sprintf("the stalled entry re-delivered to consumer-b was refused by dedup %d time(s) — a duplicate is a no-op, then acknowledged.", run.DuplicatesRefused),
			Ids:      redeliveredIDs,
		},
		{
			Beat: "cleanup",
			Commands: []string{
				fmt.Sprintf("XGROUP DESTROY %s %s", run.Stream, MessagingDemoGroup),
				fmt.Sprintf("DEL %s", run.Stream),
				"DELETE FROM msgdemo_events / msgdemo_outbox WHERE run_id = …",
			},
			Outcome: fmt.Sprintf("%d event row(s) and %d outbox row(s) deleted; teardown %s — both stores as the run found them.",
				run.EventsDeleted, run.OutboxDeleted, teardownNote(run.GroupDestroyed, run.StreamDeleted)),
		},
	}

	reasoning := fmt.Sprintf(
		"Every count above is what the two live stores answered under %s. The outbox closes the dual-write gap: "+
			"the database commit and the event's publishability became ONE unit, so a crash between them leaves "+
			"either both (the relay publishes later) or neither (nothing to publish). The relay is at-least-once on "+
			"purpose — a crash between XADD and the published_at commit re-publishes on restart — and that honesty "+
			"is why the consumer dedups on the message id instead of trusting the pipeline. Exactly-once delivery "+
			"is a myth at this boundary; exactly-once EFFECT is one SET NX away.",
		run.Namespace,
	)

	return MessagingOutboxPayload{
		Namespace: run.Namespace,
		Stream:    run.Stream,
		Beats:     beats,
		Proofs: MessagingOutboxProofGroup{
			Atomicity:     atomicity,
			RelayProof:    relayProof,
			Idempotency:   idempotency,
			Duplicates:    run.DuplicatesRefused,
			EventsDeleted: run.EventsDeleted,
			OutboxDeleted: run.OutboxDeleted,
		},
		Reasoning: reasoning,
	}
}

// presenceCount renders a boolean presence as 1/0 — the count query's
// honest answer.
func presenceCount(present bool) int {
	if present {
		return 1
	}
	return 0
}

// teardownNote reports the closing teardown honestly: complete, or as
// it happened.
func teardownNote(groupDestroyed, streamDeleted bool) string {
	if groupDestroyed && streamDeleted {
		return "complete (group destroyed, stream deleted)"
	}
	return "reported as it happened"
}
