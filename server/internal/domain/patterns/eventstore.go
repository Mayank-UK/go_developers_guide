// Package patterns holds the real implementations of the distributed-
// application patterns the advanced-architecture topic teaches. The
// event store below is the machinery behind event sourcing: an
// append-only, per-stream ledger of immutable facts from which state is
// rebuilt by replay, guarded by optimistic concurrency so concurrent
// writers cannot silently overwrite each other's history. It is
// deliberately dependency-free — stdlib only — because the pattern, not
// the storage engine, is the lesson: swap the in-memory map for a
// PostgreSQL table (topic 16's event table shape) and every guarantee
// below survives unchanged.
//
// The three guarantees, and where each lives:
//
//   - Immutability is STRUCTURAL. The only write path is Append; there
//     is no update and no delete method to call. Event payloads are
//     stored as strings, which Go cannot mutate in place, and Stream
//     hands back fresh copies of the records — a caller cannot reach
//     the stored facts to edit them.
//
//   - Order is the VERSION, not the clock. Every event in a stream
//     carries a 1-based version assigned by the store at append time,
//     dense and monotonic (1, 2, 3…). Replay folds events in version
//     order; no event's RecordedAt is ever consulted for ordering.
//
//   - Concurrency is OPTIMISTIC. Append takes the stream version the
//     caller believes in; if the stream moved on, the append fails with
//     a typed *EventStoreConflict and the caller reloads, revalidates
//     against fresh history, and retries. No locks are held between a
//     read and its follow-up write — the version check IS the lock.
package patterns

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Stream version sentinel values for EventStore.Append's expectedVersion.
//
// EventStreamVersionEmpty claims the stream holds nothing yet — the
// idiomatic first append ("create this stream"), so a stream that
// already exists conflicts rather than silently extending. It is the
// zero value on purpose: an empty stream and an absent stream both sit
// at version 0, which is exactly how replay treats them.
//
// EventStreamVersionAny skips the optimistic check entirely — the
// fire-and-forget mode for writers that genuinely do not care (audit
// logs, telemetry). Everything else must equal the stream's current
// version exactly.
const (
	EventStreamVersionEmpty int64 = 0
	EventStreamVersionAny   int64 = -1
)

// EventAppend is one event on its way IN — the only shape a caller may
// construct. It deliberately carries no version (the store assigns
// versions; a caller-chosen version would let history be forged), no
// stream id (the Append call carries it), and a byte payload that the
// store immediately freezes into a string. RecordedAt is supplied by
// the CALLER, never read from a clock inside the store: event time is
// data about the fact, the store stays deterministic, and replays
// reproduce byte-for-byte without mocking time.
type EventAppend struct {
	// Type names what happened, e.g. "inventory.reserved". Non-empty.
	Type string
	// Payload is the fact's body — conventionally JSON. May be empty
	// for marker events that carry no data beyond their type.
	Payload []byte
	// RecordedAt is when the fact occurred, supplied by the caller.
	// Must be non-zero; the store never reads the wall clock.
	RecordedAt time.Time
}

// EventRecord is one stored fact — the immutable unit of history. It is
// what Append returns and what Stream replays: the stream it belongs
// to, its dense 1-based version, the type, the payload as a string
// (immutable by construction — the []byte the caller handed over was
// frozen at append time), and the caller-supplied occurrence time.
// There is no way back from EventRecord to EventAppend that edits
// history: constructing a new append with an old record's payload
// creates a NEW event with a NEW version at the stream's head.
type EventRecord struct {
	StreamID   string
	Version    int64
	Type       string
	Payload    string
	RecordedAt time.Time
}

// EventStoreConflict is the typed optimistic-concurrency failure: the
// stream the caller tried to append to has moved on. ExpectedVersion
// is what the caller believed, ActualVersion is what the store found —
// the gap between them is the history the caller has not seen yet.
// Recover with the standard loop: reload the stream, revalidate the
// command against the fresh state, re-append with the fresh version.
type EventStoreConflict struct {
	StreamID        string
	ExpectedVersion int64
	ActualVersion   int64
}

// Error implements the error interface. The message teaches the
// recovery, not just the failure.
func (e *EventStoreConflict) Error() string {
	return fmt.Sprintf(
		"event store conflict on stream %q: expected version %d, actual %d — the stream gained %d event(s) this writer has not seen; reload, revalidate, retry",
		e.StreamID, e.ExpectedVersion, e.ActualVersion, e.ActualVersion-e.ExpectedVersion)
}

// EventStore is an append-only store of event streams: streams keyed by
// id, each an ordered slice of immutable records with dense 1-based
// versions. The zero value is NOT usable — build it with
// NewEventStore. A sync.RWMutex serializes appends and lets concurrent
// replays proceed in parallel; in a process-local store that mutex is
// correctness for the map, while expectedVersion remains correctness
// for the INTENT — two writers may hold the mutex in turn and still
// conflict, because the second one's view of history is stale.
type EventStore struct {
	mu      sync.RWMutex
	streams map[string][]EventRecord
}

// NewEventStore builds an empty event store.
func NewEventStore() *EventStore {
	return &EventStore{streams: make(map[string][]EventRecord)}
}

// Append validates the batch, enforces optimistic concurrency, and —
// only if everything holds — appends every event in one atomic step,
// assigning consecutive versions from the stream's head. The whole
// check-and-append runs under the write lock: a batch is one unit, so
// either every event lands with its version or none does; a caller
// never observes a half-appended batch. It returns the stored records
// (fresh copies) in append order.
//
// Failure modes, in the order they are checked:
//
//   - ctx already cancelled → the ctx error, before anything is touched
//   - malformed input (blank stream id, empty batch, blank event type,
//     zero RecordedAt, expectedVersion < EventStreamVersionAny) → a
//     descriptive error, nothing appended
//   - expectedVersion != the stream's current version (unless it is
//     EventStreamVersionAny) → *EventStoreConflict, nothing appended
func (s *EventStore) Append(ctx context.Context, streamID string, expectedVersion int64, events ...EventAppend) ([]EventRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("event store: append to %q abandoned: %w", streamID, err)
	}
	if strings.TrimSpace(streamID) == "" {
		return nil, fmt.Errorf("event store: append needs a non-blank stream id")
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("event store: append to %q carried no events — nothing to record", streamID)
	}
	if expectedVersion < EventStreamVersionAny {
		return nil, fmt.Errorf("event store: expected version %d is neither a stream version (>= %d) nor EventStreamVersionAny (%d)",
			expectedVersion, EventStreamVersionEmpty, EventStreamVersionAny)
	}
	for i, event := range events {
		if event.Type == "" {
			return nil, fmt.Errorf("event store: event %d of the batch to %q has no type — a fact must name what happened", i, streamID)
		}
		if event.RecordedAt.IsZero() {
			return nil, fmt.Errorf("event store: event %d (%s) of the batch to %q has no RecordedAt — event time is caller-owned data, never a clock the store reads",
				i, event.Type, streamID)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	current := int64(len(s.streams[streamID]))
	if expectedVersion != EventStreamVersionAny && expectedVersion != current {
		return nil, &EventStoreConflict{
			StreamID:        streamID,
			ExpectedVersion: expectedVersion,
			ActualVersion:   current,
		}
	}

	stored := make([]EventRecord, 0, len(events))
	for i, event := range events {
		stored = append(stored, EventRecord{
			StreamID:   streamID,
			Version:    current + int64(i) + 1,
			Type:       event.Type,
			Payload:    string(event.Payload), // frozen: the []byte can no longer be edited through any alias
			RecordedAt: event.RecordedAt,
		})
	}
	s.streams[streamID] = append(s.streams[streamID], stored...)
	return stored, nil
}

// Stream answers the stream's records in version order, as fresh
// copies. A stream that never received an event answers an EMPTY slice,
// not an error and not nil: in event sourcing there is no "does not
// exist" state, only "no events yet" — an absent stream and an empty
// stream fold to the same initial state, so the store answers both the
// same way. The copies matter: mutating the returned slice or its
// records cannot reach the stored history.
func (s *EventStore) Stream(ctx context.Context, streamID string) ([]EventRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("event store: read of %q abandoned: %w", streamID, err)
	}
	if strings.TrimSpace(streamID) == "" {
		return nil, fmt.Errorf("event store: stream read needs a non-blank stream id")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	source := s.streams[streamID]
	if len(source) == 0 {
		return []EventRecord{}, nil
	}
	copies := make([]EventRecord, len(source))
	copy(copies, source)
	return copies, nil
}

// Version answers the stream's current version — the number of events
// it holds, 0 for an empty or absent stream. It is the value to hand
// back as expectedVersion on the next append, which is why Project
// returns it beside the rebuilt state.
func (s *EventStore) Version(ctx context.Context, streamID string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("event store: version read of %q abandoned: %w", streamID, err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return int64(len(s.streams[streamID])), nil
}

// EventProjection is the answer of a replay: the state the fold built,
// and the stream version that state provably reflects. The pair is the
// heart of event-sourced concurrency — Version is exactly the
// expectedVersion the next command may claim, so "rebuild, decide,
// append" closes into one safe loop without ever storing mutable state.
type EventProjection[S any] struct {
	State   S
	Version int64
}

// EventProject rebuilds a stream's current state by replay: it reads
// the stream, folds every event in version order over initial with
// apply, and returns the state beside the version it was built from.
// An empty or absent stream returns initial untouched at version 0 —
// the zero-event fold. This is the event-sourcing rebuild; the store
// does the reading, apply does the domain, and neither knows the
// other's business.
//
// It is a free FUNCTION, not a method, on purpose: Go methods cannot
// introduce their own type parameters, so a generic helper over an
// arbitrary state shape lives at package scope — the same place the
// standard library puts its generic helpers (slices.SortFunc,
// maps.Clone). The store keeps its role — reading — and the fold keeps
// its purity.
func EventProject[S any](ctx context.Context, store *EventStore, streamID string, initial S, apply func(S, EventRecord) S) (EventProjection[S], error) {
	events, err := store.Stream(ctx, streamID)
	if err != nil {
		var zero EventProjection[S]
		return zero, fmt.Errorf("event store: projection of %q: %w", streamID, err)
	}
	if apply == nil {
		var zero EventProjection[S]
		return zero, fmt.Errorf("event store: projection of %q needs an apply fold", streamID)
	}
	state := EventFold(events, initial, apply)
	return EventProjection[S]{State: state, Version: int64(len(events))}, nil
}

// EventFold is replay in its purest form: fold events, in order, over
// initial with apply. It is a free function on purpose — replay is not
// a store feature, it is a property of the EVENTS; the same fold over
// the same events must always build the same state, which is what makes
// rebuilds trustworthy and projections disposable. Determinism is the
// caller's contract: apply must be a pure function of (state, event),
// or the whole discipline collapses.
func EventFold[S any](events []EventRecord, initial S, apply func(S, EventRecord) S) S {
	state := initial
	for _, event := range events {
		state = apply(state, event)
	}
	return state
}
