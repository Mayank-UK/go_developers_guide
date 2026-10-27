// Package messaging is the guide's messaging domain area (topic 22): the
// ports and vocabulary for Redis Streams produce/consume through consumer
// groups, the transactional outbox that makes "write the database AND
// publish the event" all-or-nothing, and the deduplication seam that makes
// consumers safe under at-least-once delivery. Implementations live in
// internal/data/messaging (go-redis v9 + pgx v5); nothing here imports a
// driver — the dependency rule keeps this package the seam, not the store.
package messaging

import (
	"context"
	"time"
)

// Entry is one stream entry as the domain sees it: the store-assigned id
// (milliseconds-sequence, so ids order as insertion order) and the entry's
// field→value pairs. Producers write string values; consumers read them
// back as strings — the adapter owns any wire-type translation.
type Entry struct {
	ID     string
	Values map[string]string
}

// PendingEntry is one entry the group has DELIVERED but nobody has
// acknowledged yet: the id, the consumer that currently owns it, how long
// it has sat unacknowledged, and how many times it has been delivered.
// The pending list is the group's ledger of "delivered, not done" — the
// raw material every crash-recovery story is built from.
type PendingEntry struct {
	ID         string
	Consumer   string
	Idle       time.Duration
	Deliveries int64
}

// Producer appends entries to a stream — XADD underneath. The store
// assigns and returns the entry id; callers never mint one, because the
// id's ordering IS the stream's ordering guarantee.
type Producer interface {
	// Append writes one entry to stream and answers its store-assigned id.
	Append(ctx context.Context, stream string, values map[string]string) (string, error)
}

// GroupStore is the consumer-group port: the full lifecycle a worker
// lives — create the group, read new entries as one consumer, acknowledge
// finished work, inspect the pending ledger, re-claim stalled entries,
// and tear the group down. Reads with ">" see only entries never
// delivered in this group; pending reads and claims are how a group
// recovers from a consumer that died mid-flight.
type GroupStore interface {
	// EnsureGroup creates the group reading from start ("0-0" to include
	// history, "$" for new entries only); an existing group is not an
	// error — the idempotent posture a restarting worker needs.
	EnsureGroup(ctx context.Context, stream, group, start string) error
	// ReadNew delivers up to count never-delivered entries to consumer.
	// A healthy empty read is an empty slice, not an error.
	ReadNew(ctx context.Context, stream, group, consumer string, count int) ([]Entry, error)
	// Ack marks ids done, removing them from the group's pending list.
	Ack(ctx context.Context, stream, group string, ids ...string) (int64, error)
	// PendingEntries reports the group's delivered-not-acknowledged
	// ledger, oldest first.
	PendingEntries(ctx context.Context, stream, group string) ([]PendingEntry, error)
	// ClaimIdle takes up to count pending entries idle for at least
	// minIdle away from their current owner and reassigns them to
	// consumer — XAUTOCLAIM underneath, the crash-recovery primitive.
	ClaimIdle(ctx context.Context, stream, group, consumer string, minIdle time.Duration, count int) ([]Entry, error)
	// Destroy removes the group (and the stream key) — demo teardown, not
	// a production operation.
	Destroy(ctx context.Context, stream, group string) error
}

// Deduplicator is the idempotent-consumer seam: ProcessOnce answers
// whether id was seen for the FIRST time (true → do the work) or is a
// repeat (false → skip it). One atomic check-and-mark — SET NX under one
// implementation — because a check-then-set of two commands races two
// deliveries of the same message.
type Deduplicator interface {
	ProcessOnce(ctx context.Context, id string) (bool, error)
}
