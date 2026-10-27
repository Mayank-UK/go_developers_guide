package messaging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"guide/internal/domain/demos"
	"guide/internal/domain/messaging"
)

// Compile-time proofs the story runners satisfy the demo ports.
var (
	_ demos.MessagingStreamRunner = (*StreamStoryRunner)(nil)
	_ demos.MessagingOutboxRunner = (*OutboxStoryRunner)(nil)
)

// claimIdleZero is the stories' re-claim threshold: entries idle for at
// least zero milliseconds — every pending entry is claimable now. A
// production consumer uses seconds or minutes (a live worker mid-flight
// is NOT stalled); the demo uses zero because its "crash" is staged and
// the recovery must happen inside one request.
const claimIdleZero = 0

// readBatch bounds one story read: every story appends three entries, so
// ten is comfortably the whole story in one command.
const readBatch = 10

// StreamStoryRunner choreographs the msg:stream demo: the consumer-group
// lifecycle walked in teaching order against the live Redis.
type StreamStoryRunner struct {
	store *StreamStore
}

// NewStreamStoryRunner wires the stream story over its client.
func NewStreamStoryRunner(client *goredis.Client) *StreamStoryRunner {
	return &StreamStoryRunner{store: NewStreamStore(client)}
}

// RunStreamStory walks the group lifecycle under namespace and reports
// what the store answered at every beat. The first failure aborts the
// story with the command's context wrapped (%w) for the handler's honest
// 503; the closing teardown runs even on success only (a failed story
// leaves its keys to the handler's error path — the shared store's
// throwaway prefixes are the safety net, as in topic 20).
func (r *StreamStoryRunner) RunStreamStory(ctx context.Context, namespace string) (demos.MessagingStreamRun, error) {
	stream := namespace + ":orders"
	group := demos.MessagingDemoGroup

	if err := r.store.EnsureGroup(ctx, stream, group, "0-0"); err != nil {
		return demos.MessagingStreamRun{}, err
	}

	appended := make([]string, 0, len(demos.MessagingStreamSteps))
	for _, step := range demos.MessagingStreamSteps {
		id, err := r.store.Append(ctx, stream, map[string]string{
			"order": "o-1001",
			"step":  step,
		})
		if err != nil {
			return demos.MessagingStreamRun{}, err
		}
		appended = append(appended, id)
	}

	consumerA, err := r.store.ReadNew(ctx, stream, group, "consumer-a", readBatch)
	if err != nil {
		return demos.MessagingStreamRun{}, err
	}
	if len(consumerA) != len(appended) {
		return demos.MessagingStreamRun{}, fmt.Errorf("consumer-a read %d entries, want all %d just appended", len(consumerA), len(appended))
	}
	readA := make([]demos.MessagingEntryRow, 0, len(consumerA))
	for _, entry := range consumerA {
		readA = append(readA, demos.MessagingEntryRow{ID: entry.ID, Step: entry.Values["step"]})
	}

	// The queue beat: consumer-b asks for NEW entries and the group has
	// none for it — every entry is already owned by consumer-a.
	consumerB, err := r.store.ReadNew(ctx, stream, group, "consumer-b", readBatch)
	if err != nil {
		return demos.MessagingStreamRun{}, err
	}
	readB := make([]demos.MessagingEntryRow, 0, len(consumerB))
	for _, entry := range consumerB {
		readB = append(readB, demos.MessagingEntryRow{ID: entry.ID, Step: entry.Values["step"]})
	}

	// The crash beat: consumer-a finishes the first and last entries,
	// then dies with the middle one delivered but unacknowledged.
	var ackedByA []string
	for i, entry := range consumerA {
		if i == 1 {
			continue // the stalled one
		}
		if _, err := r.store.Ack(ctx, stream, group, entry.ID); err != nil {
			return demos.MessagingStreamRun{}, err
		}
		ackedByA = append(ackedByA, entry.ID)
	}
	stalled := consumerA[1].ID

	pendingBefore, err := r.store.PendingEntries(ctx, stream, group)
	if err != nil {
		return demos.MessagingStreamRun{}, err
	}
	pendingRows := make([]demos.MessagingPendingRow, 0, len(pendingBefore))
	for _, row := range pendingBefore {
		pendingRows = append(pendingRows, demos.MessagingPendingRow{
			ID:         row.ID,
			Consumer:   row.Consumer,
			Deliveries: row.Deliveries,
		})
	}

	// The recovery beat: consumer-b re-claims the stalled entry — the
	// whole crash-recovery loop in one command.
	claimed, err := r.store.ClaimIdle(ctx, stream, group, "consumer-b", claimIdleZero, readBatch)
	if err != nil {
		return demos.MessagingStreamRun{}, err
	}
	claimedRows := make([]demos.MessagingEntryRow, 0, len(claimed))
	for _, entry := range claimed {
		claimedRows = append(claimedRows, demos.MessagingEntryRow{ID: entry.ID, Step: entry.Values["step"]})
		if _, err := r.store.Ack(ctx, stream, group, entry.ID); err != nil {
			return demos.MessagingStreamRun{}, err
		}
	}

	pendingAfter, err := r.store.PendingEntries(ctx, stream, group)
	if err != nil {
		return demos.MessagingStreamRun{}, err
	}

	if err := r.store.Destroy(ctx, stream, group); err != nil {
		return demos.MessagingStreamRun{}, err
	}

	return demos.MessagingStreamRun{
		Namespace:         namespace,
		Stream:            stream,
		Group:             group,
		AppendedIDs:       appended,
		ConsumerARead:     readA,
		ConsumerBRead:     readB,
		AckedByA:          ackedByA,
		StalledID:         stalled,
		PendingBefore:     pendingRows,
		ClaimedByB:        claimedRows,
		PendingAfterCount: int64(len(pendingAfter)),
		GroupDestroyed:    true,
		StreamDeleted:     true,
	}, nil
}

// OutboxStoryRunner choreographs the msg:outbox demo: the transactional
// pattern walked across BOTH live stores — PostgreSQL owns the
// all-or-nothing commit, Redis owns the fan-out.
type OutboxStoryRunner struct {
	outbox *Outbox
	store  *StreamStore
	client *goredis.Client
}

// NewOutboxStoryRunner wires the outbox story over the shared pool and
// the stream client (whose pool also carries the dedup marks).
func NewOutboxStoryRunner(pool *pgxpool.Pool, client *goredis.Client) *OutboxStoryRunner {
	store := NewStreamStore(client)
	return &OutboxStoryRunner{outbox: NewOutbox(pool, store), store: store, client: client}
}

// outboxScript is the story's fixed script: two committed events and one
// rolled-back pair, so every run teaches with the same words.
var outboxScript = []struct {
	kind    string
	payload string
}{
	{kind: "order.created", payload: "order o-1001 created"},
	{kind: "order.paid", payload: "order o-1001 paid"},
	{kind: "order.cancelled", payload: "order o-1001 cancelled"}, // the rolled-back pair
}

// RunOutboxStory walks the outbox pattern under namespace: record pairs
// (one committed each, one rolled back on purpose), relay committed rows
// onto the stream twice (the second pass must drain nothing), consume
// with dedup on the message id, force one redelivery of an
// already-processed message and watch the dedup seam refuse it, then
// clean every row and key the story created.
func (r *OutboxStoryRunner) RunOutboxStory(ctx context.Context, namespace string) (demos.MessagingOutboxRun, error) {
	stream := namespace + ":outbox"
	group := demos.MessagingDemoGroup

	if err := r.outbox.EnsureSchema(ctx); err != nil {
		return demos.MessagingOutboxRun{}, err
	}

	recorded := make([]demos.MessagingRecordedEventRow, 0, len(outboxScript)-1)
	var messageIDs []string
	for i, beat := range outboxScript {
		id, err := randomMessageID()
		if err != nil {
			return demos.MessagingOutboxRun{}, err
		}
		messageIDs = append(messageIDs, id)
		event := messagingEvent(namespace, id, beat.kind, beat.payload)
		outboxRow := messagingOutboxRow(namespace, id, stream, beat.payload)
		if i < len(outboxScript)-1 {
			if err := r.outbox.RecordEventWithOutbox(ctx, event, outboxRow); err != nil {
				return demos.MessagingOutboxRun{}, err
			}
			recorded = append(recorded, demos.MessagingRecordedEventRow{
				ID: id, Kind: beat.kind, Payload: beat.payload, Committed: true,
			})
		} else if err := r.outbox.RecordEventWithOutboxThenRollback(ctx, event, outboxRow); err != nil {
			return demos.MessagingOutboxRun{}, err
		}
	}
	rolledBackID := messageIDs[len(messageIDs)-1]

	rolledBackCount, err := r.outbox.CountEvent(ctx, namespace, rolledBackID)
	if err != nil {
		return demos.MessagingOutboxRun{}, err
	}

	firstPass, err := r.outbox.RelayPending(ctx, namespace, readBatch)
	if err != nil {
		return demos.MessagingOutboxRun{}, err
	}
	secondPass, err := r.outbox.RelayPending(ctx, namespace, readBatch)
	if err != nil {
		return demos.MessagingOutboxRun{}, err
	}

	if err := r.store.EnsureGroup(ctx, stream, group, "0-0"); err != nil {
		return demos.MessagingOutboxRun{}, err
	}
	entries, err := r.store.ReadNew(ctx, stream, group, "consumer-a", readBatch)
	if err != nil {
		return demos.MessagingOutboxRun{}, err
	}
	if len(entries) != len(firstPass.Published) {
		return demos.MessagingOutboxRun{}, fmt.Errorf("consumer-a read %d entries, want the %d the relay published", len(entries), len(firstPass.Published))
	}

	// The consume beat: process each message exactly once. The FIRST
	// entry is processed but its ack is "lost" (the crash the next beat
	// recovers from) — at-least-once in miniature.
	var consumed []demos.MessagingDeliveryRow
	for i, entry := range entries {
		messageID := entry.Values["messageId"]
		first, err := r.store.ProcessOnce(ctx, messageID)
		if err != nil {
			return demos.MessagingOutboxRun{}, err
		}
		if !first {
			return demos.MessagingOutboxRun{}, fmt.Errorf("message %s was already marked processed before the consumer ran", messageID)
		}
		consumed = append(consumed, demos.MessagingDeliveryRow{
			EntryID:   entry.ID,
			MessageID: messageID,
			Payload:   entry.Values["payload"],
			FirstTime: true,
		})
		if i > 0 {
			if _, err := r.store.Ack(ctx, stream, group, entry.ID); err != nil {
				return demos.MessagingOutboxRun{}, err
			}
		}
	}

	// The redelivery beat: consumer-b re-claims the stalled first entry
	// and processes it AGAIN — the dedup seam must refuse the duplicate.
	claimed, err := r.store.ClaimIdle(ctx, stream, group, "consumer-b", claimIdleZero, readBatch)
	if err != nil {
		return demos.MessagingOutboxRun{}, err
	}
	var redelivered []demos.MessagingDeliveryRow
	duplicates := 0
	for _, entry := range claimed {
		messageID := entry.Values["messageId"]
		first, err := r.store.ProcessOnce(ctx, messageID)
		if err != nil {
			return demos.MessagingOutboxRun{}, err
		}
		if !first {
			duplicates++
		}
		redelivered = append(redelivered, demos.MessagingDeliveryRow{
			EntryID:   entry.ID,
			MessageID: messageID,
			Payload:   entry.Values["payload"],
			FirstTime: first,
		})
		if _, err := r.store.Ack(ctx, stream, group, entry.ID); err != nil {
			return demos.MessagingOutboxRun{}, err
		}
	}

	eventsDeleted, outboxDeleted, err := r.outbox.Cleanup(ctx, namespace)
	if err != nil {
		return demos.MessagingOutboxRun{}, err
	}
	dedupKeys := make([]string, 0, len(messageIDs))
	for _, id := range messageIDs {
		dedupKeys = append(dedupKeys, dedupKey(id))
	}
	if err := r.client.Del(ctx, dedupKeys...).Err(); err != nil {
		return demos.MessagingOutboxRun{}, fmt.Errorf("DEL dedup keys: %w", err)
	}
	if err := r.store.Destroy(ctx, stream, group); err != nil {
		return demos.MessagingOutboxRun{}, err
	}

	published := make([]demos.MessagingPublishedRow, 0, len(firstPass.Published))
	for _, row := range firstPass.Published {
		published = append(published, demos.MessagingPublishedRow{
			OutboxID:    row.OutboxID,
			StreamEntry: row.StreamEntry,
		})
	}

	return demos.MessagingOutboxRun{
		Namespace:         namespace,
		Stream:            stream,
		Recorded:          recorded,
		RolledBackEventID: rolledBackID,
		RolledBackPresent: rolledBackCount > 0,
		Relay:             demos.MessagingRelayRow{Published: published},
		RelayAgainCount:   len(secondPass.Published),
		Consumed:          consumed,
		Redelivered:       redelivered,
		DuplicatesRefused: duplicates,
		EventsDeleted:     eventsDeleted,
		OutboxDeleted:     outboxDeleted,
		GroupDestroyed:    true,
		StreamDeleted:     true,
	}, nil
}

// randomMessageID mints a story message id: 16 hex characters from
// crypto/rand. App-minted (not store-assigned) on purpose — the id is
// the dedup key, so it must be identical across every redelivery.
func randomMessageID() (string, error) {
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("read message id entropy: %w", err)
	}
	return "m-" + hex.EncodeToString(entropy[:]), nil
}

// messagingEvent builds the story's business row.
func messagingEvent(runID, id, kind, payload string) messaging.EventRecord {
	return messaging.EventRecord{ID: id, RunID: runID, Kind: kind, Payload: payload}
}

// messagingOutboxRow builds the story's outbox row.
func messagingOutboxRow(runID, id, stream, payload string) messaging.OutboxMessage {
	return messaging.OutboxMessage{ID: id, RunID: runID, Stream: stream, Payload: payload}
}
