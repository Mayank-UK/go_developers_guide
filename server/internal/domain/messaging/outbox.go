package messaging

import (
	"context"
)

// The transactional outbox: the pattern that makes "commit the database
// change AND publish the event" one all-or-nothing unit. The write side
// records the business row and its outbox row in the SAME PostgreSQL
// transaction (the adapter owns the tx — a domain port must not speak
// pgx); the read side relays committed outbox rows onto the stream and
// marks them published. A relay crash between publish and mark publishes
// again on restart — duplicate delivery by design, which is exactly why
// consumers dedup on the message id (Deduplicator) rather than trusting
// the pipeline to be exactly-once.

// EventRecord is the demo's business row: the state change the
// application commits. In production this is an order, a post, a payment;
// here it is a named event, but the transactional shape is identical —
// the event row and its outbox row commit together or not at all.
type EventRecord struct {
	ID      string
	RunID   string
	Kind    string
	Payload string
}

// OutboxMessage is one outbox row as the writer hands it over: WHAT to
// publish (payload) and WHERE (stream). The ID is app-minted (a UUID)
// rather than store-assigned: consumers dedup on it, so it must be
// identical across every redelivery of the same logical message — a
// store-assigned per-append id would make every duplicate look unique
// and defeat dedup. The created-at/published-at stamps stay inside the
// adapter; the write side never reads them.
type OutboxMessage struct {
	ID      string
	RunID   string
	Stream  string
	Payload string
}

// RelayOutcome is one row the relay moved from the outbox onto its
// stream: the outbox id (the message id consumers dedup on), the stream
// it rode, and the stream entry id the append answered.
type RelayOutcome struct {
	OutboxID    string
	Stream      string
	StreamEntry string
}

// RelayReport is what one relay pass did: the outbox rows it drained and
// published this pass. A second pass draining zero rows is the relay's
// own idempotency proof — published rows never ride twice from the same
// committed state.
type RelayReport struct {
	Published []RelayOutcome
}

// OutboxRecorder is the write side of the outbox: record the business
// event AND its outbox row in one PostgreSQL transaction. If either
// write fails, BOTH roll back — there is no committed state change whose
// event never became publishable, and no event for a change that never
// happened.
type OutboxRecorder interface {
	// RecordEventWithOutbox commits event and message together, or
	// neither.
	RecordEventWithOutbox(ctx context.Context, event EventRecord, message OutboxMessage) error
}

// OutboxRelay is the read side of the outbox: drain committed, not-yet-
// published rows, publish each to its stream, mark it published. The
// drain is bounded (limit), scoped to one run's rows (the demo's
// isolation seam — a production relay drains globally), and
// lock-skipping, so concurrent relay instances never fight over rows.
type OutboxRelay interface {
	// RelayPending publishes at most limit pending outbox rows for
	// runID and reports what it moved.
	RelayPending(ctx context.Context, runID string, limit int) (RelayReport, error)
}
