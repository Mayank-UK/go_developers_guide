// Package messaging is the guide's messaging data layer (topic 22): the
// go-redis v9 adapter behind the domain's stream ports (XADD produce,
// XREADGROUP consume, XPENDING inspect, XAUTOCLAIM re-claim), the
// pgx-backed transactional outbox (business row + outbox row in ONE
// transaction, then a relay that publishes committed rows onto the
// stream), and the demo story runners that choreograph both patterns
// against the RUNNING stack stores. An unreachable Redis or PostgreSQL
// surfaces as an ordinary wrapped error the handler maps to the typed
// service_unavailable envelope — the optional-service discipline, never a
// crash.
package messaging

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"guide/internal/domain/messaging"
)

// Compile-time proofs the adapter satisfies the domain ports.
var (
	_ messaging.Producer     = (*StreamStore)(nil)
	_ messaging.GroupStore   = (*StreamStore)(nil)
	_ messaging.Deduplicator = (*StreamStore)(nil)
)

// busyGroup is the sentinel XGROUP CREATE answers when the group already
// exists — for EnsureGroup that is success (a restarting worker must not
// treat "already created" as failure), matched by string because
// go-redis does not type this reply.
const busyGroup = "BUSYGROUP"

// StreamStore is the Redis Streams adapter over the process's one client
// (wired from GDG_REDIS_ADDR at the composition root, same lazy
// construction and short timeouts as topic 20's playground). Every
// method is one command translated to the domain's vocabulary; every
// failure wraps its command with %w for the handler's 503 mapping.
type StreamStore struct {
	client *goredis.Client
}

// NewStreamStore wires the store over its client.
func NewStreamStore(client *goredis.Client) *StreamStore {
	return &StreamStore{client: client}
}

// Append implements XADD: one entry onto the stream; the store assigns
// and answers the id. NoMkStream is deliberately NOT set — appending to a
// fresh stream creates it, which is the ergonomics producers want (and
// EnsureGroup's MKSTREAM twin for groups).
func (s *StreamStore) Append(ctx context.Context, stream string, values map[string]string) (string, error) {
	id, err := s.client.XAdd(ctx, &goredis.XAddArgs{
		Stream: stream,
		Values: stringMapToAny(values),
	}).Result()
	if err != nil {
		return "", fmt.Errorf("XADD %s: %w", stream, err)
	}
	return id, nil
}

// EnsureGroup implements XGROUP CREATE with MKSTREAM: the group exists
// after the call whether or not the stream does, and start decides what
// "new" means for the group ("0-0" replays history, "$" starts at the
// tail). BUSYGROUP is success in disguise — group creation is
// idempotent, a posture a restarting worker depends on.
func (s *StreamStore) EnsureGroup(ctx context.Context, stream, group, start string) error {
	if err := s.client.XGroupCreateMkStream(ctx, stream, group, start).Err(); err != nil {
		if strings.Contains(err.Error(), busyGroup) {
			return nil
		}
		return fmt.Errorf("XGROUP CREATE %s %s: %w", stream, group, err)
	}
	return nil
}

// ReadNew implements XREADGROUP with ">": up to count entries never
// before delivered in this group, delivered to (and owned by) consumer.
// The read is NON-BLOCKING (Block: -1 — go-redis omits the BLOCK option
// for negative values; the zero value would send BLOCK 0, which blocks
// FOREVER — the trap this adapter's own tests caught). An empty answer
// is a healthy empty slice — a worker polling an idle stream must see
// "nothing to do", never an error and never a nil.
func (s *StreamStore) ReadNew(ctx context.Context, stream, group, consumer string, count int) ([]messaging.Entry, error) {
	result, err := s.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    int64(count),
		Block:    -1,
		NoAck:    false,
	}).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return []messaging.Entry{}, nil
		}
		return nil, fmt.Errorf("XREADGROUP %s %s %s: %w", stream, group, consumer, err)
	}
	entries := make([]messaging.Entry, 0, len(result))
	for _, batch := range result {
		for _, message := range batch.Messages {
			entries = append(entries, messaging.Entry{
				ID:     message.ID,
				Values: anyMapToString(message.Values),
			})
		}
	}
	return entries, nil
}

// Ack implements XACK: finished work leaves the group's pending ledger.
// The answer is how many of the ids were actually pending — an ack of an
// unknown id answers 0, silently and harmlessly (acknowledging twice is
// safe by the same at-least-once logic that makes everything else here
// safe to repeat).
func (s *StreamStore) Ack(ctx context.Context, stream, group string, ids ...string) (int64, error) {
	acknowledged, err := s.client.XAck(ctx, stream, group, ids...).Result()
	if err != nil {
		return 0, fmt.Errorf("XACK %s %s: %w", stream, group, err)
	}
	return acknowledged, nil
}

// PendingEntries implements XPENDING in its EXTENDED form: the group's
// delivered-not-acknowledged ledger entry by entry — id, owner, idle
// time, delivery count — oldest first, which is also claim priority
// order.
func (s *StreamStore) PendingEntries(ctx context.Context, stream, group string) ([]messaging.PendingEntry, error) {
	rows, err := s.client.XPendingExt(ctx, &goredis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  "-",
		End:    "+",
		Count:  100,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("XPENDING %s %s: %w", stream, group, err)
	}
	pending := make([]messaging.PendingEntry, 0, len(rows))
	for _, row := range rows {
		pending = append(pending, messaging.PendingEntry{
			ID:         row.ID,
			Consumer:   row.Consumer,
			Idle:       row.Idle,
			Deliveries: row.RetryCount,
		})
	}
	return pending, nil
}

// ClaimIdle implements XAUTOCLAIM: pending entries idle for at least
// minIdle are reassigned to consumer and handed back with their values —
// the crash-recovery loop every consumer group needs (a worker died with
// entries delivered-but-unacknowledged; its stalled work is claimed and
// finished by a live worker instead of waiting forever).
func (s *StreamStore) ClaimIdle(ctx context.Context, stream, group, consumer string, minIdle time.Duration, count int) ([]messaging.Entry, error) {
	claimed, _, err := s.client.XAutoClaim(ctx, &goredis.XAutoClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumer,
		MinIdle:  minIdle,
		Start:    "0-0",
		Count:    int64(count),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("XAUTOCLAIM %s %s %s: %w", stream, group, consumer, err)
	}
	entries := make([]messaging.Entry, 0, len(claimed))
	for _, message := range claimed {
		entries = append(entries, messaging.Entry{
			ID:     message.ID,
			Values: anyMapToString(message.Values),
		})
	}
	return entries, nil
}

// Destroy implements XGROUP DESTROY plus DEL of the stream key — demo
// teardown so a throwaway namespace leaves the shared store exactly as it
// found it. Production groups are long-lived state; this method exists
// because the demos clean up after themselves.
func (s *StreamStore) Destroy(ctx context.Context, stream, group string) error {
	if err := s.client.XGroupDestroy(ctx, stream, group).Err(); err != nil {
		return fmt.Errorf("XGROUP DESTROY %s %s: %w", stream, group, err)
	}
	if err := s.client.Del(ctx, stream).Err(); err != nil {
		return fmt.Errorf("DEL %s: %w", stream, err)
	}
	return nil
}

// ProcessOnce implements the deduplication seam with SET NX: the key is
// created only if absent, so the ONE atomic command answers "first time?"
// for both deliveries of a redelivered message — a two-command
// check-then-set would race two concurrent deliveries into both
// answering "first".
func (s *StreamStore) ProcessOnce(ctx context.Context, id string) (bool, error) {
	first, err := s.client.SetNX(ctx, dedupKey(id), "processed", 0).Result()
	if err != nil {
		return false, fmt.Errorf("SET NX %s: %w", dedupKey(id), err)
	}
	return first, nil
}

// dedupKey namespaces dedup marks under the demo's prefix so they SCAN
// away with the rest of a run's keys.
func dedupKey(id string) string { return "gdg:demo:msg:dedup:" + id }

// stringMapToAny widens map[string]string to the map[string]any go-redis
// accepts — the only translation the domain's string vocabulary needs.
func stringMapToAny(values map[string]string) map[string]any {
	widened := make(map[string]any, len(values))
	for key, value := range values {
		widened[key] = value
	}
	return widened
}

// anyMapToString narrows a stream reply's map[string]any back to
// strings. Stream fields written as strings come back as strings; a
// non-string field would mean a producer outside this guide's vocabulary
// wrote to the stream, rendered here with %v rather than asserted into a
// panic.
func anyMapToString(values map[string]any) map[string]string {
	narrowed := make(map[string]string, len(values))
	for key, value := range values {
		narrowed[key] = fmt.Sprintf("%v", value)
	}
	return narrowed
}
